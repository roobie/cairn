# Architecture Research

**Domain:** Multi-language SDK — append-only SQLite event store
**Researched:** 2026-03-18
**Confidence:** HIGH (core patterns well-established; daily-rotation details MEDIUM)

---

## Standard Architecture

### System Overview

```
cairn/
├── spec/                        ← Language-agnostic contract
│   ├── API.md                   ← Operation definitions (Open, Close, Append, ...)
│   └── vectors/                 ← JSON test fixtures (input → expected output)
│       ├── append.json
│       ├── query.json
│       ├── batch.json
│       └── schema.json          ← JSON Schema for vector format
│
├── sdks/
│   ├── go/                      ← Go implementation
│   │   ├── cairn.go             ← Public API surface
│   │   ├── store.go             ← SQLite interaction
│   │   ├── rotation.go          ← Daily segment logic
│   │   └── cairn_test.go        ← Spec compliance + unit tests
│   │
│   ├── typescript/              ← TypeScript implementation
│   │   ├── src/
│   │   │   ├── index.ts         ← Public API surface
│   │   │   ├── store.ts         ← SQLite interaction
│   │   │   └── rotation.ts
│   │   ├── test/
│   │   │   └── spec.test.ts     ← Spec compliance test harness
│   │   └── package.json
│   │
│   └── rust/                    ← Rust implementation
│       ├── src/
│       │   ├── lib.rs           ← Public API surface
│       │   ├── store.rs         ← SQLite interaction
│       │   └── rotation.rs
│       ├── tests/
│       │   └── spec.rs          ← Spec compliance test harness
│       └── Cargo.toml
│
├── .github/
│   └── workflows/
│       ├── go.yml
│       ├── typescript.yml
│       └── rust.yml
│
└── README.md
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| `spec/API.md` | Single source of truth for operation signatures, error semantics, edge cases | Markdown prose + pseudocode |
| `spec/vectors/` | Executable contract: JSON fixtures consumed by each SDK's test harness | JSON files following Wycheproof-style structure |
| SDK public API | Exposes `Open`, `Close`, `Append`, `AppendBatch`, `Query`, `QueryAll`, `Topics`, `Count`, `Earliest`, `Latest` | One file per SDK, thin wrapper over store |
| SDK store layer | All SQLite interaction: schema init, WAL config, trigger setup, read/write | Per-SDK, no shared code |
| SDK rotation layer | Computes active segment file path from current UTC day; opens new DB when day rolls over | Per-SDK, algorithm must match spec |
| Test harness | Reads `spec/vectors/*.json`, drives the SDK, asserts outputs match expected | Per-SDK, consumes shared fixture files |

---

## Recommended Project Structure

The `spec/` directory is the anchor. It has no build dependencies and no language runtime. Every SDK is a consumer of it. This order is non-negotiable:

1. `spec/` — defines the contract
2. `sdks/go/` — first implementation, establishes idioms
3. `sdks/typescript/` and `sdks/rust/` — follow patterns Go establishes

Each SDK directory is self-contained. A reader can `cd sdks/go && go test ./...` without understanding the other SDKs. No build orchestration tool (Bazel, Nx, Turborepo) is required for this scope; plain `make` or per-language native tooling is sufficient.

---

## Test Vector Format

Following the Wycheproof pattern (confidence: HIGH — actively maintained by C2SP, used by Go standard library and others), test vectors use a JSON envelope:

```json
{
  "operation": "append",
  "schema_version": "1",
  "description": "Basic append and query roundtrip",
  "test_groups": [
    {
      "description": "single-event roundtrip",
      "tests": [
        {
          "tc_id": 1,
          "comment": "minimal valid event",
          "input": {
            "topic": "sensor",
            "payload": "aGVsbG8="
          },
          "expected": {
            "result": "valid",
            "returned_count": 1
          }
        },
        {
          "tc_id": 2,
          "comment": "payload exceeds 1MB limit",
          "input": {
            "topic": "sensor",
            "payload_size_bytes": 1048577
          },
          "expected": {
            "result": "invalid",
            "error_kind": "payload_too_large"
          }
        }
      ]
    }
  ]
}
```

Each SDK test harness reads these files, drives its own implementation, and asserts the `expected` block. No test harness code is shared between languages — the fixture data is shared, not the runner.

---

## SQLite Layer Architecture

### Schema

```sql
-- One table per database file (one file = one day segment)
CREATE TABLE IF NOT EXISTS events (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    topic     TEXT    NOT NULL,
    ts        INTEGER NOT NULL,   -- nanoseconds since Unix epoch
    payload   BLOB    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_topic_ts ON events (topic, ts);

-- Immutability enforcement
CREATE TRIGGER IF NOT EXISTS no_update
    BEFORE UPDATE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: events are immutable');
END;

CREATE TRIGGER IF NOT EXISTS no_delete
    BEFORE DELETE ON events
BEGIN
    SELECT RAISE(ABORT, 'cairn: events are immutable');
END;
```

Rationale: `RAISE(ABORT, ...)` rolls back the statement and surfaces an error to the caller at the SQLite API level — no application code path can accidentally bypass this (confidence: HIGH — SQLite trigger docs + community confirmation).

### WAL Configuration

Applied once on `Open()`:

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;   -- safe with WAL, faster than FULL
PRAGMA busy_timeout=5000;    -- ms, avoids immediate SQLITE_BUSY on reader contention
```

WAL mode gives concurrent readers + one writer without blocking. `synchronous=NORMAL` is safe under WAL because the WAL itself provides durability (confidence: HIGH — SQLite official docs).

---

## Daily Rotation Architecture

This is cairn-specific; no off-the-shelf pattern exists (confidence: MEDIUM).

### Segment File Naming

```
<base_path>/
├── cairn-2026-03-17.db
├── cairn-2026-03-17.db-wal
├── cairn-2026-03-17.db-shm
└── cairn-2026-03-18.db       ← active
```

File name format: `cairn-YYYY-MM-DD.db` derived from UTC date at write time.

### Component Flow

```
Append(topic, payload)
        │
        ▼
  compute UTC date
        │
        ├── same as cached date ──► use open db handle
        │
        └── new date ─────────────► close old handle
                                    open/create new file
                                    run schema + PRAGMA init
                                    update cached handle + date
                                    proceed with insert
```

Query across a time range may span multiple segment files. The `Query(start, end)` implementation must:

1. Enumerate segment files whose date overlaps `[start, end]`
2. Open each (read-only), query, merge results
3. Return merged result set ordered by `ts`

The spec must define the merging contract clearly (insertion order within a file, file order by date).

---

## Data Flow

### Write Path

```
caller
  │  Append(topic, payload []byte)
  ▼
cairn public API
  │  validate payload ≤ 1MB
  │  get nanosecond timestamp
  ▼
rotation layer
  │  resolve active segment file (date check)
  ▼
SQLite store
  │  INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)
  │  (trigger fires — rejects UPDATE/DELETE, INSERT passes)
  ▼
  return (id, nil)
```

### Read Path

```
caller
  │  Query(topic, start_ns, end_ns)
  ▼
cairn public API
  ▼
rotation layer
  │  enumerate segment files overlapping [start_ns, end_ns]
  ▼
SQLite store (per segment, in parallel or serial)
  │  SELECT id, topic, ts, payload FROM events
  │  WHERE topic = ? AND ts >= ? AND ts <= ?
  │  ORDER BY ts
  ▼
merge results across segments
  ▼
  return []Event
```

---

## Architectural Patterns

### Pattern 1: Spec-First, Implementations Follow

Write `spec/API.md` and `spec/vectors/` before any SDK code. This prevents the first implementation (Go) from becoming the de facto spec by accident. When TypeScript and Rust implementations disagree with Go, the tie-break is the spec, not Go.

### Pattern 2: Zero Shared Runtime Code

Each SDK uses only its language's SQLite binding (`modernc.org/sqlite`, `better-sqlite3`, `rusqlite`). There is no shared library or FFI. Duplication of the rotation algorithm and schema SQL across three implementations is intentional — coupling three language runtimes to eliminate 50 lines of SQL is the wrong tradeoff.

### Pattern 3: Shared Fixtures, Independent Harnesses

Test vectors live in `spec/vectors/`. Each SDK ships its own harness that reads those JSON files. The harness is the glue; the data is the contract. This pattern is validated at scale by Project Wycheproof (used by Go standard library, BoringSSL, and others).

### Pattern 4: Immutability at the Storage Layer

Triggers enforce immutability below the application layer. This means:
- A bug in Go code cannot accidentally delete events
- A developer cannot run `UPDATE` from a SQLite CLI by mistake
- No `IF NOT DELETED` flag pattern needed — deletion is impossible, not just discouraged

### Pattern 5: Single-Writer Per Segment File

SQLite WAL supports one writer. The cairn API does not need to handle multi-writer scenarios (explicitly out of scope). The `Open(path)` call owns one handle. Callers needing multi-process writes should use separate cairn instances writing to separate base paths.

---

## Anti-Patterns

### Anti-Pattern 1: Shared Go/C Library with FFI Bindings

**What goes wrong:** Attempt to write the core in one language and bind TypeScript and Rust to it via FFI or WASM.
**Why bad:** CGo constraints, cross-compilation complexity, debugging across FFI boundaries, runtime version coupling. Adds significant build complexity for what is ~300 lines of logic per language.
**Instead:** Implement independently in each language. The spec + test vectors are the correctness guarantee.

### Anti-Pattern 2: Single SQLite File (No Rotation)

**What goes wrong:** All events in one file grows unbounded; backup/archival of old data becomes a full-lock operation; file size balloons for IoT/audit use cases over time.
**Instead:** Daily segment rotation. The caller sees a single logical store; files segment transparently.

### Anti-Pattern 3: Application-Level Immutability Only

**What goes wrong:** A `DELETE FROM events WHERE id = ?` executed directly against the SQLite file bypasses all application logic and silently corrupts the append-only guarantee.
**Instead:** SQLite triggers enforcing `RAISE(ABORT, ...)` on UPDATE and DELETE. This is database-level enforcement that survives any application bug or direct access.

### Anti-Pattern 4: Shared Mutable State Across Segment Files

**What goes wrong:** Keeping a global autoincrement sequence that spans files. When a new segment file is opened, the sequence resets to 1 — IDs are only unique within a file.
**Instead:** Event identity is `(segment_date, id)` or callers use `ts` + `topic` for correlation. The API should not expose raw `id` as a stable global identifier.

### Anti-Pattern 5: Polyglot Build Orchestration for v1

**What goes wrong:** Introducing Bazel, Nx, or a custom Makefile that builds all three SDKs as a unit. This adds tooling overhead with no benefit at this scale.
**Instead:** Each SDK builds and tests independently with its native tool (`go test`, `npm test`, `cargo test`). A root `Makefile` can provide `make test-all` as a convenience without deep integration.

---

## Build Order Implications

Dependencies are one-directional:

```
spec/ ──► sdks/go/ ──► sdks/typescript/
                  └──► sdks/rust/
```

- `spec/` has no code dependencies; it is pure documentation and data
- `sdks/go/` depends only on `spec/` (reads vector JSON at test time)
- `sdks/typescript/` and `sdks/rust/` depend on `spec/` and benefit from Go patterns, but have no runtime dependency on `sdks/go/`
- CI jobs for each SDK are independent; they can run in parallel once spec is stable

Recommended build sequence for the project:

1. `spec/API.md` — prose spec of all operations and error semantics
2. `spec/vectors/schema.json` — JSON Schema for the vector format
3. `spec/vectors/*.json` — initial test cases covering the happy path and boundary conditions
4. `sdks/go/` — full implementation passing all vectors
5. `sdks/typescript/` and `sdks/rust/` — implement against spec, validate with vectors; Go serves as reference behavior

---

## Sources

- [Project Wycheproof — test vector format documentation](https://github.com/C2SP/wycheproof/blob/main/doc/formats.md) — HIGH confidence, actively maintained
- [SQLite WAL mode — official documentation](https://www.sqlite.org/wal.html) — HIGH confidence, official source
- [SQLite triggers — enforcement patterns](https://www.sqlitetutorial.net/sqlite-trigger/) — HIGH confidence
- [Building Event Sourcing Systems with SQLite](https://www.sqliteforum.com/p/building-event-sourcing-systems-with) — MEDIUM confidence, community source
- [SQLite and immutable audit trails](https://www.sqliteforum.com/p/sqlite-and-blockchain-storing-immutable) — MEDIUM confidence
- [Multi-language SDK monorepo management](https://medium.com/@parserdigital/how-to-manage-multi-language-open-source-sdks-on-githug-best-practices-tools-1a401b22544e) — MEDIUM confidence
- [SQLite log rotation discussion](https://groups.google.com/g/sqlite_users/c/uYL0rH1xCRQ) — MEDIUM confidence, community pattern

---
*Architecture research for: Cairn — multi-language append-only SQLite event store SDK*
*Researched: 2026-03-18*
