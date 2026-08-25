package store

import (
	"database/sql"
	"fmt"
)

// CurrentVersion is the schema version written into the relational database.
// The migration layer below bumps this whenever the on-disk layout changes.
const CurrentVersion = 1

// migrations is the ordered list of DDL statements that build the relational
// schema. Each core table from the approved data model is materialized as a
// normalized SQL table with a primary key (or unique index) so that conflicting
// writes are rejected by the database and a crashed process can be resumed from
// the last fully-committed transaction. schema_migrations records the applied
// version; meta stores the single-row aggregate metadata (currently the schema
// version carried by State).
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`,

	// Rule catalog snapshot (immutable once a cycle is locked).
	`CREATE TABLE IF NOT EXISTS snapshots (
		cycle_id TEXT PRIMARY KEY,
		data     TEXT NOT NULL
	);`,

	// Pollination cycle aggregate and its locked configuration.
	`CREATE TABLE IF NOT EXISTS cycles (
		id   TEXT PRIMARY KEY,
		data TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS configs (
		cycle_id TEXT PRIMARY KEY,
		data     TEXT NOT NULL
	);`,

	// Time-window occupancies and flight budget accounts.
	`CREATE TABLE IF NOT EXISTS occupancies (
		cycle_id TEXT    NOT NULL,
		idx      INTEGER NOT NULL,
		data     TEXT    NOT NULL,
		PRIMARY KEY (cycle_id, idx)
	);`,
	`CREATE TABLE IF NOT EXISTS budgets (
		id   TEXT PRIMARY KEY,
		data TEXT NOT NULL
	);`,

	// Coverage cells keyed by zone, inflorescence batch and observation point.
	`CREATE TABLE IF NOT EXISTS coverage_cells (
		key  TEXT PRIMARY KEY,
		data TEXT NOT NULL
	);`,

	// Append-only evidence stream ordered by sequence.
	`CREATE TABLE IF NOT EXISTS evidence (
		cycle_id TEXT    NOT NULL,
		idx      INTEGER NOT NULL,
		data     TEXT    NOT NULL,
		PRIMARY KEY (cycle_id, idx)
	);`,

	// Idempotency table keyed by operation id.
	`CREATE TABLE IF NOT EXISTS operations (
		operation_id TEXT PRIMARY KEY,
		data         TEXT NOT NULL
	);`,

	// Pesticide events, freeze barriers and recovery credentials.
	`CREATE TABLE IF NOT EXISTS pesticides (
		cycle_id TEXT    NOT NULL,
		idx      INTEGER NOT NULL,
		data     TEXT    NOT NULL,
		PRIMARY KEY (cycle_id, idx)
	);`,
	`CREATE TABLE IF NOT EXISTS barriers (
		cycle_id TEXT    NOT NULL,
		idx      INTEGER NOT NULL,
		data     TEXT    NOT NULL,
		PRIMARY KEY (cycle_id, idx)
	);`,
	`CREATE TABLE IF NOT EXISTS recovery_credentials (
		cycle_id TEXT    NOT NULL,
		idx      INTEGER NOT NULL,
		data     TEXT    NOT NULL,
		PRIMARY KEY (cycle_id, idx)
	);`,

	// Independent reviews.
	`CREATE TABLE IF NOT EXISTS reviews (
		cycle_id TEXT    NOT NULL,
		idx      INTEGER NOT NULL,
		data     TEXT    NOT NULL,
		PRIMARY KEY (cycle_id, idx)
	);`,

	// Device calls and their deterministic retry attempts.
	`CREATE TABLE IF NOT EXISTS device_calls (
		key  TEXT PRIMARY KEY,
		data TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS retry_attempts (
		key TEXT    NOT NULL,
		idx INTEGER NOT NULL,
		data TEXT   NOT NULL,
		PRIMARY KEY (key, idx)
	);`,

	// Unique terminal credential per cycle (single final state).
	`CREATE TABLE IF NOT EXISTS terminals (
		cycle_id TEXT PRIMARY KEY,
		data     TEXT NOT NULL
	);`,
}

// applyMigrations runs every pending DDL statement and records the current
// schema version. Migrations are idempotent so reopening an existing database
// is safe.
func applyMigrations(db *sql.DB) error {
	for i, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("apply migration %d: %w", i, err)
		}
	}
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		CurrentVersion, 0,
	); err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}
	return nil
}
