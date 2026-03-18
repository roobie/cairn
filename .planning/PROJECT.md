# Cairn

## What This Is

An opinionated SDK for append-only event storage, wrapping SQLite. Five operations: `Open`, `Close`, `Append`, `AppendBatch`, `Query`. Three language implementations (Go, TypeScript, Rust) sharing a common spec and 21 test vectors. The constraint is the product — no updates, no deletes, no joins, no schema migrations. Data corruption from misuse becomes structurally impossible.

## Core Value

Append-only immutability enforced at the storage level — if it's in cairn, it can't be altered or removed.

## Requirements

### Validated

- ✓ Language-agnostic API spec and shared test vectors — v0.1.0
- ✓ Go SDK with full API (Open, Close, Append, AppendBatch, Query) — v0.1.0
- ✓ TypeScript SDK matching the same API contract — v0.1.0
- ✓ Rust SDK matching the same API contract — v0.1.0
- ✓ SQLite triggers enforcing immutability (reject UPDATE and DELETE) — v0.1.0
- ✓ 1MB payload size limit (hard error on larger) — v0.1.0
- ✓ WAL mode for write performance — v0.1.0
- ✓ Nanosecond-precision timestamps — v0.1.0
- ✓ Zero-configuration setup — `Open(path)` is the only entry point — v0.1.0
- ✓ README with "why cairn" positioning and 5-line quickstart per language — v0.1.0
- ✓ Per-language README with high-level intro — v0.1.0

### Active

- [ ] Metadata operations: Topics(), Count(), Earliest(), Latest()
- [ ] QueryAll(start, end) — cross-topic time-range query
- [ ] Time-based file rotation (daily segments, transparent to caller)
- [ ] Benchmark suite with published throughput numbers
- [ ] Query with limit/offset (cursor pagination)
- [ ] Structured error types per language

### Out of Scope

- Event updates or deletes — the entire point is immutability
- Arbitrary WHERE clauses — time-range only, prevents scope creep into analytics
- Schema-on-write — cairn stores bytes, caller interprets
- Multi-writer support — SQLite constraint; covers target personas; event sourcing excluded
- Pub/sub or consumer groups — cairn is storage, not a message broker
- Aggregations or GROUP BY — not an analytics engine
- Browser runtime for TypeScript — Node/Bun only
- HTTP shim / server mode — natural evolution, but out of scope for now
- Retention policies — future feature
- Compression — caller's responsibility, cairn stores raw bytes
- Chronological ordering guarantees — insertion order is sufficient

## Context

- **Name origin**: A cairn is a stack of stones used as a trail marker. You add stones, never remove them.
- **Storage**: Single SQLite file per instance, WAL mode, B-tree index on (topic, ts)
- **Concurrency**: Single-writer, multiple-reader (SQLite WAL)
- **Target personas**: IoT/edge telemetry, audit logging, application event trails
- **Shipped v0.1.0**: 2,896 LOC across Go, TypeScript, Rust + spec. All 3 SDKs pass 21 shared test vectors.
- **Tech stack**: Go (modernc.org/sqlite v1.47.0), TypeScript (better-sqlite3 v12.8.0, tsdown), Rust (rusqlite v0.38 bundled)

## Constraints

- **Tech stack (Go)**: `modernc.org/sqlite` (pure Go, no CGo) — confirmed working
- **Tech stack (TypeScript)**: `better-sqlite3` (Node, synchronous) — confirmed working
- **Tech stack (Rust)**: `rusqlite` with bundled SQLite feature — confirmed working
- **API surface**: v1 is Open/Close/Append/AppendBatch/Query — metadata ops are v2
- **Simplicity**: When in doubt, choose the simpler option

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go first after spec | Establishes patterns for TypeScript and Rust to follow | ✓ Good — Go idioms guided both ports |
| Spec + test vectors before implementation | Cleanest contract — implement against spec, not the other way around | ✓ Good — 21 shared vectors caught TS clock bug |
| Insertion order, not chronological | Simpler, avoids clock-skew complexity | ✓ Good — ORDER BY id ASC is unambiguous |
| 1MB payload limit | Reasonable cap, hard error on larger | ✓ Good — enforced in all 3 SDKs |
| Compression is caller's job | Keeps cairn simple, payload is opaque bytes | ✓ Good — no complexity |
| Canonical schema.sql executed verbatim | All SDKs embed the same DDL — no drift | ✓ Good — trigger messages match exactly |
| SQLITE_DBCONFIG_DEFENSIVE fallback in Go | database/sql cannot access C-level config | ⚠️ Revisit — PRAGMA trusted_schema = OFF is weaker |
| process.hrtime.bigint() initially in TS | Was monotonic, not Unix epoch — caught by audit | ✓ Fixed — now BigInt(Date.now()) * 1_000_000n |

---
*Last updated: 2026-03-18 after v0.1.0 milestone*
