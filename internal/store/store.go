// Package store implements the persistence boundary for the pollination backend.
// It persists all core tables in a relational database through transactions and
// unique constraints so the process can recover unfinished cycles, pending device
// retries and undecided arbitration after a restart. Two backends are provided:
// an in-memory database for deterministic tests and a SQLite-backed relational
// database with migrations for real restart recovery.
package store

import (
	"sync"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/cycle"
	"tomato-bumblebee-pollination-rotation/internal/occupancy"
	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/safety"
)

// State is the full persisted database. Every field is a core table keyed so
// that transactions can be applied atomically and rolled back on failure.
type State struct {
	Version             int
	Cycles              map[string]cycle.PollinationCycle
	Configs             map[string]cycle.LockConfig
	Snapshots           map[string]rules.RuleSnapshot
	Occupancies         map[string][]occupancy.WindowOccupancy
	Budgets             map[string]occupancy.FlightBudgetAccount
	CoverageCells       map[string]coverage.CoverageCell
	Evidence            map[string][]cycle.EvidenceEvent
	Operations          map[string]cycle.OperationRecord
	Pesticides          map[string][]safety.PesticideEvent
	Barriers            map[string][]safety.FreezeBarrier
	RecoveryCredentials map[string][]safety.RecoveryCredential
	Reviews             map[string][]safety.Review
	DeviceCalls         map[string]coverage.DeviceCall
	RetryAttempts       map[string][]coverage.RetryAttempt
	Terminals           map[string]cycle.TerminalCredential
}

// NewState returns an empty, initialized database state.
func NewState() *State {
	return &State{
		Version:             CurrentVersion,
		Cycles:              make(map[string]cycle.PollinationCycle),
		Configs:             make(map[string]cycle.LockConfig),
		Snapshots:           make(map[string]rules.RuleSnapshot),
		Occupancies:         make(map[string][]occupancy.WindowOccupancy),
		Budgets:             make(map[string]occupancy.FlightBudgetAccount),
		CoverageCells:       make(map[string]coverage.CoverageCell),
		Evidence:            make(map[string][]cycle.EvidenceEvent),
		Operations:          make(map[string]cycle.OperationRecord),
		Pesticides:          make(map[string][]safety.PesticideEvent),
		Barriers:            make(map[string][]safety.FreezeBarrier),
		RecoveryCredentials: make(map[string][]safety.RecoveryCredential),
		Reviews:             make(map[string][]safety.Review),
		DeviceCalls:         make(map[string]coverage.DeviceCall),
		RetryAttempts:       make(map[string][]coverage.RetryAttempt),
		Terminals:           make(map[string]cycle.TerminalCredential),
	}
}

// Store is the persistence boundary consumed by the application service.
type Store interface {
	Update(fn func(*State) error) error
	View(fn func(*State) error) error
	Close() error
}

// DB is a mutex-protected, in-memory transactional database used by tests and
// deterministic scenarios. Update applies fn against the current state inside a
// critical section and rolls back all mutations on error. View applies fn
// read-only. Production deployments use SQLStore for durable restart recovery.
type DB struct {
	mu     sync.Mutex
	state  *State
	closed bool
}

// NewMemoryStore returns an in-memory database (no persistence to disk).
func NewMemoryStore() *DB {
	return &DB{state: NewState()}
}

// Update runs fn against the current state, rolling back all mutations when fn
// returns an error so that a failed transaction leaves no partial state — the
// same all-or-nothing contract the SQL backend provides. Snapshotting happens
// before fn runs; restoring the snapshot on error discards every mutation fn
// made, including mutations to maps and slices reached through the State value.
func (db *DB) Update(fn func(*State) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	snapshot := cloneState(db.state)
	if err := fn(db.state); err != nil {
		db.state = snapshot
		return err
	}
	return nil
}

// View runs fn read-only against the current state.
func (db *DB) View(fn func(*State) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	return fn(db.state)
}

// Close marks the in-memory database closed.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.closed = true
	return nil
}

// ErrClosed is returned when operating on a closed database.
var ErrClosed = &dbClosedError{}

type dbClosedError struct{}

func (e *dbClosedError) Error() string { return "store: database closed" }

// cloneState returns an independent deep copy of s so that mutating the copy's
// maps or the slices reachable through it cannot affect the original. Value
// maps are copied element-wise (the values are copied by assignment); slice
// maps additionally get a fresh backing array per slice so appends into the copy
// cannot retroactively alter the original's slice. This is what makes the
// in-memory transaction snapshot safe to restore after a failed Update.
func cloneState(s *State) *State {
	c := &State{
		Version:             s.Version,
		Cycles:              cloneValueMap(s.Cycles),
		Configs:             cloneValueMap(s.Configs),
		Snapshots:           cloneValueMap(s.Snapshots),
		Occupancies:         cloneSliceMap(s.Occupancies),
		Budgets:             cloneValueMap(s.Budgets),
		CoverageCells:       cloneValueMap(s.CoverageCells),
		Evidence:            cloneSliceMap(s.Evidence),
		Operations:          cloneValueMap(s.Operations),
		Pesticides:          cloneSliceMap(s.Pesticides),
		Barriers:            cloneSliceMap(s.Barriers),
		RecoveryCredentials: cloneSliceMap(s.RecoveryCredentials),
		Reviews:             cloneSliceMap(s.Reviews),
		DeviceCalls:         cloneValueMap(s.DeviceCalls),
		RetryAttempts:       cloneSliceMap(s.RetryAttempts),
		Terminals:           cloneValueMap(s.Terminals),
	}
	return c
}

// cloneValueMap returns a copy of m with the same key/value entries. The value
// type T must be a pure value type (no pointer or slice fields that callers
// mutate in place); the structs stored in State satisfy this.
func cloneValueMap[T any](m map[string]T) map[string]T {
	out := make(map[string]T, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneSliceMap returns a copy of m where each slice has a fresh backing array,
// so an append into the copy cannot mutate the original's slice header or its
// underlying array.
func cloneSliceMap[T any](m map[string][]T) map[string][]T {
	out := make(map[string][]T, len(m))
	for k, v := range m {
		dup := append([]T(nil), v...)
		out[k] = dup
	}
	return out
}

// CellKey builds the map key for a coverage cell from its identity components.
func CellKey(cycleID, zoneID, batchID string, point int64) string {
	return cycleID + "|" + zoneID + "|" + batchID + "|" + itoa(point)
}

// DeviceKey builds the map key for a device call.
func DeviceKey(cycleID string, callSeq int64) string {
	return cycleID + "|" + itoa(callSeq)
}

// RetryKey builds the map key for retry attempts from a coverage retry key.
func RetryKey(cycleID, deviceType, zoneID string, point, callSeq int64) string {
	return cycleID + "|" + deviceType + "|" + zoneID + "|" + itoa(point) + "|" + itoa(callSeq)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
