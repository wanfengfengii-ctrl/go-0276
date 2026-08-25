package store

import (
	"database/sql"
	"encoding/json"
	"sort"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/cycle"
	"tomato-bumblebee-pollination-rotation/internal/occupancy"
	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/safety"
)

// SQLStore is a relational (SQLite) persistence backend. Every core table from
// the data model is stored as a normalized SQL table with a primary key, and all
// writes run inside a single database transaction so that a crash or error
// leaves either the complete before-state or the complete after-state.
type SQLStore struct {
	db *sql.DB
}

// OpenSQLStore opens (or initializes) a SQLite database at path, applying any
// pending migrations and leaving an empty database ready for use when the file
// does not yet exist. An empty path selects an in-memory database.
func OpenSQLStore(path string) (*SQLStore, error) {
	dsn := path
	if dsn == "" {
		dsn = ":memory:"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite accepts a single writer; serializing access avoids "database is
	// locked" errors while preserving transactional isolation.
	db.SetMaxOpenConns(1)
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLStore{db: db}, nil
}

// Update runs fn inside a single database transaction. On error the transaction
// is rolled back and no partial state is persisted.
func (s *SQLStore) Update(fn func(*State) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	state, err := loadState(tx)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := fn(state); err != nil {
		tx.Rollback()
		return err
	}
	if err := saveState(tx, state); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// View loads the current state and runs fn read-only against it.
func (s *SQLStore) View(fn func(*State) error) error {
	state, err := loadState(s.db)
	if err != nil {
		return err
	}
	return fn(state)
}

// Close releases the underlying database handle.
func (s *SQLStore) Close() error { return s.db.Close() }

// queryer abstracts the SQL surface shared by *sql.DB and *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func loadState(q queryer) (*State, error) {
	s := NewState()
	var err error
	if s.Cycles, err = loadMap[cycle.PollinationCycle](q, "cycles", "id"); err != nil {
		return nil, err
	}
	if s.Configs, err = loadMap[cycle.LockConfig](q, "configs", "cycle_id"); err != nil {
		return nil, err
	}
	if s.Snapshots, err = loadMap[rules.RuleSnapshot](q, "snapshots", "cycle_id"); err != nil {
		return nil, err
	}
	if s.Occupancies, err = loadSlice[occupancy.WindowOccupancy](q, "occupancies", "cycle_id"); err != nil {
		return nil, err
	}
	if s.Budgets, err = loadMap[occupancy.FlightBudgetAccount](q, "budgets", "id"); err != nil {
		return nil, err
	}
	if s.CoverageCells, err = loadMap[coverage.CoverageCell](q, "coverage_cells", "key"); err != nil {
		return nil, err
	}
	if s.Evidence, err = loadSlice[cycle.EvidenceEvent](q, "evidence", "cycle_id"); err != nil {
		return nil, err
	}
	if s.Operations, err = loadMap[cycle.OperationRecord](q, "operations", "operation_id"); err != nil {
		return nil, err
	}
	if s.Pesticides, err = loadSlice[safety.PesticideEvent](q, "pesticides", "cycle_id"); err != nil {
		return nil, err
	}
	if s.Barriers, err = loadSlice[safety.FreezeBarrier](q, "barriers", "cycle_id"); err != nil {
		return nil, err
	}
	if s.RecoveryCredentials, err = loadSlice[safety.RecoveryCredential](q, "recovery_credentials", "cycle_id"); err != nil {
		return nil, err
	}
	if s.Reviews, err = loadSlice[safety.Review](q, "reviews", "cycle_id"); err != nil {
		return nil, err
	}
	if s.DeviceCalls, err = loadMap[coverage.DeviceCall](q, "device_calls", "key"); err != nil {
		return nil, err
	}
	if s.RetryAttempts, err = loadSlice[coverage.RetryAttempt](q, "retry_attempts", "key"); err != nil {
		return nil, err
	}
	if s.Terminals, err = loadMap[cycle.TerminalCredential](q, "terminals", "cycle_id"); err != nil {
		return nil, err
	}
	return s, nil
}

func saveState(tx *sql.Tx, s *State) error {
	var err error
	if err = saveMap(tx, "cycles", "id", s.Cycles); err != nil {
		return err
	}
	if err = saveMap(tx, "configs", "cycle_id", s.Configs); err != nil {
		return err
	}
	if err = saveMap(tx, "snapshots", "cycle_id", s.Snapshots); err != nil {
		return err
	}
	if err = saveSlice(tx, "occupancies", "cycle_id", s.Occupancies); err != nil {
		return err
	}
	if err = saveMap(tx, "budgets", "id", s.Budgets); err != nil {
		return err
	}
	if err = saveMap(tx, "coverage_cells", "key", s.CoverageCells); err != nil {
		return err
	}
	if err = saveSlice(tx, "evidence", "cycle_id", s.Evidence); err != nil {
		return err
	}
	if err = saveSlice(tx, "pesticides", "cycle_id", s.Pesticides); err != nil {
		return err
	}
	if err = saveSlice(tx, "barriers", "cycle_id", s.Barriers); err != nil {
		return err
	}
	if err = saveSlice(tx, "recovery_credentials", "cycle_id", s.RecoveryCredentials); err != nil {
		return err
	}
	if err = saveSlice(tx, "reviews", "cycle_id", s.Reviews); err != nil {
		return err
	}
	if err = saveMap(tx, "device_calls", "key", s.DeviceCalls); err != nil {
		return err
	}
	if err = saveSlice(tx, "retry_attempts", "key", s.RetryAttempts); err != nil {
		return err
	}
	if err = saveMap(tx, "terminals", "cycle_id", s.Terminals); err != nil {
		return err
	}
	if _, err = tx.Exec(
		`INSERT OR REPLACE INTO meta (key, value) VALUES ('version', ?)`, itoa(int64(s.Version)),
	); err != nil {
		return err
	}
	return nil
}

// loadMap reads a map[string]T table whose rows are (key, data) pairs.
func loadMap[T any](q queryer, table, keyCol string) (map[string]T, error) {
	rows, err := q.Query("SELECT " + keyCol + ", data FROM " + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]T)
	for rows.Next() {
		var key, data string
		if err := rows.Scan(&key, &data); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, rows.Err()
}

// loadSlice reads a map[string][]T table whose rows are (key, idx, data).
func loadSlice[T any](q queryer, table, keyCol string) (map[string][]T, error) {
	rows, err := q.Query("SELECT " + keyCol + ", idx, data FROM " + table + " ORDER BY " + keyCol + ", idx")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]T)
	for rows.Next() {
		var key, data string
		var idx int
		if err := rows.Scan(&key, &idx, &data); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out[key] = append(out[key], v)
	}
	return out, rows.Err()
}

// saveMap fully replaces the contents of a map[string]T table inside the
// transaction, inserting rows in deterministic key order.
func saveMap[T any](tx *sql.Tx, table, keyCol string, m map[string]T) error {
	if _, err := tx.Exec("DELETE FROM " + table); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO " + table + " (" + keyCol + ", data) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, k := range sortedKeys(m) {
		b, err := json.Marshal(m[k])
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(k, string(b)); err != nil {
			return err
		}
	}
	return nil
}

// saveSlice fully replaces the contents of a map[string][]T table inside the
// transaction, preserving element order.
func saveSlice[T any](tx *sql.Tx, table, keyCol string, m map[string][]T) error {
	if _, err := tx.Exec("DELETE FROM " + table); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO " + table + " (" + keyCol + ", idx, data) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, k := range sortedKeys(m) {
		for i, v := range m[k] {
			b, err := json.Marshal(v)
			if err != nil {
				return err
			}
			if _, err := stmt.Exec(k, i, string(b)); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var _ Store = (*SQLStore)(nil)
