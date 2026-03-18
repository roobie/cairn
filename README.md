# cairn

**cairn** — a stack of stones used as a trail marker. You add stones, never remove them.

Cairn is an append-only event store backed by SQLite. Events are written once and can never be modified or deleted. Immutability is enforced at the storage layer via SQLite triggers — not by API convention. Data corruption from accidental updates or deletes becomes structurally impossible.

Target use cases: audit logs, IoT telemetry, application event trails.

## Why cairn

The constraint is the product. No updates, no deletes, no schema migrations. Once an event is in cairn, it stays exactly as written. Immutability is enforced by `BEFORE UPDATE` and `BEFORE DELETE` triggers on the events table — a direct SQL connection to the database file cannot bypass them.

Zero configuration: `Open("events.db")` is the only entry point. WAL mode, defensive mode, and the immutability schema are applied automatically on every open.

## Quickstart

### Go

```go
import "cairn.dev/sdk/go"

store, _ := cairn.Open("events.db")
defer store.Close()
id, _ := store.Append("sensor.temp", []byte(`{"value": 22.5}`))
events, _ := store.Query("sensor.temp", 0, time.Now().UnixNano())
```

### TypeScript

```typescript
import { open } from 'cairn'

const store = open('events.db')
const id = store.append('sensor.temp', Buffer.from('{"value": 22.5}'))
const events = store.query('sensor.temp', 0n, BigInt(Date.now()) * 1_000_000n)
store.close()
```

> **Note:** All timestamps and event IDs are `bigint`. Nanosecond values exceed `Number.MAX_SAFE_INTEGER`.

### Rust

```rust
let mut store = cairn::open("events.db")?;
let id = store.append("sensor.temp", b"{\"value\": 22.5}")?;
let events = store.query("sensor.temp", 0, i64::MAX)?;
store.close()?; // or let Drop handle it
```

## Storage Guarantees

- **WAL mode** — write-ahead logging enabled on every `Open`; better concurrent read/write throughput
- **Nanosecond timestamps** — `int64` nanoseconds since Unix epoch; stored as SQLite `INTEGER`
- **1 MiB payload limit** — `len(payload) <= 1,048,576` bytes; validated before any INSERT
- **Immutability triggers** — `no_update` and `no_delete` triggers raise `ABORT` on any attempt to modify or remove an event
- **Defensive mode** — `SQLITE_DBCONFIG_DEFENSIVE` prevents disabling triggers via `writable_schema`
- **WAL checkpoint on close** — `PRAGMA wal_checkpoint(TRUNCATE)` run on `Close`; database left in clean state

## API Overview

| Operation      | Description                                                           |
|----------------|-----------------------------------------------------------------------|
| `Open`         | Open or create a cairn database at path; apply schema and PRAGMAs    |
| `Close`        | Checkpoint WAL and close the connection; idempotent                   |
| `Append`       | Insert one event; return its `EventID`                                |
| `AppendBatch`  | Insert multiple events atomically; validate all before inserting any  |
| `Query`        | Return events for a topic in time range `[start, end]` inclusive      |

See per-language READMEs for full signatures, error types, and language-specific notes.

## Language SDKs

- [Go SDK](go/README.md) — pure Go, no CGo, cross-compiles without a C toolchain
- [TypeScript SDK](ts/README.md) — Node.js, synchronous API, dual ESM/CJS
- [Rust SDK](rs/README.md) — bundled SQLite, no system SQLite dependency
