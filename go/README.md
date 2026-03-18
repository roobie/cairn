# Cairn Go SDK

Append-only event store backed by SQLite.

## Install

```
go get cairn.dev/sdk/go
```

## Quick Example

```go
import (
    "time"
    "cairn.dev/sdk/go"
)

store, err := cairn.Open("events.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

id, err := store.Append("sensor.temp", []byte(`{"value": 22.5}`))
if err != nil {
    log.Fatal(err)
}

now := time.Now().UnixNano()
events, err := store.Query("sensor.temp", 0, now)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("appended id=%d, found %d events\n", id, len(events))
```

## API

### `Open(path string) (*Store, error)`

Opens or creates a cairn database at `path`. The parent directory must exist; `Open` does not create parent directories.

On success, schema DDL has been applied, WAL mode is active, and immutability triggers are in place.

### `(*Store) Close() error`

Checkpoints the WAL (`PRAGMA wal_checkpoint(TRUNCATE)`) and closes the connection. Idempotent — safe to call multiple times. Returns `nil` after the first call.

### `(*Store) Append(topic string, payload []byte) (uint64, error)`

Inserts a single event. Returns the `EventID` (SQLite `last_insert_rowid()`, always >= 1).

Validates preconditions before inserting. Returns the first applicable error:

| Error                   | Condition                              |
|-------------------------|----------------------------------------|
| `ErrStoreNotOpen`       | Store has been closed                  |
| `ErrEmptyTopic`         | `topic == ""`                          |
| `ErrEmptyPayload`       | `len(payload) == 0`                    |
| `ErrPayloadTooLarge`    | `len(payload) > 1,048,576`             |

### `(*Store) AppendBatch(events []BatchEvent) ([]uint64, error)`

Inserts multiple events in a single atomic transaction. Validates all events before inserting any. Empty input returns `[]uint64{}` with no error and no transaction opened.

### `(*Store) Query(topic string, start, end int64) ([]Event, error)`

Returns all events for `topic` with timestamp in `[start, end]` inclusive, ordered by `id` ASC (insertion order). Returns an empty slice (not an error) when no events match.

Returns `ErrStoreNotOpen` if the store has been closed.

## Types

```go
type Event struct {
    ID      uint64
    Topic   string
    TS      int64  // nanoseconds since Unix epoch
    Payload []byte
}

type BatchEvent struct {
    Topic   string
    Payload []byte
}
```

## Errors

```go
var (
    ErrPayloadTooLarge       = errors.New("cairn: payload too large")
    ErrStoreNotOpen          = errors.New("cairn: store not open")
    ErrImmutabilityViolation = errors.New("cairn: immutability violation")
    ErrEmptyTopic            = errors.New("cairn: empty topic")
    ErrEmptyPayload          = errors.New("cairn: empty payload")
)

const MaxPayloadSize = 1_048_576
```

## Language-Specific Notes

**No CGo required.** Uses `modernc.org/sqlite` — a pure Go port of SQLite. Cross-compiles without a C toolchain. No system SQLite dependency.

**`SQLITE_DBCONFIG_DEFENSIVE`** is not accessible through the `database/sql` `conn.Raw()` interface in `modernc.org/sqlite`. The Go SDK falls back to `PRAGMA trusted_schema = OFF`. Immutability is still enforced by the `no_update` and `no_delete` triggers.

**`MaxPayloadSize`** constant is exported (`1,048,576` bytes). Payloads of exactly this size are valid; payloads exceeding it return `ErrPayloadTooLarge`.

**`Query`** returns an empty slice (not `nil`) when no events match. `AppendBatch` with empty input returns `[]uint64{}` (not `nil`) immediately.
