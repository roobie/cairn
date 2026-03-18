# Phase 2: Go SDK - Research

**Researched:** 2026-03-18
**Domain:** Go library development with modernc.org/sqlite (pure Go, no CGo), database/sql patterns, test vector harness
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| GO-01 | Open(path) creates or opens a cairn store with zero configuration | database/sql `sql.Open("sqlite", path)` + RegisterConnectionHook for PRAGMA setup + schema DDL execution |
| GO-02 | Close() cleanly shuts down with WAL checkpoint | `db.ExecContext("PRAGMA wal_checkpoint(TRUNCATE)")` then `db.Close()` with idempotency guard |
| GO-03 | Append(topic, payload) returns EventID, atomic write | Single `INSERT INTO events` + `result.LastInsertId()` wrapped in validation; EventID is uint64 |
| GO-04 | AppendBatch(events) transactional multi-write, all-or-nothing | `db.BeginTx` + loop of prepared INSERT + commit; validate all before any insert |
| GO-05 | Query(topic, start, end) returns iterator of events in insertion order | `db.QueryContext` with `WHERE topic=? AND ts>=? AND ts<=? ORDER BY id ASC` + rows.Close() discipline |
| GO-06 | All shared test vectors pass | Table-driven tests loading spec/vectors/*.json; custom harness interpreting `store_closed`, `payload_size_bytes`, `setup` fields |
| GO-07 | Uses modernc.org/sqlite (pure Go, no CGo) | `_ "modernc.org/sqlite"` blank import with `"sqlite"` driver name; `go build` works without C toolchain |
</phase_requirements>

---

## Summary

Phase 2 implements the Go reference SDK for cairn. The surface area is small (5 public operations), but correctness matters more than completeness: this SDK sets the idiom baseline for TypeScript and Rust. The key technical decisions are already locked by the spec — modernc.org/sqlite as the driver (GO-07), and all SQLite behavior is defined by spec/api.md.

The main technical challenge is enabling `SQLITE_DBCONFIG_DEFENSIVE`. The modernc.org/sqlite package does not expose a `SetDefensive()` method directly — it is a `database/sql` driver with connection hooks. The recommended approach is to use `RegisterConnectionHook` to execute PRAGMAs on every connection. For SQLITE_DBCONFIG_DEFENSIVE specifically, the `modernc.org/sqlite/lib` package exposes `Xsqlite3_db_config` and `lib.SQLITE_DBCONFIG_DEFENSIVE`, but calling it requires a raw connection handle. The practical approach for this SDK is: use `RegisterConnectionHook` for all SQL PRAGMAs (WAL, busy_timeout, foreign_keys), and for SQLITE_DBCONFIG_DEFENSIVE, execute `PRAGMA trusted_schema = OFF` as a partial substitute (available via SQL) and document that the C-level defensive flag requires a lower-level binding. The spec itself already acknowledges this complexity per-language and the test suite validates immutability via trigger behavior, which does not require defensive mode to pass.

The test vector harness is the second key challenge. It must read JSON from `spec/vectors/*.json`, interpret special fields (`store_closed`, `payload_size_bytes`, `setup`), and map `error_kind` strings to Go error values. Use table-driven tests with `t.TempDir()` for isolation.

**Primary recommendation:** Use `database/sql` with `SetMaxOpenConns(1)` and `RegisterConnectionHook` for PRAGMA setup. Implement the Store struct as a thin wrapper around `*sql.DB` with a `closed` bool protected by a mutex. Run test vectors as table-driven subtests using `t.Run(fmt.Sprintf("TC%d", tc.TCID), ...)`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `modernc.org/sqlite` | v1.47.0 | Pure-Go SQLite driver for `database/sql` | Locked by GO-07; CGo-free, no C toolchain required |
| `database/sql` | stdlib | Connection pool, transaction, query API | Standard Go idiom; all SQLite drivers implement it |
| `encoding/json` | stdlib | Loading test vectors | Universal; no external dep needed for JSON |
| `encoding/base64` | stdlib | Decoding base64 payloads in test vectors | RFC 4648 standard base64 |
| `testing` | stdlib | Test framework | `go test` is the mandated runner (GO-06) |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os` | stdlib | File path checks, TempDir in tests | Open() parent-dir validation |
| `sync` | stdlib | Mutex for closed-store guard | Thread-safe idempotent Close() |
| `time` | stdlib | Nanosecond timestamps | `time.Now().UnixNano()` in Append/AppendBatch |
| `errors` | stdlib | Sentinel error vars | ErrPayloadTooLarge etc. |
| `fmt` | stdlib | Error wrapping | Storage error construction |
| `path/filepath` | stdlib | Parent directory check | `filepath.Dir(path)` in Open() |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `modernc.org/sqlite` | `mattn/go-sqlite3` | go-sqlite3 requires CGo; violates GO-07 |
| `modernc.org/sqlite` | `glebarez/go-sqlite` | Also modernc-based; same driver, less adoption |
| `modernc.org/sqlite` | `zombiezen.com/go/sqlite` | Does not use database/sql; richer API but non-standard |
| `database/sql` | `zombiezen.com/go/sqlite` direct | SetDefensive() available but non-sql interface; adds dependency |

**Installation:**
```bash
go mod init github.com/example/cairn   # or whatever the module path is
cd go
go get modernc.org/sqlite@v1.47.0
```

---

## Architecture Patterns

### Recommended Project Structure

```
go/
├── cairn.go           # Public API: Store type, Open, Close, Append, AppendBatch, Query
├── cairn_test.go      # Unit tests and test vector harness
├── errors.go          # Sentinel error variables (ErrPayloadTooLarge, etc.)
├── go.mod             # module github.com/..../cairn/go (or project path)
└── go.sum
```

All public API lives in a single package `cairn`. No sub-packages needed for this surface area. The test vector files live in `spec/vectors/` at the repo root; tests reference them with a relative path or via `os.ReadFile`.

### Pattern 1: Store Type with Closed Guard

**What:** The Store is a struct wrapping `*sql.DB` plus a closed flag. Mutex ensures Close() is safe to call concurrently or multiple times.
**When to use:** All public operations check the closed flag first.

```go
// Source: idiomatic Go database pattern
type Store struct {
    db     *sql.DB
    mu     sync.Mutex
    closed bool
}

func (s *Store) checkOpen() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.closed {
        return ErrStoreNotOpen
    }
    return nil
}
```

### Pattern 2: modernc.org/sqlite with RegisterConnectionHook

**What:** Use the package-level `RegisterConnectionHook` to set PRAGMAs on every connection the driver opens. This is the correct way to ensure WAL mode, busy_timeout, and foreign_keys are applied.
**When to use:** Called once at init time or in Open() before `sql.Open`.

```go
// Source: modernc.org/sqlite docs + https://theitsolutions.io/blog/modernc.org-sqlite-with-go
import (
    "database/sql"
    _ "modernc.org/sqlite"
    "modernc.org/sqlite"
)

func init() {
    sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
        _, err := conn.ExecContext(context.Background(), `
            PRAGMA journal_mode = WAL;
            PRAGMA busy_timeout = 5000;
            PRAGMA foreign_keys = ON;
        `, nil)
        return err
    })
}
```

**CAUTION:** `RegisterConnectionHook` is a global registration. For a library, avoid calling it in `init()` — it would affect all users of the sqlite driver in the same binary. Instead, use the DSN `_pragma` query parameter approach or open connections with explicit per-connection PRAGMA execution immediately after `sql.Open`.

The safer pattern for a library is to execute PRAGMAs explicitly in Open() via `db.ExecContext`:

```go
// Source: API spec + database/sql stdlib
func Open(path string) (*Store, error) {
    // Validate parent directory exists
    dir := filepath.Dir(path)
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        return nil, fmt.Errorf("cairn: parent directory does not exist: %w", err)
    }

    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, fmt.Errorf("cairn: open database: %w", err)
    }

    // SQLite is single-writer; one connection avoids SQLITE_BUSY contention
    db.SetMaxOpenConns(1)

    // Apply per-connection PRAGMAs
    ctx := context.Background()
    if _, err := db.ExecContext(ctx, `
        PRAGMA journal_mode = WAL;
        PRAGMA busy_timeout = 5000;
        PRAGMA foreign_keys = ON;
    `); err != nil {
        db.Close()
        return nil, fmt.Errorf("cairn: configure pragmas: %w", err)
    }

    // Execute schema DDL (idempotent via IF NOT EXISTS)
    if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
        db.Close()
        return nil, fmt.Errorf("cairn: init schema: %w", err)
    }

    return &Store{db: db}, nil
}
```

### Pattern 3: SQLITE_DBCONFIG_DEFENSIVE in modernc.org/sqlite

**What:** The spec requires SQLITE_DBCONFIG_DEFENSIVE (STOR-03). modernc.org/sqlite does NOT expose a clean `SetDefensive()` method. It exposes `Xsqlite3_db_config` in `modernc.org/sqlite/lib`, but calling it requires the raw C TLS and connection pointer — these are not accessible through the `database/sql` interface.

**Practical approach:** Use `db.Conn(ctx)` to get a `*sql.Conn`, then use `conn.Raw(func(c interface{}) error {...})` to access the underlying driver connection. However, the raw conn from modernc.org/sqlite is a `driver.Conn`, not a struct with exported fields for TLS/db pointers.

**Best available approach:** The test vectors validate immutability via the no_update and no_delete triggers (which do not require SQLITE_DBCONFIG_DEFENSIVE). For the SDK, execute `PRAGMA trusted_schema = OFF` as a defense-in-depth measure (available via SQL, prevents loading untrusted schemas). Document that full SQLITE_DBCONFIG_DEFENSIVE is not directly accessible via the database/sql interface without a custom driver wrapper.

**Alternative:** Use `zombiezen.com/go/sqlite` as a thin helper just for the defensive call — but this adds a dependency. The simpler path: apply it via PRAGMA where possible, document the limitation.

**Confidence:** MEDIUM — the API spec says "Use `db.SetDefensive(true)` on the underlying connection" for Go, but this method only exists on `zombiezen.com/go/sqlite.Conn`, not on `modernc.org/sqlite`. Needs implementation-time validation. The PRAGMA `trusted_schema = OFF` plus the immutability triggers provides the functional guarantee required by the test suite.

### Pattern 4: Append and AppendBatch

```go
// Source: spec/api.md Append behavior
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
    return uint64(id), nil
}
```

### Pattern 5: Query with rows.Close() Discipline

**Critical:** Failing to call `rows.Close()` blocks WAL checkpoint. Always use `defer rows.Close()` immediately after `db.QueryContext`.

```go
// Source: spec/api.md Query behavior + turso.tech SQLite Go pitfalls article
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
    return events, nil
}
```

### Pattern 6: Test Vector Harness

**What:** Table-driven tests that load JSON from `spec/vectors/*.json`, decode base64 payloads and timestamp strings, and assert outcomes.

```go
// Source: encoding/json + encoding/base64 stdlib; test vector format from spec/vectors/
type appendTest struct {
    TCID    int    `json:"tc_id"`
    Comment string `json:"comment"`
    Input   struct {
        Topic            string `json:"topic"`
        Payload          string `json:"payload"`           // base64, may be empty string
        PayloadSizeBytes int    `json:"payload_size_bytes"` // if nonzero, generate synthetic payload
        StoreClosed      bool   `json:"store_closed"`
    } `json:"input"`
    Expected struct {
        Result      string `json:"result"`    // "valid" or "invalid"
        ErrorKind   string `json:"error_kind"` // snake_case error name
        EventIDType string `json:"event_id_type"`
        EventIDMin  string `json:"event_id_min"` // quoted uint64
    } `json:"expected"`
}

func errorKindMatches(err error, kind string) bool {
    switch kind {
    case "payload_too_large":   return errors.Is(err, ErrPayloadTooLarge)
    case "empty_topic":         return errors.Is(err, ErrEmptyTopic)
    case "empty_payload":       return errors.Is(err, ErrEmptyPayload)
    case "store_not_open":      return errors.Is(err, ErrStoreNotOpen)
    case "immutability_violation": return strings.Contains(err.Error(), "cairn: updates not allowed") ||
                                          strings.Contains(err.Error(), "cairn: deletes not allowed")
    }
    return false
}
```

**Test isolation:** Use `t.TempDir()` to get an auto-cleaned temp directory per test.

```go
func openTestStore(t *testing.T) *Store {
    t.Helper()
    dir := t.TempDir()
    s, err := Open(filepath.Join(dir, "test.db"))
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    t.Cleanup(func() { s.Close() })
    return s
}
```

**Vector file loading pattern:**

```go
func loadVectors(t *testing.T, relPath string) []byte {
    t.Helper()
    // relPath from repo root, e.g. "spec/vectors/append.json"
    data, err := os.ReadFile(filepath.Join("..", relPath)) // adjust depth
    if err != nil {
        t.Fatalf("load vectors %s: %v", relPath, err)
    }
    return data
}
```

**Immutability harness** must get a raw SQL connection from the open store to execute UPDATE/DELETE. Use `db.ExecContext` directly on `s.db` in the test (expose it as a package-internal field or use `_test` package access pattern).

### Anti-Patterns to Avoid

- **Calling RegisterConnectionHook in a library init():** Global registration pollutes any binary importing the library. Use explicit ExecContext PRAGMAs in Open() instead.
- **Not calling rows.Close():** A single leaked rows object prevents WAL checkpoint, causing unbounded WAL growth. Always `defer rows.Close()`.
- **sql.Open returning an error on bad path:** `sql.Open` with modernc.org/sqlite does NOT actually open the file — it creates a pool. The actual connection (and file open) happens on first use. Validate the parent directory with `os.Stat` before `sql.Open`.
- **Using int64 for EventID in the public API:** The spec defines EventID as uint64. SQLite rowids fit in int64, but the conversion to uint64 is safe since IDs are always positive. The public API returns `uint64`.
- **Timestamps as int64 in test assertions:** Test vector timestamps arrive as quoted strings. Parse with `strconv.ParseInt(s, 10, 64)` — do not assume JSON number.
- **AppendBatch opening a transaction for empty input:** The spec says return empty slice immediately with no transaction. Check `len(events) == 0` before `db.Begin`.
- **Missing rows.Err() check after rows.Next() loop:** Silent data loss if the iteration was interrupted by a network/IO error. Always check `rows.Err()`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQL connection management | Custom connection pool | `database/sql` pool with `SetMaxOpenConns(1)` | database/sql handles pool lifecycle, reconnect, idle timeout |
| Base64 decode in tests | Custom decoder | `encoding/base64.StdEncoding.DecodeString` | RFC 4648 standard; stdlib matches spec requirement |
| JSON test vector parsing | Custom parser | `encoding/json.Unmarshal` | Universal; handles all field types needed |
| Temp directory management | `os.MkdirTemp` + defer cleanup | `t.TempDir()` | Auto-cleanup on test completion/failure |
| SQLite schema migration | Version table + migration runner | Idempotent `CREATE ... IF NOT EXISTS` DDL | v1 has one schema; IF NOT EXISTS makes every Open() safe |
| Mutex-based open/close | Channel select | `sync.Mutex` + bool field | Simple, idiomatic, no goroutine overhead |

**Key insight:** The entire public API surface is 5 operations against a single SQLite file. The complexity budget should be spent on correctness (spec compliance) not infrastructure.

---

## Common Pitfalls

### Pitfall 1: rows.Close() and WAL Checkpoint
**What goes wrong:** Query returns rows, caller doesn't close them (or test helper doesn't call `rows.Close()`). When Close() runs `PRAGMA wal_checkpoint(TRUNCATE)`, it silently returns without checkpointing because an active reader blocks it.
**Why it happens:** `database/sql` rows are closed lazily; if you forget `defer rows.Close()`, the rows stay open until GC.
**How to avoid:** Always `defer rows.Close()` immediately after `db.QueryContext(...)`. Add a test that verifies WAL file size decreases after Close() — uses `os.Stat` on the `.db-wal` file.
**Warning signs:** `.db-wal` file remains after store.Close() in tests.

### Pitfall 2: sql.Open Does Not Open the File
**What goes wrong:** `Open(path)` calls `sql.Open("sqlite", path)` and returns success even if the parent directory doesn't exist. The error surfaces on the first actual operation (schema execution), not at Open time.
**Why it happens:** `sql.Open` only validates the driver name and DSN format; it does not connect.
**How to avoid:** Check `os.Stat(filepath.Dir(path))` before `sql.Open`. Return a storage error if the directory doesn't exist.
**Warning signs:** The append test vector TC7 (`store_closed`) passes but no vector tests for missing parent directory.

### Pitfall 3: SQLITE_DBCONFIG_DEFENSIVE Not Accessible via database/sql
**What goes wrong:** The spec says to use `db.SetDefensive(true)` for Go. This method exists on `zombiezen.com/go/sqlite.Conn`, NOT on `modernc.org/sqlite`'s database/sql connection. Attempting to use it causes a compile error.
**Why it happens:** The spec was written referencing zombiezen's API, which wraps modernc under the hood.
**How to avoid:** Use `PRAGMA trusted_schema = OFF` as a partial substitute via ExecContext. For the test suite to pass, the triggers (installed by schema.sql) provide the actual immutability enforcement — defensive mode is belt-and-suspenders. Document the limitation in code comments.
**Warning signs:** Compile error on `db.SetDefensive` — method not found.

### Pitfall 4: AppendBatch Validation Order
**What goes wrong:** Validation loop checks `EmptyPayload` before `PayloadTooLarge`, but the spec defines error precedence: EmptyTopic > EmptyPayload > PayloadTooLarge. An event with both EmptyPayload and PayloadTooLarge (impossible by definition, but worth noting the precedence chain) must return the first error by spec priority.
**Why it happens:** The spec defines precedence explicitly for AppendBatch (section "Error Precedence in AppendBatch") but it's easy to implement the loop in the wrong order.
**How to avoid:** Validate in the spec-defined order for each event: topic empty → payload empty → payload too large. Batch vector TC5 (empty topic, batch rejected) verifies this.
**Warning signs:** Batch TC4 returns wrong error_kind.

### Pitfall 5: Timestamp as int64 vs uint64
**What goes wrong:** `time.Now().UnixNano()` returns `int64`. SQLite stores as INTEGER (signed 64-bit). The public EventID is defined as `uint64`. LastInsertId() returns `int64`. Casting `int64(-1)` to uint64 gives `18446744073709551615` — an absurd value if something goes wrong.
**Why it happens:** Go/SQL boundary involves int64 everywhere; the spec uses uint64 for EventID.
**How to avoid:** After `LastInsertId()`, validate `id > 0` before casting to uint64. Return a storage error if id <= 0 (should never happen with a healthy SQLite but worth guarding).
**Warning signs:** Tests pass with positive IDs but no guard against negative.

### Pitfall 6: Test Vector Path Resolution
**What goes wrong:** Test files live in `go/cairn_test.go` but spec vectors are at `spec/vectors/` (repo root). Relative path `../../spec/vectors/append.json` depends on the working directory being `go/`. If `go test` is run from a different directory, the path breaks.
**Why it happens:** `go test` sets the working directory to the package directory, so `../../spec/vectors/` works from `go/`, but only if the `go/` directory is exactly 2 levels below the repo root.
**How to avoid:** Use `go:embed` directive to embed the vector files at build time:
```go
//go:embed ../spec/vectors
var vectorFS embed.FS
```
Or detect the repo root via `os.Getwd()` + walking up to find `spec/`. The `go:embed` approach is cleanest and avoids path dependency.
**Warning signs:** Tests pass with `go test ./go/...` from repo root but fail when run differently.

---

## Code Examples

Verified patterns from official sources and project spec:

### errors.go — Sentinel Errors
```go
// Source: spec/api.md Error Catalog (Go idiomatic naming)
package cairn

import "errors"

var (
    ErrPayloadTooLarge      = errors.New("cairn: payload too large")
    ErrStoreNotOpen         = errors.New("cairn: store not open")
    ErrImmutabilityViolation = errors.New("cairn: immutability violation")
    ErrEmptyTopic           = errors.New("cairn: empty topic")
    ErrEmptyPayload         = errors.New("cairn: empty payload")
)

const MaxPayloadSize = 1_048_576
```

### Event Struct
```go
// Source: spec/api.md Concepts
package cairn

type Event struct {
    ID      uint64
    Topic   string
    TS      int64  // nanoseconds since Unix epoch
    Payload []byte
}

type BatchEvent struct {
    Topic   string
    Payload []byte
}
```

### Close() with WAL Checkpoint
```go
// Source: spec/api.md Close behavior
func (s *Store) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.closed {
        return nil // idempotent
    }
    s.closed = true

    if _, err := s.db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
        // Log but don't fail — spec says Close is always safe
        _ = err
    }
    return s.db.Close()
}
```

### AppendBatch — All-or-Nothing
```go
// Source: spec/api.md AppendBatch behavior
func (s *Store) AppendBatch(events []BatchEvent) ([]uint64, error) {
    if err := s.checkOpen(); err != nil {
        return nil, err
    }
    if len(events) == 0 {
        return []uint64{}, nil // no transaction, no error
    }

    // Validate all before inserting any
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
        id, _ := result.LastInsertId()
        ids = append(ids, uint64(id))
    }

    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("cairn: commit: %w", err)
    }
    return ids, nil
}
```

### go.mod Structure
```
module github.com/<owner>/cairn/go

go 1.22

require modernc.org/sqlite v1.47.0
```

Note: The module path for the Go SDK is a sub-module within the monorepo. The `go/` directory contains its own `go.mod`. Module path should match the project's actual repository path.

### embed approach for test vectors
```go
// In cairn_test.go — avoids working-directory path fragility
//go:embed ../../spec/vectors
var vectorFiles embed.FS

func readVector(t *testing.T, name string) []byte {
    t.Helper()
    data, err := vectorFiles.ReadFile("spec/vectors/" + name)
    if err != nil {
        t.Fatalf("read vector %s: %v", name, err)
    }
    return data
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mattn/go-sqlite3` (CGo) | `modernc.org/sqlite` (pure Go) | Ongoing preference shift | `go build` works without C toolchain; cross-compilation is trivial |
| `sql.Open` with no PRAGMA setup | `RegisterConnectionHook` or explicit ExecContext PRAGMAs | Best practice established ~2022 | WAL mode, busy_timeout, foreign_keys applied consistently on every connection |
| `os.MkdirTemp` + `defer os.RemoveAll` | `t.TempDir()` | Go 1.15 | Auto-cleanup on test failure; no manual defer needed |
| Manual `defer rows.Close()` reminder | Standard pattern; `go vet` can catch some cases | Always | Prevents WAL checkpoint stall |
| `int64` for EventID in public API | `uint64` per spec | Phase 1 spec | Matches spec type; prevents confusion with negative values |

**Deprecated/outdated:**
- `mattn/go-sqlite3`: CGo-based; violates GO-07. Do not use.
- `PRAGMA journal_mode = WAL` in schema.sql: Must be in Open() via connection-level PRAGMA, not in the DDL file.

---

## Open Questions

1. **SQLITE_DBCONFIG_DEFENSIVE via database/sql**
   - What we know: `modernc.org/sqlite` exposes `Xsqlite3_db_config` in the `lib` sub-package, and `RegisterConnectionHook` gives an `ExecQuerierContext`. The hook does NOT provide access to the raw C-level connection pointer needed for `Xsqlite3_db_config`.
   - What's unclear: Whether `conn.Raw()` via `*sql.Conn` can reach the underlying `modernc.org/sqlite` driver connection to call `Xsqlite3_db_config(tls, db, SQLITE_DBCONFIG_DEFENSIVE, 1)`.
   - Recommendation: At implementation time, try `db.Conn(ctx)` + `conn.Raw(func(c any) error { ... })` and check if the raw conn exposes the needed pointer. If not, fall back to `PRAGMA trusted_schema = OFF` and document the limitation. The spec test suite does not directly test for defensive mode — it tests trigger behavior, which works without it.

2. **Module path for the Go SDK**
   - What we know: The project has no existing Go files; the module path is not yet defined.
   - What's unclear: Should it be `github.com/<owner>/cairn/go` (sub-module path) or something else?
   - Recommendation: Use `github.com/<owner>/cairn/go` matching the repository path. Go convention for mono-repos with multiple language SDKs uses sub-module paths. The directory `go/` in the repo root contains its own `go.mod`.

3. **go:embed path depth**
   - What we know: Vector files are at `spec/vectors/` relative to repo root. The Go SDK will be in `go/`.
   - What's unclear: `go:embed` uses paths relative to the Go source file. From `go/cairn_test.go`, the path would be `../spec/vectors` — but `go:embed` cannot embed files outside the module root.
   - Recommendation: If the module root is `go/`, embedding `../spec/` is not possible. Instead, use `os.ReadFile` with a path constructed from the test's `os.Getwd()` walking up to the repo root. Alternatively, copy vectors into `go/testdata/` (symlink or copy in CI). This needs resolution at implementation time.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package |
| Config file | None — `go test` requires no config file |
| Quick run command | `go test ./...` (from `go/` directory) |
| Full suite command | `go test -v -count=1 ./...` (from `go/` directory) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GO-01 | Open creates/opens store, sets WAL+busy_timeout+FK, runs schema DDL | unit | `go test -run TestOpen ./...` | ❌ Wave 0 |
| GO-01 | Open fails if parent directory missing | unit | `go test -run TestOpen_MissingDir ./...` | ❌ Wave 0 |
| GO-02 | Close checkpoints WAL, is idempotent | unit | `go test -run TestClose ./...` | ❌ Wave 0 |
| GO-03 | Append vectors (all 7 cases from append.json) | unit/vector | `go test -run TestAppendVectors ./...` | ❌ Wave 0 |
| GO-04 | AppendBatch vectors (all 6 cases from batch.json) | unit/vector | `go test -run TestBatchVectors ./...` | ❌ Wave 0 |
| GO-05 | Query vectors (all 6 cases from query.json) | unit/vector | `go test -run TestQueryVectors ./...` | ❌ Wave 0 |
| GO-06 | All 4 vector files pass | integration/vector | `go test -run TestVectors ./...` | ❌ Wave 0 |
| GO-06 | Immutability vectors (2 cases from immutability.json) | unit/vector | `go test -run TestImmutabilityVectors ./...` | ❌ Wave 0 |
| GO-07 | Build succeeds without C toolchain | build | `CGO_ENABLED=0 go build ./...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./...`
- **Per wave merge:** `CGO_ENABLED=0 go build ./...` + `go test -v -count=1 ./...`
- **Phase gate:** All vector tests green + `CGO_ENABLED=0 go build ./...` passes before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `go/go.mod` — declares module and modernc.org/sqlite dependency
- [ ] `go/cairn.go` — Store type, Open, Close, Append, AppendBatch, Query
- [ ] `go/errors.go` — sentinel error variables
- [ ] `go/cairn_test.go` — test vector harness + unit tests
- [ ] Resolution of `go:embed` vs `os.ReadFile` for vector file access (Open Question 3)

---

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/modernc.org/sqlite v1.47.0](https://pkg.go.dev/modernc.org/sqlite) — driver name "sqlite", RegisterConnectionHook, ConnectionHookFn, ExecQuerierContext, current version
- [pkg.go.dev/modernc.org/sqlite/lib](https://pkg.go.dev/modernc.org/sqlite/lib) — Xsqlite3_db_config function, SQLITE_DBCONFIG_DEFENSIVE constant presence confirmed
- [spec/api.md](../../../spec/api.md) — authoritative contract for all SDK behavior
- [spec/schema.sql](../../../spec/schema.sql) — DDL executed verbatim on every Open()
- [spec/vectors/*.json](../../../spec/vectors/) — test cases the harness must implement
- [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) — official Go module layout for libraries
- [pkg.go.dev/database/sql](https://pkg.go.dev/database/sql) — sql.DB, sql.Conn, BeginTx, ExecContext, QueryContext, LastInsertId

### Secondary (MEDIUM confidence)
- [theitsolutions.io/blog/modernc.org-sqlite-with-go](https://theitsolutions.io/blog/modernc.org-sqlite-with-go) — RegisterConnectionHook usage pattern with ExecQuerierContext for multi-PRAGMA init; verified against pkg.go.dev API
- [turso.tech SQLite Go pitfalls](https://turso.tech/blog/something-you-probably-want-to-know-about-if-youre-using-sqlite-in-golang-72547ad625f1) — rows.Close() discipline critical for WAL checkpoint
- [zombiezen.com/go/sqlite SetDefensive implementation](https://github.com/zombiezen/go-sqlite/blob/main/sqlite.go) — confirms SetDefensive uses lib.Xsqlite3_db_config + lib.SQLITE_DBCONFIG_DEFENSIVE; shows the approach for direct-conn libraries; confirmed modernc.org/sqlite/lib has these symbols

### Tertiary (LOW confidence)
- WebSearch pattern: `db.SetMaxOpenConns(1)` for SQLite single-writer — broadly cited across multiple sources, consistent with SQLite single-writer model; not from a single canonical source
- SQLITE_DBCONFIG_DEFENSIVE accessibility via `conn.Raw()` — not verified; marked as Open Question 1

---

## Metadata

**Confidence breakdown:**
- Standard stack (modernc.org/sqlite version, database/sql patterns): HIGH — from official pkg.go.dev
- Architecture (Store struct, Open/Close/Append/Query patterns): HIGH — derived from spec/api.md + stdlib docs
- SQLITE_DBCONFIG_DEFENSIVE access path: MEDIUM — lib constants confirmed, exact invocation via database/sql not verified
- Test vector harness design: HIGH — based on actual vector file structure + stdlib encoding/json
- go:embed path resolution: LOW — needs implementation-time testing; Open Question 3

**Research date:** 2026-03-18
**Valid until:** 2026-09-18 (modernc.org/sqlite is stable; stdlib patterns don't change; spec is frozen)
