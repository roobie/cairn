# Pitfalls Research

**Domain:** Append-only event storage SDK wrapping SQLite, multi-language (Go/TypeScript/Rust) monorepo with shared test vectors
**Researched:** 2026-03-18
**Confidence:** HIGH (SQLite pitfalls well-documented in official sources; monorepo pitfalls MEDIUM from community sources)

---

## Critical Pitfalls

### Pitfall 1: WAL Checkpoint Starvation — WAL File Grows Without Bound

**What goes wrong:**
In WAL mode, a checkpoint cannot reset (recycle) the WAL file while any reader has an open transaction. If cairn has continuous read activity — which is the expected pattern for IoT telemetry readers polling constantly — the WAL file grows without bound. Production reports show WAL files reaching 400MB+ before the issue is noticed. Read performance degrades as the WAL grows because readers must scan an ever-longer WAL before reaching the main database file.

**Why it happens:**
SQLite WAL auto-checkpoint fires at 1000 pages by default, but it can only run to completion (and reset the file) when zero readers are active. A single long-running or un-closed read cursor is enough to block the reset indefinitely. The `better-sqlite3` docs note that a `cursor.execute()` without `cursor.fetchall()` leaves a pending prepared statement that blocks checkpointing. The same class of bug exists in all three language bindings.

**How to avoid:**
- Set `busy_timeout` on all connections (5000ms minimum).
- Explicitly call `PRAGMA wal_checkpoint(TRUNCATE)` in cairn's `Close()` path on the writer connection.
- For long-lived processes, periodically trigger a checkpoint after batch appends exceed a threshold (e.g., every 10K events or 60 seconds).
- Always fully consume iterators returned from `Query` — document this contract for callers.
- Consider exposing a `cairn.Checkpoint()` method in v1 for callers who run long-lived processes.

**Warning signs:**
- `.wal` sidecar file growing past 10MB during normal operation.
- Read latency increasing over time without corresponding write increase.
- `du -sh *.db *.db-wal` showing WAL significantly larger than the database file.

**Phase to address:** Spec phase (document the checkpoint contract); Go SDK phase (implement checkpoint in `Close()` and batch append path); validate in all three language test suites.

---

### Pitfall 2: AUTOINCREMENT Overhead on the Hot Write Path

**What goes wrong:**
The schema uses `INTEGER PRIMARY KEY AUTOINCREMENT`. The `AUTOINCREMENT` keyword causes SQLite to write to the internal `sqlite_sequence` table on every insert to enforce "never reuse a rowid" semantics. For a pure append-only store (where rowid reuse is structurally impossible anyway since there are no deletes), this is wasted overhead — SQLite's official docs call it out as extra CPU, memory, and disk I/O that should be avoided if not strictly needed.

**Why it happens:**
`AUTOINCREMENT` is the intuitive choice for a "give me a unique ID" column, but cairn never deletes, so the reuse-prevention guarantee is redundant. Plain `INTEGER PRIMARY KEY` aliases the rowid and is always monotonically increasing as long as the table is never truncated — which cairn enforces at the trigger level.

**How to avoid:**
Drop `AUTOINCREMENT` from the schema. Use `INTEGER PRIMARY KEY` only. The rowid still auto-increments; it just doesn't carry the sequence-table overhead.

```sql
-- Use this:
id INTEGER PRIMARY KEY,
-- Not this:
id INTEGER PRIMARY KEY AUTOINCREMENT,
```

**Warning signs:**
- Benchmark shows insert throughput well below the 50K events/sec target when individual inserts are tested.
- `EXPLAIN QUERY PLAN` on an insert shows access to `sqlite_sequence`.

**Phase to address:** Spec phase — fix the schema before any implementation builds against it.

---

### Pitfall 3: Nanosecond Timestamps Stored as INTEGER Exceed Language-Level Integer Ranges

**What goes wrong:**
Nanoseconds since Unix epoch for dates beyond 2262 exceed 2^63-1 (the maximum for a signed 64-bit integer). More immediately, the current epoch in nanoseconds is already a 19-digit number (~1.7 × 10^18), which is within signed int64 range but close to the edge. The real problem is language-level: JavaScript's `Number` type has only 53 bits of mantissa precision, so nanosecond timestamps passed as plain JS numbers lose precision and become non-unique. Prisma has a documented issue where 19-digit nanosecond integers cause integer conversion errors in the TypeScript layer.

**Why it happens:**
The schema stores `ts INTEGER NOT NULL` which is correct at the SQLite level (SQLite integers are up to 8-byte signed). But the TypeScript SDK using `better-sqlite3` returns large integers as JavaScript `number` by default, losing the last ~3 digits of precision. This makes events that are nanoseconds apart appear to have the same timestamp.

**How to avoid:**
- In the TypeScript SDK: use `BigInt` for timestamp values, or use `better-sqlite3`'s `.safeIntegers(true)` mode which returns int64 values as BigInt instead of number.
- In the spec/test-vectors: represent timestamps as strings in JSON (e.g., `"1710000000000000000"`) not as JSON numbers, since JSON numbers have the same 53-bit mantissa constraint.
- Document the BigInt requirement prominently in the TypeScript API surface — callers passing nanosecond timestamps as `number` is a latent bug.

**Warning signs:**
- Test vectors with nanosecond-apart events pass in Go and Rust but fail (return same timestamp) in TypeScript.
- Sorting events by `ts` in TypeScript produces non-deterministic order for events written in rapid succession.

**Phase to address:** Spec phase (test vector timestamp representation); TypeScript SDK phase (enforce BigInt/safeIntegers).

---

### Pitfall 4: Trigger-Based Immutability Is Bypassable Without Defensive Mode

**What goes wrong:**
The `no_update` and `no_delete` triggers prevent accidental modification through the normal SQLite API, but they do not constitute a security boundary. An attacker or misbehaving caller with direct file access, `ATTACH DATABASE` access, or the ability to disable triggers via `PRAGMA writable_schema` can bypass them. Additionally, someone using `VACUUM INTO` or `ATTACH` on the cairn database file from another connection gets a copy without the triggers enforced if they recreate the schema.

**Why it happens:**
SQLite triggers are a data integrity mechanism, not a security mechanism. The project brief acknowledges "the file is cairn's, not yours," but the SDK itself opens the file — any code in the same process that gets a reference to the raw `*sql.DB` or equivalent can execute arbitrary SQL including `PRAGMA writable_schema=ON`.

**How to avoid:**
- Enable `SQLITE_DBCONFIG_DEFENSIVE` on all cairn database connections (available in SQLite 3.26.0+, 2018). This prevents writes to shadow tables and schema version changes from SQL-level injection.
- Never expose the underlying database handle through the public API. The SDK wrapper must be the only path.
- Document the trust model clearly: cairn enforces immutability against accidental misuse by the application using cairn, not against a malicious actor with filesystem access.
- In Rust (`rusqlite`) and Go (`modernc`/`mattn`): set defensive mode immediately after opening the connection, before returning the cairn handle to the caller.

**Warning signs:**
- Public API exposes any `DB()`, `Conn()`, or `Handle()` method that returns the raw database connection.
- Integration tests can execute `DELETE FROM events` without getting an error (means triggers aren't installed or defensive mode isn't set).

**Phase to address:** Go SDK phase (set defensive mode in `Open()`); replicate in TypeScript and Rust SDKs. Verify in shared test vectors with a "direct delete returns error" test.

---

### Pitfall 5: File Rotation Creates an Invisible Consistency Boundary

**What goes wrong:**
When cairn rotates from one segment file to the next (daily rotation), a `Query(topic, start, end)` that spans the rotation boundary must open and query multiple files. If this is not implemented correctly, queries that cross midnight return incomplete results — silently, not with an error. This is the class of bug that passes all unit tests (which typically don't cross rotation boundaries) and only surfaces in production after 24 hours.

**Why it happens:**
The rotation boundary is a new code path that is easy to miss in test coverage. Each segment is a valid cairn file in isolation; the problem is the aggregation layer that stitches multiple segments together. The rotation logic may also have a race: the writer is rotating to a new file at the same moment a reader queries across the boundary.

**How to avoid:**
- Write rotation boundary tests explicitly in the test vectors, e.g., a test case where `start` is in segment N and `end` is in segment N+1.
- Implement rotation as a purely additive operation: the new segment file is only made visible (added to the segment index) after it is fully opened and the old one is flushed.
- The reader path should open all segment files covering the requested time range before reading any of them — no interleaved open/read.
- Store segment metadata (file paths, time ranges) in a small manifest structure (even just an in-memory sorted list built from the directory listing) so the query path can determine which files to open without filesystem scanning at query time.

**Warning signs:**
- Integration tests only ever use a single-file cairn instance.
- No test explicitly writes events with timestamps on opposite sides of a rotation boundary.
- `Query` tests pass when `end` is in the same segment as `start`.

**Phase to address:** Spec phase (add rotation-boundary test vectors); Go SDK rotation implementation; validate same boundary test vectors in TypeScript and Rust.

---

### Pitfall 6: SQLITE_BUSY Errors Under Concurrent Access Despite Single-Writer Design

**What goes wrong:**
Even with a single-writer process, `SQLITE_BUSY` (database is locked) errors can occur in two scenarios specific to WAL mode:
1. A read transaction that internally upgrades to a write transaction (e.g., the connection was opened with a deferred transaction and encounters a write mid-transaction).
2. Multiple threads within the same process sharing one connection pool without a busy timeout set — the default timeout is zero.

In the TypeScript SDK, `better-sqlite3` uses SYNCHRONOUS=NORMAL in WAL mode by default (the Signal fork documents this), which trades a small durability window for performance but may surprise callers who expect FULL durability.

**Why it happens:**
`busy_timeout` is not a persistent pragma — it must be set on every new connection. Wrappers often set it once on the initial connection but forget to set it on reader connections opened later, or on connections created in worker threads.

**How to avoid:**
- Set `PRAGMA busy_timeout = 5000` immediately after every connection open, before any queries.
- Use `BEGIN IMMEDIATE` for all write transactions — do not rely on deferred transaction upgrades.
- In the Go SDK with `modernc`: configure `sql.DB.SetMaxOpenConns(1)` for the writer connection pool to prevent concurrent write attempts from the same process.
- Document the `SYNCHRONOUS=NORMAL` default in the TypeScript SDK's durability section — callers who need power-loss guarantees should set `PRAGMA synchronous = FULL`.

**Warning signs:**
- Intermittent "database is locked" errors in test suites running concurrent appends and queries.
- Writer throughput drops non-linearly under concurrent read load.

**Phase to address:** Go SDK phase (configure connection after open); TypeScript SDK phase (same); Rust SDK phase (same). All three language SDKs need a connection initialization checklist.

---

### Pitfall 7: Test Vectors Cannot Express Binary Payload Correctly in JSON

**What goes wrong:**
Test vectors are JSON files. JSON has no binary type. The cairn payload field is opaque bytes. Encoding binary payloads as JSON strings requires a convention (base64, hex, UTF-8 literal), but the convention must be specified explicitly and implemented consistently in all three SDKs. If Go uses standard base64, Rust uses URL-safe base64, and TypeScript interprets the field as a UTF-8 string, the test vector "passes" in each language but tests different data — defeating cross-language verification.

**Why it happens:**
The test vector format is designed once by whoever writes the spec first. If the binary encoding convention is implicit or underdocumented, each implementer makes a local assumption. The JSON spec has no binary type, so this ambiguity is structural.

**How to avoid:**
- In `spec/test-vectors/`, explicitly define: all binary payload fields are standard base64 (RFC 4648, not URL-safe, with padding).
- Add a test vector that includes a known binary payload (e.g., a 16-byte sequence with all byte values 0x00–0xFF represented) and specifies the exact expected base64 encoding.
- Add a test vector with a payload that contains bytes that would be invalid UTF-8 — this forces implementers to handle the binary-not-string case.
- Store timestamps in test vectors as JSON strings (quoted), not JSON numbers, to avoid the 53-bit precision loss described in Pitfall 3.

**Warning signs:**
- Test vector payload fields contain only ASCII-safe strings like `"hello world"`.
- No test vector contains a payload with byte values above 0x7F.
- Different language implementations produce different base64 variants for the same binary data.

**Phase to address:** Spec phase — define the encoding convention before any SDK is built.

---

### Pitfall 8: modernc/sqlite Performance Gap May Block Throughput Target

**What goes wrong:**
The project brief recommends `modernc.org/sqlite` (pure Go, no CGo) for cross-compilation convenience. Benchmarks show the pure-Go translation is 25–50% slower than `mattn/go-sqlite3` (CGo) for write-heavy workloads — specifically inserts. The 50K events/sec throughput target is directional, but if the Go SDK is used as the reference implementation for TypeScript and Rust to follow, an unexpectedly slow Go baseline may mislead architecture decisions.

**Why it happens:**
`modernc.org/sqlite` is a machine-translated version of the SQLite C source into Go. The translation overhead is real and consistent across workloads. The performance gap is worst for small, frequent inserts — exactly cairn's primary workload.

**How to avoid:**
- Run throughput benchmarks with `AppendBatch` (bulk insert via a single transaction) as the primary path. Batching amortizes the per-transaction overhead and largely closes the gap between modernc and CGo.
- The 50K events/sec target is achievable with modernc if batch sizes are >= 100 events per transaction.
- Document that single-event `Append()` calls are for correctness/simplicity; high-throughput callers should use `AppendBatch()`.
- If cross-compilation is not a requirement (e.g., the Go SDK is only used on Linux/amd64), `mattn/go-sqlite3` is the better choice for write throughput.

**Warning signs:**
- Single-event `Append()` benchmark shows < 10K events/sec with modernc.
- Throughput benchmark uses only single-event inserts, not batches.

**Phase to address:** Go SDK phase — establish benchmark early, before the API is locked in; ensure `AppendBatch` is a first-class path, not an afterthought.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| No checkpoint in `Close()` | Simpler implementation | WAL file never shrinks; callers see bloated disk usage | Never — add it to `Close()` from day one |
| Keep `AUTOINCREMENT` | Familiar, feels safer | ~10% insert overhead for no benefit in append-only store | Never — remove before any implementation |
| Timestamps as JSON numbers in test vectors | Simpler to write | TypeScript loses nanosecond precision silently | Never — use quoted strings |
| Skip rotation-boundary test vectors | Faster initial spec | Rotation bugs only appear in production after day 1 | Never for v1 |
| Expose raw DB handle for "power users" | Easier workarounds | Breaks immutability guarantee, the core value prop | Never |
| Single-event `Append()` as the only write path | Simpler API to implement | Cannot hit throughput target for IoT workloads | Acceptable only if `AppendBatch()` is also implemented |
| Not setting `busy_timeout` on every connection | Less boilerplate | Intermittent SQLITE_BUSY under any concurrent load | Never — make it part of the connection init utility |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| WAL file unbounded growth | Reads slow over time; disk fills; `.db-wal` larger than `.db` | Call `PRAGMA wal_checkpoint(TRUNCATE)` in `Close()` and periodically | Day 1 in long-lived reader processes |
| Missing `busy_timeout` | Intermittent "database is locked" in concurrent tests | Set `PRAGMA busy_timeout = 5000` on every connection open | First test with concurrent read + write |
| Large batch in a single transaction exceeding 100MB | WAL I/O error or disk-full error mid-batch | Cap `AppendBatch` at ~10K events or 50MB per transaction | IoT burst writes |
| Index scan instead of range seek | Query latency O(n) instead of O(log n + results) | Ensure index is `(topic, ts)` not just `(ts)`; verify with `EXPLAIN QUERY PLAN` | First query on a large dataset |
| `AUTOINCREMENT` sequence table write | ~10% throughput degradation on every insert | Use `INTEGER PRIMARY KEY` without `AUTOINCREMENT` | Every insert, compounding at high volume |
| Nanosecond precision loss in JS | Events within the same millisecond appear identical; wrong sort order | Use `BigInt` / `.safeIntegers(true)` in `better-sqlite3` | First write of two events within 1ms |
| Per-event `Append()` without batching via modernc | < 10K events/sec on commodity hardware | Use `AppendBatch()` wrapped in a single transaction | IoT high-frequency sensor writes |

---

## "Looks Done But Isn't" Checklist

The following states are reachable where all tests pass but the implementation has serious latent bugs:

- [ ] `Query` returns correct results when `start` and `end` are in the same segment file, but silently drops events when they span a rotation boundary.
- [ ] `Append` returns success but the WAL is not checkpointed — after process crash, the last N events are not in the main database file (WAL-sync durability depends on `synchronous` setting).
- [ ] TypeScript test vectors pass because all test payloads are ASCII strings; the base64/binary encoding path is never exercised.
- [ ] Timestamps in TypeScript are accepted as `number` without error, but nanosecond-precision events from Go or Rust produce incorrect sort order when round-tripped through the TypeScript SDK.
- [ ] Immutability triggers fire correctly in unit tests, but defensive mode is not enabled — a caller with access to the raw connection can bypass them.
- [ ] `Close()` does not call `PRAGMA wal_checkpoint(TRUNCATE)` — long-running process leaves a multi-hundred-MB WAL file.
- [ ] `AppendBatch` succeeds for 100 events but silently truncates or errors at 10K due to SQLite's large-transaction WAL limitation.
- [ ] `AUTOINCREMENT` is in the schema and nobody notices the performance overhead because the benchmark only tests single-event appends at low volume.
- [ ] The `-shm` and `-wal` sidecar files are not documented — callers back up the `.db` file alone and lose uncommitted WAL data.

---

## Sources

- [SQLite Write-Ahead Logging — official WAL documentation](https://www.sqlite.org/wal.html)
- [SQLite AUTOINCREMENT — official documentation](https://sqlite.org/autoinc.html)
- [SQLite SQLITE_DBCONFIG_DEFENSIVE — official C API docs](https://www.sqlite.org/c3ref/c_dbconfig_defensive.html)
- [SQLite Defense Against The Dark Arts](https://sqlite.org/security.html)
- [better-sqlite3 performance guide (WAL and checkpointing)](https://github.com/WiseLibs/better-sqlite3/blob/master/docs/performance.md)
- [SQLite concurrent writes and "database is locked" errors](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/)
- [SQLite checkpoint starvation — SQLite User Forum](https://sqlite.org/forum/info/7da967e0141c7a1466755f8659f7cb5e38ddbdb9aec8c78df5cb0fea22f75cf6)
- [WAL file growth in production — Hacker News discussion](https://news.ycombinator.com/item?id=40688987)
- [SQLite in Go with and without CGo — benchmark analysis](https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html)
- [modernc.org/sqlite Go package](https://pkg.go.dev/modernc.org/sqlite)
- [Prisma nanosecond timestamp BigInt overflow issue](https://github.com/prisma/prisma/issues/28350)
- [High-precision datetime in SQLite — Simon Willison](https://simonwillison.net/2024/Aug/9/high-precision-datetime-in-sqlite/)
- [What to do about SQLITE_BUSY despite timeout — Bert Hubert](https://berthug.eu/articles/posts/a-brief-post-on-sqlite3-database-locked-despite-timeout/)
- [Understanding SQLITE_BUSY — ActiveSphere](http://activesphere.com/blog/2018/12/24/understanding-sqlite-busy)
- [JSON Schema Test Suite — cross-language test vector reference](https://github.com/json-schema-org/JSON-Schema-Test-Suite)

---
*Pitfalls research for: Cairn — append-only event storage SDK (SQLite wrapper, Go/TypeScript/Rust monorepo)*
*Researched: 2026-03-18*
