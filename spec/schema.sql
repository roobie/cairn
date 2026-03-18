-- Cairn schema DDL — execute verbatim on every Open()
-- All statements use IF NOT EXISTS for idempotent execution.
-- No PRAGMAs — connection-level settings belong in Open() behavior.

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
