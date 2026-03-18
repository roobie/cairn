# Phase 1: Spec and Schema - Research

**Researched:** 2026-03-18
**Domain:** Language-agnostic API specification, SQLite DDL, and JSON test vectors
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Spec document structure:**
- Organized by operation: Concepts section up front, then one section per operation (Open, Close, Append, AppendBatch, Query), then Storage Invariants, then Error Catalog
- Pseudocode signatures for all operations (e.g., `Append(topic: string, payload: bytes) -> EventID | PayloadTooLarge | StoreNotOpen`)
- Formal type definitions in Concepts section: EventID (uint64), Timestamp (int64 nanos), Topic (string), Payload (bytes), Event struct
- Each operation section includes: signature, preconditions, behavior, errors

**Error contract:**
- Named error codes defined in the spec: PayloadTooLarge, StoreNotOpen, ImmutabilityViolation, EmptyTopic, EmptyPayload
- SDKs must use these names in idiomatic form (Go: ErrPayloadTooLarge, TS: PayloadTooLargeError, Rust: Error::PayloadTooLarge)
- Error Catalog section with table: Code, Operation, Trigger

**Schema DDL:**
- Single canonical `spec/schema.sql` file that all SDKs execute verbatim via exec()
- schema.sql contains only CREATE TABLE/INDEX/TRIGGER — no PRAGMAs
- INTEGER PRIMARY KEY (no AUTOINCREMENT) as decided in STATE.md
- Composite index on (topic, ts)
- Immutability triggers with descriptive messages: `'cairn: updates not allowed'` and `'cairn: deletes not allowed'`

**Connection PRAGMAs:**
- Documented in api.md under Open behavior, not in schema.sql
- PRAGMAs are per-connection, set every time: journal_mode=WAL, busy_timeout=5000, foreign_keys=ON, SQLITE_DBCONFIG_DEFENSIVE=ON

### Claude's Discretion

- Test vector JSON structure and file organization
- Exact wording of spec prose between signatures
- Whether to include a versioning header in api.md
- Appendix content (if any)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SPEC-01 | Language-agnostic API spec document (api.md) defining the contract all SDKs implement | Locked document structure, pseudocode style, type definitions, error catalog |
| SPEC-02 | Shared test vectors covering: append single, append batch, query by range, query empty range, refuse update (error), refuse delete (error) | Wycheproof-pattern JSON format with operation/test_groups/tests hierarchy |
| SPEC-03 | Test vectors encode timestamps as quoted strings (not JSON numbers) and payloads as RFC 4648 base64 | Required to prevent JS 53-bit precision loss; RFC 4648 standard (not URL-safe) base64 with padding |
| SPEC-04 | Schema DDL without AUTOINCREMENT, with immutability triggers (no_update, no_delete) | INTEGER PRIMARY KEY only; RAISE(ABORT, ...) triggers with exact message strings |
| STOR-01 | SQLite WAL mode enabled at Open time | journal_mode=WAL PRAGMA in Open, documented in api.md not schema.sql |
| STOR-02 | Immutability triggers reject UPDATE and DELETE on events table | BEFORE UPDATE / BEFORE DELETE triggers using RAISE(ABORT, message) |
| STOR-03 | SQLITE_DBCONFIG_DEFENSIVE enabled at Open time | C-level sqlite3_db_config() call, not SQL pragma; must be documented in spec |
| STOR-04 | Nanosecond-precision timestamps stored as INTEGER | int64 range sufficient; 19-digit values must be represented as strings in JSON vectors |
| STOR-05 | Opaque BLOB payloads (schema-on-read) | BLOB column type; spec documents that cairn never interprets payload content |
| STOR-06 | 1MB payload size limit with hard error on oversize | Validated before INSERT; error code PayloadTooLarge in Error Catalog |
| STOR-07 | WAL checkpoint on Close | PRAGMA wal_checkpoint(TRUNCATE) in Close behavior section of spec |
</phase_requirements>

---

## Summary

Phase 1 produces three artifacts: `spec/api.md`, `spec/schema.sql`, and `spec/vectors/*.json`. All three must exist before any SDK code is written. This phase has no code dependencies and no runtime — it is pure documentation and structured data. The deliverables are the cross-language contract that Phases 2 and 3 implement against.

The technical decisions for this phase are already locked (see User Constraints above). The research task is: understand the exact content requirements for each artifact, identify what the test vectors must cover, and flag formatting/encoding choices that must be consistent across all three language SDKs. The two highest-risk decisions — `INTEGER PRIMARY KEY` without `AUTOINCREMENT`, and timestamps as quoted strings in JSON — are already resolved and documented in CONTEXT.md and STATE.md. What remains is translating those decisions into specific file content.

The spec document style follows the LSP/Protocol Buffers model: language-neutral pseudocode signatures with formal type definitions, not implementation-language-specific interfaces. The test vector format follows the Wycheproof pattern: a JSON envelope per operation with `test_groups` → `tests` hierarchy, where each test has `tc_id`, `comment`, `input`, and `expected`. The spec explicitly documents WAL mode, SQLITE_DBCONFIG_DEFENSIVE, nanosecond timestamps, opaque BLOBs, and the 1MB limit as required by STOR-01 through STOR-07.

**Primary recommendation:** Write `spec/api.md` and `spec/schema.sql` first to establish the authoritative contract, then derive the test vectors from the error catalog and success criteria. Test vectors should be the last artifact created in this phase so they reflect the final spec, not an early draft.

---

## Standard Stack

This phase produces no code. There is no runtime stack to install. The artifacts are:

| Artifact | Format | Tool |
|----------|--------|------|
| `spec/api.md` | Markdown | Text editor / Claude |
| `spec/schema.sql` | SQL DDL | Text editor / Claude |
| `spec/vectors/*.json` | JSON | Text editor / Claude |

No build tools, package managers, or language runtimes are required to produce these files.

---

## Architecture Patterns

### Recommended Project Structure

```
spec/
├── api.md               # Language-agnostic API contract
├── schema.sql           # Canonical DDL executed verbatim by all SDKs
└── vectors/
    ├── append.json      # Single-event append: happy path, PayloadTooLarge, EmptyTopic, EmptyPayload, StoreNotOpen
    ├── batch.json       # AppendBatch: happy path, partial failure, empty batch
    ├── query.json       # Query: happy path, empty range, large range, StoreNotOpen
    └── immutability.json # UPDATE attempt, DELETE attempt — both must return ImmutabilityViolation
```

### Pattern 1: Spec Document Structure (Locked)

**What:** api.md is organized in strict section order.
**When to use:** This is the only structure. Do not reorganize.

```
# Cairn API Specification

## Version

## Concepts
  - EventID (uint64)
  - Timestamp (int64, nanoseconds since Unix epoch)
  - Topic (string, non-empty UTF-8)
  - Payload (bytes, opaque, 1MB max)
  - Event struct { id EventID, topic Topic, ts Timestamp, payload Payload }

## Operations
  ### Open
  ### Close
  ### Append
  ### AppendBatch
  ### Query

## Storage Invariants
  - WAL mode
  - SQLITE_DBCONFIG_DEFENSIVE
  - Nanosecond INTEGER timestamps
  - Opaque BLOB payloads
  - 1MB payload hard limit
  - WAL checkpoint on Close

## Error Catalog
  | Code | Operation | Trigger |
  |------|-----------|---------|
  | PayloadTooLarge | Append, AppendBatch | payload > 1MB |
  | StoreNotOpen | All | operation called before Open or after Close |
  | ImmutabilityViolation | (storage-level) | UPDATE or DELETE on events table |
  | EmptyTopic | Append, AppendBatch | topic == "" |
  | EmptyPayload | Append, AppendBatch | payload length == 0 |
```

### Pattern 2: Operation Spec Section Structure (Locked)

Each operation section follows the same four-part structure:

```markdown
### Append

**Signature:** `Append(topic: Topic, payload: Payload) -> EventID`

**Preconditions:**
- Store is open
- topic is non-empty
- payload is non-empty and <= 1MB

**Behavior:**
1. Validate preconditions; return appropriate error on failure
2. Obtain current nanosecond timestamp from system clock
3. INSERT INTO events (topic, ts, payload) VALUES (topic, ts, payload)
4. Return inserted row's INTEGER PRIMARY KEY as EventID

**Errors:**
| Error | Condition |
|-------|-----------|
| StoreNotOpen | Store is not open |
| EmptyTopic | topic == "" |
| EmptyPayload | len(payload) == 0 |
| PayloadTooLarge | len(payload) > 1048576 |
```

### Pattern 3: Test Vector Format (Wycheproof-style)

**What:** One JSON file per logical operation group. Each file has a top-level envelope with `operation`, `schema_version`, and `test_groups`. Tests within groups have `tc_id`, `comment`, `input`, `expected`.

**When to use:** All test vector files follow this exact format. SDK harnesses can use a single generic reader.

```json
{
  "operation": "append",
  "schema_version": "1",
  "description": "Append operation test vectors",
  "test_groups": [
    {
      "description": "happy path",
      "tests": [
        {
          "tc_id": 1,
          "comment": "minimal valid event returns EventID",
          "input": {
            "topic": "sensor",
            "payload": "aGVsbG8="
          },
          "expected": {
            "result": "valid",
            "event_id_type": "uint64",
            "event_id_min": "1"
          }
        },
        {
          "tc_id": 2,
          "comment": "payload exactly at 1MB limit is accepted",
          "input": {
            "topic": "sensor",
            "payload_size_bytes": 1048576
          },
          "expected": {
            "result": "valid"
          }
        }
      ]
    },
    {
      "description": "error cases",
      "tests": [
        {
          "tc_id": 3,
          "comment": "payload one byte over 1MB limit",
          "input": {
            "topic": "sensor",
            "payload_size_bytes": 1048577
          },
          "expected": {
            "result": "invalid",
            "error_kind": "payload_too_large"
          }
        },
        {
          "tc_id": 4,
          "comment": "empty topic rejected",
          "input": {
            "topic": "",
            "payload": "aGVsbG8="
          },
          "expected": {
            "result": "invalid",
            "error_kind": "empty_topic"
          }
        },
        {
          "tc_id": 5,
          "comment": "empty payload rejected",
          "input": {
            "topic": "sensor",
            "payload": ""
          },
          "expected": {
            "result": "invalid",
            "error_kind": "empty_payload"
          }
        }
      ]
    }
  ]
}
```

### Pattern 4: Query Test Vector with Timestamp Strings

Timestamps in test vectors are ALWAYS quoted strings (not JSON numbers). This is non-negotiable:

```json
{
  "tc_id": 1,
  "comment": "query returns events in insertion order within time range",
  "input": {
    "events": [
      { "topic": "sensor", "ts": "1710000000000000001", "payload": "AAEC" },
      { "topic": "sensor", "ts": "1710000000000000002", "payload": "AwQF" }
    ],
    "query": {
      "topic": "sensor",
      "start": "1710000000000000000",
      "end": "1710000000000000999"
    }
  },
  "expected": {
    "result": "valid",
    "returned_count": 2,
    "events": [
      { "ts": "1710000000000000001", "payload": "AAEC" },
      { "ts": "1710000000000000002", "payload": "AwQF" }
    ]
  }
}
```

### Pattern 5: Schema DDL (Locked)

The canonical `spec/schema.sql` executed verbatim by all SDKs:

```sql
CREATE TABLE IF NOT EXISTS events (
    id      INTEGER PRIMARY KEY,
    topic   TEXT    NOT NULL,
    ts      INTEGER NOT NULL,
    payload BLOB    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_topic_ts ON events (topic, ts);

CREATE TRIGGER IF NOT EXISTS no_update
    BEFORE UPDATE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: updates not allowed');
END;

CREATE TRIGGER IF NOT EXISTS no_delete
    BEFORE DELETE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: deletes not allowed');
END;
```

Key points:
- `INTEGER PRIMARY KEY` — no AUTOINCREMENT (prevents sqlite_sequence write on every insert)
- `CREATE ... IF NOT EXISTS` on all three statements — safe to call on every Open()
- Trigger messages `'cairn: updates not allowed'` and `'cairn: deletes not allowed'` are exact strings (locked)
- No PRAGMAs in schema.sql — they belong in the Open behavior section of api.md

### Anti-Patterns to Avoid

- **Including PRAGMAs in schema.sql:** PRAGMAs are connection-level and must be re-applied on every new connection. Putting them in schema.sql creates a false impression that they are persistent. They belong in the Open operation description in api.md.
- **Using AUTOINCREMENT:** Adds a write to `sqlite_sequence` on every INSERT with no benefit in an append-only store. The immutability triggers make rowid reuse structurally impossible.
- **Timestamps as JSON numbers:** JavaScript's `Number` has 53-bit mantissa. A nanosecond-epoch value is ~1.7×10^18, which exceeds 53-bit precision (~9×10^15). Events nanoseconds apart will appear to have the same timestamp when deserialized in JS. Use quoted strings.
- **Using URL-safe base64 in vectors:** Standard RFC 4648 base64 (uses `+` and `/`, not `-` and `_`) must be used. If left implicit, Go, Rust, and TypeScript implementations may each use different variants silently.
- **Empty test vector payloads:** Using ASCII-only payloads like `"hello"` base64-encoded means the binary encoding path is never exercised. Include at least one vector with a payload containing non-ASCII byte values (e.g., bytes 0x00–0xFF).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON encoding for test vectors | Custom binary format | Standard JSON with base64 strings for binary | JSON has universal parsers in all three languages; base64 is specified in RFC 4648 |
| Cross-language type negotiation | Language-specific type files | Single pseudocode type block in api.md Concepts | Pseudocode has no runtime; SDKs map to idiomatic types in their own layer |
| Schema versioning | Migration system | `CREATE TABLE IF NOT EXISTS` idempotent DDL | v1 has one schema; IF NOT EXISTS makes every Open() safe |
| Error code registry | Numeric error codes | Named string codes in Error Catalog table | Named codes survive refactoring; numeric codes require a lookup table in every SDK |

**Key insight:** This phase produces no code. The risk is under-specifying the contract, not over-engineering it. The right output is a spec so complete that an SDK author needs to read nothing else to implement a correct, compliant SDK.

---

## Common Pitfalls

### Pitfall 1: Insufficient Error Catalog Coverage
**What goes wrong:** Spec defines operation signatures but omits the exact conditions that trigger each error code. SDK implementers make different assumptions about boundary conditions (e.g., is a 1MB payload valid or does the limit apply strictly below 1MB?).
**Why it happens:** Spec is written from the happy-path perspective; edge cases are added as afterthoughts.
**How to avoid:** Write the Error Catalog before writing operation behavior prose. Every error in the catalog must have a test vector. Work backwards from the vectors to verify the catalog is complete.
**Warning signs:** A test vector references `error_kind: "payload_too_large"` but the Error Catalog doesn't specify whether 1MB exactly is valid or invalid.

### Pitfall 2: Missing Immutability Test Vectors
**What goes wrong:** The spec documents the immutability triggers but no test vector exercises them. SDK implementations pass all test vectors even if they forget to execute schema.sql (skipping the triggers entirely).
**Why it happens:** The immutability vectors require a slightly different harness structure — the harness must attempt a direct SQL UPDATE/DELETE and expect an error, which is a different pattern from the Append/Query vectors.
**How to avoid:** Include explicit vectors in `immutability.json` that exercise both `no_update` and `no_delete` triggers. The harness must be able to execute raw SQL for these tests (bypassing the SDK's public API).
**Warning signs:** No `immutability.json` file in `spec/vectors/`. No test vector has `error_kind: "immutability_violation"`.

### Pitfall 3: SQLITE_DBCONFIG_DEFENSIVE Not Documentable as a SQL PRAGMA
**What goes wrong:** The spec says "enable SQLITE_DBCONFIG_DEFENSIVE" but SDK implementers try to enable it with a PRAGMA statement. There is no `PRAGMA defensive` that works reliably across all three SQLite bindings.
**Why it happens:** SQLite's defensive mode is a C-level `sqlite3_db_config()` call, not a SQL pragma. The forum post at sqlite.org confirms it is "not supposed to be toggled using SQL commands." Each language binding exposes it differently.
**How to avoid:** The spec must document this as a language-binding-specific call with per-language guidance:
- Go (`modernc.org/sqlite`): `db.SetDefensive(true)` or use the C bridge via `_sqlite3_db_config`
- TypeScript (`better-sqlite3`): `db.pragma('defensive=ON')` — better-sqlite3 does support this pragma
- Rust (`rusqlite`): `conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)?`

Document in api.md as: "Enable defensive mode using the binding-appropriate mechanism. This prevents writes to shadow tables and disabling of triggers via PRAGMA writable_schema."
**Warning signs:** The spec says `PRAGMA defensive = ON` as if it's a standard SQL statement.

### Pitfall 4: Open/Close Behavior Not Fully Specified
**What goes wrong:** The spec says "Open creates or opens a cairn store" but doesn't specify: What happens if the file exists but is not a cairn database? What happens if the file is locked by another process? What happens if the directory doesn't exist?
**Why it happens:** Happy-path thinking. The spec was written for the case where everything is fine.
**How to avoid:** Add a "Preconditions" block to Open and Close that covers these cases:
- Open: If the file is a SQLite database but has no `events` table, schema.sql is executed (CREATE IF NOT EXISTS is safe)
- Open: If the file path's parent directory does not exist, return a storage error (not a panic)
- Open: If another writer holds the write lock, busy_timeout=5000ms applies before returning SQLITE_BUSY
- Close: Close is idempotent — calling Close on an already-closed store is a no-op, not an error
**Warning signs:** Open has no error paths documented. The preconditions block is missing.

### Pitfall 5: Test Vector File Structure Inconsistency
**What goes wrong:** Different vector files use slightly different JSON keys (`input.ts` vs `input.timestamp`, `expected.count` vs `expected.returned_count`). SDK harnesses must handle multiple schemas.
**Why it happens:** Vectors are written incrementally across multiple sessions without a formal JSON Schema.
**How to avoid:** Define the vector JSON Schema as the first artifact — before writing any actual test cases. Every vector file must validate against it. Include a `schema_version: "1"` field in every vector file so future versions can be detected.
**Warning signs:** Two vector files use different key names for the same concept. No JSON Schema document exists for the vector format.

---

## Code Examples

Verified patterns from official sources and project research:

### schema.sql (Complete File)
```sql
-- Cairn schema DDL — execute verbatim on every Open()
-- Source: CONTEXT.md locked decisions + SQLite official docs

CREATE TABLE IF NOT EXISTS events (
    id      INTEGER PRIMARY KEY,
    topic   TEXT    NOT NULL,
    ts      INTEGER NOT NULL,
    payload BLOB    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_topic_ts ON events (topic, ts);

CREATE TRIGGER IF NOT EXISTS no_update
    BEFORE UPDATE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: updates not allowed');
END;

CREATE TRIGGER IF NOT EXISTS no_delete
    BEFORE DELETE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: deletes not allowed');
END;
```

### Open Behavior PRAGMA Sequence (for api.md prose)
```
-- Applied on every connection open, after schema DDL:
PRAGMA journal_mode = WAL;       -- enable WAL mode (persistent, only needs setting once but harmless to re-apply)
PRAGMA busy_timeout = 5000;      -- 5 second wait before SQLITE_BUSY (must set on every connection)
PRAGMA foreign_keys = ON;        -- enable FK enforcement
-- SQLITE_DBCONFIG_DEFENSIVE enabled via language binding (not SQL)
```

### Close Behavior Checkpoint (for api.md prose)
```
-- On Close():
PRAGMA wal_checkpoint(TRUNCATE); -- checkpoint and truncate WAL file to zero bytes
-- Then close the database connection
```

### Error Catalog Table (for api.md)
```markdown
| Code | Operation | Trigger |
|------|-----------|---------|
| PayloadTooLarge | Append, AppendBatch | payload byte length > 1,048,576 (1MB) |
| StoreNotOpen | Open (redundant), Close, Append, AppendBatch, Query | Called when store is not in open state |
| ImmutabilityViolation | (storage-layer, not public API) | Any UPDATE or DELETE on events table |
| EmptyTopic | Append, AppendBatch | topic is empty string |
| EmptyPayload | Append, AppendBatch | payload has zero bytes |
```

### Immutability Test Vector Structure
```json
{
  "operation": "immutability",
  "schema_version": "1",
  "description": "Storage-level immutability enforcement test vectors",
  "test_groups": [
    {
      "description": "direct SQL bypass attempts (harness executes raw SQL, not SDK API)",
      "tests": [
        {
          "tc_id": 1,
          "comment": "direct UPDATE rejected by no_update trigger",
          "input": {
            "setup": [
              { "topic": "t", "payload": "aGVsbG8=" }
            ],
            "sql": "UPDATE events SET payload = X'00' WHERE id = 1"
          },
          "expected": {
            "result": "invalid",
            "error_kind": "immutability_violation",
            "error_message_contains": "cairn: updates not allowed"
          }
        },
        {
          "tc_id": 2,
          "comment": "direct DELETE rejected by no_delete trigger",
          "input": {
            "setup": [
              { "topic": "t", "payload": "aGVsbG8=" }
            ],
            "sql": "DELETE FROM events WHERE id = 1"
          },
          "expected": {
            "result": "invalid",
            "error_kind": "immutability_violation",
            "error_message_contains": "cairn: deletes not allowed"
          }
        }
      ]
    }
  ]
}
```

### Query Empty Range Vector
```json
{
  "tc_id": 5,
  "comment": "query over time range with no matching events returns empty result, not error",
  "input": {
    "events": [
      { "topic": "sensor", "ts": "1710000000000000001", "payload": "aGVsbG8=" }
    ],
    "query": {
      "topic": "sensor",
      "start": "2000000000000000000",
      "end":   "2000000000000000999"
    }
  },
  "expected": {
    "result": "valid",
    "returned_count": 0,
    "events": []
  }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `INTEGER PRIMARY KEY` (no AUTOINCREMENT) | Decided in STATE.md before Phase 1 | Removes sqlite_sequence write on every INSERT; ~10% throughput improvement at volume |
| JSON number for timestamps | JSON quoted string for timestamps | SPEC-03, before Phase 1 | Prevents JS 53-bit precision loss for 19-digit nanosecond values |
| Implicit binary encoding in vectors | Explicit RFC 4648 standard base64 | SPEC-03 | Prevents Go/TS/Rust from silently using different base64 variants |
| PRAGMA defensive = ON (unreliable) | sqlite3_db_config(SQLITE_DBCONFIG_DEFENSIVE) per binding | SQLite 3.26.0 (2018) | Language-binding-specific call; spec must document per-language mechanism |

**Deprecated/outdated:**
- `AUTOINCREMENT` on append-only tables: unnecessary, adds overhead, must not appear in schema.sql
- Timestamp encoding as plain JSON numbers: unsafe at nanosecond precision in JavaScript

---

## Open Questions

1. **Exact boundary for 1MB payload limit**
   - What we know: STOR-06 says "1MB payload size limit"; spec must be precise
   - What's unclear: Is exactly 1,048,576 bytes valid, or must payload be strictly less than 1MB?
   - Recommendation: Define as `len(payload) <= 1,048,576` (1MB inclusive is valid); `len(payload) > 1,048,576` triggers PayloadTooLarge. Include a test vector with exactly 1,048,576 bytes that expects `result: "valid"` and one with 1,048,577 bytes that expects `error_kind: "payload_too_large"`.

2. **Query range semantics: inclusive or exclusive endpoints?**
   - What we know: `Query(topic, start, end)` — start and end are Timestamps
   - What's unclear: Is the range `[start, end]` (both inclusive), `[start, end)` (end exclusive), or something else?
   - Recommendation: Use `[start, end]` (both inclusive) — consistent with SQL `WHERE ts >= start AND ts <= end`. Document explicitly in the Query section. Include a vector that queries with start == end and verifies the single event at that timestamp is returned.

3. **AppendBatch empty input behavior**
   - What we know: `AppendBatch(events) -> []EventID`
   - What's unclear: Is `AppendBatch([])` (empty slice) valid (returns empty list) or an error?
   - Recommendation: Return empty list, no error — consistent with how most batch APIs behave. Include an explicit test vector for this case.

4. **SQLITE_DBCONFIG_DEFENSIVE in modernc.org/sqlite**
   - What we know: Rust (`rusqlite`) has `DbConfig::SQLITE_DBCONFIG_DEFENSIVE`, TypeScript (`better-sqlite3`) has `db.pragma('defensive=ON')`
   - What's unclear: Whether `modernc.org/sqlite` exposes this through `database/sql` interface or requires a lower-level call
   - Recommendation: Research this in Plan 01-01 before writing the Open behavior section. The spec must give accurate Go guidance. If modernc doesn't expose it cleanly, the spec notes "set via the binding's native C config mechanism" and Phase 2 research resolves the implementation.

5. **spec/vectors/ JSON Schema document**
   - What we know: Claude's Discretion section permits defining the vector file organization
   - What's unclear: Whether to include a formal JSON Schema file (e.g., `spec/vectors/schema.json`) that all vector files validate against
   - Recommendation: Include a simple `spec/vectors/README.md` describing the format rather than a formal JSON Schema, to avoid the overhead of maintaining a schema validator. The Wycheproof pattern uses a schema, but for this project scale, prose documentation of the format is sufficient.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | None — Phase 1 produces no executable code |
| Config file | N/A |
| Quick run command | Manual review of JSON syntax: `python3 -m json.tool spec/vectors/*.json` |
| Full suite command | N/A — validation is structural (do files exist, do they parse as JSON) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SPEC-01 | api.md defines all 5 operations with full error semantics | manual | `test -f spec/api.md` | ❌ Wave 0 |
| SPEC-02 | vectors cover all 6 required cases | manual | `ls spec/vectors/*.json` | ❌ Wave 0 |
| SPEC-03 | timestamps are quoted strings, payloads are RFC 4648 base64 | manual/grep | `grep -r '"ts":' spec/vectors/ \| grep -v '"ts": "'` (should return empty) | ❌ Wave 0 |
| SPEC-04 | schema.sql has INTEGER PRIMARY KEY, no AUTOINCREMENT, both triggers | manual/grep | `grep -c AUTOINCREMENT spec/schema.sql` (should return 0) | ❌ Wave 0 |
| STOR-01 | api.md documents WAL pragma in Open behavior | manual | `grep -c journal_mode spec/api.md` | ❌ Wave 0 |
| STOR-02 | schema.sql has no_update and no_delete triggers | grep | `grep -c BEFORE DELETE spec/schema.sql` | ❌ Wave 0 |
| STOR-03 | api.md documents SQLITE_DBCONFIG_DEFENSIVE in Open behavior | manual | `grep -c SQLITE_DBCONFIG_DEFENSIVE spec/api.md` | ❌ Wave 0 |
| STOR-04 | api.md documents nanosecond INTEGER timestamps | manual | `grep -c nanosecond spec/api.md` | ❌ Wave 0 |
| STOR-05 | schema.sql uses BLOB column, api.md documents opaque payload | grep | `grep -c BLOB spec/schema.sql` | ❌ Wave 0 |
| STOR-06 | api.md documents 1MB hard limit, vectors include PayloadTooLarge case | manual | `grep -c 1048576 spec/api.md` | ❌ Wave 0 |
| STOR-07 | api.md documents WAL checkpoint in Close behavior | manual | `grep -c wal_checkpoint spec/api.md` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `python3 -m json.tool spec/vectors/*.json > /dev/null` (JSON syntax validation)
- **Per wave merge:** Manual checklist review against Success Criteria
- **Phase gate:** All 4 Success Criteria from ROADMAP.md verified before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `spec/api.md` — covers SPEC-01, STOR-01, STOR-03, STOR-04, STOR-05, STOR-06, STOR-07
- [ ] `spec/schema.sql` — covers SPEC-04, STOR-02, STOR-05
- [ ] `spec/vectors/append.json` — covers SPEC-02, SPEC-03 (partial)
- [ ] `spec/vectors/batch.json` — covers SPEC-02 (batch case)
- [ ] `spec/vectors/query.json` — covers SPEC-02, SPEC-03 (timestamp strings)
- [ ] `spec/vectors/immutability.json` — covers SPEC-02 (immutability rejection cases)

---

## Sources

### Primary (HIGH confidence)
- [SQLite AUTOINCREMENT — official documentation](https://sqlite.org/autoinc.html) — INTEGER PRIMARY KEY vs AUTOINCREMENT, sqlite_sequence overhead
- [SQLite Write-Ahead Logging — official documentation](https://www.sqlite.org/wal.html) — WAL mode, checkpoint modes (PASSIVE/FULL/TRUNCATE), checkpoint starvation
- [SQLite Pragma Statements](https://www.sqlite.org/pragma.html) — journal_mode, busy_timeout, foreign_keys, wal_checkpoint
- [SQLite SQLITE_DBCONFIG_DEFENSIVE — C API](https://www.sqlite.org/c3ref/c_dbconfig_defensive.html) — defensive mode scope, what it prevents
- [SQLite Defense Against The Dark Arts](https://sqlite.org/security.html) — security model, writable_schema, shadow tables
- [SQLite Triggers](https://www.sqlite.org/lang_createtrigger.html) — BEFORE UPDATE/DELETE, RAISE(ABORT, message)
- [RFC 4648 — Base64 Data Encodings](https://datatracker.ietf.org/doc/html/rfc4648) — standard base64 alphabet (+/), padding (=), URL-safe variant differentiation
- [Project Wycheproof — test vector format](https://github.com/C2SP/wycheproof/blob/main/doc/formats.md) — test_groups/tests hierarchy, tc_id, result/error_kind fields
- Project research: `.planning/research/ARCHITECTURE.md`, `.planning/research/PITFALLS.md`, `.planning/research/STACK.md`, `.planning/research/SUMMARY.md` — all HIGH confidence, researched 2026-03-18

### Secondary (MEDIUM confidence)
- [SQLite User Forum: SQLITE_DBCONFIG_DEFENSIVE via SQL](https://sqlite.org/forum/forumpost/a621d9baf3) — confirms defensive mode is not reliably toggled via SQL in all bindings
- Project CONTEXT.md locked decisions — all implementation decisions for this phase

### Tertiary (LOW confidence)
- `PRAGMA defensive=ON` in better-sqlite3 — needs validation during Plan 01-01; better-sqlite3 docs suggest it works, SQLite forum suggests C API is preferred

---

## Metadata

**Confidence breakdown:**
- Spec document structure: HIGH — fully locked in CONTEXT.md; no alternatives to research
- Schema DDL: HIGH — INTEGER PRIMARY KEY, triggers, and IF NOT EXISTS patterns from SQLite official docs
- Test vector format: HIGH — Wycheproof pattern well-documented; file organization is Claude's discretion
- Error catalog: HIGH — all five error codes are locked; boundary conditions are Open Questions above
- SQLITE_DBCONFIG_DEFENSIVE: MEDIUM — C API is confirmed; per-binding mechanism for modernc/Go needs Plan 01-01 verification

**Research date:** 2026-03-18
**Valid until:** 2026-09-18 (stable domain — SQLite spec, RFC 4648 do not change; Wycheproof format is stable)
