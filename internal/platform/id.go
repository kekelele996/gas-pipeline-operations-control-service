package platform

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// idCounter is a process-wide monotonic counter that disambiguates IDs
// generated within the same millisecond.
var idCounter uint64

// initSeed primes the counter from a random source so two processes started at
// the same instant do not collide.
func init() {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read on Go never fails in practice, but fall back to time.
		binary.BigEndian.PutUint64(b[:], uint64(time.Now().UnixNano()))
	}
	atomic.StoreUint64(&idCounter, binary.BigEndian.Uint64(b[:]))
}

// NewID returns a short, lexicographically sortable identifier of the form
// "1710000000-a1b2c3" — a unix-seconds prefix for ordering plus a random
// suffix for uniqueness. It is safe for concurrent use.
func NewID() string {
	now := time.Now().Unix()
	seq := atomic.AddUint64(&idCounter, 1)
	var b [6]byte
	binary.BigEndian.PutUint32(b[:4], uint32(seq))
	binary.BigEndian.PutUint16(b[4:], uint16(seq>>32))
	// mix in 2 bytes of crypto-random for extra entropy
	var r [2]byte
	_, _ = rand.Read(r[:])
	b[4] = r[0]
	b[5] = r[1]
	return fmt.Sprintf("%d-%s", now, hex.EncodeToString(b[:]))
}

// NewPrefixedID returns an ID prefixed by a short domain tag, e.g.
// "SEG-1710000000-a1b2c3". Useful for making IDs self-describing in logs.
func NewPrefixedID(prefix string) string {
	return prefix + "-" + NewID()
}

// Common ID prefixes for the domain packages, kept here to avoid drift.
const (
	PrefixSegment      = "SEG"
	PrefixStation      = "STN"
	PrefixCompressor   = "CMP"
	PrefixValve        = "VLV"
	PrefixPoint        = "PNT"
	PrefixMeter        = "MTR"
	PrefixContract     = "CTR"
	PrefixNomination   = "NOM"
	PrefixPermit       = "PRM"
	PrefixOrder        = "ORD"
	PrefixIncident     = "INC"
	PrefixAlarm        = "ALM"
	PrefixReading      = "RDG"
	PrefixSettlement   = "STL"
	PrefixNotification = "NTF"
	PrefixAudit        = "AUD"
	PrefixLeakAlert    = "LKA"
)

// NewSegmentID and friends are thin wrappers so call sites read clearly.
func NewSegmentID() string      { return NewPrefixedID(PrefixSegment) }
func NewStationID() string      { return NewPrefixedID(PrefixStation) }
func NewCompressorID() string   { return NewPrefixedID(PrefixCompressor) }
func NewValveID() string        { return NewPrefixedID(PrefixValve) }
func NewPointID() string        { return NewPrefixedID(PrefixPoint) }
func NewMeterID() string        { return NewPrefixedID(PrefixMeter) }
func NewContractID() string     { return NewPrefixedID(PrefixContract) }
func NewNominationID() string   { return NewPrefixedID(PrefixNomination) }
func NewPermitID() string       { return NewPrefixedID(PrefixPermit) }
func NewOrderID() string        { return NewPrefixedID(PrefixOrder) }
func NewIncidentID() string     { return NewPrefixedID(PrefixIncident) }
func NewAlarmID() string        { return NewPrefixedID(PrefixAlarm) }
func NewSettlementID() string   { return NewPrefixedID(PrefixSettlement) }
func NewNotificationID() string { return NewPrefixedID(PrefixNotification) }
func NewAuditID() string        { return NewPrefixedID(PrefixAudit) }
func NewLeakAlertID() string    { return NewPrefixedID(PrefixLeakAlert) }
