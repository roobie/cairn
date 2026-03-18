//! Integration test harness consuming all spec/vectors/*.json files.
//!
//! Mirrors the pattern from go/cairn_test.go.
//!
//! cargo test CWD is the crate root (rs/), so vector files are at:
//!   ../spec/vectors/{name}

use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine;
use cairn::{open, BatchEvent, Error, Store};
use tempfile::TempDir;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Reads a spec vector file by name. Panics if the file cannot be read.
fn read_vector_file(name: &str) -> String {
    std::fs::read_to_string(format!("../spec/vectors/{}", name))
        .unwrap_or_else(|e| panic!("read_vector_file {}: {}", name, e))
}

/// Decodes a base64 payload string. Empty string → empty Vec.
fn decode_payload(b64: &str) -> Vec<u8> {
    if b64.is_empty() {
        return vec![];
    }
    BASE64
        .decode(b64)
        .unwrap_or_else(|e| panic!("decode_payload: invalid base64 {:?}: {}", b64, e))
}

/// Asserts that `err.kind()` matches the expected error_kind string from the vector file.
fn assert_error_kind(err: &Error, expected: &str) {
    assert_eq!(
        err.kind(),
        expected,
        "expected error kind {:?}, got error: {:?}",
        expected,
        err
    );
}

/// Opens a temporary store. Both the Store and TempDir are returned; the caller
/// must keep TempDir alive as long as Store is in use.
fn open_test_store() -> (Store, TempDir) {
    let dir = TempDir::new().expect("create tempdir");
    let path = dir.path().join("test.db");
    let store = open(path.to_str().unwrap()).expect("open store");
    (store, dir)
}

// ---------------------------------------------------------------------------
// Open / Close unit tests
// ---------------------------------------------------------------------------

#[test]
fn test_open_creates_db_file() {
    let (_store, dir) = open_test_store();
    let db_path = dir.path().join("test.db");
    assert!(db_path.exists(), "database file should exist after open()");
}

#[test]
fn test_open_missing_parent_dir_returns_error() {
    let result = open("/nonexistent/dir/that/does/not/exist/test.db");
    assert!(
        result.is_err(),
        "open() should fail when parent directory does not exist"
    );
    match result.unwrap_err() {
        Error::Storage(_) => {} // expected
        other => panic!("expected Error::Storage, got {:?}", other),
    }
}

#[test]
fn test_open_idempotent_on_existing_db() {
    let dir = TempDir::new().expect("create tempdir");
    let path = dir.path().join("test.db");
    let path_str = path.to_str().unwrap();

    let mut s1 = open(path_str).expect("first open");
    s1.close().expect("first close");

    let s2 = open(path_str).expect("second open (idempotent)");
    drop(s2); // Drop runs close
}

#[test]
fn test_close_idempotent() {
    let (mut store, _dir) = open_test_store();
    store.close().expect("first close");
    store.close().expect("second close should be no-op");
}

#[test]
fn test_drop_wal_checkpoint() {
    let dir = TempDir::new().expect("create tempdir");
    let db_path = dir.path().join("test.db");
    let wal_path = dir.path().join("test.db-wal");

    {
        let mut store = open(db_path.to_str().unwrap()).expect("open store");
        // Write data to put frames into the WAL
        store.append("test", b"hello").expect("append");
        // store is dropped here — Drop runs the WAL checkpoint
    }

    // WAL file should be absent or zero-length after Drop
    if let Ok(meta) = std::fs::metadata(&wal_path) {
        assert_eq!(
            meta.len(),
            0,
            "WAL file should be truncated to 0 bytes after Drop, got {} bytes",
            meta.len()
        );
    }
    // If WAL file doesn't exist at all, that's also acceptable
}

// ---------------------------------------------------------------------------
// Append vector tests
// ---------------------------------------------------------------------------

#[test]
fn test_append_vectors() {
    let raw = read_vector_file("append.json");
    let v: serde_json::Value = serde_json::from_str(&raw).expect("parse append.json");

    for group in v["test_groups"].as_array().expect("test_groups array") {
        for tc in group["tests"].as_array().expect("tests array") {
            let tc_id = tc["tc_id"].as_u64().unwrap_or(0);
            let comment = tc["comment"].as_str().unwrap_or("");

            let (mut store, _dir) = open_test_store();

            let store_closed = tc["input"]["store_closed"].as_bool().unwrap_or(false);
            if store_closed {
                store.close().expect("close before test");
            }

            // Build payload
            let payload: Vec<u8> = if let Some(sz) = tc["input"]["payload_size_bytes"].as_u64() {
                vec![0u8; sz as usize]
            } else {
                decode_payload(tc["input"]["payload"].as_str().unwrap_or(""))
            };

            let topic = tc["input"]["topic"].as_str().unwrap_or("");
            let result = store.append(topic, &payload);

            let expected_result = tc["expected"]["result"].as_str().unwrap_or("");
            if expected_result == "valid" {
                match result {
                    Ok(id) => {
                        let min_id: u64 = tc["expected"]["event_id_min"]
                            .as_str()
                            .unwrap_or("1")
                            .parse()
                            .unwrap_or(1);
                        assert!(
                            id >= min_id,
                            "TC{} ({}): id {} < min {}",
                            tc_id,
                            comment,
                            id,
                            min_id
                        );
                    }
                    Err(e) => panic!("TC{} ({}): unexpected error: {:?}", tc_id, comment, e),
                }
            } else {
                let expected_kind = tc["expected"]["error_kind"].as_str().unwrap_or("");
                match result {
                    Ok(id) => panic!(
                        "TC{} ({}): expected error {:?}, got id {}",
                        tc_id, comment, expected_kind, id
                    ),
                    Err(ref e) => assert_error_kind(e, expected_kind),
                }
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Batch vector tests
// ---------------------------------------------------------------------------

#[test]
fn test_batch_vectors() {
    let raw = read_vector_file("batch.json");
    let v: serde_json::Value = serde_json::from_str(&raw).expect("parse batch.json");

    for group in v["test_groups"].as_array().expect("test_groups array") {
        for tc in group["tests"].as_array().expect("tests array") {
            let tc_id = tc["tc_id"].as_u64().unwrap_or(0);
            let comment = tc["comment"].as_str().unwrap_or("");

            let (mut store, _dir) = open_test_store();

            let store_closed = tc["input"]["store_closed"].as_bool().unwrap_or(false);
            if store_closed {
                store.close().expect("close before test");
            }

            // Build batch events
            let mut batch: Vec<BatchEvent> = Vec::new();
            if let Some(events_arr) = tc["input"]["events"].as_array() {
                for e in events_arr {
                    let payload: Vec<u8> =
                        if let Some(sz) = e["payload_size_bytes"].as_u64() {
                            vec![0u8; sz as usize]
                        } else {
                            decode_payload(e["payload"].as_str().unwrap_or(""))
                        };
                    batch.push(BatchEvent {
                        topic: e["topic"].as_str().unwrap_or("").to_string(),
                        payload,
                    });
                }
            }

            let result = store.append_batch(&batch);

            let expected_result = tc["expected"]["result"].as_str().unwrap_or("");
            if expected_result == "valid" {
                match result {
                    Ok(ids) => {
                        let expected_count =
                            tc["expected"]["event_id_count"].as_u64().unwrap_or(0) as usize;
                        assert_eq!(
                            ids.len(),
                            expected_count,
                            "TC{} ({}): ids.len() = {}, want {}",
                            tc_id,
                            comment,
                            ids.len(),
                            expected_count
                        );
                    }
                    Err(e) => panic!("TC{} ({}): unexpected error: {:?}", tc_id, comment, e),
                }
            } else {
                let expected_kind = tc["expected"]["error_kind"].as_str().unwrap_or("");
                match result {
                    Ok(ids) => panic!(
                        "TC{} ({}): expected error {:?}, got {} ids",
                        tc_id,
                        comment,
                        expected_kind,
                        ids.len()
                    ),
                    Err(ref e) => assert_error_kind(e, expected_kind),
                }
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Query vector tests
// ---------------------------------------------------------------------------

#[test]
fn test_query_vectors() {
    let raw = read_vector_file("query.json");
    let v: serde_json::Value = serde_json::from_str(&raw).expect("parse query.json");

    for group in v["test_groups"].as_array().expect("test_groups array") {
        for tc in group["tests"].as_array().expect("tests array") {
            let tc_id = tc["tc_id"].as_u64().unwrap_or(0);
            let comment = tc["comment"].as_str().unwrap_or("");

            let (mut store, _dir) = open_test_store();

            // Setup: insert events with EXACT timestamps via raw SQL
            let store_closed = tc["input"]["store_closed"].as_bool().unwrap_or(false);
            if !store_closed {
                if let Some(setup_events) = tc["input"]["events"].as_array() {
                    for e in setup_events {
                        let ts: i64 = e["ts"]
                            .as_str()
                            .unwrap_or("0")
                            .parse()
                            .unwrap_or(0);
                        let payload = decode_payload(e["payload"].as_str().unwrap_or(""));
                        let topic = e["topic"].as_str().unwrap_or("");
                        // Access raw connection for exact-timestamp insert
                        store
                            .raw_conn()
                            .execute(
                                "INSERT INTO events (topic, ts, payload) VALUES (?1, ?2, ?3)",
                                rusqlite::params![topic, ts, payload],
                            )
                            .unwrap_or_else(|e| {
                                panic!("TC{}: setup INSERT failed: {}", tc_id, e)
                            });
                    }
                }
            }

            if store_closed {
                store.close().expect("close before test");
            }

            let start: i64 = tc["input"]["query"]["start"]
                .as_str()
                .unwrap_or("0")
                .parse()
                .unwrap_or(0);
            let end: i64 = tc["input"]["query"]["end"]
                .as_str()
                .unwrap_or("0")
                .parse()
                .unwrap_or(0);
            let topic = tc["input"]["query"]["topic"].as_str().unwrap_or("");

            let result = store.query(topic, start, end);

            let expected_result = tc["expected"]["result"].as_str().unwrap_or("");
            if expected_result == "valid" {
                let events = match result {
                    Ok(e) => e,
                    Err(e) => panic!("TC{} ({}): unexpected error: {:?}", tc_id, comment, e),
                };

                let expected_count =
                    tc["expected"]["returned_count"].as_u64().unwrap_or(0) as usize;
                assert_eq!(
                    events.len(),
                    expected_count,
                    "TC{} ({}): returned {} events, want {}",
                    tc_id,
                    comment,
                    events.len(),
                    expected_count
                );

                if let Some(expected_events) = tc["expected"]["events"].as_array() {
                    for (i, exp) in expected_events.iter().enumerate() {
                        let exp_ts: i64 = exp["ts"]
                            .as_str()
                            .unwrap_or("0")
                            .parse()
                            .unwrap_or(0);
                        let exp_payload = decode_payload(exp["payload"].as_str().unwrap_or(""));

                        assert_eq!(
                            events[i].ts, exp_ts,
                            "TC{} ({}): events[{}].ts = {}, want {}",
                            tc_id, comment, i, events[i].ts, exp_ts
                        );
                        assert_eq!(
                            events[i].payload, exp_payload,
                            "TC{} ({}): events[{}].payload mismatch",
                            tc_id, comment, i
                        );
                    }
                }
            } else {
                let expected_kind = tc["expected"]["error_kind"].as_str().unwrap_or("");
                match result {
                    Ok(events) => panic!(
                        "TC{} ({}): expected error {:?}, got {} events",
                        tc_id,
                        comment,
                        expected_kind,
                        events.len()
                    ),
                    Err(ref e) => assert_error_kind(e, expected_kind),
                }
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Immutability vector tests
// ---------------------------------------------------------------------------

#[test]
fn test_immutability_vectors() {
    let raw = read_vector_file("immutability.json");
    let v: serde_json::Value = serde_json::from_str(&raw).expect("parse immutability.json");

    for group in v["test_groups"].as_array().expect("test_groups array") {
        for tc in group["tests"].as_array().expect("tests array") {
            let tc_id = tc["tc_id"].as_u64().unwrap_or(0);
            let comment = tc["comment"].as_str().unwrap_or("");

            let (mut store, _dir) = open_test_store();

            // Setup: insert events via the public Append API
            if let Some(setup_arr) = tc["input"]["setup"].as_array() {
                for e in setup_arr {
                    let payload = decode_payload(e["payload"].as_str().unwrap_or(""));
                    let topic = e["topic"].as_str().unwrap_or("");
                    store
                        .append(topic, &payload)
                        .unwrap_or_else(|e| panic!("TC{}: setup append: {:?}", tc_id, e));
                }
            }

            // Execute raw SQL bypass attempt via the connection
            let sql = tc["input"]["sql"].as_str().unwrap_or("");
            let err = store
                .raw_conn()
                .execute(sql, [])
                .expect_err("expected immutability trigger to reject the SQL");

            let expected_contains = tc["expected"]["error_message_contains"]
                .as_str()
                .unwrap_or("");
            let err_msg = err.to_string();
            assert!(
                err_msg.contains(expected_contains),
                "TC{} ({}): error {:?} does not contain {:?}",
                tc_id,
                comment,
                err_msg,
                expected_contains
            );
        }
    }
}
