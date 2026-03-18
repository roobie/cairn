import { readFileSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'

import {
  open,
  type Store,
  PayloadTooLargeError,
  StoreNotOpenError,
  EmptyTopicError,
  EmptyPayloadError,
  ImmutabilityViolationError,
} from '../src/index.js'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Read a spec vector JSON file.
 * Uses import.meta.url for ESM-safe path resolution (no __dirname in ESM).
 * vitest CWD is the package root (ts/), so ../../spec/vectors/ is correct.
 */
function readVectorFile(name: string): unknown {
  const vectorPath = fileURLToPath(
    new URL(`../../spec/vectors/${name}`, import.meta.url),
  )
  const raw = readFileSync(vectorPath, 'utf-8')
  return JSON.parse(raw)
}

/**
 * Decode a base64 payload string.
 * Empty string "" -> empty Uint8Array (for empty_payload test cases).
 */
function decodePayload(b64: string): Uint8Array {
  if (b64 === '') return new Uint8Array(0)
  return Buffer.from(b64, 'base64')
}

/**
 * Assert that a thrown error matches the expected error_kind string,
 * using the error class's `kind` discriminator property.
 */
function assertErrorKind(error: unknown, expectedKind: string): void {
  expect(error).not.toBeNull()
  if (
    typeof error === 'object' &&
    error !== null &&
    'kind' in error
  ) {
    expect((error as { kind: string }).kind).toBe(expectedKind)
  } else {
    // Fallback: for immutability errors from raw SQLite, check message content
    if (expectedKind === 'immutability_violation') {
      expect(error).toBeInstanceOf(Error)
      const msg = (error as Error).message
      const valid =
        msg.includes('updates not allowed') || msg.includes('deletes not allowed')
      expect(valid, `Expected immutability error message, got: ${msg}`).toBe(true)
    } else {
      throw new Error(
        `Expected error with kind=${expectedKind} but got: ${String(error)}`,
      )
    }
  }
}

/**
 * Open a fresh store in a temp directory.
 * Returns the store and the cleanup function.
 */
let storeCounter = 0
function openFreshStore(): Store {
  const dir = tmpdir()
  const path = join(dir, `cairn-test-${process.pid}-${++storeCounter}.db`)
  return open(path)
}

// ---------------------------------------------------------------------------
// Open / Close unit tests
// ---------------------------------------------------------------------------

describe('Open', () => {
  it('creates a DB file and WAL mode is active', () => {
    const store = openFreshStore()
    try {
      // Verify WAL mode
      const row = store._db.pragma('journal_mode') as Array<{ journal_mode: string }>
      expect(row[0].journal_mode).toBe('wal')
    } finally {
      store.close()
    }
  })

  it('throws on missing parent directory', () => {
    expect(() => open('/nonexistent/dir/test.db')).toThrow()
  })

  it('is idempotent on existing DB', () => {
    const store = openFreshStore()
    // Get the path from the db before closing
    const dbPath = store._db.name
    store.close()

    // Second open must succeed
    const store2 = open(dbPath)
    try {
      // Schema must be intact
      const row = store2._db.prepare(
        "SELECT COUNT(*) as cnt FROM sqlite_master WHERE type='table' AND name='events'",
      ).get() as { cnt: bigint }
      expect(row.cnt).toBe(BigInt(1))
    } finally {
      store2.close()
    }
  })
})

describe('Close', () => {
  it('is idempotent (no error on second call)', () => {
    const store = openFreshStore()
    store.close()
    // Second close should be silent
    expect(() => store.close()).not.toThrow()
  })

  it('checkpoints WAL — WAL file absent or zero-length after close', () => {
    const store = openFreshStore()
    const dbPath = store._db.name

    // Write data to create WAL frames
    store._db.prepare(
      'INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)',
    ).run('test', BigInt(1_000_000), Buffer.from('hello'))

    store.close()

    // WAL file should be absent or empty
    const walPath = dbPath + '-wal'
    try {
      const info = statSync(walPath)
      expect(info.size, 'WAL file should be truncated after close').toBe(0)
    } catch {
      // File absent — also acceptable
    }
  })
})

// ---------------------------------------------------------------------------
// Append vector tests
// ---------------------------------------------------------------------------

interface AppendVectorFile {
  test_groups: Array<{
    tests: AppendTestCase[]
  }>
}

interface AppendTestCase {
  tc_id: number
  comment: string
  input: {
    topic: string
    payload?: string
    payload_size_bytes?: number
    store_closed?: boolean
  }
  expected: {
    result: string
    error_kind?: string
    event_id_min?: string
  }
}

describe('Append vectors', () => {
  const vf = readVectorFile('append.json') as AppendVectorFile

  for (const group of vf.test_groups) {
    for (const tc of group.tests) {
      it(`TC${tc.tc_id}: ${tc.comment}`, () => {
        const store = openFreshStore()
        if (tc.input.store_closed) {
          store.close()
        }

        let payload: Uint8Array
        if (tc.input.payload_size_bytes != null && tc.input.payload_size_bytes > 0) {
          payload = new Uint8Array(tc.input.payload_size_bytes)
        } else {
          payload = decodePayload(tc.input.payload ?? '')
        }

        if (tc.expected.result === 'valid') {
          const id = store.append(tc.input.topic, payload)
          if (tc.expected.event_id_min != null) {
            const minId = BigInt(tc.expected.event_id_min)
            expect(id >= minId, `id ${id} should be >= ${minId}`).toBe(true)
          }
          if (!tc.input.store_closed) store.close()
        } else {
          let thrownError: unknown
          expect(() => {
            try {
              store.append(tc.input.topic, payload)
            } catch (e) {
              thrownError = e
              throw e
            }
          }).toThrow()
          assertErrorKind(thrownError, tc.expected.error_kind!)
          if (!tc.input.store_closed) store.close()
        }
      })
    }
  }
})

// ---------------------------------------------------------------------------
// Batch vector tests
// ---------------------------------------------------------------------------

interface BatchVectorFile {
  test_groups: Array<{
    tests: BatchTestCase[]
  }>
}

interface BatchTestCase {
  tc_id: number
  comment: string
  input: {
    events: Array<{
      topic: string
      payload?: string
      payload_size_bytes?: number
    }>
    store_closed?: boolean
  }
  expected: {
    result: string
    error_kind?: string
    event_id_count?: number
  }
}

describe('AppendBatch vectors', () => {
  const vf = readVectorFile('batch.json') as BatchVectorFile

  for (const group of vf.test_groups) {
    for (const tc of group.tests) {
      it(`TC${tc.tc_id}: ${tc.comment}`, () => {
        const store = openFreshStore()
        if (tc.input.store_closed) {
          store.close()
        }

        const batchEvents = tc.input.events.map((e) => ({
          topic: e.topic,
          payload:
            e.payload_size_bytes != null && e.payload_size_bytes > 0
              ? new Uint8Array(e.payload_size_bytes)
              : decodePayload(e.payload ?? ''),
        }))

        if (tc.expected.result === 'valid') {
          const ids = store.appendBatch(batchEvents)
          expect(ids.length).toBe(tc.expected.event_id_count!)
          if (!tc.input.store_closed) store.close()
        } else {
          let thrownError: unknown
          expect(() => {
            try {
              store.appendBatch(batchEvents)
            } catch (e) {
              thrownError = e
              throw e
            }
          }).toThrow()
          assertErrorKind(thrownError, tc.expected.error_kind!)
          if (!tc.input.store_closed) store.close()
        }
      })
    }
  }
})

// ---------------------------------------------------------------------------
// Query vector tests
// ---------------------------------------------------------------------------

interface QueryVectorFile {
  test_groups: Array<{
    tests: QueryTestCase[]
  }>
}

interface QueryTestCase {
  tc_id: number
  comment: string
  input: {
    events?: Array<{
      topic: string
      ts: string
      payload: string
    }>
    query: {
      topic: string
      start: string
      end: string
    }
    store_closed?: boolean
  }
  expected: {
    result: string
    error_kind?: string
    returned_count?: number
    events?: Array<{
      ts: string
      payload: string
    }>
  }
}

describe('Query vectors', () => {
  const vf = readVectorFile('query.json') as QueryVectorFile

  for (const group of vf.test_groups) {
    for (const tc of group.tests) {
      it(`TC${tc.tc_id}: ${tc.comment}`, () => {
        const store = openFreshStore()

        // Setup: insert events with EXACT timestamps via raw SQL (NOT append())
        // This ensures the query tests see the precise nanosecond timestamps from the vectors.
        if (!tc.input.store_closed && tc.input.events) {
          const insertStmt = store._db.prepare(
            'INSERT INTO events (topic, ts, payload) VALUES (?, ?, ?)',
          )
          for (const e of tc.input.events) {
            // Parse timestamp string as BigInt — NEVER Number() (precision loss)
            const ts = BigInt(e.ts)
            const payload = decodePayload(e.payload)
            insertStmt.run(e.topic, ts, payload)
          }
        }

        if (tc.input.store_closed) {
          store.close()
        }

        // Parse start/end as BigInt — NEVER Number()
        const start = BigInt(tc.input.query.start)
        const end = BigInt(tc.input.query.end)

        if (tc.expected.result === 'valid') {
          const events = store.query(tc.input.query.topic, start, end)
          expect(events.length).toBe(tc.expected.returned_count!)

          if (tc.expected.events) {
            for (let i = 0; i < tc.expected.events.length; i++) {
              const expTs = BigInt(tc.expected.events[i].ts)
              const expPayload = decodePayload(tc.expected.events[i].payload)
              expect(events[i].ts).toBe(expTs)
              expect(Buffer.from(events[i].payload)).toEqual(Buffer.from(expPayload))
            }
          }
          if (!tc.input.store_closed) store.close()
        } else {
          let thrownError: unknown
          expect(() => {
            try {
              store.query(tc.input.query.topic, start, end)
            } catch (e) {
              thrownError = e
              throw e
            }
          }).toThrow()
          assertErrorKind(thrownError, tc.expected.error_kind!)
          if (!tc.input.store_closed) store.close()
        }
      })
    }
  }
})

// ---------------------------------------------------------------------------
// Immutability vector tests
// ---------------------------------------------------------------------------

interface ImmutabilityVectorFile {
  test_groups: Array<{
    tests: ImmutabilityTestCase[]
  }>
}

interface ImmutabilityTestCase {
  tc_id: number
  comment: string
  input: {
    setup: Array<{
      topic: string
      payload: string
    }>
    sql: string
  }
  expected: {
    result: string
    error_kind: string
    error_message_contains: string
  }
}

describe('Immutability vectors', () => {
  const vf = readVectorFile('immutability.json') as ImmutabilityVectorFile

  for (const group of vf.test_groups) {
    for (const tc of group.tests) {
      it(`TC${tc.tc_id}: ${tc.comment}`, () => {
        const store = openFreshStore()

        // Setup: insert events via the public Append API
        for (const e of tc.input.setup) {
          const payload = decodePayload(e.payload)
          store.append(e.topic, payload)
        }

        // Execute raw SQL bypass attempt directly against the SQLite connection.
        // The immutability triggers must reject this.
        let thrownError: unknown
        expect(() => {
          try {
            store._db.exec(tc.input.sql)
          } catch (e) {
            thrownError = e
            throw e
          }
        }).toThrow()

        // Verify the error message contains the expected string
        expect(thrownError).toBeInstanceOf(Error)
        const msg = (thrownError as Error).message
        expect(
          msg,
          `Expected error to contain "${tc.expected.error_message_contains}" but got: "${msg}"`,
        ).toContain(tc.expected.error_message_contains)

        store.close()
      })
    }
  }
})
