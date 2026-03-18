package cairn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

// schemaSQL is the cairn schema DDL executed verbatim on every Open().
// Copied from spec/schema.sql — all statements use IF NOT EXISTS for idempotent execution.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS events (
    id      INTEGER PRIMARY KEY,
    topic   TEXT    NOT NULL,
    ts      INTEGER NOT NULL,
    payload BLOB    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_topic_ts ON events (topic, ts);

CREATE TRIGGER IF NOT EXISTS no_update
    BEFORE UPDATE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: updates not allowed');
END;

CREATE TRIGGER IF NOT EXISTS no_delete
    BEFORE DELETE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: deletes not allowed');
END;
`

// Event is a record returned by Query.
type Event struct {
	ID      uint64
	Topic   string
	TS      int64  // nanoseconds since Unix epoch
	Payload []byte
}

// BatchEvent is an input record for AppendBatch.
type BatchEvent struct {
	Topic   string
	Payload []byte
}

// Store is an open handle to a cairn database.
// Create with Open; invalidated by Close.
type Store struct {
	db     *sql.DB
	mu     sync.Mutex
	closed bool
}

// Open opens or creates a cairn database at path.
// The parent directory of path must exist; Open does not create parent directories.
// On success, schema DDL has been applied and WAL mode is active.
func Open(path string) (*Store, error) {
	// Validate parent directory exists (sql.Open is lazy — it won't error until first use)
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("cairn: parent directory does not exist: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("cairn: open database: %w", err)
	}

	// SQLite is single-writer; one open connection avoids SQLITE_BUSY contention
	db.SetMaxOpenConns(1)

	ctx := context.Background()

	// Apply connection-level PRAGMAs
	if _, err := db.ExecContext(ctx, `
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
		PRAGMA foreign_keys = ON;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("cairn: configure pragmas: %w", err)
	}

	// Attempt SQLITE_DBCONFIG_DEFENSIVE via conn.Raw().
	// modernc.org/sqlite/lib exposes Xsqlite3_db_config and SQLITE_DBCONFIG_DEFENSIVE,
	// but the raw pointer is not accessible through the database/sql interface.
	// We attempt conn.Raw() and fall back to PRAGMA trusted_schema = OFF.
	conn, err := db.Conn(ctx)
	if err == nil {
		rawErr := conn.Raw(func(driverConn any) error {
			// modernc.org/sqlite does not expose the TLS/db handle through database/sql conn.Raw().
			// The raw conn here is a driver.Conn interface, not a struct with exported fields.
			// SQLITE_DBCONFIG_DEFENSIVE is not accessible via this path.
			return errors.New("not accessible")
		})
		conn.Close()

		if rawErr != nil {
			// SQLITE_DBCONFIG_DEFENSIVE not accessible via database/sql conn.Raw();
			// using PRAGMA trusted_schema = OFF as fallback.
			// Immutability triggers (no_update, no_delete) provide the actual enforcement.
			if _, pragmaErr := db.ExecContext(ctx, "PRAGMA trusted_schema = OFF"); pragmaErr != nil {
				db.Close()
				return nil, fmt.Errorf("cairn: trusted_schema pragma: %w", pragmaErr)
			}
		}
	}

	// Execute schema DDL — idempotent via IF NOT EXISTS
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("cairn: init schema: %w", err)
	}

	return &Store{db: db}, nil
}

// checkOpen returns ErrStoreNotOpen if the store has been closed.
// Caller must NOT hold mu when calling this — it acquires the lock internally.
func (s *Store) checkOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrStoreNotOpen
	}
	return nil
}

// Close checkpoints the WAL and closes the database connection.
// Close is idempotent — calling it multiple times is safe and returns nil after the first call.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil // idempotent
	}
	s.closed = true

	// Checkpoint WAL back into main database file and truncate the WAL.
	// Spec: Close is always safe — ignore checkpoint errors.
	_, _ = s.db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")

	return s.db.Close()
}

// Append inserts a single event and returns its EventID.
// Stub — implemented in Plan 02.
func (s *Store) Append(topic string, payload []byte) (uint64, error) {
	return 0, errors.New("not implemented")
}

// AppendBatch inserts multiple events in a single transaction and returns their EventIDs.
// Stub — implemented in Plan 02.
func (s *Store) AppendBatch(events []BatchEvent) ([]uint64, error) {
	return nil, errors.New("not implemented")
}

// Query returns all events in the given topic with timestamp in [start, end] (inclusive).
// Stub — implemented in Plan 02.
func (s *Store) Query(topic string, start, end int64) ([]Event, error) {
	return nil, errors.New("not implemented")
}
