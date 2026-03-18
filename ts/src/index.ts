import { statSync } from 'node:fs'
import { dirname } from 'node:path'
import Database from 'better-sqlite3'

import {
  MAX_PAYLOAD_SIZE,
  PayloadTooLargeError,
  StoreNotOpenError,
  ImmutabilityViolationError,
  EmptyTopicError,
  EmptyPayloadError,
} from './errors.js'

export {
  MAX_PAYLOAD_SIZE,
  PayloadTooLargeError,
  StoreNotOpenError,
  ImmutabilityViolationError,
  EmptyTopicError,
  EmptyPayloadError,
}

// schemaSQL is the cairn schema DDL executed verbatim on every open().
// Copied from spec/schema.sql — all statements use IF NOT EXISTS for idempotent execution.
const schemaSQL = `
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
`

/**
 * An event returned by Query.
 */
export interface Event {
  id: bigint
  topic: string
  ts: bigint
  payload: Buffer
}

/**
 * An input event for appendBatch.
 */
export interface BatchEvent {
  topic: string
  payload: Uint8Array
}

/**
 * Opens or creates a cairn database at the given path.
 * The parent directory of path must exist; open() does not create parent directories.
 *
 * @param path - Filesystem path for the SQLite database file.
 * @returns An open Store ready for use.
 * @throws Error if the parent directory does not exist.
 */
export function open(path: string): Store {
  // Validate that the parent directory exists (SQLite won't error until first use otherwise)
  const dir = dirname(path)
  try {
    statSync(dir)
  } catch {
    throw new Error(`cairn: parent directory does not exist: ${dir}`)
  }

  const db = new Database(path)

  // CRITICAL: must call before any integer reads to ensure all integers are BigInt.
  // Without this, lastInsertRowid and scanned integer columns return number,
  // causing silent precision loss for nanosecond timestamps (~1.7e18 > 2^53).
  db.defaultSafeIntegers(true)

  // Apply connection-level PRAGMAs per spec
  db.pragma('journal_mode = WAL')
  db.pragma('busy_timeout = 5000')
  db.pragma('foreign_keys = ON')
  // SQLITE_DBCONFIG_DEFENSIVE via better-sqlite3 PRAGMA alias
  db.pragma('defensive = ON')

  // Execute schema DDL — idempotent via IF NOT EXISTS
  db.exec(schemaSQL)

  return new Store(db)
}

/**
 * An open handle to a cairn database.
 * Create with open(); invalidated by close().
 */
export class Store {
  /** @internal exposed for test vector harness raw SQL setup */
  readonly _db: Database.Database

  private closed = false
  private readonly insertStmt: Database.Statement
  private readonly queryStmt: Database.Statement

  constructor(db: Database.Database) {
    this._db = db
    // Prepare statements once at construction for performance
    this.insertStmt = db.prepare(
      'INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)',
    )
    this.queryStmt = db.prepare(
      'SELECT id, topic, ts, payload FROM events WHERE topic = ? AND ts >= ? AND ts <= ? ORDER BY id ASC',
    )
  }

  /**
   * Checkpoints the WAL and closes the database connection.
   * Idempotent — safe to call multiple times.
   */
  close(): void {
    if (this.closed) return
    this.closed = true
    // Checkpoint WAL back into main database file and truncate.
    // Spec: Close is always safe — ignore checkpoint errors.
    try {
      this._db.pragma('wal_checkpoint(TRUNCATE)')
    } catch {
      // swallow — close must not throw
    }
    this._db.close()
  }

  private checkOpen(): void {
    if (this.closed) throw new StoreNotOpenError()
  }

  /**
   * Inserts a single event and returns its EventID (>= 1).
   *
   * @param topic - Non-empty event stream name.
   * @param payload - 1 to 1,048,576 bytes of opaque binary data.
   * @returns The EventID (bigint, SQLite rowid).
   */
  append(topic: string, payload: Uint8Array): bigint {
    this.checkOpen()
    if (topic === '') throw new EmptyTopicError()
    if (payload.length === 0) throw new EmptyPayloadError()
    if (payload.length > MAX_PAYLOAD_SIZE) throw new PayloadTooLargeError()

    const ts = BigInt(Date.now()) * 1_000_000n // nanoseconds since Unix epoch
    const result = this.insertStmt.run(topic, ts, payload)
    return result.lastInsertRowid as bigint // safe: defaultSafeIntegers(true)
  }

  /**
   * Inserts multiple events in a single atomic transaction and returns their EventIDs.
   * All events are validated before any are inserted.
   * Empty input returns an empty array with no error and no transaction.
   *
   * @param events - Array of {topic, payload} pairs.
   * @returns Array of EventIDs in the same order as input.
   */
  appendBatch(events: BatchEvent[]): bigint[] {
    this.checkOpen()
    if (events.length === 0) return []

    // Validate ALL events before inserting any (all-or-nothing validation)
    // Precedence per spec: EmptyTopic > EmptyPayload > PayloadTooLarge
    for (const e of events) {
      if (e.topic === '') throw new EmptyTopicError()
      if (e.payload.length === 0) throw new EmptyPayloadError()
      if (e.payload.length > MAX_PAYLOAD_SIZE) throw new PayloadTooLargeError()
    }

    const insertBatch = this._db.transaction((evts: BatchEvent[]) => {
      return evts.map((e) => {
        const ts = BigInt(Date.now()) * 1_000_000n
        const r = this.insertStmt.run(e.topic, ts, e.payload)
        return r.lastInsertRowid as bigint // safe: defaultSafeIntegers(true)
      })
    })

    return insertBatch(events)
  }

  /**
   * Returns all events in the given topic with timestamp in [start, end] inclusive,
   * ordered by id ASC (insertion order).
   * Returns an empty array (not an error) when no events match.
   *
   * @param topic - Event stream name to query.
   * @param start - Inclusive start timestamp (nanoseconds, bigint).
   * @param end - Inclusive end timestamp (nanoseconds, bigint).
   * @returns Array of matching Event objects.
   */
  query(topic: string, start: bigint, end: bigint): Event[] {
    this.checkOpen()
    const rows = this.queryStmt.all(topic, start, end) as Array<{
      id: bigint
      topic: string
      ts: bigint
      payload: Buffer
    }>
    return rows.map((r) => ({
      id: r.id,
      topic: r.topic,
      ts: r.ts,
      payload: r.payload,
    }))
  }
}
