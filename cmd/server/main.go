// Package main is the entry point for the gas pipeline operations control
// service. It loads configuration, constructs the in-memory stores and
// services, seeds demo data, registers HTTP routes, and serves on the
// configured port.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/config"
	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/dispatch"
	"gas-pipeline-operations-control-service/internal/httpapi"
	"gas-pipeline-operations-control-service/internal/incident"
	"gas-pipeline-operations-control-service/internal/leakdetect"
	"gas-pipeline-operations-control-service/internal/metering"
	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/nomination"
	"gas-pipeline-operations-control-service/internal/notify"
	"gas-pipeline-operations-control-service/internal/permit"
	"gas-pipeline-operations-control-service/internal/scada"
	"gas-pipeline-operations-control-service/internal/seed"
)

func main() {
	cfgFlag := flag.String("config", "", "path to a config file (optional; env vars used otherwise)")
	addr := flag.String("addr", "", "override listen address (e.g. :18090)")
	noSeed := flag.Bool("no-seed", false, "skip seeding demo data")
	flag.Parse()
	_ = cfgFlag

	cfg := config.FromEnv()
	if *addr != "" {
		cfg.HTTPAddr = *addr
	}

	// ---- platform ----
	clock := platformClock{}

	// ---- audit (depends on nothing) ----
	auditStore := audit.NewStore(cfg.AuditRetention)
	auditSvc := audit.NewService(auditStore, clock)

	// ---- network ----
	netStore := network.NewStore()
	netSvc := network.NewService(netStore, clock, auditSvc)
	netSvc.SetLimitProvider(cfg.LimitProvider)

	// ---- scada ----
	scadaStore := scada.NewStore(cfg.ScadaReadingBuffer)
	scadaSvc := scada.NewService(scadaStore, clock, cfg.ScadaRateLimit)

	// ---- metering ----
	meterStore := metering.NewStore(5000)
	meterSvc := metering.NewService(meterStore, clock, auditSvc)

	// ---- contract + nomination ----
	contractStore := contract.NewStore()
	contractSvc := contract.NewService(contractStore, clock, auditSvc)
	nominationStore := nomination.NewStore()
	nominationSvc := nomination.NewService(nominationStore, contractSvc, clock, auditSvc)

	// ---- incident (resolves alarms via scada) ----
	incidentStore := incident.NewStore()
	incidentSvc := incident.NewService(incidentStore, clock, auditSvc, scadaAlarmResolver{scadaSvc})

	// ---- dispatch (drives network devices) ----
	dispatchStore := dispatch.NewStore()
	dispatchSvc := dispatch.NewService(dispatchStore, netSvc, clock, auditSvc)

	// ---- permit (checks dispatch + incident conflicts) ----
	permitStore := permit.NewStore()
	permitSvc := permit.NewService(permitStore, clock, dispatchSvc, incidentSvc, auditSvc)

	// ---- leak detect ----
	leakSvc := leakdetect.NewService(clock, leakdetect.Thresholds{
		DropRate:      cfg.LeakDetectDropRateThreshold,
		Imbalance:     cfg.LeakDetectImbalanceThreshold,
		WindowMinutes: cfg.LeakDetectWindowMinutes,
	})

	// ---- notify (sinks SCADA alarms) ----
	notifyStore := notify.NewStore()
	notifySvc := notify.NewService(notifyStore, clock, notify.Config{
		FailureRate: cfg.NotifyFailureRate,
		MaxAttempts: cfg.NotifyMaxAttempts,
		Backoff:     cfg.NotifyBackoff,
	})
	scadaSvc.AddSink(notifySvc)
	leakSvc.AddSink(leakToNotifySink{notifySvc})

	// ---- seed demo data ----
	if !*noSeed {
		fixture, err := seed.All(context.Background(), netSvc, scadaSvc, meterSvc, contractSvc)
		if err != nil {
			log.Printf("seed: %v", err)
		} else {
			log.Printf("seed: %d segments, %d stations, %d meters, %d points, contract=%s",
				len(fixture.Segments), len(fixture.Stations), len(fixture.Meters), len(fixture.Points), fixture.ContractID)
		}
	}

	// ---- HTTP ----
	deps := httpapi.Deps{
		Network:    netSvc,
		SCADA:      scadaSvc,
		Metering:   meterSvc,
		Contract:   contractSvc,
		Nomination: nominationSvc,
		Permit:     permitSvc,
		Dispatch:   dispatchSvc,
		Incident:   incidentSvc,
		Leak:       leakSvc,
		Audit:      auditSvc,
		Notify:     notifySvc,
	}
	srv := httpapi.New(cfg.HTTPAddr, deps)

	// graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	log.Printf("gas-pipeline-operations-control-service ready at %s", cfg.HTTPAddr)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Printf("shutdown: signal received, draining for %s", cfg.ShutdownTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Printf("shutdown: complete")
}

// platformClock is a thin adapter implementing platform.Clock.
type platformClock struct{}

func (platformClock) Now() time.Time { return time.Now() }

// scadaAlarmResolver adapts the scada service to incident.AlarmResolver
// (ResolveAlarm returns (Alarm, error); the interface wants error).
type scadaAlarmResolver struct{ s *scada.Service }

func (a scadaAlarmResolver) ResolveAlarm(ctx context.Context, id string) error {
	_, err := a.s.ResolveAlarm(ctx, id)
	return err
}

// leakToNotifySink forwards a leak alert into the notify queue so operators
// are alerted on leak detection.
type leakToNotifySink struct{ n *notify.Service }

func (s leakToNotifySink) OnLeak(ctx context.Context, a leakdetect.LeakAlert) error {
	_, err := s.n.Enqueue(ctx, "oncall", notify.ChannelSMS,
		"LEAK "+string(a.Severity), a.Message)
	return err
}
