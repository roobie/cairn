# Roadmap: Cairn

## Overview

Three language SDKs (Go, TypeScript, Rust) sharing a single canonical spec and test vectors for append-only SQLite-backed event storage. The build order is strict: spec and schema constraints first, Go reference implementation second, TypeScript and Rust in parallel third, documentation last. Every phase delivers something independently verifiable — no phase is just scaffolding.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Spec and Schema** - Language-agnostic API spec, shared test vectors, and schema DDL establishing the cross-language contract (completed 2026-03-18)
- [x] **Phase 2: Go SDK** - Reference implementation that all other SDKs validate against (completed 2026-03-18)
- [ ] **Phase 3: TypeScript and Rust SDKs** - Both language SDKs implemented in parallel against the proven spec
- [ ] **Phase 4: Documentation and Release** - Root README, per-language READMEs, and release preparation

## Phase Details

### Phase 1: Spec and Schema
**Goal**: The cross-language contract exists and is executable — all SDK authors (human or Claude) can implement against spec/API.md and verify correctness using spec/vectors/*.json before writing any SDK code
**Depends on**: Nothing (first phase)
**Requirements**: SPEC-01, SPEC-02, SPEC-03, SPEC-04, STOR-01, STOR-02, STOR-03, STOR-04, STOR-05, STOR-06, STOR-07
**Success Criteria** (what must be TRUE):
  1. spec/API.md defines Open, Close, Append, AppendBatch, and Query (single topic) with full error semantics — a developer can implement an SDK from this document alone without reading existing SDK code
  2. spec/vectors/*.json test vectors cover: append single event, append batch, query by time range, query empty range, immutability rejection (UPDATE attempt), immutability rejection (DELETE attempt), with timestamps as quoted strings and payloads as RFC 4648 base64
  3. The schema DDL file exists with INTEGER PRIMARY KEY (no AUTOINCREMENT), the (topic, ts) composite index, and no_update / no_delete triggers
  4. The spec explicitly documents WAL mode, SQLITE_DBCONFIG_DEFENSIVE, nanosecond INTEGER timestamps, opaque BLOB payloads, and the 1MB payload hard limit
**Plans**: 2 plans

Plans:
- [ ] 01-01: API spec document and schema DDL
- [ ] 01-02: Test vectors (happy path, error cases, edge cases)

### Phase 2: Go SDK
**Goal**: A complete, spec-passing Go SDK that serves as the reference implementation — TypeScript and Rust authors can look to it for idiom decisions when the spec is ambiguous
**Depends on**: Phase 1
**Requirements**: GO-01, GO-02, GO-03, GO-04, GO-05, GO-06, GO-07
**Success Criteria** (what must be TRUE):
  1. A Go caller can Open a cairn store, Append events, run AppendBatch, Query by topic and time range, and Close — all with zero configuration beyond the file path
  2. All shared spec test vectors pass under `go test`
  3. WAL checkpoint runs on Close, SQLITE_DBCONFIG_DEFENSIVE is enabled, and busy_timeout is set on every connection — observable by running the test suite against a store that was interrupted mid-write
  4. The SDK uses modernc.org/sqlite (pure Go, no CGo) — `go build` succeeds without a C toolchain
**Plans**: 2 plans

Plans:
- [ ] 02-01-PLAN.md — Store layer (Open/Close, schema init, WAL/PRAGMA setup)
- [ ] 02-02-PLAN.md — Write/read path (Append, AppendBatch, Query) with test vector harness

### Phase 3: TypeScript and Rust SDKs
**Goal**: Both remaining language SDKs pass all spec test vectors — cairn is a working cross-language library
**Depends on**: Phase 2
**Requirements**: TS-01, TS-02, TS-03, TS-04, TS-05, TS-06, TS-07, RS-01, RS-02, RS-03, RS-04, RS-05, RS-06, RS-07
**Success Criteria** (what must be TRUE):
  1. A TypeScript caller (Node/Bun) can Open, Append, AppendBatch, Query, and Close — nanosecond timestamps round-trip correctly as BigInt without precision loss
  2. A Rust caller can Open, Append, AppendBatch, Query, and Close (via Drop) — `cargo build` succeeds without system SQLite installed (bundled feature)
  3. All shared spec test vectors pass under `vitest run` (TypeScript) and `cargo test` (Rust)
  4. The TypeScript package builds dual ESM/CJS output with .d.ts declarations using tsdown
**Plans**: TBD

Plans:
- [ ] 03-01: TypeScript SDK (store layer, write/read path, test vector harness, tsdown build)
- [ ] 03-02: Rust SDK (store layer, write/read path, test vector harness)

### Phase 4: Documentation and Release
**Goal**: A developer discovering cairn can understand why it exists, get started in under a minute in their language, and ship
**Depends on**: Phase 3
**Requirements**: DOC-01, DOC-02, DOC-03
**Success Criteria** (what must be TRUE):
  1. The root README explains the cairn constraint (append-only, no updates, no deletes) in the opening paragraph and gives a 5-line quickstart for each of Go, TypeScript, and Rust
  2. Each language directory has its own README with a high-level intro covering the API surface and any language-specific notes (BigInt for TypeScript, Drop for Rust)
  3. All three SDKs have passing test suites after a fresh clone — `go test ./...`, `vitest run`, and `cargo test` all succeed with no additional setup
**Plans**: TBD

Plans:
- [ ] 04-01: Root README and per-language READMEs

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Spec and Schema | 2/2 | Complete    | 2026-03-18 |
| 2. Go SDK | 2/2 | Complete    | 2026-03-18 |
| 3. TypeScript and Rust SDKs | 0/2 | Not started | - |
| 4. Documentation and Release | 0/1 | Not started | - |
