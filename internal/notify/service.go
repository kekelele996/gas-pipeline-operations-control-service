package notify

// Notify service: enqueue messages, simulate batch delivery against a
// configured failure rate, and retry failed messages with exponential
// backoff. The SCADA alarm sink and incident service can enqueue alerts here.

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
	"gas-pipeline-operations-control-service/internal/scada"
)

// Clock is the time source.
type Clock = platform.Clock

// Config tunes the notify service.
type Config struct {
	FailureRate float64       // simulated delivery failure rate in [0,1]
	MaxAttempts int           // max delivery attempts before failing terminally
	Backoff     time.Duration // base backoff between retries
}

// Service implements notification delivery.
type Service struct {
	store *Store
	clock platform.Clock
	cfg   Config
	rng   *counterPRNG // deterministic-ish PRNG for failure simulation
}

// NewService builds a notify service.
func NewService(store *Store, clock platform.Clock, cfg Config) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 30 * time.Second
	}
	if cfg.FailureRate < 0 {
		cfg.FailureRate = 0
	}
	if cfg.FailureRate > 1 {
		cfg.FailureRate = 1
	}
	return &Service{store: store, clock: clock, cfg: cfg, rng: newCounterPRNG()}
}

// Store exposes the underlying store (read-only consumers).
func (s *Service) Store() *Store { return s.store }

// Enqueue adds a notification to the queue.
func (s *Service) Enqueue(ctx context.Context, recipient string, ch Channel, subject, body string) (Notification, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("recipient", recipient)
	v.RequireNonEmpty("subject", subject)
	validCh := false
	for _, c := range AllChannels() {
		if c == ch {
			validCh = true
			break
		}
	}
	v.Require(validCh, "channel is invalid")
	if err := v.Error(); err != nil {
		return Notification{}, err
	}
	n := Notification{
		ID: platform.NewNotificationID(), Recipient: recipient, Channel: ch,
		Subject: subject, Body: body, State: StateQueued,
		MaxAttempts: s.cfg.MaxAttempts, Attempt: 0,
	}
	s.store.Put(n)
	out, _ := s.store.Get(n.ID)
	return out, nil
}

// PushBatch attempts delivery of all due notifications concurrently. Each
// delivery succeeds or fails (simulated against the configured failure rate);
// failed messages that have remaining attempts move to retrying, otherwise to
// failed terminal. Returns the number sent and the number that failed.
func (s *Service) PushBatch(ctx context.Context) (sent, failed int, err error) {
	due := s.store.Due(s.clock.Now())
	if len(due) == 0 {
		return 0, 0, nil
	}
	const workers = 4
	jobs := make(chan Notification)
	errCh := make(chan error, len(due))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				errCh <- s.deliver(n)
			}
		}()
	}
	// producer feeds jobs to the workers
	go func() {
		defer close(jobs)
		for _, n := range due {
			if ctx.Err() != nil {
				return
			}
			jobs <- n
		}
	}()
	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			failed++
		} else {
			sent++
		}
	}
	return sent, failed, nil
}

// deliver attempts a single notification and updates its state. It returns a
// non-nil error when the delivery did not succeed.
func (s *Service) deliver(n Notification) error {
	if s.simulateSend() {
		s.store.Update(n.ID, func(x *Notification) {
			x.State = StateSent
			x.Attempt++
			x.SentAt = s.clock.Now()
		})
		return nil
	}
	attempt := n.Attempt + 1
	if attempt >= n.MaxAttempts {
		s.store.Update(n.ID, func(x *Notification) {
			x.State = StateFailed
			x.Attempt = attempt
			x.LastError = "delivery failed (max attempts reached)"
		})
		return fmt.Errorf("delivery failed (max attempts reached)")
	}
	backoff := s.backoff(attempt)
	s.store.Update(n.ID, func(x *Notification) {
		x.State = StateRetrying
		x.Attempt = attempt
		x.NextRetryAt = s.clock.Now().Add(backoff)
		x.LastError = "delivery failed (simulated)"
	})
	return fmt.Errorf("delivery failed (simulated)")
}

// RetryFailed forces all failed-terminal notifications back into retrying
// (resetting the attempt counter and next-retry time). This is an operator
// override; PushBatch performs the actual (re)delivery.
func (s *Service) RetryFailed(ctx context.Context) int {
	count := 0
	for _, n := range s.store.All() {
		if n.State != StateFailed {
			continue
		}
		s.store.Update(n.ID, func(x *Notification) {
			x.State = StateRetrying
			x.Attempt = 0
			x.NextRetryAt = s.clock.Now().Add(s.backoff(1))
			x.LastError = ""
		})
		count++
	}
	return count
}

// List returns all notifications (copies).
func (s *Service) List(ctx context.Context) []Notification {
	return s.store.All()
}

// CountPending returns the number of notifications still awaiting delivery
// (queued or retrying), for the dashboard summary.
func (s *Service) CountPending(ctx context.Context) int {
	tally := s.store.CountByState()
	return tally[StateQueued.String()] + tally[StateRetrying.String()]
}

// OnAlarm implements scada.AlarmSink: when a SCADA alarm fires, enqueue a
// notification to the on-call channel.
func (s *Service) OnAlarm(ctx context.Context, a scada.Alarm) error {
	_, err := s.Enqueue(ctx, "oncall", ChannelSMS, fmt.Sprintf("ALARM %s", a.Level), a.Message)
	return err
}

// backoff returns the retry delay for the nth attempt: base * 2^(n-1).
func (s *Service) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	mult := math.Pow(2, float64(attempt-1))
	return time.Duration(float64(s.cfg.Backoff) * mult)
}

// simulateSend returns true with probability (1 - FailureRate).
func (s *Service) simulateSend() bool {
	if s.cfg.FailureRate <= 0 {
		return true
	}
	return s.rng.float() >= s.cfg.FailureRate
}

// counterPRNG is a tiny, self-contained PRNG so delivery simulation is
// deterministic per-process without importing crypto/rand in the hot path.
type counterPRNG struct {
	state uint64
}

func newCounterPRNG() *counterPRNG {
	return &counterPRNG{state: uint64(time.Now().UnixNano())}
}

func (p *counterPRNG) float() float64 {
	// xorshift64
	x := atomic.AddUint64(&p.state, 1)
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	return float64(x>>11) / float64(1<<53)
}
