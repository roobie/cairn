package cairn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestOpen verifies that Open creates a SQLite database file with WAL mode active.
func TestOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	defer s.Close()

	// Verify the .db file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Open: database file was not created")
	}

	// Verify WAL mode is active
	var journalMode string
	row := s.db.QueryRow("PRAGMA journal_mode")
	if err := row.Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode scan: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

// TestOpen_MissingDir verifies that Open returns an error when the parent directory
// does not exist.
func TestOpen_MissingDir(t *testing.T) {
	_, err := Open("/nonexistent/dir/test.db")
	if err == nil {
		t.Fatal("Open: expected error for missing parent directory, got nil")
	}
}

// TestOpen_Idempotent verifies that opening the same path twice succeeds and the
// schema remains intact on the second open.
func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()

	// Schema should be intact — verify the events table exists
	var count int
	row := s2.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events'")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("schema check scan: %v", err)
	}
	if count != 1 {
		t.Error("second open: events table not present")
	}
}

// TestClose_Idempotent verifies that calling Close twice on the same store does not
// return an error on the second call.
func TestClose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: expected nil, got %v", err)
	}
}

// openTestStore opens a fresh store in a temp dir and registers t.Cleanup to close it.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openTestStore: Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestAppend verifies the basic Append contract.
func TestAppend(t *testing.T) {
	t.Run("valid event returns id >= 1", func(t *testing.T) {
		s := openTestStore(t)
		id, err := s.Append("sensor", []byte("hello"))
		if err != nil {
			t.Fatalf("Append: unexpected error: %v", err)
		}
		if id < 1 {
			t.Errorf("Append: id = %d, want >= 1", id)
		}
	})

	t.Run("empty topic returns ErrEmptyTopic", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.Append("", []byte("hello"))
		if !errors.Is(err, ErrEmptyTopic) {
			t.Errorf("Append empty topic: got %v, want ErrEmptyTopic", err)
		}
	})

	t.Run("empty payload returns ErrEmptyPayload", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.Append("sensor", []byte{})
		if !errors.Is(err, ErrEmptyPayload) {
			t.Errorf("Append empty payload: got %v, want ErrEmptyPayload", err)
		}
	})

	t.Run("payload over 1MB returns ErrPayloadTooLarge", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.Append("sensor", make([]byte, MaxPayloadSize+1))
		if !errors.Is(err, ErrPayloadTooLarge) {
			t.Errorf("Append oversized payload: got %v, want ErrPayloadTooLarge", err)
		}
	})

	t.Run("payload exactly 1MB is valid", func(t *testing.T) {
		s := openTestStore(t)
		id, err := s.Append("sensor", make([]byte, MaxPayloadSize))
		if err != nil {
			t.Fatalf("Append 1MB payload: unexpected error: %v", err)
		}
		if id < 1 {
			t.Errorf("Append 1MB payload: id = %d, want >= 1", id)
		}
	})

	t.Run("closed store returns ErrStoreNotOpen", func(t *testing.T) {
		s := openTestStore(t)
		s.Close()
		_, err := s.Append("sensor", []byte("hello"))
		if !errors.Is(err, ErrStoreNotOpen) {
			t.Errorf("Append on closed store: got %v, want ErrStoreNotOpen", err)
		}
	})
}

// TestAppendBatch verifies the AppendBatch contract.
func TestAppendBatch(t *testing.T) {
	t.Run("batch of 3 events returns 3 ids", func(t *testing.T) {
		s := openTestStore(t)
		ids, err := s.AppendBatch([]BatchEvent{
			{Topic: "sensor", Payload: []byte("a")},
			{Topic: "metrics", Payload: []byte("b")},
			{Topic: "sensor", Payload: []byte("c")},
		})
		if err != nil {
			t.Fatalf("AppendBatch: unexpected error: %v", err)
		}
		if len(ids) != 3 {
			t.Errorf("AppendBatch: len(ids) = %d, want 3", len(ids))
		}
		for i, id := range ids {
			if id < 1 {
				t.Errorf("AppendBatch: ids[%d] = %d, want >= 1", i, id)
			}
		}
	})

	t.Run("empty batch returns empty slice, no error", func(t *testing.T) {
		s := openTestStore(t)
		ids, err := s.AppendBatch([]BatchEvent{})
		if err != nil {
			t.Fatalf("AppendBatch empty: unexpected error: %v", err)
		}
		if ids == nil {
			t.Error("AppendBatch empty: expected empty slice, got nil")
		}
		if len(ids) != 0 {
			t.Errorf("AppendBatch empty: len(ids) = %d, want 0", len(ids))
		}
	})

	t.Run("batch with empty topic rejects entire batch", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.AppendBatch([]BatchEvent{
			{Topic: "", Payload: []byte("a")},
			{Topic: "sensor", Payload: []byte("b")},
		})
		if !errors.Is(err, ErrEmptyTopic) {
			t.Errorf("AppendBatch empty topic: got %v, want ErrEmptyTopic", err)
		}
	})

	t.Run("batch with oversized payload rejects entire batch", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.AppendBatch([]BatchEvent{
			{Topic: "sensor", Payload: []byte("ok")},
			{Topic: "sensor", Payload: make([]byte, MaxPayloadSize+1)},
		})
		if !errors.Is(err, ErrPayloadTooLarge) {
			t.Errorf("AppendBatch oversized: got %v, want ErrPayloadTooLarge", err)
		}
	})

	t.Run("closed store returns ErrStoreNotOpen", func(t *testing.T) {
		s := openTestStore(t)
		s.Close()
		_, err := s.AppendBatch([]BatchEvent{{Topic: "sensor", Payload: []byte("a")}})
		if !errors.Is(err, ErrStoreNotOpen) {
			t.Errorf("AppendBatch closed: got %v, want ErrStoreNotOpen", err)
		}
	})
}

// TestQuery verifies the Query contract.
func TestQuery(t *testing.T) {
	t.Run("returns events in id ASC order matching topic and time range", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		// Insert directly with fixed timestamps
		if _, err := s.db.ExecContext(ctx,
			"INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)",
			"sensor", int64(100), []byte("first")); err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		if _, err := s.db.ExecContext(ctx,
			"INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)",
			"sensor", int64(200), []byte("second")); err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		events, err := s.Query("sensor", 50, 250)
		if err != nil {
			t.Fatalf("Query: unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("Query: got %d events, want 2", len(events))
		}
		if events[0].TS != 100 || string(events[0].Payload) != "first" {
			t.Errorf("Query: events[0] = %+v, want ts=100 payload=first", events[0])
		}
		if events[1].TS != 200 || string(events[1].Payload) != "second" {
			t.Errorf("Query: events[1] = %+v, want ts=200 payload=second", events[1])
		}
	})

	t.Run("empty result for non-matching topic is not an error", func(t *testing.T) {
		s := openTestStore(t)
		events, err := s.Query("nonexistent", 0, 9_000_000_000_000_000_000)
		if err != nil {
			t.Fatalf("Query: unexpected error: %v", err)
		}
		if events == nil {
			t.Error("Query: expected empty slice, got nil")
		}
		if len(events) != 0 {
			t.Errorf("Query: got %d events, want 0", len(events))
		}
	})

	t.Run("closed store returns ErrStoreNotOpen", func(t *testing.T) {
		s := openTestStore(t)
		s.Close()
		_, err := s.Query("sensor", 0, 9_000_000_000_000_000_000)
		if !errors.Is(err, ErrStoreNotOpen) {
			t.Errorf("Query closed: got %v, want ErrStoreNotOpen", err)
		}
	})
}

// TestClose_WALCheckpoint verifies that Close() checkpoints the WAL — the .db-wal
// file is absent or zero-length after Close.
func TestClose_WALCheckpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Write data directly to put frames into WAL
	if _, err := s.db.Exec(
		"INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)",
		"test", int64(1000000), []byte("hello"),
	); err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	// Close should checkpoint and truncate WAL
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// WAL file should be absent or empty
	walPath := path + "-wal"
	info, err := os.Stat(walPath)
	if err == nil && info.Size() > 0 {
		t.Errorf("WAL file %s not checkpointed: size = %d bytes", walPath, info.Size())
	}
}
