# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-18)

**Core value:** Append-only immutability enforced at the storage level — if it's in cairn, it can't be altered or removed.
**Current focus:** Phase 1 — Spec and Schema

## Current Position

Phase: 1 of 4 (Spec and Schema)
Plan: 0 of 2 in current phase
Status: Ready to plan
Last activity: 2026-03-18 — Roadmap created

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: -
- Total execution time: -

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: v1 API surface is Open/Close, Append, AppendBatch, Query (single topic) — no rotation, no metadata ops, no QueryAll in v1
- [Roadmap]: TypeScript and Rust SDKs build in parallel (Phase 3) after Go reference implementation (Phase 2)
- [Spec]: Schema must use INTEGER PRIMARY KEY (no AUTOINCREMENT) — removes sqlite_sequence write overhead
- [Spec]: Test vector timestamps as quoted strings, payloads as RFC 4648 base64 — prevents cross-language encoding divergence

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 1]: Rotation-boundary multi-segment merge contract needs explicit design — how are events ordered when two segment files have equal timestamps at midnight boundary? (Deferred: rotation is v2, but spec should note the open question)
- [Phase 2]: modernc.org/sqlite throughput floor needs early validation — run AppendBatch micro-benchmark before committing to >50K/sec README claim

## Session Continuity

Last session: 2026-03-18
Stopped at: Roadmap created, STATE.md initialized — ready to plan Phase 1
Resume file: None
