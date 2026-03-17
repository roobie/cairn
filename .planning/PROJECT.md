# Cairn

## What This Is

An opinionated SDK for append-only event storage, wrapping SQLite. Two operations: `append(event)` and `query(time_range)`. Three language implementations (Go, TypeScript, Rust) sharing a common spec and test vectors. The constraint is the product — no updates, no deletes, no joins, no schema migrations. Data corruption from misuse becomes structurally impossible.

## Core Value

Append-only immutability enforced at the storage level — if it's in cairn, it can't be altered or removed.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Language-agnostic API spec and shared test vectors
- [ ] Go SDK with full API (Open, Close, Append, AppendBatch, Query, QueryAll, Topics, Count, Earliest, Latest)
- [ ] TypeScript SDK matching the same API contract
- [ ] Rust SDK matching the same API contract
- [ ] SQLite triggers enforcing immutability (reject UPDATE and DELETE)
- [ ] Time-based file rotation (daily segments, transparent to caller)
- [ ] 1MB payload size limit (hard error on larger)
- [ ] WAL mode for write performance
- [ ] Nanosecond-precision timestamps
- [ ] Zero-configuration setup — `Open(path)` is the only entry point
- [ ] README with "why cairn" positioning and 5-line quickstart per language
- [ ] Per-language README with high-level intro

### Out of Scope

- Event updates or deletes — the entire point is immutability
- Arbitrary WHERE clauses — time-range only, prevents scope creep into analytics
- Schema-on-write — cairn stores bytes, caller interprets
- Multi-writer support — SQLite constraint; covers target personas; event sourcing excluded
- Pub/sub or consumer groups — cairn is storage, not a message broker
- Aggregations or GROUP BY — not an analytics engine
- Browser runtime for TypeScript — Node/Bun only
- HTTP shim / server mode — natural evolution, but out of scope for v1
- Retention policies — future feature
- Compression — caller's responsibility, cairn stores raw bytes
- Chronological ordering guarantees — insertion order is sufficient

## Context

- **Name origin**: A cairn is a stack of stones used as a trail marker. You add stones, never remove them.
- **Storage**: Single SQLite file per instance, WAL mode, B-tree index on (topic, ts)
- **Concurrency**: Single-writer, multiple-reader (SQLite WAL)
- **Target personas**: IoT/edge telemetry, audit logging, application event trails
- **Build order**: Spec + test vectors first, then Go (establishes patterns), then TypeScript and Rust
- **Throughput target**: >50K events/sec (directional, not a hard gate for v1)

## Constraints

- **Tech stack (Go)**: `modernc.org/sqlite` (pure Go, no CGo) preferred for cross-compilation
- **Tech stack (TypeScript)**: `better-sqlite3` (Node, synchronous) for v1
- **Tech stack (Rust)**: `rusqlite` with bundled SQLite feature
- **API surface**: Exactly the operations in the brief — no extras
- **Simplicity**: When in doubt, choose the simpler option

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go first after spec | Establishes patterns for TypeScript and Rust to follow | — Pending |
| Spec + test vectors before implementation | Cleanest contract — implement against spec, not the other way around | — Pending |
| Insertion order, not chronological | Simpler, avoids clock-skew complexity | — Pending |
| Time-based rotation (daily) | Predictability over size-based | — Pending |
| 1MB payload limit | Reasonable cap, hard error on larger | — Pending |
| Compression is caller's job | Keeps cairn simple, payload is opaque bytes | — Pending |
| Throughput target is directional | Design for >50K/sec but don't block v1 release on benchmarks | — Pending |

---
*Last updated: 2026-03-18 after initialization*
