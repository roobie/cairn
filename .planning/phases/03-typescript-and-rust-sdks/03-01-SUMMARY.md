---
phase: 03-typescript-and-rust-sdks
plan: "01"
subsystem: sdk
tags: [typescript, better-sqlite3, vitest, tsdown, bigint, sqlite, wasm]

requires:
  - phase: 02-go-sdk
    provides: "Go reference implementation patterns (Store, Open, Close, Append, AppendBatch, Query) and spec test vectors to mirror"
  - phase: 01-spec-and-schema
    provides: "spec/schema.sql DDL, spec/vectors/*.json test vectors, spec/api.md contract"

provides:
  - "TypeScript SDK in ts/ with Store class, open/close/append/appendBatch/query"
  - "Dual ESM/CJS output (index.mjs + index.cjs) with .d.mts/.d.cts declarations via tsdown"
  - "All 21 spec test vector cases passing under vitest"
  - "BigInt nanosecond timestamps via db.defaultSafeIntegers(true)"
  - "Five error classes with kind discriminator property"

affects: [04-documentation, ci-setup]

tech-stack:
  added:
    - better-sqlite3@12.8.0 (synchronous SQLite bindings for Node.js)
    - "@types/better-sqlite3@7.6.x (TypeScript declarations)"
    - tsdown@0.10.2 (Rolldown-based dual ESM/CJS bundler)
    - vitest@3.x (test runner)
    - typescript@5.x (compiler)
  patterns:
    - "db.defaultSafeIntegers(true) at open time ensures ALL integers are BigInt — no per-statement config"
    - "process.hrtime.bigint() for nanosecond timestamps"
    - "Prepared statements in Store constructor for insert and query performance"
    - "_db getter for test harness raw SQL access (prefixed _ signals internal use)"
    - "import.meta.url + fileURLToPath for ESM-safe vector file path resolution"
    - "BigInt(stringValue) for timestamp parsing — never Number()"

key-files:
  created:
    - ts/src/index.ts
    - ts/src/errors.ts
    - ts/tests/vectors.test.ts
    - ts/package.json
    - ts/tsconfig.json
    - ts/tsdown.config.ts
    - ts/vitest.config.ts
    - ts/.gitignore
  modified: []

key-decisions:
  - "Removed type:module from package.json so tsdown produces index.mjs/index.cjs with explicit extensions (tsdown uses .js when type:module, .mjs when not set)"
  - "Added outExtensions to tsdown config to force .cjs for CJS and .mjs for ESM output regardless of package.json type field"
  - "Used _db getter (not private) to expose underlying Database for test harness raw SQL — prefix signals internal use"
  - "store counter using process.pid + incrementing number for temp store uniqueness in parallel vitest workers"
  - "tsdown v0.10 produces index.d.mts + index.d.cts dual declarations instead of single index.d.ts — package.json exports updated accordingly"

patterns-established:
  - "TypeScript SDK mirrors Go reference pattern exactly: Store class wrapping db, checkOpen() guard, prepared statements in constructor"
  - "Error classes use readonly kind property for discriminated union matching — assertErrorKind uses kind not instanceof"
  - "Vitest tests use import.meta.url for ESM vector file resolution, not __dirname"
  - "Raw SQL test setup inserts with BigInt timestamps — query vector tests bypass append() to control exact ts values"

requirements-completed: [TS-01, TS-02, TS-03, TS-04, TS-05, TS-06, TS-07]

duration: 4min
completed: 2026-03-18
---

# Phase 3 Plan 1: TypeScript SDK Summary

**TypeScript cairn SDK with Store class (open/close/append/appendBatch/query), BigInt nanosecond timestamps via better-sqlite3 safeIntegers, dual ESM/CJS build via tsdown, and all 26 tests (21 spec vectors + 5 unit tests) passing under vitest**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-18T02:31:09Z
- **Completed:** 2026-03-18T02:35:00Z
- **Tasks:** 2
- **Files created:** 8

## Accomplishments

- Full Store class with all five operations: open(), close(), append(), appendBatch(), query()
- All 21 spec test vector cases pass: append (7 TCs), batch (6 TCs), query (6 TCs), immutability (2 TCs)
- 5 unit tests for open/close lifecycle (WAL mode, idempotent close, WAL checkpoint, missing dir error)
- Dual ESM/CJS build produces index.mjs + index.cjs + index.d.mts + index.d.cts via tsdown
- BigInt timestamps throughout — zero Number() conversion paths, no precision loss

## Task Commits

1. **Task 1: Project scaffolding, Store class, and error types** - `68aeff6` (feat)
2. **Task 2: Test vector harness and full spec validation** - `03fe3fd` (feat)

**Plan metadata:** (docs commit — see final commit)

## Files Created

- `ts/src/index.ts` - Store class with open/close/append/appendBatch/query, BigInt timestamps
- `ts/src/errors.ts` - PayloadTooLargeError, StoreNotOpenError, ImmutabilityViolationError, EmptyTopicError, EmptyPayloadError with kind discriminator
- `ts/tests/vectors.test.ts` - 453-line vitest harness consuming all four spec vector files
- `ts/package.json` - cairn package with better-sqlite3 dep, vitest/tsdown/typescript devDeps
- `ts/tsconfig.json` - ESNext target, bundler moduleResolution, strict mode
- `ts/tsdown.config.ts` - dual ESM/CJS with outExtensions forcing .mjs/.cjs
- `ts/vitest.config.ts` - minimal vitest config
- `ts/.gitignore` - excludes node_modules/ and dist/

## Decisions Made

- **tsdown output extensions:** Removed `"type": "module"` from package.json and added `outExtensions` to tsdown.config.ts to explicitly force `.cjs` for CJS and `.mjs` for ESM. When `"type": "module"` is set, tsdown uses `.js` for ESM (standard per Node.js spec) but the plan verification and research pattern both expect `.mjs`. The outExtensions approach produces unambiguous filenames.
- **`_db` getter for test access:** Rather than a separate second connection, exposed `_db` (prefixed with underscore to signal internal/test use) as a public getter returning the underlying `Database.Database`. This avoids concurrency issues with better-sqlite3 (one connection at a time) and is the simplest approach.
- **package.json exports:** Updated to use `index.d.mts`/`index.d.cts` (tsdown v0.10 dual declarations) instead of a single `index.d.ts`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] tsdown output filename mismatch (index.js vs index.mjs)**
- **Found during:** Task 1 verification
- **Issue:** With `"type": "module"` in package.json, tsdown v0.10 produces `dist/index.js` for ESM and the plan verification step checks for `dist/index.mjs`
- **Fix:** Removed `"type": "module"` from package.json and added `outExtensions` to tsdown config to explicitly set `.mjs` for ESM and `.cjs` for CJS
- **Files modified:** `ts/package.json`, `ts/tsdown.config.ts`
- **Verification:** `ls dist/` shows `index.mjs`, `index.cjs`, `index.d.mts`, `index.d.cts`
- **Committed in:** `68aeff6` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - output filename correction)
**Impact on plan:** Fix necessary for build verification. No scope creep. The produced artifacts are semantically identical — only filenames differ.

## Issues Encountered

None beyond the tsdown filename deviation documented above.

## User Setup Required

None — no external service configuration required. `npm install` is sufficient.

## Self-Check: PASSED

All created files verified on disk. Both task commits confirmed in git log.

## Next Phase Readiness

- TypeScript SDK complete and spec-validated
- ts/ directory ready for documentation and publishing
- Rust SDK (03-02) can proceed in parallel (no dependency on TS SDK)
- All TS-01 through TS-07 requirements satisfied

---
*Phase: 03-typescript-and-rust-sdks*
*Completed: 2026-03-18*
