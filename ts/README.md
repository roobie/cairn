# Cairn TypeScript SDK

Append-only event store backed by SQLite.

## Install

```
npm install cairn
```

## Quick Example

```typescript
import { open } from 'cairn'

const store = open('events.db')

const id = store.append('sensor.temp', Buffer.from('{"value": 22.5}'))
console.log('appended id:', id) // bigint

// Query last hour of events
const now = BigInt(Date.now()) * 1_000_000n  // ms → ns
const oneHourAgo = now - 3_600_000_000_000n
const events = store.query('sensor.temp', oneHourAgo, now)
console.log('events:', events.length)

store.close()
```

## API

### `open(path: string): Store`

Opens or creates a cairn database at `path`. The parent directory must exist; `open` does not create parent directories.

On success, schema DDL has been applied, WAL mode is active, and immutability triggers are in place. Throws `Error` if the parent directory does not exist.

### `store.close(): void`

Checkpoints the WAL (`wal_checkpoint(TRUNCATE)`) and closes the connection. Idempotent — safe to call multiple times.

### `store.append(topic: string, payload: Uint8Array): bigint`

Inserts a single event. Returns the `EventID` as `bigint` (SQLite rowid, always >= 1).

Validates preconditions before inserting. Throws the first applicable error:

| Error                      | Condition                              |
|----------------------------|----------------------------------------|
| `StoreNotOpenError`        | Store has been closed                  |
| `EmptyTopicError`          | `topic === ""`                         |
| `EmptyPayloadError`        | `payload.length === 0`                 |
| `PayloadTooLargeError`     | `payload.length > 1,048,576`           |

### `store.appendBatch(events: BatchEvent[]): bigint[]`

Inserts multiple events in a single atomic transaction. Validates all events before inserting any. Empty input returns `[]` with no error and no transaction opened.

### `store.query(topic: string, start: bigint, end: bigint): Event[]`

Returns all events for `topic` with timestamp in `[start, end]` inclusive, ordered by `id` ascending (insertion order). Returns an empty array (not an error) when no events match.

Throws `StoreNotOpenError` if the store has been closed.

## Types

```typescript
interface Event {
  id: bigint      // SQLite rowid (insertion order)
  topic: string
  ts: bigint      // nanoseconds since Unix epoch
  payload: Buffer
}

interface BatchEvent {
  topic: string
  payload: Uint8Array
}
```

## Errors

All error classes have a `.kind` property for programmatic matching:

```typescript
class PayloadTooLargeError extends Error    { kind: 'payload_too_large' }
class StoreNotOpenError extends Error       { kind: 'store_not_open' }
class ImmutabilityViolationError extends Error { kind: 'immutability_violation' }
class EmptyTopicError extends Error         { kind: 'empty_topic' }
class EmptyPayloadError extends Error       { kind: 'empty_payload' }

const MAX_PAYLOAD_SIZE = 1_048_576
```

## Language-Specific Notes

**BigInt timestamps.** All timestamps and event IDs are `bigint`, not `number`. Nanosecond epoch values (~1.7×10¹⁸) exceed `Number.MAX_SAFE_INTEGER` (2⁵³ ≈ 9×10¹⁵). Using `number` for nanosecond timestamps causes silent precision loss.

Convert `Date.now()` (milliseconds) to nanoseconds:
```typescript
const nowNs = BigInt(Date.now()) * 1_000_000n
```

Query parameters `start` and `end` are `bigint`. To query from the beginning of time use `0n`; to query to the end use a far-future value like `9_999_999_999_999_999_999n`.

**Synchronous API.** Uses `better-sqlite3` which is synchronous — no `async/await` needed. All operations block until complete.

**Dual ESM/CJS output.** The package ships `.mjs` (ESM) and `.cjs` (CJS) files. Both module systems are supported without configuration.

**Node.js / Bun only.** Browser runtime is not supported. `better-sqlite3` requires a native binding.
