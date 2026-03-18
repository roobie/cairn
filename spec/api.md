# Cairn API Specification

## Version

1.0

Cairn is an append-only event store with immutability enforced at the storage layer. Events are written once and can never be modified or deleted. The canonical storage backend is SQLite with WAL mode, defensive mode, and immutability triggers enabled at open time. An SDK author reading this document and `spec/schema.sql` has the complete contract needed to implement a compliant cairn SDK without reading any other SDK code.

---

## Concepts

### Type Definitions

```
EventID    uint64           -- SQLite rowid of the inserted event (INTEGER PRIMARY KEY)
Timestamp  int64            -- nanoseconds since Unix epoch (1970-01-01T00:00:00Z)
Topic      string           -- non-empty UTF-8 string identifying an event stream
Payload    bytes            -- opaque binary data, 1 byte to 1,048,576 bytes inclusive
```

### Event Struct

```
Event {
    id:      EventID
    topic:   Topic
    ts:      Timestamp
    payload: Payload
}
```

### Store

An open handle to a cairn database. Created by `Open`, invalidated by `Close`. All operations except `Open` require a valid open Store.

---

## Operations

### Open

**Signature:**
```
Open(path: string) -> Store
```

**Preconditions:**
- The parent directory of `path` must exist. If it does not exist, return a storage error. `Open` does not create parent directories.

**Behavior:**

1. Open or create a SQLite database file at `path`.
2. Execute `spec/schema.sql` verbatim using the connection's SQL execution mechanism. All statements in `schema.sql` use `IF NOT EXISTS`, making this call idempotent — safe to run on every `Open` whether the database is new or existing.
3. Set connection-level PRAGMAs on every open connection:
   ```sql
   PRAGMA journal_mode = WAL;    -- enable WAL mode (persistent after first set, but harmless to re-apply)
   PRAGMA busy_timeout = 5000;   -- wait up to 5000ms before returning SQLITE_BUSY
   PRAGMA foreign_keys = ON;     -- enable foreign key enforcement
   ```
4. Enable defensive mode using the binding-appropriate mechanism (see note below). Defensive mode prevents disabling of triggers via `PRAGMA writable_schema` and prevents writes to shadow tables.

**Defensive Mode — Per-Language Mechanism:**

`SQLITE_DBCONFIG_DEFENSIVE` is a C-level `sqlite3_db_config()` call, not a SQL PRAGMA. It must be enabled using the binding's native mechanism:

- **Go** (`modernc.org/sqlite`): Use `db.SetDefensive(true)` on the underlying connection, or invoke the C bridge `_sqlite3_db_config` with `SQLITE_DBCONFIG_DEFENSIVE = 1009`.
- **TypeScript** (`better-sqlite3`): `db.pragma('defensive=ON')` — better-sqlite3 exposes this as a PRAGMA alias.
- **Rust** (`rusqlite`): `conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)?`

**Errors:**
- Parent directory does not exist: return a storage error (SDK-idiomatic; not a panic).

---

### Close

**Signature:**
```
Close(store: Store)
```

**Preconditions:**
- None. `Close` is idempotent.

**Behavior:**

1. If the store is already closed, return immediately (no-op, no error).
2. Execute WAL checkpoint:
   ```sql
   PRAGMA wal_checkpoint(TRUNCATE);
   ```
   This checkpoints all WAL frames back into the main database file and truncates the WAL file to zero bytes, leaving the database in a clean state.
3. Close the database connection.

**Errors:**
- None. `Close` is always safe to call.

---

### Append

**Signature:**
```
Append(store: Store, topic: Topic, payload: Payload) -> EventID
```

**Preconditions:**
- `store` is open
- `topic` is non-empty
- `payload` is non-empty and `len(payload) <= 1,048,576`

**Behavior:**

1. Validate all preconditions. Return the first applicable error without performing any INSERT.
2. Obtain the current nanosecond timestamp from the system clock: `ts = clock.now_nanoseconds()`.
3. Execute:
   ```sql
   INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)
   ```
4. Return the inserted row's `INTEGER PRIMARY KEY` (SQLite `last_insert_rowid()`) as `EventID`.

**Errors:**

| Error | Condition |
|-------|-----------|
| `StoreNotOpen` | `store` is not in open state |
| `EmptyTopic` | `topic == ""` |
| `EmptyPayload` | `len(payload) == 0` |
| `PayloadTooLarge` | `len(payload) > 1,048,576` |

---

### AppendBatch

**Signature:**
```
AppendBatch(store: Store, events: []{topic: Topic, payload: Payload}) -> []EventID
```

**Preconditions:**
- `store` is open
- Each event's `topic` is non-empty
- Each event's `payload` is non-empty and `len(payload) <= 1,048,576`

**Behavior:**

1. If `events` is an empty slice, return an empty slice immediately. No error, no transaction.
2. Validate ALL events before inserting any. If any event fails validation, return the error for the first failing event without inserting anything.
3. Begin a transaction.
4. For each event, in order:
   a. Obtain the current nanosecond timestamp from the system clock.
   b. Execute:
      ```sql
      INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)
      ```
   c. Record the returned `INTEGER PRIMARY KEY` as the event's `EventID`.
5. Commit the transaction. All inserts are atomic — either all succeed or none do.
6. Return the list of `EventID` values in the same order as the input events.

**Errors:**

| Error | Condition |
|-------|-----------|
| `StoreNotOpen` | `store` is not in open state |
| `EmptyTopic` | First event with `topic == ""` |
| `EmptyPayload` | First event with `len(payload) == 0` |
| `PayloadTooLarge` | First event with `len(payload) > 1,048,576` |

---

### Query

**Signature:**
```
Query(store: Store, topic: Topic, start: Timestamp, end: Timestamp) -> []Event
```

**Preconditions:**
- `store` is open

**Behavior:**

1. Execute:
   ```sql
   SELECT id, topic, ts, payload
   FROM events
   WHERE topic = ? AND ts >= ? AND ts <= ?
   ORDER BY id ASC
   ```
   The time range `[start, end]` is **inclusive on both ends**: an event at exactly `start` or exactly `end` is included.
2. Return all matching events as a slice of `Event` structs, ordered by `id` ascending (insertion order).
3. An empty result is valid and not an error.

**Errors:**

| Error | Condition |
|-------|-----------|
| `StoreNotOpen` | `store` is not in open state |

---

## Storage Invariants

These invariants are enforced at the storage layer. SDKs must document and maintain them.

### STOR-01 — WAL Mode

SQLite WAL (Write-Ahead Logging) mode is enabled with `PRAGMA journal_mode = WAL` during `Open`. WAL mode is persistent — once set, it survives connection close and reopen — but re-applying the PRAGMA on every `Open` is harmless. WAL mode provides better concurrent read/write throughput and avoids database corruption on incomplete writes.

### STOR-02 — Immutability Triggers

The `spec/schema.sql` DDL creates two triggers that fire before any `UPDATE` or `DELETE` on the `events` table:

- `no_update`: fires on `BEFORE UPDATE`, raises `ABORT` with message `cairn: updates not allowed`
- `no_delete`: fires on `BEFORE DELETE`, raises `ABORT` with message `cairn: deletes not allowed`

These triggers make the `events` table append-only at the SQLite level, independent of the application layer. Even a direct SQL connection to the database file cannot update or delete events.

### STOR-03 — Defensive Mode

`SQLITE_DBCONFIG_DEFENSIVE` is enabled at `Open` time via the language binding's native mechanism (see `Open` operation for per-language details). Defensive mode prevents:

- Use of `PRAGMA writable_schema` to disable or remove triggers
- Direct writes to shadow tables (e.g., `sqlite_master`)

This ensures the immutability triggers in STOR-02 cannot be bypassed by a process with direct database file access.

### STOR-04 — Nanosecond Timestamps

Timestamps are `int64` values representing nanoseconds since the Unix epoch (1970-01-01T00:00:00Z). The `int64` range accommodates values up to year 2262. Nanosecond timestamps are stored in the `ts` column as `INTEGER` (SQLite's 64-bit signed integer type).

When timestamps appear in test vectors or JSON serialization, they MUST be encoded as quoted strings (e.g., `"1710000000000000001"`) to prevent precision loss in JavaScript, where `Number` has only 53-bit mantissa (~9×10^15 max safe integer, while nanosecond epoch values are ~1.7×10^18).

### STOR-05 — Opaque BLOB Payloads

The `payload` column uses SQLite's `BLOB` type. Cairn never interprets payload content — no encoding, no compression, no schema. The payload is stored exactly as provided and returned exactly as stored. SDKs must not add framing, headers, or compression to payload bytes.

### STOR-06 — 1MB Payload Hard Limit

The maximum payload size is **1,048,576 bytes** (1 MiB, or 1048576 bytes exactly). This limit is validated before any INSERT — it is not enforced by a SQLite column constraint. The boundary semantics are:

- `len(payload) <= 1,048,576`: **valid** — insert proceeds
- `len(payload) > 1,048,576`: **invalid** — return `PayloadTooLarge` without inserting

### STOR-07 — WAL Checkpoint on Close

During `Close`, `PRAGMA wal_checkpoint(TRUNCATE)` is executed before closing the connection. The `TRUNCATE` mode checkpoints all WAL frames back into the main database file and truncates the WAL file to zero bytes. This leaves the database in a clean, compact state and ensures a reader can access all data from the main file without reading the WAL.

---

## Error Catalog

### Error Codes

| Code | Operation | Trigger |
|------|-----------|---------|
| `PayloadTooLarge` | `Append`, `AppendBatch` | `len(payload) > 1,048,576` bytes |
| `StoreNotOpen` | `Close`, `Append`, `AppendBatch`, `Query` | Operation called on a closed or uninitialized store |
| `ImmutabilityViolation` | (storage-level, not public API) | Any `UPDATE` or `DELETE` on the `events` table — raised by SQLite trigger |
| `EmptyTopic` | `Append`, `AppendBatch` | `topic == ""` |
| `EmptyPayload` | `Append`, `AppendBatch` | `len(payload) == 0` |

### Per-Language Idiomatic Naming

SDKs must expose these error codes using idiomatic naming for their language:

**Go:**
```
var (
    ErrPayloadTooLarge      = errors.New("cairn: payload too large")
    ErrStoreNotOpen         = errors.New("cairn: store not open")
    ErrImmutabilityViolation = errors.New("cairn: immutability violation")
    ErrEmptyTopic           = errors.New("cairn: empty topic")
    ErrEmptyPayload         = errors.New("cairn: empty payload")
)
```

**TypeScript:**
```typescript
class PayloadTooLargeError extends Error { ... }
class StoreNotOpenError extends Error { ... }
class ImmutabilityViolationError extends Error { ... }
class EmptyTopicError extends Error { ... }
class EmptyPayloadError extends Error { ... }
```

**Rust:**
```rust
#[derive(Debug)]
pub enum Error {
    PayloadTooLarge,
    StoreNotOpen,
    ImmutabilityViolation,
    EmptyTopic,
    EmptyPayload,
    Storage(String), // wraps underlying SQLite/IO errors
}
```

---

## Implementation Notes

### Schema Execution

The DDL in `spec/schema.sql` must be executed verbatim on every `Open` call. Implementations must not parse, split, or modify the DDL. Execute it as a single `exec()` call or as individual statements — both are acceptable, as all statements are idempotent (`IF NOT EXISTS`).

### Timestamp Source

All timestamps are obtained from the system's monotonic or wall clock at nanosecond precision. The system clock is read once per event, immediately before the `INSERT`. Cairn does not validate or adjust timestamps — if two events are appended within the same nanosecond, they may have identical timestamps. Events are ordered by `id` (insertion order), not by `ts`.

### Error Precedence in AppendBatch

When validating a batch, errors are returned in this priority order for the first failing event:
1. `StoreNotOpen`
2. `EmptyTopic`
3. `EmptyPayload`
4. `PayloadTooLarge`

All events are validated before any are inserted.
