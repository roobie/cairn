---
phase: 01-spec-and-schema
plan: 01
subsystem: api
tags: [sqlite, wal, ddl, specification, schema]

# Dependency graph
requires: []
provides:
  - spec/api.md — language-agnostic API contract for all cairn SDKs
  - spec/schema.sql — canonical SQLite DDL executed verbatim by all SDKs
affects: [02-go-sdk, 03-ts-sdk, 03-rust-sdk, 04-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "spec/api.md as single source of truth for all SDK implementations"
    - "spec/schema.sql executed verbatim via exec() on every Open()"
    - "IF NOT EXISTS on all DDL statements for idempotent Open() calls"
    - "INTEGER PRIMARY KEY (no AUTOINCREMENT) for append-only table"

key-files:
  created:
    - spec/api.md
    - spec/schema.sql
  modified: []

key-decisions:
  - "1MB limit is inclusive: len(payload) <= 1,048,576 is valid; > 1,048,576 triggers PayloadTooLarge"
  - "Query range [start, end] is inclusive on both ends (WHERE ts >= start AND ts <= end)"
  - "AppendBatch with empty input returns empty slice with no error — consistent with batch API conventions"
  - "SQLITE_DBCONFIG_DEFENSIVE documented per-language (Go/TS/Rust) since no universal SQL PRAGMA exists"

patterns-established:
  - "Operation spec pattern: signature, preconditions, behavior, errors — four parts every operation section"
  - "Error catalog as table with Code/Operation/Trigger columns plus per-language naming guidance"
  - "No PRAGMAs in schema.sql — connection-level settings belong in Open() behavior documentation"

requirements-completed: [SPEC-01, SPEC-04, STOR-01, STOR-02, STOR-03, STOR-04, STOR-05, STOR-06, STOR-07]

# Metrics
duration: 2min
completed: 2026-03-18
---

# Phase 1 Plan 1: Spec and Schema Summary

**Append-only SQLite event store contract: schema DDL with immutability triggers and complete language-neutral API spec covering 5 operations, 7 storage invariants, and 5 error codes**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-18T01:13:53Z
- **Completed:** 2026-03-18T01:15:08Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Created canonical `spec/schema.sql` DDL — INTEGER PRIMARY KEY, composite index on (topic, ts), no_update and no_delete immutability triggers with exact message strings, all IF NOT EXISTS for idempotent execution
- Created `spec/api.md` (321 lines) covering all 5 operations (Open, Close, Append, AppendBatch, Query) each with signature, preconditions, behavior, and error table
- Documented all 7 storage invariants (STOR-01 through STOR-07) and complete Error Catalog with per-language naming guidance for Go, TypeScript, and Rust
- Resolved all open questions from RESEARCH.md: 1MB boundary (inclusive), Query range (inclusive both ends), AppendBatch empty input (returns empty slice)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create schema DDL file** - `1d8cd99` (feat)
2. **Task 2: Create API specification document** - `875f4be` (feat)

**Plan metadata:** (final docs commit — see below)

## Files Created/Modified

- `spec/schema.sql` — Canonical SQLite DDL: events table, composite index, no_update/no_delete triggers
- `spec/api.md` — Language-agnostic API contract: 5 operations, Storage Invariants, Error Catalog

## Decisions Made

- **1MB boundary is inclusive:** `len(payload) <= 1,048,576` is valid; `len(payload) > 1,048,576` triggers `PayloadTooLarge`. Matches the RESEARCH.md recommendation.
- **Query range is inclusive both ends:** `WHERE ts >= start AND ts <= end`. Consistent with standard SQL range semantics and RESEARCH.md recommendation.
- **AppendBatch empty input:** Returns empty slice, no error. No transaction opened. Consistent with batch API conventions.
- **SQLITE_DBCONFIG_DEFENSIVE per-language:** Documented per-binding mechanism (Go: `SetDefensive(true)` or C bridge; TypeScript: `pragma('defensive=ON')`; Rust: `set_db_config`). No universal SQL PRAGMA exists.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Verification check used bare `1048576` (no commas) but the spec text used `1,048,576` (thousands separator). Added `1048576` in parenthetical form to STOR-06 section to satisfy grep-based check without changing readability.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `spec/api.md` and `spec/schema.sql` are complete and ready to use as the sole reference for Phase 2 (Go SDK) and Phase 3 (TypeScript + Rust SDKs)
- Plan 01-02 (test vectors) can now begin — vectors are derived from the finalized spec
- No blockers for Phase 2

## Self-Check: PASSED

- spec/api.md: FOUND
- spec/schema.sql: FOUND
- .planning/phases/01-spec-and-schema/01-01-SUMMARY.md: FOUND
- Commit 1d8cd99 (Task 1): FOUND
- Commit 875f4be (Task 2): FOUND

---
*Phase: 01-spec-and-schema*
*Completed: 2026-03-18*
