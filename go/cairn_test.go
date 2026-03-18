package cairn

import (
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
