# Phase 3: TypeScript and Rust SDKs - Research

**Researched:** 2026-03-18
**Domain:** TypeScript SDK (better-sqlite3 + tsdown + vitest), Rust SDK (rusqlite bundled)
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TS-01 | Open(path) creates or opens a cairn store with zero configuration | better-sqlite3 `new Database(path)` + pragma calls pattern documented |
| TS-02 | Close() cleanly shuts down with WAL checkpoint | `db.pragma('wal_checkpoint(TRUNCATE)')` then `db.close()` pattern |
| TS-03 | Append(topic, payload) returns EventID, atomic write | `stmt.run()` returns `{ lastInsertRowid: bigint }` with safeIntegers |
| TS-04 | AppendBatch(events) transactional multi-write, all-or-nothing | `db.transaction(fn)` wraps all inserts, throws rollback on error |
| TS-05 | Query(topic, start, end) returns iterator of events in insertion order | `stmt.all()` with BigInt params, returns rows with BigInt ts |
| TS-06 | All shared spec test vectors pass | vitest + `fs.readFileSync` + JSON.parse pattern from Go reference |
| TS-07 | Uses better-sqlite3 with `.safeIntegers(true)` for BigInt timestamps | `db.defaultSafeIntegers(true)` at open time, verified working |
| RS-01 | Open(path) creates or opens a cairn store with zero configuration | `Connection::open(path)` + execute_batch for schema + pragmas |
| RS-02 | Close (Drop) cleanly shuts down with WAL checkpoint | Implement `Drop` trait; execute `PRAGMA wal_checkpoint(TRUNCATE)` |
| RS-03 | Append(topic, payload) returns EventID, atomic write | `conn.execute()` + `conn.last_insert_rowid()` as u64 |
| RS-04 | AppendBatch(events) transactional multi-write, all-or-nothing | `conn.transaction()` with prepared stmt in loop |
| RS-05 | Query(topic, start, end) returns iterator of events in insertion order | `stmt.query_map()` collecting into Vec<Event> |
| RS-06 | All shared test vectors pass | `cargo test` + `std::fs::read_to_string("../spec/vectors/")` |
| RS-07 | Uses rusqlite with bundled feature | `rusqlite = { version = "0.38", features = ["bundled"] }` |
</phase_requirements>

---

## Summary

Phase 3 implements two SDKs in parallel: a TypeScript SDK (Node/Bun) and a Rust SDK, both conforming to the same spec contract already validated by the Go reference implementation. The TypeScript SDK uses `better-sqlite3` (synchronous, native addon, v12.x) as the mandated SQLite binding; the critical detail is that all integer IDs and nanosecond timestamps must flow through JavaScript as `BigInt` via `db.defaultSafeIntegers(true)`, because nanosecond epoch values (~1.7×10^18) exceed JavaScript's 53-bit safe integer limit. The Rust SDK uses `rusqlite` (v0.38) with the `bundled` feature so the build is self-contained with no system SQLite requirement.

Both SDKs follow the exact same logical structure as `go/cairn.go`: an opaque `Store` struct (or class) wrapping a database connection, with `Open`, `Close`, `Append`, `AppendBatch`, and `Query`. The Go test harness in `go/cairn_test.go` is the template for the vector test harnesses in both new SDKs — the same four JSON files are consumed, the same `store_closed` and `payload_size_bytes` fields are handled, and the same error kind strings are matched. The TypeScript package must ship dual ESM+CJS output with `.d.ts` declarations using `tsdown`.

The primary technical risks are: (1) BigInt precision in TypeScript — any single path that returns an integer as `number` rather than `bigint` silently corrupts large timestamps; (2) the Rust `Drop` trait for WAL checkpoint — panicking in `Drop` is dangerous, so checkpoint errors must be silently swallowed; (3) better-sqlite3's native addon requires prebuilt binaries or node-gyp at install time.

**Primary recommendation:** Mirror the Go module structure exactly. Implement TypeScript in `ts/` and Rust in `rs/`. In TypeScript, call `db.defaultSafeIntegers(true)` immediately after opening so every integer path is BigInt without per-statement configuration.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| better-sqlite3 | 12.8.0 | Synchronous SQLite bindings for Node.js | Mandated by TS-07; fastest Node.js SQLite library; synchronous API eliminates async complexity; native `.safeIntegers()` for BigInt |
| @types/better-sqlite3 | 7.6.x | TypeScript type declarations for better-sqlite3 | DefinitelyTyped package; required for typed `Database`, `Statement`, `RunResult` |
| rusqlite | 0.38.0 | Ergonomic SQLite bindings for Rust | Mandated by RS-07; most widely used Rust SQLite crate; `bundled` feature eliminates system SQLite dependency |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| tsdown | 0.21.x | TypeScript library bundler (Rolldown-based) | Mandated for dual ESM+CJS output with .d.ts; single config file, auto-writes package.json exports |
| vitest | latest stable | Test runner for TypeScript | Mandated by success criteria; `vitest run` produces clean failure-only output; supports TypeScript natively |
| typescript | 5.x | TypeScript compiler | Required by tsdown and vitest; type checking |
| libsqlite3-sys | (transitive) | SQLite C source bundled into Rust binary | Bundled automatically via rusqlite `bundled` feature; SQLite 3.51.1 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| better-sqlite3 | node-sqlite3 | node-sqlite3 is async and callback-based — unsuitable for synchronous API contract |
| better-sqlite3 | bun:sqlite | bun:sqlite is Bun-only; TS-07 locks to better-sqlite3 |
| rusqlite | sqlx | sqlx is async; rusqlite is synchronous and simpler for this use case |
| tsdown | tsup | tsdown is tsup's successor (same author, Rolldown-based, faster); tsdown mandated |

### Installation

TypeScript:
```bash
npm install better-sqlite3
npm install --save-dev @types/better-sqlite3 tsdown typescript vitest
```

Rust (`rs/Cargo.toml`):
```toml
[dependencies]
rusqlite = { version = "0.38", features = ["bundled"] }

[dev-dependencies]
# No extra test deps needed — serde_json could help but std JSON parsing is sufficient
```

---

## Architecture Patterns

### Recommended Project Structure

```
ts/
├── src/
│   ├── index.ts         # Public API: Open, Store class, error classes, types
│   └── errors.ts        # Error class definitions (optional split)
├── tests/
│   ├── cairn.test.ts    # Unit tests + spec vector harness
│   └── vectors.test.ts  # Can be same file or split
├── package.json
├── tsconfig.json
├── tsdown.config.ts
└── dist/                # Built output (gitignored)
    ├── index.mjs
    ├── index.cjs
    └── index.d.ts

rs/
├── src/
│   └── lib.rs           # Public API: Store struct, Open fn, impl Drop
├── tests/
│   └── vectors.rs       # Integration tests consuming spec vectors
├── Cargo.toml
└── Cargo.lock
```

### Pattern 1: TypeScript Store Class

**What:** A `Store` class wrapping a `better-sqlite3` `Database` instance, with a `closed` boolean flag for idempotent close.
**When to use:** All TypeScript SDK code follows this pattern.

```typescript
// Source: better-sqlite3 docs/api.md + Go reference implementation pattern
import Database from 'better-sqlite3'

export class Store {
  private db: Database.Database
  private closed = false

  constructor(db: Database.Database) {
    this.db = db
  }

  close(): void {
    if (this.closed) return
    this.closed = true
    this.db.pragma('wal_checkpoint(TRUNCATE)')
    this.db.close()
  }

  private checkOpen(): void {
    if (this.closed) throw new StoreNotOpenError()
  }
}

export function open(path: string): Store {
  // Parent dir check: fs.statSync(dir) — throw StorageError if not found
  const db = new Database(path)
  db.defaultSafeIntegers(true)  // ALL integers become BigInt — critical for nanosecond ts
  db.pragma('journal_mode = WAL')
  db.pragma('busy_timeout = 5000')
  db.pragma('foreign_keys = ON')
  db.pragma('defensive = ON')   // SQLITE_DBCONFIG_DEFENSIVE via pragma alias
  db.exec(schemaSQL)            // idempotent IF NOT EXISTS
  return new Store(db)
}
```

### Pattern 2: TypeScript Append with BigInt

**What:** `stmt.run()` returns `{ lastInsertRowid: bigint }` when `defaultSafeIntegers(true)` is set.
**When to use:** `Append` and `AppendBatch` event ID collection.

```typescript
// Source: better-sqlite3 docs/api.md, docs/integer.md
append(topic: string, payload: Uint8Array): bigint {
  this.checkOpen()
  if (topic === '') throw new EmptyTopicError()
  if (payload.length === 0) throw new EmptyPayloadError()
  if (payload.length > MAX_PAYLOAD_SIZE) throw new PayloadTooLargeError()

  const ts = process.hrtime.bigint()  // nanoseconds as BigInt
  const result = this.insertStmt.run(topic, ts, payload)
  return result.lastInsertRowid as bigint  // safe: defaultSafeIntegers(true)
}
```

### Pattern 3: TypeScript AppendBatch with db.transaction()

**What:** Wrap multi-insert in a transaction function. `db.transaction(fn)` commits on return, rolls back on throw.
**When to use:** `AppendBatch`.

```typescript
// Source: better-sqlite3 docs/api.md — db.transaction() pattern
appendBatch(events: { topic: string; payload: Uint8Array }[]): bigint[] {
  this.checkOpen()
  if (events.length === 0) return []
  // Validate ALL before insert
  for (const e of events) {
    if (e.topic === '') throw new EmptyTopicError()
    if (e.payload.length === 0) throw new EmptyPayloadError()
    if (e.payload.length > MAX_PAYLOAD_SIZE) throw new PayloadTooLargeError()
  }
  const insertBatch = this.db.transaction((evts: typeof events) => {
    return evts.map(e => {
      const ts = process.hrtime.bigint()
      const r = this.insertStmt.run(e.topic, ts, e.payload)
      return r.lastInsertRowid as bigint
    })
  })
  return insertBatch(events)
}
```

### Pattern 4: TypeScript Query with BigInt timestamps

**What:** `stmt.all()` with BigInt parameters; result rows have BigInt `ts` and `id` fields.
**When to use:** `Query`.

```typescript
// Source: better-sqlite3 docs/api.md
query(topic: string, start: bigint, end: bigint): Event[] {
  this.checkOpen()
  const rows = this.queryStmt.all(topic, start, end) as RawRow[]
  return rows.map(r => ({
    id: r.id as bigint,
    topic: r.topic as string,
    ts: r.ts as bigint,
    payload: r.payload as Buffer,
  }))
}
```

### Pattern 5: TypeScript Error Classes

**What:** Extend `Error` with a discriminator `kind` property matching spec error_kind strings.
**When to use:** All TypeScript error types.

```typescript
// Source: spec/api.md — Per-Language Idiomatic Naming
export class PayloadTooLargeError extends Error {
  readonly kind = 'payload_too_large'
  constructor() { super('cairn: payload too large'); this.name = 'PayloadTooLargeError' }
}
export class StoreNotOpenError extends Error {
  readonly kind = 'store_not_open'
  constructor() { super('cairn: store not open'); this.name = 'StoreNotOpenError' }
}
export class EmptyTopicError extends Error {
  readonly kind = 'empty_topic'
  constructor() { super('cairn: empty topic'); this.name = 'EmptyTopicError' }
}
export class EmptyPayloadError extends Error {
  readonly kind = 'empty_payload'
  constructor() { super('cairn: empty payload'); this.name = 'EmptyPayloadError' }
}
```

### Pattern 6: Rust Store with Drop for WAL checkpoint

**What:** A `Store` struct wrapping a `Connection`. `Drop` runs the WAL checkpoint.
**When to use:** All Rust SDK code follows this pattern.

```rust
// Source: rusqlite docs.rs Connection, spec/api.md Rust error catalog
use rusqlite::{Connection, Result as SqlResult, config::DbConfig};

pub struct Store {
    conn: Connection,
    closed: bool,
}

impl Store {
    pub fn close(&mut self) -> Result<(), Error> {
        if self.closed { return Ok(()); }
        self.closed = true;
        // WAL checkpoint — ignore errors per spec (Close is always safe)
        let _ = self.conn.execute_batch("PRAGMA wal_checkpoint(TRUNCATE)");
        Ok(())
    }

    fn check_open(&self) -> Result<(), Error> {
        if self.closed { Err(Error::StoreNotOpen) } else { Ok(()) }
    }
}

impl Drop for Store {
    fn drop(&mut self) {
        // Swallow all errors — Drop must not panic
        let _ = self.close();
    }
}

pub fn open(path: &str) -> Result<Store, Error> {
    // Parent dir check first: std::path::Path::new(path).parent()...
    let conn = Connection::open(path).map_err(|e| Error::Storage(e.to_string()))?;
    conn.execute_batch("
        PRAGMA journal_mode = WAL;
        PRAGMA busy_timeout = 5000;
        PRAGMA foreign_keys = ON;
    ").map_err(|e| Error::Storage(e.to_string()))?;
    conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)
        .map_err(|e| Error::Storage(e.to_string()))?;
    conn.execute_batch(SCHEMA_SQL).map_err(|e| Error::Storage(e.to_string()))?;
    Ok(Store { conn, closed: false })
}
```

### Pattern 7: Rust AppendBatch with transaction

**What:** Use `conn.transaction()` to get a `Transaction`, prepare a statement, insert in loop.
**When to use:** `AppendBatch`.

```rust
// Source: rusqlite docs.rs Transaction + Statement
pub fn append_batch(&mut self, events: &[BatchEvent]) -> Result<Vec<u64>, Error> {
    self.check_open()?;
    if events.is_empty() { return Ok(vec![]); }
    // Validate all before any insert
    for e in events {
        if e.topic.is_empty() { return Err(Error::EmptyTopic); }
        if e.payload.is_empty() { return Err(Error::EmptyPayload); }
        if e.payload.len() > MAX_PAYLOAD_SIZE { return Err(Error::PayloadTooLarge); }
    }
    let tx = self.conn.transaction().map_err(|e| Error::Storage(e.to_string()))?;
    let mut ids = Vec::with_capacity(events.len());
    {
        let mut stmt = tx.prepare(
            "INSERT INTO events (topic, ts, payload) VALUES (?1, ?2, ?3)"
        ).map_err(|e| Error::Storage(e.to_string()))?;
        for e in events {
            let ts = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos() as i64;
            stmt.execute(rusqlite::params![e.topic, ts, e.payload])
                .map_err(|e| Error::Storage(e.to_string()))?;
            ids.push(tx.last_insert_rowid() as u64);
        }
    }
    tx.commit().map_err(|e| Error::Storage(e.to_string()))?;
    Ok(ids)
}
```

### Pattern 8: Rust Query with query_map

**What:** `stmt.query_map()` to iterate rows into `Vec<Event>`.
**When to use:** `Query`.

```rust
// Source: rusqlite docs.rs Statement::query_map
pub fn query(&self, topic: &str, start: i64, end: i64) -> Result<Vec<Event>, Error> {
    self.check_open()?;
    let mut stmt = self.conn.prepare(
        "SELECT id, topic, ts, payload FROM events WHERE topic = ?1 AND ts >= ?2 AND ts <= ?3 ORDER BY id ASC"
    ).map_err(|e| Error::Storage(e.to_string()))?;
    let events = stmt.query_map(rusqlite::params![topic, start, end], |row| {
        Ok(Event {
            id:      row.get::<_, i64>(0)? as u64,
            topic:   row.get(1)?,
            ts:      row.get(2)?,
            payload: row.get(3)?,
        })
    }).map_err(|e| Error::Storage(e.to_string()))?
    .collect::<SqlResult<Vec<_>>>()
    .map_err(|e| Error::Storage(e.to_string()))?;
    Ok(events)
}
```

### Pattern 9: TypeScript test vector harness

**What:** Mirrors Go's `readVectorFile` + JSON parsing, using `fs.readFileSync` with relative path from test file.
**When to use:** vitest vector test file.

```typescript
// Source: go/cairn_test.go pattern adapted for vitest
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, it, expect } from 'vitest'

function readVectorFile(name: string): unknown {
  // vitest CWD is the package root (ts/), so ../spec/vectors/ is correct
  const raw = readFileSync(join(__dirname, '../../spec/vectors', name), 'utf-8')
  return JSON.parse(raw)
}
```

### Pattern 10: Rust test vector harness

**What:** Mirrors Go's `readVectorFile`, using `std::fs::read_to_string` relative to manifest dir.
**When to use:** Rust integration test in `tests/vectors.rs`.

```rust
// Source: go/cairn_test.go pattern adapted for Rust
// cargo test CWD is crate root (rs/), so ../spec/vectors/ is correct
fn read_vector_file(name: &str) -> String {
    std::fs::read_to_string(format!("../spec/vectors/{}", name))
        .unwrap_or_else(|e| panic!("read_vector_file {}: {}", name, e))
}
```

### Anti-Patterns to Avoid

- **Omitting `db.defaultSafeIntegers(true)`:** Without this, `lastInsertRowid` and scanned integer columns return `number`, causing silent precision loss for nanosecond timestamps. Setting it per-statement via `stmt.safeIntegers()` is error-prone — set it once at database level.
- **Using `Number()` to convert BigInt timestamps in test assertions:** The test vectors encode timestamps as quoted strings precisely because they cannot be compared as JSON numbers. Parse with `BigInt(str)`, never `Number(str)`.
- **Panicking in `Drop` (Rust):** The WAL checkpoint in `Drop` must catch and discard all errors. A failing checkpoint should never propagate as a panic — it would unwind in unexpected contexts.
- **Opening multiple connections to the same file (TypeScript):** better-sqlite3 is synchronous; one connection is sufficient and avoids `SQLITE_BUSY`. Never pool connections for this workload.
- **Using `db.exec()` for multi-statement schema without error handling:** `db.exec(schemaSQL)` throws synchronously on error. Wrap in try/catch to convert to a `StorageError`.
- **Async TypeScript tests with better-sqlite3:** better-sqlite3 is 100% synchronous; all test code should be synchronous. Marking vitest tests `async` when they are not adds no value.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQLite connection management (TS) | Custom SQLite wrapper | better-sqlite3 `Database` class | Thread safety, connection lifecycle, statement caching already handled |
| Transaction rollback on error (TS) | Manual BEGIN/ROLLBACK/COMMIT | `db.transaction(fn)` | Automatic rollback on exception; handles nested savepoints |
| BigInt integer handling (TS) | Custom integer marshaling | `db.defaultSafeIntegers(true)` | Transparently converts all integers to BigInt without per-column code |
| SQLite bundling (Rust) | Vendoring SQLite C source | `rusqlite` `bundled` feature | cc crate compiles and links SQLite 3.51.1 automatically |
| Transaction safety (Rust) | Manual `BEGIN`/`COMMIT` SQL | `conn.transaction()` + `tx.commit()` | `Transaction` drops with rollback by default; no leak on panic |
| Defensive mode (Rust) | SQL PRAGMA workaround | `conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)` | Direct C API binding; PRAGMA approach is not equivalent |
| Dual ESM/CJS output | Custom rollup config | `tsdown` | Auto-writes `exports` field in package.json; handles `.d.ts` generation |

**Key insight:** The entire surface of these SDKs is thin wrappers — the SQLite libraries handle all the hard parts. Resist any temptation to add abstraction layers or custom error wrapping beyond what the spec requires.

---

## Common Pitfalls

### Pitfall 1: BigInt precision loss via JSON.parse in test harness

**What goes wrong:** The test vector files contain timestamps as quoted strings (e.g., `"1710000000000000001"`). If the harness does `JSON.parse(raw)` and then accesses a numeric field that was NOT quoted, it silently loses precision.
**Why it happens:** All fields in the vector files are explicitly quoted strings, but if a future vector were to use unquoted numbers, `JSON.parse` would corrupt them.
**How to avoid:** Always parse timestamp fields with `BigInt(field)` from the string value. Never pass timestamp strings through `Number()`.
**Warning signs:** Test vector TC3 in query.json involves a single-timestamp range (start == end == ts). If precision is lost, the test returns 0 events instead of 1.

### Pitfall 2: better-sqlite3 native addon not found (CI / Bun)

**What goes wrong:** `npm install better-sqlite3` requires either prebuilt binaries (downloaded for the target platform) or a working node-gyp build environment. In CI or non-standard environments, `better_sqlite3.node` may be missing.
**Why it happens:** better-sqlite3 is a native C++ addon; it cannot be loaded from a pure-JS bundle.
**How to avoid:** Ensure CI has `node-gyp` dependencies (build-essential, python3 on Linux). For Bun, use Bun's native FFI or verify that Bun supports better-sqlite3 native addons at the target version. v1 requirement says Node/Bun; better-sqlite3 works with Bun but requires the addon to be built for the correct runtime.
**Warning signs:** `Error: Cannot find module 'better-sqlite3'` or native binding path errors.

### Pitfall 3: Rust Drop silently ignoring all connection errors

**What goes wrong:** If the WAL checkpoint in `Drop` fails (e.g., the database file was deleted), the error is discarded. For tests, this is correct; for production callers, it means they might not know the checkpoint failed.
**Why it happens:** `Drop` cannot return an error in Rust. The spec says "Close is always safe to call" — this is the correct behavior.
**How to avoid:** Document that `close()` (the explicit method) should be called in application code if checkpoint confirmation is needed. `Drop` is a safety net, not the primary close path.
**Warning signs:** No warnings — this is expected behavior per spec.

### Pitfall 4: TypeScript `lastInsertRowid` typed as `number | bigint`

**What goes wrong:** The `@types/better-sqlite3` types declare `lastInsertRowid` as `number | bigint`. After calling `db.defaultSafeIntegers(true)`, it is always `bigint` at runtime, but TypeScript requires a type assertion or narrowing.
**Why it happens:** The type definition reflects both possible modes.
**How to avoid:** Use `result.lastInsertRowid as bigint` after any `stmt.run()` call when `defaultSafeIntegers(true)` is set.
**Warning signs:** TypeScript errors saying "cannot assign bigint to number".

### Pitfall 5: Rust `execute_batch` vs `execute` for schema DDL

**What goes wrong:** `Connection::execute()` runs a single statement. The schema DDL in `spec/schema.sql` has multiple statements (CREATE TABLE, CREATE INDEX, CREATE TRIGGER x2). Passing it to `execute()` will fail or silently skip statements.
**Why it happens:** `execute()` uses `sqlite3_prepare_v2` which only compiles the first statement in the input.
**How to avoid:** Use `conn.execute_batch(SCHEMA_SQL)` which calls `sqlite3_exec` and runs all semicolon-separated statements.
**Warning signs:** Tests fail with "no such table: events" — the table wasn't created.

### Pitfall 6: TypeScript vitest path resolution for vector files

**What goes wrong:** The Go harness uses `../spec/vectors/` relative to the package directory. In vitest, `__dirname` may not be available in ESM mode.
**Why it happens:** `__dirname` is a CommonJS global. In ESM, use `import.meta.url` + `fileURLToPath`.
**How to avoid:** Use `new URL('../../spec/vectors/' + name, import.meta.url)` or configure `vitest` with `globals: false` and import explicitly. Alternatively, keep the test file as CJS by naming it `.test.cjs` or set `"type": "commonjs"` in the test package.
**Warning signs:** `ReferenceError: __dirname is not defined`.

### Pitfall 7: Rust timestamp i64 vs u64 overflow

**What goes wrong:** `std::time::SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos()` returns `u128`. Casting to `i64` will truncate. For current epoch nanoseconds (~1.7×10^18), this value fits in i64 (max ~9.2×10^18), but the cast must be deliberate.
**Why it happens:** SQLite stores timestamps as `INTEGER` (i64). Rust's duration API returns u128 for nanoseconds.
**How to avoid:** Cast via `as i64` with a comment noting the valid range. Year 2262 is when i64 overflows for nanosecond timestamps — well beyond v1 scope.
**Warning signs:** Negative timestamps in the database, or test vector mismatches.

---

## Code Examples

Verified patterns from official sources:

### better-sqlite3: Open with safeIntegers

```typescript
// Source: https://github.com/WiseLibs/better-sqlite3/blob/master/docs/api.md
//         https://github.com/WiseLibs/better-sqlite3/blob/master/docs/integer.md
import Database from 'better-sqlite3'

const db = new Database('/path/to/store.db')
db.defaultSafeIntegers(true)  // all integers -> BigInt
db.pragma('journal_mode = WAL')
db.pragma('busy_timeout = 5000')
db.pragma('foreign_keys = ON')
db.pragma('defensive = ON')   // better-sqlite3 exposes SQLITE_DBCONFIG_DEFENSIVE as PRAGMA
```

### better-sqlite3: Transaction pattern

```typescript
// Source: https://github.com/WiseLibs/better-sqlite3/blob/master/docs/api.md
const insertMany = db.transaction((items: Array<{topic: string; ts: bigint; payload: Buffer}>) => {
  const stmt = db.prepare('INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)')
  return items.map(item => {
    const info = stmt.run(item.topic, item.ts, item.payload)
    return info.lastInsertRowid as bigint
  })
})
const ids = insertMany(items)  // auto-commits, rolls back on throw
```

### rusqlite: Open with SQLITE_DBCONFIG_DEFENSIVE

```rust
// Source: https://docs.rs/rusqlite/latest/rusqlite/struct.Connection.html
//         https://docs.rs/rusqlite/latest/rusqlite/config/enum.DbConfig.html
use rusqlite::{Connection, config::DbConfig};

let conn = Connection::open(path)?;
conn.execute_batch("
    PRAGMA journal_mode = WAL;
    PRAGMA busy_timeout = 5000;
    PRAGMA foreign_keys = ON;
")?;
conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)?;
conn.execute_batch(SCHEMA_SQL)?;
```

### rusqlite: Query with query_map

```rust
// Source: https://docs.rs/rusqlite/latest/rusqlite/struct.Statement.html
let mut stmt = conn.prepare(
    "SELECT id, topic, ts, payload FROM events WHERE topic=?1 AND ts>=?2 AND ts<=?3 ORDER BY id ASC"
)?;
let events: Vec<Event> = stmt.query_map(params![topic, start, end], |row| {
    Ok(Event {
        id:      row.get::<_, i64>(0)? as u64,
        topic:   row.get(1)?,
        ts:      row.get(2)?,
        payload: row.get(3)?,
    })
})?.collect::<rusqlite::Result<_>>()?;
```

### tsdown.config.ts: Dual ESM+CJS with declarations

```typescript
// Source: https://tsdown.dev/guide/getting-started
import { defineConfig } from 'tsdown'

export default defineConfig({
  entry: ['./src/index.ts'],
  format: ['esm', 'cjs'],
  dts: true,
})
```

### tsdown: package.json exports

```json
{
  "type": "module",
  "main": "./dist/index.cjs",
  "module": "./dist/index.mjs",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "require": "./dist/index.cjs",
      "import": "./dist/index.mjs",
      "types": "./dist/index.d.ts"
    }
  }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tsup (esbuild-based) | tsdown (Rolldown-based) | 2024-2025 | tsdown is the successor from the same author; faster, better tree-shaking |
| node-sqlite3 (async, callbacks) | better-sqlite3 (sync) | Stable since v7 | Synchronous API removes async complexity for library code |
| rusqlite manual SQLite bundling | `rusqlite` `bundled` feature | Stable | cc crate compiles SQLite 3.51.1 from embedded source; no system SQLite needed |
| `db.safeIntegers()` per-statement | `db.defaultSafeIntegers(true)` | Stable since v5 | Database-level default eliminates per-statement configuration risk |

**Deprecated/outdated:**
- `node-sqlite3`: async callback-based API is unsuitable for synchronous library contract; do not use.
- `tsup` for new projects: tsdown supersedes it with Rolldown backend; tsdown is mandated.

---

## Open Questions

1. **Bun compatibility for better-sqlite3**
   - What we know: better-sqlite3 is a native C++ addon. Bun supports Node.js native addons via its Node.js compatibility layer. The success criteria say "Node/Bun."
   - What's unclear: Whether Bun's Node addon compatibility reliably loads `better_sqlite3.node` at the current Bun version (1.x). Bun has its own built-in SQLite (`bun:sqlite`), but TS-07 mandates better-sqlite3.
   - Recommendation: Implement and test against Node.js first. Add a CI step for Bun if Bun addon support is confirmed. The spec constraint (TS-07) is clear — do not switch to `bun:sqlite`.

2. **TypeScript `__dirname` in ESM vitest context**
   - What we know: vitest supports both ESM and CJS. If `"type": "module"` is set in package.json, test files run as ESM where `__dirname` is unavailable.
   - What's unclear: Whether the test file should use `import.meta.url` + `fileURLToPath` or whether a CJS-mode vitest config is simpler for this package.
   - Recommendation: Use `import.meta.dirname` (Node 21.2+) or `fileURLToPath(new URL('../../spec/vectors/' + name, import.meta.url))` for portability. This is a Wave 0 setup decision for the planner.

3. **Rust Cargo workspace vs standalone crate**
   - What we know: The Go SDK lives in `go/` as a standalone module. The Rust crate should live in `rs/` similarly.
   - What's unclear: Whether to create a workspace at the repo root (combining `ts/` and `rs/`) or keep each language completely independent.
   - Recommendation: Keep `rs/` as a standalone Cargo crate (no workspace). Consistency with the Go module approach; simpler per-language CI.

---

## Validation Architecture

### Test Framework

**TypeScript:**

| Property | Value |
|----------|-------|
| Framework | vitest (latest stable) |
| Config file | `ts/vitest.config.ts` — Wave 0 creation |
| Quick run command | `cd ts && npx vitest run --reporter=verbose` |
| Full suite command | `cd ts && npx vitest run` |

**Rust:**

| Property | Value |
|----------|-------|
| Framework | Rust built-in test harness (`cargo test`) |
| Config file | none (Cargo.toml `[dev-dependencies]` only) |
| Quick run command | `cd rs && rtk cargo test` |
| Full suite command | `cd rs && rtk cargo test` |

### Phase Requirements — Test Map

| Req ID | Behavior | Test Type | Automated Command |
|--------|----------|-----------|-------------------|
| TS-01 | Open creates/opens DB, applies schema, WAL mode | unit | `cd ts && npx vitest run -t "Open"` |
| TS-02 | Close checkpoints WAL, idempotent | unit | `cd ts && npx vitest run -t "Close"` |
| TS-03 | Append returns BigInt EventID, validates inputs | unit + vectors | `cd ts && npx vitest run -t "Append"` |
| TS-04 | AppendBatch all-or-nothing, validates all events | unit + vectors | `cd ts && npx vitest run -t "AppendBatch"` |
| TS-05 | Query returns events in id ASC order, BigInt ts round-trips | unit + vectors | `cd ts && npx vitest run -t "Query"` |
| TS-06 | All spec vector files pass | vectors | `cd ts && npx vitest run` |
| TS-07 | better-sqlite3 with safeIntegers — no Number integer paths | unit | verified by `db.defaultSafeIntegers(true)` call in Open |
| RS-01 | open() creates/opens DB, applies schema, WAL mode | integration | `cd rs && cargo test test_open` |
| RS-02 | Drop runs WAL checkpoint | integration | `cd rs && cargo test test_drop` |
| RS-03 | append() returns u64 EventID, validates inputs | integration + vectors | `cd rs && cargo test append` |
| RS-04 | append_batch() all-or-nothing, validates all events | integration + vectors | `cd rs && cargo test append_batch` |
| RS-05 | query() returns Vec<Event> in id ASC order | integration + vectors | `cd rs && cargo test query` |
| RS-06 | All spec vector files pass | integration | `cd rs && cargo test vectors` |
| RS-07 | bundled feature — no system SQLite required | build | `cd rs && cargo build` (no libsqlite3-dev installed) |

### Sampling Rate

- **Per task commit:** `cd ts && npx vitest run` and `cd rs && rtk cargo test`
- **Per wave merge:** Full suite for both SDKs
- **Phase gate:** Both full suites green before `/gsd:verify-work`

### Wave 0 Gaps

TypeScript:
- [ ] `ts/package.json` — fresh npm package with better-sqlite3, vitest, tsdown devDeps
- [ ] `ts/tsconfig.json` — target ESNext, moduleResolution bundler
- [ ] `ts/tsdown.config.ts` — dual ESM+CJS with dts:true
- [ ] `ts/vitest.config.ts` — minimal vitest config

Rust:
- [ ] `rs/Cargo.toml` — `rusqlite = { version = "0.38", features = ["bundled"] }`
- [ ] `rs/src/lib.rs` — empty library crate stub

---

## Sources

### Primary (HIGH confidence)

- better-sqlite3 GitHub `docs/api.md` — Database constructor, pragma, prepare, run, all, transaction, safeIntegers API
- better-sqlite3 GitHub `docs/integer.md` — BigInt/safeIntegers semantics, `db.defaultSafeIntegers()` behavior
- rusqlite `docs.rs` Connection struct — `open`, `execute`, `prepare`, `last_insert_rowid`, `transaction`, `set_db_config` signatures
- rusqlite `docs.rs` DbConfig enum — `SQLITE_DBCONFIG_DEFENSIVE` variant and `set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)` usage
- tsdown.dev Getting Started — `defineConfig`, `format: ['esm', 'cjs']`, `dts: true`, package.json exports template
- spec/api.md — Per-language mechanism for defensive mode (TypeScript: `db.pragma('defensive=ON')`)
- go/cairn.go, go/cairn_test.go — Reference implementation patterns to mirror

### Secondary (MEDIUM confidence)

- WebSearch: rusqlite 0.38.0 current stable (December 2025), bundled feature bundles SQLite 3.51.1 (confirmed by multiple sources)
- WebSearch: better-sqlite3 12.8.0 current version (npm, March 2026)
- WebSearch: tsdown 0.21.x current version, built on Rolldown

### Tertiary (LOW confidence)

- Bun compatibility with better-sqlite3 native addon — documented as supported but not independently verified for current Bun version; flag for Wave 0 validation

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions verified via WebSearch against official sources; spec mandates specific libraries
- Architecture: HIGH — patterns derived from Go reference implementation + verified library API docs
- Pitfalls: HIGH (TS BigInt, Rust Drop, execute_batch) / MEDIUM (Bun compat) — BigInt pitfall is mathematically certain; Bun compat is MEDIUM

**Research date:** 2026-03-18
**Valid until:** 2026-04-18 (stable ecosystem; rusqlite and better-sqlite3 move slowly)
