# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v0.1.0 — Initial Release

**Shipped:** 2026-03-18
**Phases:** 4 | **Plans:** 7 | **Sessions:** 1

### What Was Built
- Language-agnostic API spec with 21 shared test vectors and canonical schema DDL
- Go reference SDK (pure Go, no CGo) — 47 tests, 280 LOC
- TypeScript SDK (better-sqlite3, BigInt timestamps, dual ESM/CJS) — 26 tests, 222 LOC
- Rust SDK (rusqlite bundled, Drop cleanup) — 10 tests, 396 LOC
- Root README + 3 per-language READMEs

### What Worked
- Spec-first approach paid off — shared test vectors caught the TypeScript clock bug during milestone audit
- Go-first reference implementation gave TypeScript and Rust clear idioms to follow
- Parallel execution of TypeScript and Rust SDKs in Phase 3 Wave 1 saved time
- Plan checker caught the SQLITE_DBCONFIG_DEFENSIVE gap and forced the conn.Raw() attempt

### What Was Inefficient
- TypeScript `process.hrtime.bigint()` slipped through Phase 3 verification and was only caught at milestone audit — per-phase verifiers should check clock source against spec
- SUMMARY.md frontmatter `requirements_completed` was incomplete for some plans (SPEC-02, SPEC-03, DOC-01/02/03 missing) — executors should populate this more consistently
- Phase 4 (docs) had no VALIDATION.md — documentation phases should still have basic validation

### Patterns Established
- Canonical schema.sql embedded as const in each SDK — copy approach works for v1 with frozen schema
- Error kind strings (`payload_too_large`, `store_not_open`, etc.) consistent across spec and all SDKs
- Test vector harness pattern: load JSON, decode base64, assert per `expected.result`

### Key Lessons
1. Monotonic clocks (`process.hrtime.bigint()`, `std::time::Instant`) must never be used for stored timestamps — always wall-clock Unix epoch
2. Integration audits at milestone boundary catch bugs that per-phase verification misses
3. The plan checker revision loop (planner → checker → revise) consistently improved plan quality

### Cost Observations
- Model mix: ~30% opus (orchestration), ~70% sonnet (research/planning/execution/verification)
- Sessions: 1 continuous session
- Notable: Entire project from spec to shipped in a single session

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v0.1.0 | 1 | 4 | Initial process — spec-first, Go reference, parallel ports |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v0.1.0 | 83 total (47 Go + 26 TS + 10 Rust) | 21 shared vectors | 0 (all deps are SQLite drivers) |

### Top Lessons (Verified Across Milestones)

1. Spec + shared test vectors before implementation catches cross-language divergence early
2. Milestone audit is essential — per-phase verification alone is insufficient
