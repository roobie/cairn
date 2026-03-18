# Project Research Summary

**Project:** Cairn — Opinionated Append-Only Event SDK
**Domain:** Embedded SQLite-backed event store SDK, multi-language monorepo (Go / TypeScript / Rust)
**Researched:** 2026-03-18
**Confidence:** HIGH

## Executive Summary

Cairn is a narrowly-scoped storage primitive: an append-only event log backed by SQLite, shipped as three independent language SDKs sharing a single canonical spec and JSON test vectors. The right way to build this class of product is spec-first — the language-agnostic contract in `spec/` must exist and be testable before any SDK code is written. This prevents the first implementation (Go) from becoming the accidental spec by drift, and makes cross-language compatibility verifiable rather than assumed. The Wycheproof pattern (shared JSON fixture files, independent per-language harnesses) is the established model for this kind of multi-language correctness guarantee.

The recommended technical approach is straightforward and well-validated: `modernc.org/sqlite` (pure Go, no CGo) for the Go SDK, `better-sqlite3` with synchronous API for the TypeScript/Node SDK, and `rusqlite` with the `bundled` feature for the Rust SDK. All three currently ship SQLite 3.51.3, which means test vector behavior is consistent across languages out of the box. The synchronous API model chosen for all three is correct for an append-only single-writer store: async SQLite wrappers add queue complexity with no architectural benefit given SQLite's WAL single-writer constraint.

The dominant risks are all SQLite-specific and are well-understood: WAL checkpoint starvation in long-lived reader processes, nanosecond timestamp precision loss in the JavaScript layer, and the `AUTOINCREMENT` overhead that the BRIEF.md schema currently includes (which PITFALLS research identifies as unnecessary in an append-only store and measurably harmful to write throughput). All of these must be resolved at the spec and schema definition phase — before any SDK implementation begins. The rotation-boundary test coverage gap is the other high-priority spec concern: bugs in cross-segment queries only surface in production after day one, so the spec must include explicit rotation-boundary test vectors.

## Key Findings

### Recommended Stack

All three SQLite drivers are pre-decided and correct for the use case. The key insight from stack research is that versions are well-aligned: all three wrappers currently bundle SQLite 3.51.3, providing a consistent behavioral baseline across languages. Two tooling notes with high impact: `tsup` is no longer maintained and must not be used — `tsdown` 0.20.3 is the direct successor; and `vitest` 4.x is the current TypeScript test standard, not Jest.

**Core technologies:**
- `modernc.org/sqlite` v1.47.0 (Go): pure Go SQLite driver — required for cross-compilation to IoT/edge targets without CGo toolchain complexity
- `better-sqlite3` 12.8.0 (TypeScript): synchronous SQLite for Node — matches the single-writer append model; async wrappers fight the design
- `rusqlite` 0.39.0 with `bundled` feature (Rust): statically links SQLite 3.51.3 — eliminates system library dependency for portable edge binaries
- `vitest` 4.1.0 (TypeScript testing): native TypeScript, no transpilation overhead
- `tsdown` 0.20.3 (TypeScript build): dual ESM/CJS output with `.d.ts`; `tsup` is abandoned
- `testify` v1.11.1 (Go testing): standard assertions for SDK-sized test suites

### Expected Features

Research confirms the BRIEF.md feature scope is well-calibrated against comparable event store libraries. The table stakes list maps cleanly to the project brief's API surface. One addition with high confidence: exposing a `Checkpoint()` method (or calling it automatically in `Close()`) is not optional — it is required to prevent WAL bloat in production.

**Must have (table stakes):**
- `Open(path)` / `Close()` with WAL + PRAGMA init inside — zero external configuration
- `Append(topic, payload)` and `AppendBatch(events)` as a first-class transactional path
- `Query(topic, start, end)` and `QueryAll(start, end)` — time-range reads with index
- `Topics()`, `Count(topic)`, `Earliest()`, `Latest()` — metadata operations
- Immutability triggers (UPDATE + DELETE rejected at storage layer, not application layer)
- Nanosecond timestamps as `INTEGER` (not milliseconds — IoT events arrive sub-millisecond)
- Opaque `BLOB` payload, 1 MB hard cap with explicit error
- WAL mode, `PRAGMA synchronous=NORMAL`, `busy_timeout=5000` on every connection
- Daily segment file rotation (transparent to caller, required for IoT disk management)
- Shared spec + test vectors, all three SDKs pass them in CI

**Should have (competitive):**
- `Checkpoint()` exposure for long-lived process callers
- `SQLITE_DBCONFIG_DEFENSIVE` mode enabled on all connections (blocks raw handle bypass)
- Documented benchmark numbers (>50K events/sec with `AppendBatch`, clearly not single-event)
- Rotation-boundary test vectors in spec (cross-midnight query coverage)
- README per language with "why cairn" positioning and 5-line quickstart

**Defer (v2+):**
- HTTP shim / server mode
- WASM/browser runtime for TypeScript (requires driver swap from `better-sqlite3`)
- Litestream-style replication to object storage
- Encryption-at-rest (SQLite SEE or SQLCipher)
- Retention/segment enumeration helpers (list and prune old segments)

**Confirmed anti-features (never build):**
- Event updates or deletes
- Arbitrary WHERE clause queries
- Schema-on-write / typed event validation
- Multi-writer / concurrent process writes to same file
- Pub/sub or live tailing

### Architecture Approach

The architecture is a monorepo with a strict dependency order: `spec/` has no build dependencies; each SDK directory is self-contained and can be tested in isolation with its language's native tooling. No polyglot build orchestration (Bazel, Nx) is needed or recommended for this scope. The test vector pattern (Wycheproof-style JSON fixtures, independent per-language harnesses) is the correctness anchor — the data is shared, the runner code is not.

**Major components:**
1. `spec/API.md` — language-agnostic operation definitions, error semantics, edge cases; single source of truth
2. `spec/vectors/` — executable JSON fixtures (Wycheproof pattern); consumed by all three SDK harnesses
3. SDK store layer (per language) — all SQLite interaction: schema DDL, WAL config, trigger setup, read/write; no shared runtime code
4. SDK rotation layer (per language) — daily segment file routing; algorithm must match spec; MEDIUM confidence on edge cases
5. SDK public API (per language) — thin wrapper over store and rotation layers
6. Per-SDK test harness — reads `spec/vectors/*.json`, drives the SDK, asserts outputs

**Key architectural patterns (non-negotiable):**
- Spec-first: `spec/` must be written before any SDK code
- Zero shared runtime: each SDK uses only its own language's SQLite binding; cross-language correctness via shared test data, not shared code
- Immutability below application layer: SQLite triggers, not application guards
- Single-writer per segment file: WAL constraint; not a design choice to reconsider
- `(topic, ts)` composite index on every segment's `events` table

### Critical Pitfalls

1. **WAL checkpoint starvation** — the WAL file grows without bound under continuous read load; long-running readers block checkpoint resets. Prevention: call `PRAGMA wal_checkpoint(TRUNCATE)` in `Close()`, set `busy_timeout=5000` on every connection opened (not just the first), and expose `Checkpoint()` for callers who run long-lived processes. This is a day-one production failure mode, not a theoretical concern.

2. **`AUTOINCREMENT` on the schema** — the BRIEF.md schema currently specifies `INTEGER PRIMARY KEY AUTOINCREMENT`. PITFALLS research identifies this as incorrect for an append-only store: it adds a write to `sqlite_sequence` on every insert for reuse-prevention semantics that are structurally impossible given the immutability triggers. Change to `INTEGER PRIMARY KEY` before any implementation builds against the schema. Approximately 10% throughput penalty at volume.

3. **Nanosecond timestamp precision loss in JavaScript** — nanosecond-epoch values are 19-digit integers that exceed JavaScript's 53-bit `Number` mantissa. Events written nanoseconds apart will appear to have the same timestamp when round-tripped through the TypeScript SDK using plain `number`. Prevention: use `better-sqlite3`'s `.safeIntegers(true)` mode (returns int64 as BigInt); represent timestamps as quoted JSON strings in test vectors (not JSON numbers).

4. **Binary payload encoding in test vectors** — JSON has no binary type; the vector format must explicitly mandate standard base64 (RFC 4648, not URL-safe) for all payload fields. If left implicit, Go, Rust, and TypeScript implementations will each make different assumptions and silently test different data while appearing to pass.

5. **Rotation-boundary query gap** — queries that span midnight (where `start` is in segment N and `end` is in segment N+1) are a distinct code path that unit tests never exercise unless explicitly constructed. Prevention: write rotation-boundary test vectors in the spec phase; implement multi-segment query merging in the Go SDK and validate the same vectors in TypeScript and Rust.

## Implications for Roadmap

Based on the dependency structure (spec before SDKs, Go before TypeScript/Rust) and the distribution of pitfalls (most critical issues must be resolved at the spec layer), the following phase structure is recommended:

### Phase 1: Spec and Schema Foundation
**Rationale:** All eight critical pitfalls have roots in spec-level decisions. The `AUTOINCREMENT` schema defect, timestamp encoding convention, binary payload encoding, and rotation-boundary test coverage must be established before any SDK implementation locks in behavior that contradicts them. The architecture research is explicit: `spec/` is the anchor with no build dependencies; it must exist first.
**Delivers:** `spec/API.md` (full operation definitions, error semantics, edge cases), `spec/vectors/schema.json` (vector format schema), initial `spec/vectors/*.json` (happy path + all boundary conditions including rotation boundary, binary payloads, BigInt timestamps, immutability rejection), corrected SQLite schema (no `AUTOINCREMENT`).
**Addresses:** All table stakes features; establishes cross-language compatibility contract.
**Avoids:** Pitfalls 2 (AUTOINCREMENT), 3 (BigInt timestamps in vectors), 4 (binary encoding convention), 5 (rotation-boundary coverage gap).

### Phase 2: Go SDK (Reference Implementation)
**Rationale:** Go is the reference implementation — TypeScript and Rust will look to it for idiom decisions when the spec is ambiguous. It must be complete and fully spec-passing before the other SDKs start, or they will silently diverge. The Go SDK also establishes the benchmark baseline for the throughput target.
**Delivers:** `sdks/go/` passing all spec vectors; `Open`/`Close` with full PRAGMA init and `wal_checkpoint(TRUNCATE)` in `Close()`; `Append`/`AppendBatch` with connection-level mutex; `Query`/`QueryAll` with multi-segment merging; `Topics`/`Count`/`Earliest`/`Latest`; daily rotation; `SQLITE_DBCONFIG_DEFENSIVE` mode; benchmark establishing >50K events/sec with `AppendBatch`.
**Uses:** `modernc.org/sqlite` v1.47.0, `testify` v1.11.1, `golangci-lint`.
**Avoids:** Pitfalls 1 (WAL checkpoint), 6 (busy_timeout on every connection), 8 (modernc throughput — addressed by making `AppendBatch` the primary high-volume path).

### Phase 3: TypeScript SDK
**Rationale:** TypeScript has the most language-specific pitfalls (BigInt timestamps, `better-sqlite3` synchronous model, dual ESM/CJS publishing). Building after the Go SDK is complete means the spec is stable and Go behavior can serve as a reference when edge cases arise.
**Delivers:** `sdks/typescript/` passing all spec vectors; BigInt / `.safeIntegers(true)` throughout; `tsdown` dual ESM+CJS build; `vitest` test suite consuming spec vectors; durability documentation (`synchronous=NORMAL` behavior noted for callers).
**Uses:** `better-sqlite3` 12.8.0, `vitest` 4.1.0, `tsdown` 0.20.3, `typescript`.
**Avoids:** Pitfalls 3 (BigInt timestamps), 6 (busy_timeout), 7 (binary payload encoding verified against Go reference).

### Phase 4: Rust SDK
**Rationale:** Rust has the fewest language-specific pitfalls in this domain (type system prevents most timestamp precision issues; `rusqlite` API is explicit). Building last means the spec is proven by two implementations and the Rust SDK can be validated for divergences.
**Delivers:** `sdks/rust/` passing all spec vectors; `rusqlite` with `bundled` feature; `cargo clippy -D warnings` passing in CI; `cargo fmt --check` enforced; all spec vectors passing including rotation boundary.
**Uses:** `rusqlite` 0.39.0 with `bundled` feature, `cargo test`, `cargo nextest` (optional), `cargo-release`.
**Avoids:** Pitfalls 4 (binary encoding verified against Go and TypeScript references), 5 (rotation boundary already covered by spec vectors from Phase 1).

### Phase 5: CI, README, and Release Preparation
**Rationale:** CI must run all three SDKs against the shared spec vectors — this is the primary correctness guarantee. The release blockers (README positioning, per-language quickstarts, published benchmark numbers) are the v1 success criteria from the BRIEF.md.
**Delivers:** GitHub Actions workflows for each SDK (parallel, independent); root `Makefile` with `make test-all`; README with "why cairn" positioning and 5-line quickstart per language; benchmark results (>50K events/sec with `AppendBatch`, single-event numbers documented separately); `.db-wal` and `.db-shm` sidecar file documentation in backup guidance.
**Addresses:** v1 success criteria; pitfall documentation for callers (WAL checkpoint, `.db-wal` backup pitfall).

### Phase Ordering Rationale

- The spec must precede all SDKs because it defines the contract that makes cross-language compatibility verifiable, not just claimed. The eight critical pitfalls from research are all spec-layer concerns.
- Go precedes TypeScript and Rust because it is the reference implementation; TypeScript and Rust have no runtime dependency on Go, but they benefit from Go establishing idioms for rotation, multi-segment merging, and connection initialization order.
- TypeScript before Rust is a weak ordering — they could run in parallel. TypeScript has more pitfalls (BigInt, bundler configuration) so slightly more risk if sequenced in parallel during planning.
- CI and release preparation is last because it depends on all three SDKs being stable.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (Spec):** The rotation-boundary multi-segment merge contract needs explicit design work; no off-the-shelf pattern exists for this specific case (ARCHITECTURE.md confidence: MEDIUM for daily rotation details). Specifically: how are events ordered when merged from two segment files with overlapping timestamps? How does the reader enumerate segment files without scanning the directory on every query?
- **Phase 2 (Go SDK):** The `modernc.org/sqlite` throughput floor at high write rates needs early validation. Run a micro-benchmark before committing to the `AppendBatch` batch size guidance (>= 100 events per transaction to meet 50K/sec target). If modernc cannot meet the target even with batching, the decision to avoid `mattn/go-sqlite3` (CGo) needs revisiting.

Phases with standard patterns (skip research-phase):
- **Phase 3 (TypeScript SDK):** Well-documented patterns for `better-sqlite3`, `tsdown`, and `vitest`. The pitfalls are known and the mitigations are explicit (BigInt, `.safeIntegers(true)`).
- **Phase 4 (Rust SDK):** `rusqlite` with `bundled` feature is the standard embedded SQLite approach in Rust. No novel patterns required.
- **Phase 5 (CI/README):** Standard GitHub Actions per-language workflows; no research required.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All three drivers pre-decided in BRIEF.md; versions verified against live registries on 2026-03-18. Tooling choices (tsdown over tsup, vitest over jest) verified against current maintainer statements. |
| Features | HIGH (core), MEDIUM (differentiators) | Table stakes and anti-features are well-validated against comparable event store libraries. The 50K events/sec throughput target is directional; exact achievability with modernc depends on batch size, hardware, and connection configuration. |
| Architecture | HIGH (core patterns), MEDIUM (rotation) | SQLite WAL/trigger/index patterns are from official sources. Wycheproof test vector pattern is validated at scale. Daily rotation multi-segment merge is cairn-specific with no off-the-shelf reference implementation; needs explicit design during Phase 1. |
| Pitfalls | HIGH (SQLite pitfalls), MEDIUM (monorepo coordination) | SQLite-specific pitfalls sourced from official documentation, SQLite Forum, and real production reports. Monorepo coordination pitfalls from community sources. |

**Overall confidence:** HIGH

### Gaps to Address

- **Rotation merge contract:** The spec needs to define explicitly what happens when two segment files contain events with identical timestamps at a rotation boundary (e.g., system clock at exactly midnight). Insertion order within a file is deterministic; cross-file ordering with equal timestamps is not. Decide: last-written-file-first, or treat as unordered for equal timestamps?
- **Segment enumeration strategy:** The architecture notes a "small in-memory manifest" approach but does not specify whether the manifest is rebuilt from directory listing on every `Open()` or maintained incrementally. For IoT devices with slow filesystems, this matters. Decide in Phase 1 spec.
- **`AUTOINCREMENT` fix:** The BRIEF.md schema currently has `INTEGER PRIMARY KEY AUTOINCREMENT`. This must be corrected to `INTEGER PRIMARY KEY` before Phase 1 spec is written. The schema in the BRIEF is the reference schema; if it is incorrect, all implementations will implement the wrong thing.
- **BigInt API surface in TypeScript:** The TypeScript public API must decide whether timestamps are `bigint` in the caller-visible types or `number`. Using `bigint` is correct but is a usability friction point for callers who do timestamp arithmetic. Consider a helper that converts between `bigint` ns and `Date` objects.

## Sources

### Primary (HIGH confidence)
- [SQLite WAL mode — official documentation](https://www.sqlite.org/wal.html)
- [SQLite AUTOINCREMENT — official documentation](https://sqlite.org/autoinc.html)
- [SQLite SQLITE_DBCONFIG_DEFENSIVE](https://www.sqlite.org/c3ref/c_dbconfig_defensive.html)
- [modernc.org/sqlite on pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) — v1.47.0, SQLite 3.51.3, pure Go
- [better-sqlite3 on npm](https://www.npmjs.com/package/better-sqlite3) — v12.8.0
- [rusqlite on docs.rs](https://docs.rs/crate/rusqlite/latest) — v0.39.0, bundled feature
- [vitest on npm](https://www.npmjs.com/package/vitest) — v4.1.0
- [Project Wycheproof — test vector format](https://github.com/C2SP/wycheproof/blob/main/doc/formats.md)
- [stretchr/testify releases](https://github.com/stretchr/testify/releases) — v1.11.1

### Secondary (MEDIUM confidence)
- [tsdown introduction](https://tsdown.dev/guide/) — Rolldown-based bundler, tsup successor
- [tsup README — no longer maintained](https://github.com/egoist/tsup) — maintainer confirms abandonment
- [SQLite concurrent writes analysis](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/)
- [better-sqlite3 performance guide](https://github.com/WiseLibs/better-sqlite3/blob/master/docs/performance.md)
- [Multi-language SDK monorepo management](https://medium.com/@parserdigital/how-to-manage-multi-language-open-source-sdks-on-githug-best-practices-tools-1a401b22544e)
- [Building Event Sourcing Systems with SQLite](https://www.sqliteforum.com/p/building-event-sourcing-systems-with)

### Tertiary (LOW confidence / needs validation)
- Throughput claim (>50K events/sec with AppendBatch and modernc) — directional based on modernc benchmark analysis; requires early Phase 2 validation before committing to README claim
- Daily rotation multi-segment merge correctness — no production reference implementation found; pattern must be designed from first principles in Phase 1

---
*Research completed: 2026-03-18*
*Ready for roadmap: yes*
