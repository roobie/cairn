# Feature Research

**Domain:** Append-only event storage SDK (embedded, SQLite-backed, multi-language)
**Researched:** 2026-03-18
**Confidence:** HIGH (core features), MEDIUM (differentiators), LOW (long-tail anti-features)

---

## Feature Landscape

### Table Stakes (Users Expect These)

These are the features any developer reaching for an embedded event log will assume exist. Missing one makes the library feel broken or incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `Append(event)` — single write | The primitive operation. Every event log has it. | Low | Must be atomic; partial writes must not leave corrupt state |
| `AppendBatch(events)` — multi-write | High-throughput callers batch to amortize fsync. Callers who write individually in a loop hit WAL limits. | Low-Med | Wraps single transaction; all succeed or none |
| `Query(topic, from, to)` — time-range read | Time is the natural access dimension for logs and telemetry. Users expect slice-by-time. | Low | B-tree index on `(topic, ts)` makes this fast |
| `QueryAll(from, to)` — cross-topic scan | Audit trail users want all events in a window regardless of topic. | Low | Same index, just no topic predicate |
| `Topics()` — enumerate topics | Callers need to discover what topics exist, especially during replay or debugging. | Low | `SELECT DISTINCT topic` with caching |
| `Count(topic)` — event count | Sanity checks, monitoring, UI badges. Callers don't want to pull all rows to count them. | Low | `SELECT COUNT(*)` with optional topic predicate |
| `Earliest()` / `Latest()` — edge timestamps | Callers need to know the time span covered without reading all events. Common in replication and catch-up reads. | Low | `SELECT MIN(ts)` / `SELECT MAX(ts)` |
| `Open(path)` / `Close()` — lifecycle | Zero-config initialization is mandatory for embedded use. Any required pre-setup loses the value prop. | Low | WAL mode, page size, trigger setup all happen inside `Open` |
| Immutability enforcement at storage level | The entire reason to use an append-only store over SQLite directly. If deletions are possible via a SQL client, the guarantee is worthless. | Med | SQLite triggers on `UPDATE` and `DELETE`; tested in spec |
| Nanosecond-precision timestamps | IoT and telemetry events arrive at sub-millisecond intervals. Millisecond resolution collapses concurrent events into the same slot. | Low | Store as `INTEGER` (nanoseconds since Unix epoch) |
| Opaque byte payloads | Callers must own schema. Any schema-on-write breaks cross-language compatibility and forces library upgrades on format changes. | Low | `BLOB` column; library never interprets content |
| Payload size guard (hard error on oversize) | Without a cap, a single 500 MB payload silently destroys write throughput. Callers must be told loudly. | Low | 1 MB default; validate before insert |
| WAL mode for write performance | SQLite default journal mode serializes readers during writes. WAL is the standard production choice for embedded SQLite. | Low | Set at `Open` time; not caller-configurable |
| Error propagation — no silent failures | Embedded libraries that swallow errors force users to debug phantom data loss. Every error must surface. | Low | Return errors, not panics; language-idiomatic |
| Concurrent read access | Multi-goroutine / multi-thread read access is assumed by all production callers. | Low-Med | WAL provides this; connection pool for readers |

---

### Differentiators (Competitive Advantage)

These features go beyond baseline expectations and create preference over "just use SQLite directly."

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Time-based file rotation (daily segments) | Disk management on IoT/edge devices is critical. Daily files make pruning, archival, and inspection trivial without any database tooling. Predictable naming (`store-2026-03-18.db`) beats size-based rotation. | Med | Transparent to caller; `Open` returns handle that routes writes to current segment. Query spans segments. |
| Cross-language spec + shared test vectors | Three SDKs (Go, TypeScript, Rust) that produce byte-for-byte compatible files are rare. A spec + vectors approach means any language can read any other language's files. | High | The spec IS the product. Test vectors must cover edge cases: empty payloads, max-size payloads, Unicode topics, timestamp ordering. |
| Structural impossibility of corruption-by-misuse | Not just "we don't expose update" — the database physically rejects it via triggers. No raw SQL escape hatch. | Med | Triggers tested in spec vectors; documented explicitly in README as a trust guarantee. |
| Single-dependency embedded design | No daemon, no server, no network, no sidecar. `Open(path)` and done. Contrast with EventStoreDB (server process), Kafka (cluster), NATS (daemon). | Low (design) | Pure Go (`modernc.org/sqlite`), `better-sqlite3` (Node sync), `rusqlite --bundled` (Rust). No CGo required in Go path. |
| Synchronous writes with durability by default | EventStoreDB and Kafka optimize for throughput at the cost of durability knobs. For audit and telemetry, callers want "when append returns, it's on disk." | Low | WAL + `PRAGMA synchronous=NORMAL` is the right tradeoff; document the choice explicitly. |
| Topic-scoped reads with O(log n) index | Many embedded event logs offer only full-table scans. The B-tree index on `(topic, ts)` makes per-topic reads fast even at millions of events per file. | Low (index) | Include benchmark numbers in README. |
| `>50K events/sec` throughput target | Validates fitness for IoT burst-write scenarios. Most embedded options don't publish numbers; Cairn should. | Med | Directional, not a hard gate, but must be documented and reproducible. |
| Spec-first, implementation-second ordering | Prevents "each SDK does it slightly differently" drift. Gives contributors a clear contract to implement against. | High (process) | Test vectors are executable spec; CI runs all three SDKs against same vectors. |

---

### Anti-Features (Commonly Requested, Often Problematic)

Features that users sometimes request but which would erode Cairn's core value or massively expand scope.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Event updates / corrections | "I made a mistake in the payload" | Destroys the audit guarantee. If updates are possible, the log can be falsified. Trust is gone. | Append a correction event with a reference to the original ID. Immutability is a feature. |
| Arbitrary WHERE clause queries | "I want to filter by payload field" | SQLite JSON functions are a rabbit hole. Cairn would become a query engine, not a storage primitive. Scope creep is irreversible once public API is established. | Caller reads by time range and filters in memory. Payload is opaque bytes. |
| Schema-on-write / typed events | "I want the SDK to validate my event structure" | Requires versioning, migration, cross-language schema format negotiation. Becomes a separate product. | Caller defines schema; Cairn stores bytes. Protobuf, JSON, MessagePack are the caller's choice. |
| Multi-writer / concurrent writes | "Multiple processes writing to the same file" | SQLite WAL allows only one writer. Multi-writer requires a write-serialization daemon, killing the embedded simplicity. Deadlock and corruption risk. | Use a single writer process with an internal queue. |
| Pub/sub subscriptions / live tailing | "Notify me when new events arrive" | Cairn is storage, not a broker. Adding reactive semantics requires an event loop, thread management, back-pressure handling. Doubles the API surface. | Poll `Latest()` timestamp; use OS-level file watch; or use a broker (NATS, Redis Streams) on top of Cairn. |
| Retention policies / auto-deletion | "Delete events older than 30 days" | Deletion breaks immutability guarantees. Soft-delete is a lie. | Use file rotation: daily segments are independent files. Callers delete old segment files at the OS level. Cairn never touches them. |
| Compression | "Payloads are large and repetitive" | Compression requires codec negotiation across languages, versioning, and decompression overhead on reads. Cairn stores bytes; the caller decides encoding. | Caller compresses before `Append`. Cairn returns the compressed bytes on `Query`. |
| HTTP server mode / network API | "I want to access Cairn from another host" | A server mode is an entirely different product (concurrency, auth, protocol). Natural v2 evolution, but blocks v1 from shipping. | Run Cairn in a single process with an RPC layer the caller owns. HTTP shim is a separate project. |
| Aggregations / GROUP BY / COUNT DISTINCT | "I want analytics on my events" | Cairn is not an analytics engine. SQLite can do this, but exposing it means owning the full query surface. | Export to DuckDB, ClickHouse, or any OLAP tool. Cairn files are valid SQLite; any SQLite-compatible tool can read them directly. |
| Exactly-once delivery semantics | "Guarantee no duplicate events" | Exactly-once in a local embedded context requires distributed coordination that doesn't apply. At the local level, `AppendBatch` in a transaction provides the needed atomicity. | Use idempotency keys in payload (caller responsibility). At-least-once + idempotent consumers is the honest answer even for distributed systems. |
| Browser / WASM runtime for TypeScript | "I want to use this in a browser" | `better-sqlite3` is a native Node module with no WASM path for v1. Browser SQLite (e.g., wa-sqlite, sql.js) has different performance and durability characteristics. | Node/Bun only for v1. WASM is a valid v2 target but requires a separate driver evaluation. |
| Chronological ordering guarantees | "Events must be ordered by wall clock" | Clock skew, NTP adjustments, and leap seconds make wall-clock ordering unreliable without a Lamport clock or HLC. Insertion order is what SQLite can guarantee. | Document insertion-order semantics explicitly. Callers who need strict ordering embed a sequence number in their payload. |

---

## Feature Dependencies

```
Open(path)
  └── all other operations (nothing works without a valid handle)

AppendBatch(events)
  └── Append(event) semantics (batch is just a transactional wrapper around single appends)

Query(topic, from, to)
  └── Requires B-tree index on (topic, ts) — created at Open time
  └── For multi-segment queries: requires file rotation naming convention

QueryAll(from, to)
  └── Same index as Query, topic predicate removed

Topics()
  └── Depends on topic column being indexed (covered by (topic, ts) index)

Count(topic)
  └── Efficient only with the (topic, ts) index

File rotation (daily segments)
  └── Open(path) must handle routing writes to current segment
  └── Query spanning segments must enumerate segment files by naming convention
  └── Topics() and Count() across segments require union queries

Immutability triggers
  └── Applied at Open time; tested in spec vectors
  └── Independent of all query/write features

Cross-language compatibility
  └── Requires: identical schema DDL across all three SDKs
  └── Requires: identical timestamp encoding (nanoseconds as INTEGER)
  └── Requires: identical segment naming convention
  └── Verified by: shared test vectors executed by all three SDKs in CI
```

---

## MVP Definition

### Launch With (v1)

The spec and test vectors define the contract. Every item below is in scope and must ship.

1. `Open(path)` / `Close()` — zero-config lifecycle
2. `Append(event)` — single atomic write
3. `AppendBatch(events)` — transactional multi-write
4. `Query(topic, from, to)` — time-range read, single topic
5. `QueryAll(from, to)` — time-range read, all topics
6. `Topics()` — enumerate known topics
7. `Count(topic)` — event count per topic
8. `Earliest()` / `Latest()` — edge timestamps
9. Immutability triggers (UPDATE + DELETE rejection)
10. WAL mode, nanosecond timestamps, opaque BLOB payloads
11. 1 MB payload size limit (hard error)
12. Daily segment file rotation (transparent to caller)
13. Go SDK, TypeScript SDK (Node/Bun), Rust SDK — all against shared spec
14. Shared test vectors in CI across all three SDKs
15. README with "why cairn" positioning and 5-line quickstart per language

### Add After Validation (v1.x)

Features that have clear value but require observing real usage patterns first.

- Retention via segment enumeration helpers (list segments, identify purgeable ranges)
- Benchmark suite with published numbers (>50K events/sec claim needs reproducible proof)
- `Count()` without topic filter (global count)
- `Query` with limit/offset (cursor pagination for large time windows)
- Structured error types per language (vs. string errors) — improves caller error handling

### Future Consideration (v2+)

Plausible, high-value, but architecturally separable from the v1 contract.

- HTTP shim / server mode (single-host network access, not multi-writer)
- WASM / browser runtime for TypeScript (requires driver swap)
- Replication to object storage (Litestream-style, as a separate tool or optional module)
- CloudEvents-compatible envelope option (standard `id`, `source`, `type`, `time` fields in metadata column)
- Encryption-at-rest (SQLite SEE or SQLCipher — caller supplies key to `Open`)
- Integrity verification tool (walk all segments, verify trigger constraints, report gaps)

---

## Sources

- EventStoreDB / KurrentDB documentation: https://docs.kurrent.io/clients/tcp/dotnet/21.2/appending
- KurrentDB GitHub: https://github.com/kurrent-io/KurrentDB
- Litestream: https://litestream.io/ and https://github.com/benbjohnson/litestream
- SQLite WAL mode: https://www.sqlite.org/wal.html
- CloudEvents specification: https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md
- go-event-store/eventstore Go library: https://pkg.go.dev/github.com/go-event-store/eventstore
- eventsourcing_sqlite (Gleam): https://hexdocs.pm/eventsourcing_sqlite/
- sqlite-es (Rust CQRS event store): https://github.com/johnbcodes/sqlite-es
- "Exactly Once is a Lie" — EventSourcingDB: https://docs.eventsourcingdb.io/blog/2025/11/20/exactly-once-is-a-lie/
- Append-only log fundamentals: https://questdb.com/glossary/append-only-log/
- AWS Event Sourcing pattern: https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/event-sourcing.html
- SQLite concurrent writes analysis: https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/

---
*Feature research for: Cairn — append-only event storage SDK (Go / TypeScript / Rust)*
*Researched: 2026-03-18*
