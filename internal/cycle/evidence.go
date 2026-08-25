package cycle

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// EvidenceKind classifies an append-only evidence event.
type EvidenceKind string

const (
	EvidenceLocked      EvidenceKind = "locked"
	EvidenceAdmitted    EvidenceKind = "admitted"
	EvidenceObservation EvidenceKind = "observation"
	EvidenceRejected    EvidenceKind = "rejected_observation"
	EvidenceQuarantine  EvidenceKind = "quarantine"
	EvidenceDeviceCall  EvidenceKind = "device_call"
	EvidencePesticide   EvidenceKind = "pesticide_event"
	EvidenceFreeze      EvidenceKind = "freeze_barrier"
	EvidenceRecovery    EvidenceKind = "recovery_check"
	EvidenceSupplement  EvidenceKind = "supplement"
	EvidenceReview      EvidenceKind = "review"
	EvidenceTerminal    EvidenceKind = "terminal"
)

// EvidenceEvent is one entry in the global append-only evidence stream. Each
// event carries a monotonically increasing sequence number per cycle.
type EvidenceEvent struct {
	CycleID     string
	Seq         int64
	Kind        EvidenceKind
	LogicalTime int64
	Generation  int64
	Digest      string
	Payload     string
}

// EvidenceStream is an append-only, ordered collection of evidence events.
type EvidenceStream struct {
	events []EvidenceEvent
	next   int64
}

// NewEvidenceStream returns an empty stream starting at sequence one.
func NewEvidenceStream() *EvidenceStream {
	return &EvidenceStream{next: 1}
}

// Append adds an event and returns its assigned sequence number.
func (s *EvidenceStream) Append(ev EvidenceEvent) int64 {
	ev.Seq = s.next
	s.next++
	s.events = append(s.events, ev)
	return ev.Seq
}

// Events returns a copy of the ordered events.
func (s *EvidenceStream) Events() []EvidenceEvent {
	return append([]EvidenceEvent(nil), s.events...)
}

// SortEvents orders a slice of events by cycle then sequence for deterministic
// output across the evidence endpoint.
func SortEvents(events []EvidenceEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].CycleID != events[j].CycleID {
			return events[i].CycleID < events[j].CycleID
		}
		return events[i].Seq < events[j].Seq
	})
}

// EvidenceRoot computes the deterministic root digest of an evidence stream by
// hashing the ordered sequence of event digests. It is signed by reviewers and
// committed into the terminal credential.
func EvidenceRoot(events []EvidenceEvent) string {
	h := sha256.New()
	for _, ev := range events {
		h.Write([]byte(ev.Digest))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// NewEvent is a convenience constructor for an evidence event.
func NewEvent(cycleID string, kind EvidenceKind, logicalTime, generation int64, digest, payload string) EvidenceEvent {
	return EvidenceEvent{
		CycleID:     cycleID,
		Kind:        kind,
		LogicalTime: logicalTime,
		Generation:  generation,
		Digest:      digest,
		Payload:     payload,
	}
}
