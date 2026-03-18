package cairn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

// Append inserts a single event and returns its EventID (>= 1).
// Returns ErrStoreNotOpen, ErrEmptyTopic, ErrEmptyPayload, or ErrPayloadTooLarge on validation failure.
func (s *Store) Append(topic string, payload []byte) (uint64, error) {
	if err := s.checkOpen(); err != nil {
		return 0, err
	}
	if topic == "" {
		return 0, ErrEmptyTopic
	}
	if len(payload) == 0 {
		return 0, ErrEmptyPayload
	}
	if len(payload) > MaxPayloadSize {
		return 0, ErrPayloadTooLarge
	}

	ts := time.Now().UnixNano()
	result, err := s.db.ExecContext(context.Background(),
		"INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)",
		topic, ts, payload,
	)
	if err != nil {
		return 0, fmt.Errorf("cairn: append: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cairn: last insert id: %w", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("cairn: append: unexpected rowid %d", id)
	}
	return uint64(id), nil
}

// AppendBatch inserts multiple events in a single atomic transaction and returns their EventIDs.
// If events is empty, returns an empty slice with no error and no transaction.
// Validates all events before inserting any; returns the first validation error found.
func (s *Store) AppendBatch(events []BatchEvent) ([]uint64, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return []uint64{}, nil // no transaction, no error
	}

	// Validate ALL events before inserting any (all-or-nothing validation)
	// Precedence: EmptyTopic > EmptyPayload > PayloadTooLarge
	for _, e := range events {
		if e.Topic == "" {
			return nil, ErrEmptyTopic
		}
		if len(e.Payload) == 0 {
			return nil, ErrEmptyPayload
		}
		if len(e.Payload) > MaxPayloadSize {
			return nil, ErrPayloadTooLarge
		}
	}

	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cairn: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	ids := make([]uint64, 0, len(events))
	for _, e := range events {
		ts := time.Now().UnixNano()
		result, err := tx.ExecContext(ctx,
			"INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)",
			e.Topic, ts, e.Payload,
		)
		if err != nil {
			return nil, fmt.Errorf("cairn: batch insert: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("cairn: last insert id: %w", err)
		}
		ids = append(ids, uint64(id))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("cairn: commit: %w", err)
	}
	return ids, nil
}

// Query returns all events in the given topic with timestamp in [start, end] inclusive,
// ordered by id ASC (insertion order).
// Returns an empty slice (not an error) when no events match.
// Returns ErrStoreNotOpen if the store has been closed.
func (s *Store) Query(topic string, start, end int64) ([]Event, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(context.Background(),
		`SELECT id, topic, ts, payload
		 FROM events
		 WHERE topic = ? AND ts >= ? AND ts <= ?
		 ORDER BY id ASC`,
		topic, start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("cairn: query: %w", err)
	}
	defer rows.Close() // CRITICAL: must close to allow WAL checkpoint

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Topic, &e.TS, &e.Payload); err != nil {
			return nil, fmt.Errorf("cairn: scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cairn: rows: %w", err)
	}
	if events == nil {
		events = []Event{} // return empty slice, not nil
	}
	return events, nil
}
