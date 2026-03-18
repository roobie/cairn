# Cairn Rust SDK

Append-only event store backed by SQLite.

## Install

Add to `Cargo.toml`:

```toml
[dependencies]
cairn = "0.1"
```

Or during development, reference the local path:

```toml
[dependencies]
cairn = { path = "../rs" }
```

## Quick Example

```rust
use cairn::BatchEvent;

fn main() -> Result<(), cairn::Error> {
    let mut store = cairn::open("events.db")?;

    let id = store.append("sensor.temp", b"{\"value\": 22.5}")?;
    println!("appended id={}", id);

    let events = store.query("sensor.temp", 0, i64::MAX)?;
    println!("found {} events", events.len());

    store.close()?; // or let Drop handle it
    Ok(())
}
```

## API

### `open(path: &str) -> Result<Store, Error>`

Opens or creates a cairn database at `path`. The parent directory must exist; `open` does not create parent directories.

On success, schema DDL has been applied, WAL mode is active, and `SQLITE_DBCONFIG_DEFENSIVE` is enabled.

Returns `Error::Storage` if the parent directory does not exist or any SQLite initialisation step fails.

### `store.close(&mut self) -> Result<(), Error>`

Checkpoints the WAL (`PRAGMA wal_checkpoint(TRUNCATE)`) and closes the connection. Idempotent — safe to call multiple times. Returns `Ok(())` after the first call.

### `store.append(&mut self, topic: &str, payload: &[u8]) -> Result<u64, Error>`

Inserts a single event. Returns the `EventID` (SQLite `last_insert_rowid()`, always >= 1).

Validates preconditions before inserting. Returns the first applicable error:

| Error                      | Condition                              |
|----------------------------|----------------------------------------|
| `Error::StoreNotOpen`      | Store has been closed                  |
| `Error::EmptyTopic`        | `topic.is_empty()`                     |
| `Error::EmptyPayload`      | `payload.is_empty()`                   |
| `Error::PayloadTooLarge`   | `payload.len() > 1,048,576`            |
| `Error::Storage(msg)`      | Underlying SQLite error                |

### `store.append_batch(&mut self, events: &[BatchEvent]) -> Result<Vec<u64>, Error>`

Inserts multiple events in a single atomic transaction. Validates all events before inserting any. Empty input returns an empty `Vec` immediately with no transaction opened.

### `store.query(&self, topic: &str, start: i64, end: i64) -> Result<Vec<Event>, Error>`

Returns all events for `topic` with timestamp in `[start, end]` inclusive, ordered by `id` ascending (insertion order). Returns an empty `Vec` (not an error) when no events match.

Returns `Error::StoreNotOpen` if the store has been closed.

## Types

```rust
pub struct Event {
    pub id: u64,       // SQLite INTEGER PRIMARY KEY (insertion order)
    pub topic: String,
    pub ts: i64,       // nanoseconds since Unix epoch
    pub payload: Vec<u8>,
}

pub struct BatchEvent {
    pub topic: String,
    pub payload: Vec<u8>,
}

pub enum Error {
    PayloadTooLarge,
    StoreNotOpen,
    ImmutabilityViolation,
    EmptyTopic,
    EmptyPayload,
    Storage(String),
}

pub const MAX_PAYLOAD_SIZE: usize = 1_048_576;
```

`Error::kind()` returns the spec-compatible error kind string (e.g. `"payload_too_large"`, `"store_not_open"`).

## Language-Specific Notes

**Drop closes the store automatically.** `Store` implements `Drop`. When a `Store` goes out of scope, it runs `PRAGMA wal_checkpoint(TRUNCATE)` and closes the connection. Call `store.close()` explicitly only if you need to handle checkpoint errors — `Drop` silently discards errors.

```rust
{
    let mut store = cairn::open("events.db")?;
    store.append("topic", b"data")?;
    // store.close() called automatically here by Drop
}
```

**`append` and `append_batch` take `&mut self`.** These operations require exclusive borrow of the store. `query` takes `&self` (shared borrow) and can be called concurrently with other reads in single-threaded code.

**Bundled SQLite.** Uses `rusqlite` with the `bundled` feature — builds SQLite from source. No system SQLite installation required. Consistent SQLite version across platforms.

**`SQLITE_DBCONFIG_DEFENSIVE`** is set via `conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)` at open time. This prevents disabling immutability triggers via `PRAGMA writable_schema`.
