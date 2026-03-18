# Requirements: Cairn

**Defined:** 2026-03-18
**Core Value:** Append-only immutability enforced at the storage level — if it's in cairn, it can't be altered or removed.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Spec

- [ ] **SPEC-01**: Language-agnostic API spec document (api.md) defining the contract all SDKs implement
- [ ] **SPEC-02**: Shared test vectors covering: append single, append batch, query by range, query empty range, refuse update (error), refuse delete (error)
- [ ] **SPEC-03**: Test vectors encode timestamps as quoted strings (not JSON numbers) and payloads as RFC 4648 base64
- [ ] **SPEC-04**: Schema DDL without AUTOINCREMENT, with immutability triggers (no_update, no_delete)

### Storage

- [ ] **STOR-01**: SQLite WAL mode enabled at Open time
- [ ] **STOR-02**: Immutability triggers reject UPDATE and DELETE on events table
- [ ] **STOR-03**: SQLITE_DBCONFIG_DEFENSIVE enabled at Open time
- [ ] **STOR-04**: Nanosecond-precision timestamps stored as INTEGER
- [ ] **STOR-05**: Opaque BLOB payloads (schema-on-read)
- [ ] **STOR-06**: 1MB payload size limit with hard error on oversize
- [ ] **STOR-07**: WAL checkpoint on Close

### Go SDK

- [ ] **GO-01**: Open(path) creates or opens a cairn store with zero configuration
- [ ] **GO-02**: Close() cleanly shuts down with WAL checkpoint
- [ ] **GO-03**: Append(topic, payload) returns EventID, atomic write
- [ ] **GO-04**: AppendBatch(events) transactional multi-write, all-or-nothing
- [ ] **GO-05**: Query(topic, start, end) returns iterator of events in insertion order
- [ ] **GO-06**: All shared test vectors pass
- [ ] **GO-07**: Uses modernc.org/sqlite (pure Go, no CGo)

### TypeScript SDK

- [ ] **TS-01**: Open(path) creates or opens a cairn store with zero configuration
- [ ] **TS-02**: Close() cleanly shuts down with WAL checkpoint
- [ ] **TS-03**: Append(topic, payload) returns EventID, atomic write
- [ ] **TS-04**: AppendBatch(events) transactional multi-write, all-or-nothing
- [ ] **TS-05**: Query(topic, start, end) returns iterator of events in insertion order
- [ ] **TS-06**: All shared test vectors pass
- [ ] **TS-07**: Uses better-sqlite3 with .safeIntegers(true) for BigInt timestamps

### Rust SDK

- [ ] **RS-01**: Open(path) creates or opens a cairn store with zero configuration
- [ ] **RS-02**: Close (Drop) cleanly shuts down with WAL checkpoint
- [ ] **RS-03**: Append(topic, payload) returns EventID, atomic write
- [ ] **RS-04**: AppendBatch(events) transactional multi-write, all-or-nothing
- [ ] **RS-05**: Query(topic, start, end) returns iterator of events in insertion order
- [ ] **RS-06**: All shared test vectors pass
- [ ] **RS-07**: Uses rusqlite with bundled feature

### Documentation

- [ ] **DOC-01**: Root README with "why cairn" positioning
- [ ] **DOC-02**: 5-line quickstart per language in root README
- [ ] **DOC-03**: Per-language README with high-level intro

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
| SPEC-01 | — | Pending |
| SPEC-02 | — | Pending |
| SPEC-03 | — | Pending |
| SPEC-04 | — | Pending |
| STOR-01 | — | Pending |
| STOR-02 | — | Pending |
| STOR-03 | — | Pending |
| STOR-04 | — | Pending |
| STOR-05 | — | Pending |
| STOR-06 | — | Pending |
| STOR-07 | — | Pending |
| GO-01 | — | Pending |
| GO-02 | — | Pending |
| GO-03 | — | Pending |
| GO-04 | — | Pending |
| GO-05 | — | Pending |
| GO-06 | — | Pending |
| GO-07 | — | Pending |
| TS-01 | — | Pending |
| TS-02 | — | Pending |
| TS-03 | — | Pending |
| TS-04 | — | Pending |
| TS-05 | — | Pending |
| TS-06 | — | Pending |
| TS-07 | — | Pending |
| RS-01 | — | Pending |
| RS-02 | — | Pending |
| RS-03 | — | Pending |
| RS-04 | — | Pending |
| RS-05 | — | Pending |
| RS-06 | — | Pending |
| RS-07 | — | Pending |
| DOC-01 | — | Pending |
| DOC-02 | — | Pending |
| DOC-03 | — | Pending |

**Coverage:**
- v1 requirements: 32 total
- Mapped to phases: 0
- Unmapped: 32 ⚠️

---
*Requirements defined: 2026-03-18*
*Last updated: 2026-03-18 after initial definition*
