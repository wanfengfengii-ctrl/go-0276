package store

import (
	"path/filepath"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/cycle"
	"tomato-bumblebee-pollination-rotation/internal/occupancy"
)

func TestSQLStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := OpenSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(s *State) error {
		s.Cycles["c1"] = cycle.PollinationCycle{ID: "c1", State: cycle.StateInFlight, TaskGeneration: 1}
		s.Budgets["c1:budget"] = occupancy.FlightBudgetAccount{ID: "c1:budget", CycleID: "c1", Initial: 10, Remaining: 10}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := OpenSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	err = db2.View(func(s *State) error {
		c, ok := s.Cycles["c1"]
		if !ok {
			t.Fatal("cycle not persisted")
		}
		if c.State != cycle.StateInFlight {
			t.Fatalf("state = %v", c.State)
		}
		if _, ok := s.Budgets["c1:budget"]; !ok {
			t.Fatal("budget not persisted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQLStoreRollbackOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := OpenSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.Update(func(s *State) error {
		s.Cycles["keep"] = cycle.PollinationCycle{ID: "keep", State: cycle.StatePendingAdmit}
		return nil
	})
	err = db.Update(func(s *State) error {
		s.Cycles["bad"] = cycle.PollinationCycle{ID: "bad", State: cycle.StateInFlight}
		return &testErr{}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	_ = db.View(func(s *State) error {
		if _, ok := s.Cycles["bad"]; ok {
			t.Fatal("partial state must be rolled back")
		}
		if _, ok := s.Cycles["keep"]; !ok {
			t.Fatal("pre-existing state must survive rollback")
		}
		return nil
	})
}

func TestSQLStoreMigrationsApplied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := OpenSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var version int
	if err := db.db.QueryRow(
		`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`,
	).Scan(&version); err != nil {
		t.Fatalf("schema_migrations not populated: %v", err)
	}
	if version != CurrentVersion {
		t.Fatalf("version = %d, want %d", version, CurrentVersion)
	}

	// Every core table from the data model must exist.
	for _, table := range []string{
		"snapshots", "cycles", "configs", "occupancies", "budgets",
		"coverage_cells", "evidence", "operations", "pesticides", "barriers",
		"recovery_credentials", "reviews", "device_calls", "retry_attempts", "terminals",
	} {
		var name string
		err := db.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}
}

func TestRecoverFindsUnfinishedCycleAndPendingRetry(t *testing.T) {
	db := NewMemoryStore()
	_ = db.Update(func(s *State) error {
		s.Cycles["c1"] = cycle.PollinationCycle{ID: "c1", State: cycle.StateInFlight, TaskGeneration: 1}
		s.Cycles["c2"] = cycle.PollinationCycle{ID: "c2", State: cycle.StateCompleted, TerminalVer: 1}
		s.DeviceCalls["k1"] = coverage.DeviceCall{CycleID: "c1", CallSeq: 1, DeviceType: coverage.DeviceEnvironmentProbe, ZoneID: "z1", ObservationPoint: 1}
		s.RetryAttempts["k1"] = []coverage.RetryAttempt{{Attempt: 1, Status: coverage.RetryPending}}
		return nil
	})
	var report RecoveryReport
	_ = db.View(func(s *State) error {
		report = Recover(s)
		return nil
	})
	if len(report.UnfinishedCycles) != 1 || report.UnfinishedCycles[0] != "c1" {
		t.Fatalf("unfinished = %v", report.UnfinishedCycles)
	}
	if len(report.PendingRetries) != 1 {
		t.Fatalf("pending retries = %v", report.PendingRetries)
	}
}

type testErr struct{}

func (e *testErr) Error() string { return "boom" }
