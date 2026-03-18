---
phase: 02-go-sdk
plan: 02
subsystem: database
tags: [go, sqlite, test-vectors, tdd, append, query]

# Dependency graph
requires:
  - phase: 02-go-sdk
    plan: 01
    provides: Store type with Open/Close lifecycle, sentinel errors, MaxPayloadSize

provides:
  - Append implementation (validation + INSERT + uint64 EventID)
  - AppendBatch implementation (all-or-nothing transaction, validation-before-insert)
  - Query implementation (topic+ts range filter, ORDER BY id ASC, rows.Close discipline)
  - Test vector harness covering all 4 spec vector files (21 cases)

affects:
  - 03-ts-sdk (Go is now the reference implementation for TypeScript port)
  - 03-rust-sdk (Go is now the reference implementation for Rust port)

# Tech tracking
tech-stack:
  added:
    - time.Now().UnixNano() (nanosecond timestamp source for Append/AppendBatch)
    - database/sql BeginTx (transactional AppendBatch)
    - database/sql QueryContext + rows.Close() (Query with WAL discipline)
    - encoding/base64 (test vector payload decoding)
    - encoding/json (test vector file unmarshaling)
    - strconv.ParseInt (quoted timestamp parsing from query vectors)
  patterns:
    - Append validates in order: checkOpen > EmptyTopic > EmptyPayload > PayloadTooLarge
    - AppendBatch validates ALL events before BeginTx (all-or-nothing validation)
    - defer tx.Rollback() after BeginTx (no-op after Commit)
    - defer rows.Close() immediately after QueryContext (WAL checkpoint discipline)
    - os.ReadFile("../spec/vectors/") resolves correctly when go test CWD is go/
    - TDD: RED commit (failing tests) then GREEN commit (implementation)

key-files:
  modified:
    - go/cairn.go
    - go/cairn_test.go

key-decisions:
  - "os.ReadFile('../spec/vectors/') preferred over go:embed — go:embed cannot traverse above module root; go test CWD is always the package dir (go/), making the relative path reliable"
  - "Query returns empty slice (not nil) when no events match — explicit if events == nil guard"
  - "AppendBatch empty input returns []uint64{} (not nil) immediately with no transaction — checked before validation loop"
  - "Immutability harness accesses s.db directly (same package) for raw SQL bypass — no internal/test-only accessor needed since tests are package cairn"

requirements-completed: [GO-03, GO-04, GO-05, GO-06]

# Metrics
duration: 3min
completed: 2026-03-18
---

# Phase 2 Plan 02: Data Operations and Test Vector Harness Summary

**Append, AppendBatch, Query implemented with full spec compliance; all 21 shared test vector cases pass across 4 vector files under `go test`**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-18T01:53:41Z
- **Completed:** 2026-03-18T01:56:39Z
- **Tasks:** 2 (Task 1 TDD: 2 commits; Task 2: 1 commit)
- **Files modified:** 2

## Accomplishments

- `Append` validates topic/payload in spec-defined precedence order, inserts with nanosecond timestamp, returns `uint64` EventID
- `AppendBatch` validates all events before any insert (all-or-nothing), uses `BeginTx`/`defer tx.Rollback()` for atomicity, returns `[]uint64{}` (not nil) for empty input
- `Query` filters by topic and inclusive time range `[start, end]`, `ORDER BY id ASC`, `defer rows.Close()` for WAL checkpoint discipline
- All 3 methods return `ErrStoreNotOpen` when the store is closed
- Test vector harness: 4 functions loading `append.json` (7 cases), `batch.json` (6 cases), `query.json` (6 cases), `immutability.json` (2 cases)
- Total test count: 47 (22 unit + 25 vector subtests), all green
- `CGO_ENABLED=0 go build ./...` passes — pure Go verified

## Task Commits

1. **RED: Failing tests for Append/AppendBatch/Query** - `f44db12` (test)
2. **GREEN: Append/AppendBatch/Query implementation** - `506032f` (feat)
3. **Test vector harness for all 4 spec vector files** - `c49ce78` (feat)

## Files Created/Modified

- `go/cairn.go` — `Append`, `AppendBatch`, `Query` replacing stubs; added `time` import
- `go/cairn_test.go` — `TestAppend`, `TestAppendBatch`, `TestQuery` unit tests; `TestAppendVectors`, `TestBatchVectors`, `TestQueryVectors`, `TestImmutabilityVectors` harness with helper functions

## Decisions Made

- **`os.ReadFile("../spec/vectors/")` not `go:embed`:** `go:embed` cannot embed files above the module root (`../spec/` is outside `go/`). The `go test` runner sets CWD to the package directory (`go/`), so `../spec/vectors/` resolves reliably. This matches the plan's direction and avoids the path fragility noted in research Open Question 3.
- **Empty Query returns `[]Event{}` not nil:** Spec says "empty result is valid and not an error." Explicitly guarding against nil return ensures callers can safely call `len()` without nil checks.
- **Immutability harness uses `s.db` directly:** Tests are in `package cairn` (not `_test` package), so the unexported `db` field is accessible. No internal/exported accessor needed.
- **Batch TC2 (empty slice):** JSON `[]` deserializes to `nil` slice in Go. Added explicit guard: `if tc.Input.Events != nil && batchEvents == nil { batchEvents = []BatchEvent{} }` to correctly pass an empty slice rather than nil to `AppendBatch`.

## Deviations from Plan

None — plan executed exactly as written. All spec vector test cases pass without deviation.

## Issues Encountered

None.

## User Setup Required

None.

## Next Phase Readiness

- Go SDK is complete: `Open`, `Append`, `AppendBatch`, `Query`, `Close` all functional and spec-compliant
- All 21 shared test vectors pass — this is the reference implementation for TypeScript (Phase 3) and Rust (Phase 3) ports
- Phase 3 (TypeScript + Rust SDKs) can use `go/cairn.go` as the idiom baseline and `go/cairn_test.go` as the harness pattern

## Self-Check: PASSED

- go/cairn.go: FOUND
- go/cairn_test.go: FOUND
- 02-02-SUMMARY.md: FOUND
- f44db12 (RED commit): FOUND
- 506032f (GREEN commit): FOUND
- c49ce78 (vector harness commit): FOUND

---
*Phase: 02-go-sdk*
*Completed: 2026-03-18*
