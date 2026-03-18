//! Cairn Rust SDK — append-only event store backed by SQLite.
//!
//! Usage:
//! ```no_run
//! use cairn::{open, BatchEvent};
//!
//! let mut store = open("/path/to/store.db").unwrap();
//! let id = store.append("sensor", b"hello").unwrap();
//! let events = store.query("sensor", 0, i64::MAX).unwrap();
//! store.close().unwrap();
//! ```

use rusqlite::{config::DbConfig, Connection};
use std::fmt;
use std::time::{SystemTime, UNIX_EPOCH};

// ---------------------------------------------------------------------------
// Schema DDL — embedded verbatim from spec/schema.sql
// ---------------------------------------------------------------------------

const SCHEMA_SQL: &str = "
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
";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Maximum payload size in bytes (1 MiB). Payloads of exactly this size are valid;
/// payloads exceeding this size return [`Error::PayloadTooLarge`].
pub const MAX_PAYLOAD_SIZE: usize = 1_048_576;

// ---------------------------------------------------------------------------
// Error enum
// ---------------------------------------------------------------------------

/// Errors returned by cairn SDK operations.
#[derive(Debug)]
pub enum Error {
    /// Payload exceeds the 1 MiB limit (`len(payload) > 1,048,576`).
    PayloadTooLarge,
    /// Operation called on a closed or uninitialized store.
    StoreNotOpen,
    /// An UPDATE or DELETE was attempted against the immutable events table.
    ImmutabilityViolation,
    /// The topic string was empty.
    EmptyTopic,
    /// The payload was zero bytes.
    EmptyPayload,
    /// An underlying SQLite or I/O error.
    Storage(String),
}

impl Error {
    /// Returns the spec error_kind string for this error variant.
    /// Used by the test harness to match expected `error_kind` fields.
    pub fn kind(&self) -> &str {
        match self {
            Error::PayloadTooLarge => "payload_too_large",
            Error::StoreNotOpen => "store_not_open",
            Error::ImmutabilityViolation => "immutability_violation",
            Error::EmptyTopic => "empty_topic",
            Error::EmptyPayload => "empty_payload",
            Error::Storage(_) => "storage",
        }
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::PayloadTooLarge => write!(f, "cairn: payload too large"),
            Error::StoreNotOpen => write!(f, "cairn: store not open"),
            Error::ImmutabilityViolation => write!(f, "cairn: immutability violation"),
            Error::EmptyTopic => write!(f, "cairn: empty topic"),
            Error::EmptyPayload => write!(f, "cairn: empty payload"),
            Error::Storage(msg) => write!(f, "cairn: storage error: {}", msg),
        }
    }
}

impl std::error::Error for Error {}

// ---------------------------------------------------------------------------
// Public data types
// ---------------------------------------------------------------------------

/// An event returned by [`Store::query`].
#[derive(Debug, Clone)]
pub struct Event {
    /// SQLite `INTEGER PRIMARY KEY` (insertion order).
    pub id: u64,
    /// Non-empty UTF-8 topic string.
    pub topic: String,
    /// Nanoseconds since Unix epoch (1970-01-01T00:00:00Z).
    pub ts: i64,
    /// Opaque binary payload.
    pub payload: Vec<u8>,
}

/// An input event for [`Store::append_batch`].
#[derive(Debug, Clone)]
pub struct BatchEvent {
    /// Non-empty UTF-8 topic string.
    pub topic: String,
    /// Non-empty payload, at most [`MAX_PAYLOAD_SIZE`] bytes.
    pub payload: Vec<u8>,
}

// ---------------------------------------------------------------------------
// Store struct
// ---------------------------------------------------------------------------

/// An open handle to a cairn database.
///
/// Create with [`open`]. Automatically closed via [`Drop`] when it goes out of scope.
/// Call [`Store::close`] explicitly if you need to handle checkpoint errors.
pub struct Store {
    /// The SQLite connection. Wrapped in Option so Drop can take ownership.
    conn: Option<Connection>,
    closed: bool,
}

impl std::fmt::Debug for Store {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Store")
            .field("closed", &self.closed)
            .finish_non_exhaustive()
    }
}

// ---------------------------------------------------------------------------
// open() — factory function
// ---------------------------------------------------------------------------

/// Opens or creates a cairn database at `path`.
///
/// The parent directory of `path` must already exist — `open` does not create
/// parent directories. On success, the schema DDL has been applied and WAL mode
/// is active.
///
/// # Errors
///
/// Returns [`Error::Storage`] if the parent directory does not exist or any
/// SQLite initialisation step fails.
pub fn open(path: &str) -> Result<Store, Error> {
    // Validate parent directory exists before attempting to open.
    let parent = std::path::Path::new(path)
        .parent()
        .ok_or_else(|| Error::Storage("cannot determine parent directory".to_string()))?;

    // An empty parent means the current directory — that always exists.
    if !parent.as_os_str().is_empty() {
        std::fs::metadata(parent)
            .map_err(|e| Error::Storage(format!("parent directory does not exist: {}", e)))?;
    }

    let conn = Connection::open(path)
        .map_err(|e| Error::Storage(e.to_string()))?;

    // Apply connection-level PRAGMAs.
    conn.execute_batch(
        "PRAGMA journal_mode = WAL;
         PRAGMA busy_timeout = 5000;
         PRAGMA foreign_keys = ON;",
    )
    .map_err(|e| Error::Storage(e.to_string()))?;

    // Enable SQLITE_DBCONFIG_DEFENSIVE — prevents disabling triggers via writable_schema.
    conn.set_db_config(DbConfig::SQLITE_DBCONFIG_DEFENSIVE, true)
        .map_err(|e| Error::Storage(e.to_string()))?;

    // Execute schema DDL — all statements use IF NOT EXISTS so this is idempotent.
    conn.execute_batch(SCHEMA_SQL)
        .map_err(|e| Error::Storage(e.to_string()))?;

    Ok(Store {
        conn: Some(conn),
        closed: false,
    })
}

// ---------------------------------------------------------------------------
// Store impl
// ---------------------------------------------------------------------------

impl Store {
    /// Returns a reference to the underlying SQLite connection.
    ///
    /// Intended for test harnesses that need to execute raw SQL (e.g. immutability
    /// and query vector tests that must insert with exact timestamps).
    ///
    /// Panics if the store has already been closed.
    #[doc(hidden)]
    pub fn raw_conn(&self) -> &Connection {
        self.conn
            .as_ref()
            .expect("raw_conn called on a closed store")
    }

    /// Returns Ok(()) if the store is open, otherwise Error::StoreNotOpen.
    fn check_open(&self) -> Result<(), Error> {
        if self.closed {
            Err(Error::StoreNotOpen)
        } else {
            Ok(())
        }
    }

    /// Closes the store, running a WAL checkpoint before closing the connection.
    ///
    /// This method is idempotent — calling it multiple times is safe and
    /// returns `Ok(())` after the first call.
    pub fn close(&mut self) -> Result<(), Error> {
        if self.closed {
            return Ok(());
        }
        self.closed = true;
        if let Some(conn) = self.conn.take() {
            // PRAGMA wal_checkpoint(TRUNCATE) — ignore errors per spec (Close is always safe).
            let _ = conn.execute_batch("PRAGMA wal_checkpoint(TRUNCATE)");
            // Connection is dropped and closed here.
        }
        Ok(())
    }

    /// Inserts a single event and returns its `EventID` (>= 1).
    ///
    /// # Errors
    ///
    /// - [`Error::StoreNotOpen`] if the store has been closed.
    /// - [`Error::EmptyTopic`] if `topic` is empty.
    /// - [`Error::EmptyPayload`] if `payload` is empty.
    /// - [`Error::PayloadTooLarge`] if `payload.len() > 1,048,576`.
    /// - [`Error::Storage`] on underlying SQLite error.
    pub fn append(&mut self, topic: &str, payload: &[u8]) -> Result<u64, Error> {
        self.check_open()?;
        if topic.is_empty() {
            return Err(Error::EmptyTopic);
        }
        if payload.is_empty() {
            return Err(Error::EmptyPayload);
        }
        if payload.len() > MAX_PAYLOAD_SIZE {
            return Err(Error::PayloadTooLarge);
        }

        let ts = now_nanos();
        let conn = self.conn.as_ref().unwrap();
        conn.execute(
            "INSERT INTO events (topic, ts, payload) VALUES (?1, ?2, ?3)",
            rusqlite::params![topic, ts, payload],
        )
        .map_err(|e| Error::Storage(e.to_string()))?;

        Ok(conn.last_insert_rowid() as u64)
    }

    /// Inserts multiple events in a single atomic transaction and returns their `EventID`s.
    ///
    /// If `events` is empty, returns an empty `Vec` immediately with no transaction opened.
    /// All events are validated before any insert — if any fails validation, no events are inserted.
    ///
    /// # Errors
    ///
    /// - [`Error::StoreNotOpen`] if the store has been closed.
    /// - [`Error::EmptyTopic`] for the first event with an empty topic.
    /// - [`Error::EmptyPayload`] for the first event with an empty payload.
    /// - [`Error::PayloadTooLarge`] for the first event with payload > 1 MiB.
    /// - [`Error::Storage`] on underlying SQLite error.
    pub fn append_batch(&mut self, events: &[BatchEvent]) -> Result<Vec<u64>, Error> {
        self.check_open()?;
        if events.is_empty() {
            return Ok(vec![]);
        }

        // Validate ALL events before inserting any (all-or-nothing).
        for e in events {
            if e.topic.is_empty() {
                return Err(Error::EmptyTopic);
            }
            if e.payload.is_empty() {
                return Err(Error::EmptyPayload);
            }
            if e.payload.len() > MAX_PAYLOAD_SIZE {
                return Err(Error::PayloadTooLarge);
            }
        }

        let conn = self.conn.as_mut().unwrap();
        let tx = conn
            .transaction()
            .map_err(|e| Error::Storage(e.to_string()))?;

        let mut ids = Vec::with_capacity(events.len());
        {
            let mut stmt = tx
                .prepare("INSERT INTO events (topic, ts, payload) VALUES (?1, ?2, ?3)")
                .map_err(|e| Error::Storage(e.to_string()))?;

            for e in events {
                let ts = now_nanos();
                stmt.execute(rusqlite::params![e.topic, ts, e.payload])
                    .map_err(|e| Error::Storage(e.to_string()))?;
                ids.push(tx.last_insert_rowid() as u64);
            }
        }

        tx.commit().map_err(|e| Error::Storage(e.to_string()))?;
        Ok(ids)
    }

    /// Returns all events in `topic` with timestamp in `[start, end]` inclusive,
    /// ordered by `id` ascending (insertion order).
    ///
    /// Returns an empty `Vec` (not an error) when no events match.
    ///
    /// # Errors
    ///
    /// - [`Error::StoreNotOpen`] if the store has been closed.
    /// - [`Error::Storage`] on underlying SQLite error.
    pub fn query(&self, topic: &str, start: i64, end: i64) -> Result<Vec<Event>, Error> {
        self.check_open()?;
        let conn = self.conn.as_ref().unwrap();
        let mut stmt = conn
            .prepare(
                "SELECT id, topic, ts, payload \
                 FROM events \
                 WHERE topic = ?1 AND ts >= ?2 AND ts <= ?3 \
                 ORDER BY id ASC",
            )
            .map_err(|e| Error::Storage(e.to_string()))?;

        let events = stmt
            .query_map(rusqlite::params![topic, start, end], |row| {
                Ok(Event {
                    id: row.get::<_, i64>(0)? as u64,
                    topic: row.get(1)?,
                    ts: row.get(2)?,
                    payload: row.get(3)?,
                })
            })
            .map_err(|e| Error::Storage(e.to_string()))?
            .collect::<rusqlite::Result<Vec<_>>>()
            .map_err(|e| Error::Storage(e.to_string()))?;

        Ok(events)
    }
}

// ---------------------------------------------------------------------------
// Drop impl — safety net WAL checkpoint
// ---------------------------------------------------------------------------

impl Drop for Store {
    /// Runs the WAL checkpoint and closes the connection.
    ///
    /// Errors are silently discarded — `Drop` must never panic.
    fn drop(&mut self) {
        let _ = self.close();
    }
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/// Returns the current time as nanoseconds since Unix epoch, cast to i64.
///
/// The i64 range accommodates nanosecond timestamps up to year 2262.
fn now_nanos() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos() as i64
}
