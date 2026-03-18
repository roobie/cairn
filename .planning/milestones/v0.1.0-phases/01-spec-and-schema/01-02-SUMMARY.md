---
phase: 01-spec-and-schema
plan: "02"
subsystem: spec
tags: [test-vectors, spec, json, base64, wycheproof]
dependency_graph:
  requires: ["01-01"]
  provides: ["spec/vectors/append.json", "spec/vectors/batch.json", "spec/vectors/query.json", "spec/vectors/immutability.json"]
  affects: ["Phase 2 Go SDK", "Phase 3 TypeScript SDK", "Phase 3 Rust SDK"]
tech_stack:
  added: []
  patterns: ["Wycheproof test vector format", "RFC 4648 standard base64", "quoted-string nanosecond timestamps"]
key_files:
  created:
    - spec/vectors/append.json
    - spec/vectors/batch.json
    - spec/vectors/query.json
    - spec/vectors/immutability.json
  modified: []
decisions:
  - "store_closed flag in input signals to harness to call operation without opening store first"
  - "payload_size_bytes field in input instructs harness to generate a synthetic payload of that size (avoids embedding 1MB literal in JSON)"
  - "query vector events array uses ts as input to harness (harness inserts with forced timestamps or uses app-level insert and maps by position)"
  - "immutability vector uses setup array of {topic, payload} — harness inserts via Append API, then executes raw sql field directly against the SQLite connection"
metrics:
  duration: "2 min"
  completed: "2026-03-18T01:19:05Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 4
  files_modified: 0
---

# Phase 1 Plan 2: Test Vectors Summary

**One-liner:** Four Wycheproof-style JSON vector files covering all 6 SPEC-02 cases with RFC 4648 base64 payloads and quoted-string nanosecond timestamps.

## What Was Built

Four JSON test vector files in `spec/vectors/` that serve as the executable specification for all cairn SDK acceptance tests. Every SDK (Go, TypeScript, Rust) implements a generic harness that reads these files and validates behavior against them.

### Files Created

**spec/vectors/append.json** — 7 tests in 2 groups:
- Happy path: minimal event (tc_id 1), 1MB boundary exact (tc_id 2), binary non-ASCII payload bytes 0x00-0x05 + 0xFD-0xFF encoded as `AAECAwQF/f7/` (tc_id 3)
- Error cases: payload_too_large (1048577 bytes, tc_id 4), empty_topic (tc_id 5), empty_payload (tc_id 6), store_not_open (tc_id 7)

**spec/vectors/batch.json** — 6 tests in 2 groups:
- Happy path: 3-event multi-topic batch (tc_id 1), empty batch returns empty list no error (tc_id 2), single-event batch (tc_id 3)
- Error cases: all-or-nothing failure when second event exceeds 1MB (tc_id 4), empty_topic in first event (tc_id 5), store_not_open (tc_id 6)

**spec/vectors/query.json** — 6 tests in 3 groups:
- Happy path: both events returned in insertion order (tc_id 1), partial range returns middle event only (tc_id 2), start==end inclusive proves [start,end] range semantics (tc_id 3)
- Empty results: cross-topic query returns empty list (tc_id 4), non-overlapping time range returns empty list (tc_id 5)
- Error cases: store_not_open (tc_id 6)

**spec/vectors/immutability.json** — 2 tests in 1 group:
- Raw SQL UPDATE rejected with exact trigger message "cairn: updates not allowed" (tc_id 1)
- Raw SQL DELETE rejected with exact trigger message "cairn: deletes not allowed" (tc_id 2)

## Decisions Made

1. **`store_closed: true` flag** — Rather than a separate error vector file format, a boolean flag in `input` signals to the harness to attempt the operation without calling Open first. This keeps the vector structure uniform across all operation types.

2. **`payload_size_bytes` field** — For the 1MB boundary tests (tc_id 2 in append.json, tc_id 4 in batch.json), the harness generates a synthetic payload of the specified byte count. Embedding 1MB or 1MB+1 of base64 in a JSON file is impractical — a numeric size hint is the idiomatic Wycheproof approach for size-boundary tests.

3. **Query vector timestamp strategy** — The query.json input events include `ts` fields as quoted strings. The harness inserts events with those exact timestamps (using a test-only insert path that accepts a forced timestamp) or inserts in order and maps results by position. The spec test harness must support forced-timestamp inserts or time-mocking for deterministic query testing.

4. **Immutability harness contract** — The `setup` array is consumed by calling the normal `Append` API (testing the happy path indirectly). After setup, the harness executes the raw `sql` field directly against the underlying SQLite connection, bypassing all SDK public API validation. The error message from SQLite's RAISE(ABORT, ...) must contain the `error_message_contains` string.

## SPEC-02 Coverage Matrix

| Required Case | File | Test IDs |
|---------------|------|----------|
| Append single (happy path) | append.json | tc_id 1, 2, 3 |
| Append batch (happy path) | batch.json | tc_id 1, 2, 3 |
| Query by range | query.json | tc_id 1, 2, 3 |
| Query empty range | query.json | tc_id 4, 5 |
| Refuse UPDATE (immutability) | immutability.json | tc_id 1 |
| Refuse DELETE (immutability) | immutability.json | tc_id 2 |

## SPEC-03 Compliance

- Timestamps in query.json: all 19-digit nanosecond values are quoted strings (e.g., `"1710000000000000001"`) — never bare JSON numbers
- Payloads: all use RFC 4648 standard base64 alphabet (uses `+` and `/`, not `-` and `_`); padding included where required
- Binary encoding: `AAECAwQF/f7/` in append.json tc_id 3 encodes bytes 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0xFD, 0xFE, 0xFF — exercises the non-ASCII binary encoding path in all three SDK languages

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- spec/vectors/append.json: FOUND (commit 7edfe36)
- spec/vectors/batch.json: FOUND (commit 7edfe36)
- spec/vectors/query.json: FOUND (commit dcd84d3)
- spec/vectors/immutability.json: FOUND (commit dcd84d3)
