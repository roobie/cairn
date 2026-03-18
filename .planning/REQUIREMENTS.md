# Requirements: Cairn

**Defined:** 2026-03-18
**Core Value:** Append-only immutability enforced at the storage level — if it's in cairn, it can't be altered or removed.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Spec

- [x] **SPEC-01**: Language-agnostic API spec document (api.md) defining the contract all SDKs implement
- [x] **SPEC-02**: Shared test vectors covering: append single, append batch, query by range, query empty range, refuse update (error), refuse delete (error)
- [x] **SPEC-03**: Test vectors encode timestamps as quoted strings (not JSON numbers) and payloads as RFC 4648 base64
- [x] **SPEC-04**: Schema DDL without AUTOINCREMENT, with immutability triggers (no_update, no_delete)

### Storage

- [x] **STOR-01**: SQLite WAL mode enabled at Open time
- [x] **STOR-02**: Immutability triggers reject UPDATE and DELETE on events table
- [x] **STOR-03**: SQLITE_DBCONFIG_DEFENSIVE enabled at Open time
- [x] **STOR-04**: Nanosecond-precision timestamps stored as INTEGER
- [x] **STOR-05**: Opaque BLOB payloads (schema-on-read)
- [x] **STOR-06**: 1MB payload size limit with hard error on oversize
- [x] **STOR-07**: WAL checkpoint on Close

### Go SDK

- [x] **GO-01**: Open(path) creates or opens a cairn store with zero configuration
- [x] **GO-02**: Close() cleanly shuts down with WAL checkpoint
- [x] **GO-03**: Append(topic, payload) returns EventID, atomic write
- [x] **GO-04**: AppendBatch(events) transactional multi-write, all-or-nothing
- [x] **GO-05**: Query(topic, start, end) returns iterator of events in insertion order
- [x] **GO-06**: All shared test vectors pass
- [x] **GO-07**: Uses modernc.org/sqlite (pure Go, no CGo)

### TypeScript SDK

- [x] **TS-01**: Open(path) creates or opens a cairn store with zero configuration
- [x] **TS-02**: Close() cleanly shuts down with WAL checkpoint
- [x] **TS-03**: Append(topic, payload) returns EventID, atomic write
- [x] **TS-04**: AppendBatch(events) transactional multi-write, all-or-nothing
- [x] **TS-05**: Query(topic, start, end) returns iterator of events in insertion order
- [x] **TS-06**: All shared test vectors pass
- [x] **TS-07**: Uses better-sqlite3 with .safeIntegers(true) for BigInt timestamps

### Rust SDK

- [x] **RS-01**: Open(path) creates or opens a cairn store with zero configuration
- [x] **RS-02**: Close (Drop) cleanly shuts down with WAL checkpoint
- [x] **RS-03**: Append(topic, payload) returns EventID, atomic write
- [x] **RS-04**: AppendBatch(events) transactional multi-write, all-or-nothing
- [x] **RS-05**: Query(topic, start, end) returns iterator of events in insertion order
- [x] **RS-06**: All shared test vectors pass
- [x] **RS-07**: Uses rusqlite with bundled feature

### Documentation

- [x] **DOC-01**: Root README with "why cairn" positioning
- [x] **DOC-02**: 5-line quickstart per language in root README
- [x] **DOC-03**: Per-language README with high-level intro

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Metadata Operations

- **META-01**: Topics() — enumerate known topics
- **META-02**: Count(topic) — event count per topic
- **META-03**: Earliest(topic) / Latest(topic) — edge timestamps

### Cross-Topic Query

- **QUERY-01**: QueryAll(start, end) — time-range read across all topics

### File Rotation

- **ROT-01**: Time-based daily file rotation (transparent to caller)
- **ROT-02**: Query spanning multiple segment files
- **ROT-03**: Segment enumeration helpers for retention management

### Extended Features

- **EXT-01**: Benchmark suite with published throughput numbers
- **EXT-02**: Query with limit/offset (cursor pagination)
- **EXT-03**: Structured error types per language

## Out of Scope

| Feature | Reason |
|---------|--------|
| Event updates or deletes | Core value is immutability — append correction events instead |
| Arbitrary WHERE clauses | Time-range only prevents scope creep into analytics |
| Schema-on-write / typed events | Cairn stores bytes; caller owns schema |
| Multi-writer / concurrent writes | SQLite WAL single-writer constraint; covers target personas |
| Pub/sub / live tailing | Storage, not a broker; poll Latest() or use OS file watch |
| Retention policies / auto-deletion | Deletion breaks immutability; use OS-level segment file management |
| Compression | Caller's responsibility; cairn stores raw bytes |
| HTTP server mode | Different product; natural v2 evolution |
| Aggregations / GROUP BY | Not an analytics engine; export to OLAP tools |
| Browser / WASM runtime | Node/Bun only for v1; WASM is a valid v2 target |
| Chronological ordering | Insertion order is sufficient; avoids clock-skew complexity |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SPEC-01 | Phase 1 | Complete |
| SPEC-02 | Phase 1 | Complete |
| SPEC-03 | Phase 1 | Complete |
| SPEC-04 | Phase 1 | Complete |
| STOR-01 | Phase 1 | Complete |
| STOR-02 | Phase 1 | Complete |
| STOR-03 | Phase 1 | Complete |
| STOR-04 | Phase 1 | Complete |
| STOR-05 | Phase 1 | Complete |
| STOR-06 | Phase 1 | Complete |
| STOR-07 | Phase 1 | Complete |
| GO-01 | Phase 2 | Complete |
| GO-02 | Phase 2 | Complete |
| GO-03 | Phase 2 | Complete |
| GO-04 | Phase 2 | Complete |
| GO-05 | Phase 2 | Complete |
| GO-06 | Phase 2 | Complete |
| GO-07 | Phase 2 | Complete |
| TS-01 | Phase 3 | Complete |
| TS-02 | Phase 3 | Complete |
| TS-03 | Phase 3 | Complete |
| TS-04 | Phase 3 | Complete |
| TS-05 | Phase 3 | Complete |
| TS-06 | Phase 3 | Complete |
| TS-07 | Phase 3 | Complete |
| RS-01 | Phase 3 | Complete |
| RS-02 | Phase 3 | Complete |
| RS-03 | Phase 3 | Complete |
| RS-04 | Phase 3 | Complete |
| RS-05 | Phase 3 | Complete |
| RS-06 | Phase 3 | Complete |
| RS-07 | Phase 3 | Complete |
| DOC-01 | Phase 4 | Complete |
| DOC-02 | Phase 4 | Complete |
| DOC-03 | Phase 4 | Complete |

**Coverage:**
- v1 requirements: 32 total
- Mapped to phases: 32
- Unmapped: 0

---
*Requirements defined: 2026-03-18*
*Last updated: 2026-03-18 after roadmap creation*
