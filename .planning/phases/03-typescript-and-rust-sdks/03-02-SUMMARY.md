---
phase: 03-typescript-and-rust-sdks
plan: 02
subsystem: database
tags: [rust, sqlite, rusqlite, bundled, cairn, sdk]

# Dependency graph
requires:
  - phase: 01-spec-and-schema
    provides: schema.sql DDL, test vectors, api.md spec
  - phase: 02-go-sdk
    provides: Go reference implementation patterns (cairn.go, cairn_test.go)
provides:
  - Rust cairn SDK crate (rs/) with open/close/append/append_batch/query
  - Integration test harness consuming all four spec vector files
  - cargo build with bundled SQLite (no system libsqlite3 required)
affects: [future-packaging, ci-integration, cross-language-vector-validation]

# Tech tracking
tech-stack:
  added:
    - rusqlite 0.38 (bundled feature — SQLite 3.x compiled from source)
    - serde_json 1 (dev-dep, flexible JSON parsing for vector harness)
    - base64 0.22 (dev-dep, vector payload decoding)
    - tempfile 3 (dev-dep, isolated test databases)
  patterns:
    - Store struct wrapping Option<Connection> to enable Drop-safe close
    - Error enum with kind() method returning spec error_kind strings
    - raw_conn() accessor (pub, #[doc(hidden)]) for integration test raw SQL
    - WAL checkpoint in Drop — errors silently discarded (never panic in Drop)
    - validate-all-before-insert in append_batch

key-files:
  created:
    - rs/Cargo.toml
    - rs/src/lib.rs
    - rs/tests/vectors.rs
  modified: []

key-decisions:
  - "Store.conn wrapped in Option<Connection> so Drop can take() ownership and close cleanly without double-free"
  - "pub raw_conn() method (doc hidden) exposes connection for integration tests — pub(crate) is not accessible from tests/ directory (separate crate boundary)"
  - "Store implements Debug manually (Connection has no Debug) — required by unwrap_err() in test assertions"
  - "rusqlite 0.38 resolved correctly with bundled feature; research estimate of 0.38 was accurate"

patterns-established:
  - "Pattern: Option<Connection> in Store — enables Drop to take() the connection without unsafe code"
  - "Pattern: validate-all-before-insert — all events checked before any SQL executed in append_batch"
  - "Pattern: now_nanos() helper casting u128 → i64 (valid until year 2262)"

requirements-completed: [RS-01, RS-02, RS-03, RS-04, RS-05, RS-06, RS-07]

# Metrics
duration: 3min
completed: 2026-03-18
---

# Phase 3 Plan 02: Rust SDK Summary

**rusqlite-bundled Rust SDK with Store/open/close/append/append_batch/query, Drop-based WAL checkpoint, and full spec vector harness (21 test cases, 4 vector files, 10 total tests passing)**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-18T02:31:08Z
- **Completed:** 2026-03-18T02:34:33Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Complete Rust SDK crate in `rs/` with bundled SQLite — no system `libsqlite3` required
- All 21 spec vector test cases passing: append (7), batch (6), query (6), immutability (2)
- Unit tests for open/close/Drop/WAL checkpoint, missing parent dir, idempotent close
- Error enum with `kind()` method returning spec `error_kind` strings for test harness matching
- `SQLITE_DBCONFIG_DEFENSIVE` enabled via `conn.set_db_config()` — direct C API, not PRAGMA fallback

## Task Commits

Each task was committed atomically:

1. **Task 1: Cargo crate, Store implementation, and Error enum** - `0ca32cc` (feat)
2. **Task 2: Test vector harness and full spec validation** - `1421a81` (feat)

**Plan metadata:** _(to be committed after SUMMARY.md)_

## Files Created/Modified

- `/home/jani/devel/cairn/rs/Cargo.toml` — cairn crate config with rusqlite 0.38 bundled + dev-deps
- `/home/jani/devel/cairn/rs/src/lib.rs` — Store struct, open(), Error enum, all methods, Drop impl, Debug impl
- `/home/jani/devel/cairn/rs/tests/vectors.rs` — Integration test harness for all 4 spec vector files plus unit tests

## Decisions Made

- `Store.conn` wrapped in `Option<Connection>` so `Drop` can call `take()` and own the connection for clean close without unsafe code or double-close issues.
- `pub raw_conn()` method (with `#[doc(hidden)]`) used instead of `pub(crate)` because integration tests in `tests/` are compiled as a separate crate and cannot access `pub(crate)` items.
- `Store` implements `Debug` manually because `rusqlite::Connection` doesn't implement `Debug`; required by Rust's `unwrap_err()` which has `T: Debug` bound.
- rusqlite `0.38` used as specified in research — resolved correctly with bundled feature.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Store::Debug and pub(crate) conn accessibility**
- **Found during:** Task 2 (test vector harness compilation)
- **Issue:** Plan said "make conn pub(crate)" but integration tests in `tests/` are a separate crate — `pub(crate)` is not accessible. Also `Store` needed `Debug` for `unwrap_err()`. Three compile errors.
- **Fix:** Added manual `impl Debug for Store`, added `pub fn raw_conn()` method with `#[doc(hidden)]`, updated tests to call `store.raw_conn()` instead of accessing `store.conn` directly.
- **Files modified:** rs/src/lib.rs, rs/tests/vectors.rs
- **Verification:** `cargo test` — all 10 tests pass
- **Committed in:** `1421a81` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — compile bug from pub(crate) vs test crate boundary)
**Impact on plan:** Necessary fix for Rust's crate visibility model. No scope creep. raw_conn() is doc-hidden to avoid polluting the public API.

## Issues Encountered

- `pub(crate)` on `Store.conn` is inaccessible from `tests/` (separate crate in Rust's module system). Fixed via public accessor with `#[doc(hidden)]`.

## User Setup Required

None — no external service configuration required. Bundled SQLite means no system dependencies.

## Next Phase Readiness

- Rust SDK complete and spec-compliant; all vector tests pass
- Phase 3 TypeScript SDK (03-01) builds in parallel — no dependency on this plan
- Both SDKs share the same spec vector files; cross-language consistency confirmed
- No blockers for Phase 4 (distribution/packaging)

---
*Phase: 03-typescript-and-rust-sdks*
*Completed: 2026-03-18*
