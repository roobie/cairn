---
phase: 02-go-sdk
plan: 01
subsystem: database
tags: [go, sqlite, modernc-sqlite, wal, database-sql]

# Dependency graph
requires:
  - phase: 01-spec-and-schema
    provides: spec/schema.sql DDL and spec/api.md error contracts

provides:
  - cairn.dev/sdk/go Go module with modernc.org/sqlite v1.47.0
  - Store type with Open/Close lifecycle (WAL, PRAGMAs, schema DDL)
  - Sentinel error variables (ErrPayloadTooLarge, ErrStoreNotOpen, ErrImmutabilityViolation, ErrEmptyTopic, ErrEmptyPayload)
  - MaxPayloadSize constant (1_048_576)
  - Event and BatchEvent types
  - Append/AppendBatch/Query stubs for Plan 02

affects:
  - 02-go-sdk/02-02 (builds Append/AppendBatch/Query on top of this Store)
  - 03-ts-sdk (reference implementation pattern for TypeScript port)
  - 03-rust-sdk (reference implementation pattern for Rust port)

# Tech tracking
tech-stack:
  added:
    - modernc.org/sqlite v1.47.0 (pure Go SQLite driver, no CGo)
    - database/sql (stdlib connection pool)
    - sync.Mutex (closed-store idempotency guard)
  patterns:
    - Store struct wraps *sql.DB with closed bool + sync.Mutex
    - Open validates parent dir with os.Stat before sql.Open (sql.Open is lazy)
    - PRAGMAs applied via db.ExecContext in Open (not RegisterConnectionHook — avoids global state in library)
    - schemaSQL const embeds spec/schema.sql verbatim as raw string literal
    - SQLITE_DBCONFIG_DEFENSIVE attempted via conn.Raw(), falls back to PRAGMA trusted_schema = OFF
    - Close locks mutex, sets closed=true, checkpoints WAL (TRUNCATE), calls db.Close()
    - TDD: RED commit (failing tests) then GREEN commit (implementation)

key-files:
  created:
    - go/go.mod
    - go/go.sum
    - go/errors.go
    - go/cairn.go
    - go/cairn_test.go

key-decisions:
  - "Module path: cairn.dev/sdk/go (generic path matching project identity)"
  - "SQLITE_DBCONFIG_DEFENSIVE not accessible via database/sql conn.Raw() in modernc.org/sqlite — PRAGMA trusted_schema = OFF used as fallback; immutability triggers provide actual enforcement"
  - "PRAGMAs applied via db.ExecContext in Open(), not RegisterConnectionHook — avoids global driver pollution in a library context"
  - "schemaSQL embedded as const string, not go:embed (spec/schema.sql is outside the go/ module root; go:embed cannot traverse above module root)"
  - "Append/AppendBatch/Query left as stubs returning 'not implemented' — Plan 02 fills these in"

patterns-established:
  - "Pattern 1: Store type — *sql.DB + sync.Mutex + closed bool; all operations call checkOpen() before use"
  - "Pattern 2: Open — os.Stat parent dir first, then sql.Open, SetMaxOpenConns(1), ExecContext PRAGMAs, ExecContext schemaSQL"
  - "Pattern 3: Close — lock, idempotency check, set closed, checkpoint WAL, db.Close"
  - "Pattern 4: TDD flow — RED commit with failing tests, GREEN commit with implementation"

requirements-completed: [GO-01, GO-02, GO-07]

# Metrics
duration: 3min
completed: 2026-03-18
---

# Phase 2 Plan 01: Go SDK Foundation Summary

**Pure-Go cairn Store with Open/Close lifecycle using modernc.org/sqlite v1.47.0 — WAL mode, immutability triggers, and CGO_ENABLED=0 build verified**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-18T01:48:04Z
- **Completed:** 2026-03-18T01:51:00Z
- **Tasks:** 1 (TDD: 3 commits)
- **Files modified:** 5

## Accomplishments

- Go module `cairn.dev/sdk/go` initialized with `modernc.org/sqlite v1.47.0` — pure Go, no C toolchain required
- `Store` type with `Open`/`Close` implementing full spec lifecycle: WAL mode, busy_timeout, foreign_keys, schema DDL, WAL checkpoint on close
- All 5 sentinel errors and `MaxPayloadSize` constant matching spec/api.md exactly
- `Event` and `BatchEvent` structs matching spec type definitions
- 5 unit tests all green: Open, MissingDir, Idempotent, Close_Idempotent, WAL checkpoint
- `CGO_ENABLED=0 go build ./...` passes (pure Go)

## Task Commits

Each task was committed atomically (TDD pattern — three commits for one task):

1. **RED: Failing tests** - `4367129` (test)
2. **GREEN: cairn.go implementation** - `8754ffb` (feat)
3. **go mod tidy** - `6ea8a88` (chore)

## Files Created/Modified

- `go/go.mod` - Module declaration `cairn.dev/sdk/go`, requires `modernc.org/sqlite v1.47.0`
- `go/go.sum` - Dependency checksums
- `go/errors.go` - 5 sentinel errors + MaxPayloadSize constant
- `go/cairn.go` - Store type, Open, Close, Event, BatchEvent, stubs for Append/AppendBatch/Query
- `go/cairn_test.go` - 5 unit tests for Open/Close lifecycle

## Decisions Made

- **Module path `cairn.dev/sdk/go`:** Generic path matching project identity; research noted `github.com/<owner>/cairn/go` as alternative but owner slug is not defined yet.
- **PRAGMAs via `db.ExecContext` not `RegisterConnectionHook`:** `RegisterConnectionHook` is a global registration — calling it in a library's `init()` would pollute every binary importing the library. Explicit `ExecContext` in `Open()` is safer.
- **`schemaSQL` as `const` string:** `go:embed` cannot embed files above the module root (`../spec/schema.sql` is outside `go/`). Verbatim copy as const is the correct approach per the plan spec.
- **SQLITE_DBCONFIG_DEFENSIVE fallback:** `conn.Raw()` in modernc.org/sqlite gives a `driver.Conn` interface with no exported fields; cannot reach the C-level db pointer needed for `Xsqlite3_db_config`. `PRAGMA trusted_schema = OFF` used as fallback. The immutability triggers (`no_update`, `no_delete`) in schema.sql provide the actual enforcement the test suite validates.

## Deviations from Plan

None — plan executed exactly as written. The SQLITE_DBCONFIG_DEFENSIVE `conn.Raw()` attempt and fallback was specified in the plan and implemented as described.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Go module foundation complete; Plan 02 can build `Append`, `AppendBatch`, and `Query` directly on top of the `Store` type
- Test vector harness (GO-06) will be implemented in Plan 02
- `go:embed` path issue for `spec/vectors/` must be resolved in Plan 02 (noted in research Open Question 3): either use `os.ReadFile` with relative path or copy vectors to `go/testdata/`

---
*Phase: 02-go-sdk*
*Completed: 2026-03-18*
