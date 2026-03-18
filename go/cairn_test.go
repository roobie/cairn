package cairn

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// ---- Test Vector Harness ----

// readVectorFile loads a spec vector JSON file.
// go test sets CWD to the package directory (go/), so ../spec/vectors/ is correct.
func readVectorFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../spec/vectors/" + name)
	if err != nil {
		t.Fatalf("readVectorFile %s: %v", name, err)
	}
	return data
}

// decodePayload decodes a base64 payload string.
// Empty string "" -> empty []byte (for empty_payload test cases).
func decodePayload(t *testing.T, b64 string) []byte {
	t.Helper()
	if b64 == "" {
		return []byte{}
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decodePayload: invalid base64 %q: %v", b64, err)
	}
	return data
}

// assertErrorKind checks that err matches the expected error_kind string.
func assertErrorKind(t *testing.T, err error, kind string) {
	t.Helper()
	switch kind {
	case "payload_too_large":
		if !errors.Is(err, ErrPayloadTooLarge) {
			t.Errorf("error kind: got %v, want ErrPayloadTooLarge", err)
		}
	case "empty_topic":
		if !errors.Is(err, ErrEmptyTopic) {
			t.Errorf("error kind: got %v, want ErrEmptyTopic", err)
		}
	case "empty_payload":
		if !errors.Is(err, ErrEmptyPayload) {
			t.Errorf("error kind: got %v, want ErrEmptyPayload", err)
		}
	case "store_not_open":
		if !errors.Is(err, ErrStoreNotOpen) {
			t.Errorf("error kind: got %v, want ErrStoreNotOpen", err)
		}
	case "immutability_violation":
		if err == nil {
			t.Errorf("error kind: got nil, want immutability_violation error")
		} else if !strings.Contains(err.Error(), "updates not allowed") &&
			!strings.Contains(err.Error(), "deletes not allowed") {
			t.Errorf("error kind: got %v, want message containing 'updates not allowed' or 'deletes not allowed'", err)
		}
	default:
		t.Errorf("assertErrorKind: unknown error kind %q", kind)
	}
}

// ---- Append Vector Tests ----

type appendVectorFile struct {
	TestGroups []struct {
		Tests []appendTestCase `json:"tests"`
	} `json:"test_groups"`
}

type appendTestCase struct {
	TCID    int    `json:"tc_id"`
	Comment string `json:"comment"`
	Input   struct {
		Topic            string `json:"topic"`
		Payload          string `json:"payload"`
		PayloadSizeBytes int    `json:"payload_size_bytes"`
		StoreClosed      bool   `json:"store_closed"`
	} `json:"input"`
	Expected struct {
		Result     string `json:"result"`
		ErrorKind  string `json:"error_kind"`
		EventIDMin string `json:"event_id_min"`
	} `json:"expected"`
}

func TestAppendVectors(t *testing.T) {
	var vf appendVectorFile
	if err := json.Unmarshal(readVectorFile(t, "append.json"), &vf); err != nil {
		t.Fatalf("unmarshal append.json: %v", err)
	}

	for _, group := range vf.TestGroups {
		for _, tc := range group.Tests {
			tc := tc
			t.Run(fmt.Sprintf("TC%d", tc.TCID), func(t *testing.T) {
				s := openTestStore(t)
				if tc.Input.StoreClosed {
					s.Close()
				}

				var payload []byte
				if tc.Input.PayloadSizeBytes > 0 {
					payload = make([]byte, tc.Input.PayloadSizeBytes)
				} else {
					payload = decodePayload(t, tc.Input.Payload)
				}

				id, err := s.Append(tc.Input.Topic, payload)

				if tc.Expected.Result == "valid" {
					if err != nil {
						t.Errorf("TC%d (%s): unexpected error: %v", tc.TCID, tc.Comment, err)
						return
					}
					if tc.Expected.EventIDMin != "" {
						minID, _ := strconv.ParseUint(tc.Expected.EventIDMin, 10, 64)
						if id < minID {
							t.Errorf("TC%d (%s): id = %d, want >= %d", tc.TCID, tc.Comment, id, minID)
						}
					}
				} else {
					if err == nil {
						t.Errorf("TC%d (%s): expected error, got nil", tc.TCID, tc.Comment)
						return
					}
					assertErrorKind(t, err, tc.Expected.ErrorKind)
				}
			})
		}
	}
}

// ---- Batch Vector Tests ----

type batchVectorFile struct {
	TestGroups []struct {
		Tests []batchTestCase `json:"tests"`
	} `json:"test_groups"`
}

type batchTestCase struct {
	TCID    int    `json:"tc_id"`
	Comment string `json:"comment"`
	Input   struct {
		Events []struct {
			Topic            string `json:"topic"`
			Payload          string `json:"payload"`
			PayloadSizeBytes int    `json:"payload_size_bytes"`
		} `json:"events"`
		StoreClosed bool `json:"store_closed"`
	} `json:"input"`
	Expected struct {
		Result       string `json:"result"`
		ErrorKind    string `json:"error_kind"`
		EventIDCount int    `json:"event_id_count"`
	} `json:"expected"`
}

func TestBatchVectors(t *testing.T) {
	var vf batchVectorFile
	if err := json.Unmarshal(readVectorFile(t, "batch.json"), &vf); err != nil {
		t.Fatalf("unmarshal batch.json: %v", err)
	}

	for _, group := range vf.TestGroups {
		for _, tc := range group.Tests {
			tc := tc
			t.Run(fmt.Sprintf("TC%d", tc.TCID), func(t *testing.T) {
				s := openTestStore(t)
				if tc.Input.StoreClosed {
					s.Close()
				}

				var batchEvents []BatchEvent
				for _, e := range tc.Input.Events {
					var payload []byte
					if e.PayloadSizeBytes > 0 {
						payload = make([]byte, e.PayloadSizeBytes)
					} else {
						payload = decodePayload(t, e.Payload)
					}
					batchEvents = append(batchEvents, BatchEvent{Topic: e.Topic, Payload: payload})
				}
				// If input.events was [] (empty JSON array), ensure we pass an empty slice not nil
				if tc.Input.Events != nil && batchEvents == nil {
					batchEvents = []BatchEvent{}
				}

				ids, err := s.AppendBatch(batchEvents)

				if tc.Expected.Result == "valid" {
					if err != nil {
						t.Errorf("TC%d (%s): unexpected error: %v", tc.TCID, tc.Comment, err)
						return
					}
					if len(ids) != tc.Expected.EventIDCount {
						t.Errorf("TC%d (%s): len(ids) = %d, want %d", tc.TCID, tc.Comment, len(ids), tc.Expected.EventIDCount)
					}
				} else {
					if err == nil {
						t.Errorf("TC%d (%s): expected error, got nil", tc.TCID, tc.Comment)
						return
					}
					assertErrorKind(t, err, tc.Expected.ErrorKind)
				}
			})
		}
	}
}

// ---- Query Vector Tests ----

type queryVectorFile struct {
	TestGroups []struct {
		Tests []queryTestCase `json:"tests"`
	} `json:"test_groups"`
}

type queryTestCase struct {
	TCID    int    `json:"tc_id"`
	Comment string `json:"comment"`
	Input   struct {
		Events []struct {
			Topic   string `json:"topic"`
			TS      string `json:"ts"`
			Payload string `json:"payload"`
		} `json:"events"`
		Query struct {
			Topic string `json:"topic"`
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"query"`
		StoreClosed bool `json:"store_closed"`
	} `json:"input"`
	Expected struct {
		Result        string `json:"result"`
		ErrorKind     string `json:"error_kind"`
		ReturnedCount int    `json:"returned_count"`
		Events        []struct {
			TS      string `json:"ts"`
			Payload string `json:"payload"`
		} `json:"events"`
	} `json:"expected"`
}

func TestQueryVectors(t *testing.T) {
	var vf queryVectorFile
	if err := json.Unmarshal(readVectorFile(t, "query.json"), &vf); err != nil {
		t.Fatalf("unmarshal query.json: %v", err)
	}

	for _, group := range vf.TestGroups {
		for _, tc := range group.Tests {
			tc := tc
			t.Run(fmt.Sprintf("TC%d", tc.TCID), func(t *testing.T) {
				s := openTestStore(t)

				// Setup: insert events with exact timestamps via raw SQL (NOT Append)
				if !tc.Input.StoreClosed {
					ctx := context.Background()
					for _, e := range tc.Input.Events {
						ts, err := strconv.ParseInt(e.TS, 10, 64)
						if err != nil {
							t.Fatalf("TC%d: parse ts %q: %v", tc.TCID, e.TS, err)
						}
						payload := decodePayload(t, e.Payload)
						if _, err := s.db.ExecContext(ctx,
							"INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)",
							e.Topic, ts, payload,
						); err != nil {
							t.Fatalf("TC%d: setup INSERT: %v", tc.TCID, err)
						}
					}
				}

				if tc.Input.StoreClosed {
					s.Close()
				}

				start, _ := strconv.ParseInt(tc.Input.Query.Start, 10, 64)
				end, _ := strconv.ParseInt(tc.Input.Query.End, 10, 64)
				events, err := s.Query(tc.Input.Query.Topic, start, end)

				if tc.Expected.Result == "valid" {
					if err != nil {
						t.Errorf("TC%d (%s): unexpected error: %v", tc.TCID, tc.Comment, err)
						return
					}
					if len(events) != tc.Expected.ReturnedCount {
						t.Errorf("TC%d (%s): len(events) = %d, want %d", tc.TCID, tc.Comment, len(events), tc.Expected.ReturnedCount)
						return
					}
					for i, exp := range tc.Expected.Events {
						expTS, _ := strconv.ParseInt(exp.TS, 10, 64)
						expPayload := decodePayload(t, exp.Payload)
						if events[i].TS != expTS {
							t.Errorf("TC%d (%s): events[%d].TS = %d, want %d", tc.TCID, tc.Comment, i, events[i].TS, expTS)
						}
						if string(events[i].Payload) != string(expPayload) {
							t.Errorf("TC%d (%s): events[%d].Payload mismatch", tc.TCID, tc.Comment, i)
						}
					}
				} else {
					if err == nil {
						t.Errorf("TC%d (%s): expected error, got nil", tc.TCID, tc.Comment)
						return
					}
					assertErrorKind(t, err, tc.Expected.ErrorKind)
				}
			})
		}
	}
}

// ---- Immutability Vector Tests ----

type immutabilityVectorFile struct {
	TestGroups []struct {
		Tests []immutabilityTestCase `json:"tests"`
	} `json:"test_groups"`
}

type immutabilityTestCase struct {
	TCID    int    `json:"tc_id"`
	Comment string `json:"comment"`
	Input   struct {
		Setup []struct {
			Topic   string `json:"topic"`
			Payload string `json:"payload"`
		} `json:"setup"`
		SQL string `json:"sql"`
	} `json:"input"`
	Expected struct {
		Result               string `json:"result"`
		ErrorKind            string `json:"error_kind"`
		ErrorMessageContains string `json:"error_message_contains"`
	} `json:"expected"`
}

func TestImmutabilityVectors(t *testing.T) {
	var vf immutabilityVectorFile
	if err := json.Unmarshal(readVectorFile(t, "immutability.json"), &vf); err != nil {
		t.Fatalf("unmarshal immutability.json: %v", err)
	}

	for _, group := range vf.TestGroups {
		for _, tc := range group.Tests {
			tc := tc
			t.Run(fmt.Sprintf("TC%d", tc.TCID), func(t *testing.T) {
				s := openTestStore(t)

				// Setup: insert events via Append API
				for _, e := range tc.Input.Setup {
					payload := decodePayload(t, e.Payload)
					if _, err := s.Append(e.Topic, payload); err != nil {
						t.Fatalf("TC%d: setup Append: %v", tc.TCID, err)
					}
				}

				// Execute the raw SQL bypass attempt directly against the SQLite connection
				_, err := s.db.ExecContext(context.Background(), tc.Input.SQL)

				if err == nil {
					t.Errorf("TC%d (%s): expected error, got nil — immutability trigger did not fire", tc.TCID, tc.Comment)
					return
				}
				if !strings.Contains(err.Error(), tc.Expected.ErrorMessageContains) {
					t.Errorf("TC%d (%s): error %q does not contain %q", tc.TCID, tc.Comment, err.Error(), tc.Expected.ErrorMessageContains)
				}
			})
		}
	}
}
