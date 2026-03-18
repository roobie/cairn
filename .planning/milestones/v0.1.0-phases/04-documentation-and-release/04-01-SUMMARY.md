---
phase: 04-documentation-and-release
plan: 01
subsystem: documentation
tags: [readme, docs, quickstart, sdk]
dependency_graph:
  requires: [02-01, 03-01, 03-02]
  provides: [root-readme, go-readme, ts-readme, rs-readme]
  affects: []
tech_stack:
  added: []
  patterns: [per-language-readme, quickstart-snippet]
key_files:
  created:
    - README.md
    - go/README.md
    - ts/README.md
    - rs/README.md
  modified: []
decisions:
  - "Root README uses trail marker metaphor in opening paragraph per PROJECT.md name origin"
  - "TypeScript BigInt note placed prominently with conversion recipe (Date.now() * 1_000_000n)"
  - "Rust Drop note explains explicit vs implicit close trade-off"
  - "Go README documents SQLITE_DBCONFIG_DEFENSIVE fallback behavior (trusted_schema pragma)"
metrics:
  duration: 5 min
  completed: 2026-03-18
  tasks_completed: 2
  files_created: 4
  files_modified: 0
---

# Phase 4 Plan 1: Documentation — Root and Per-Language READMEs Summary

Root README with append-only positioning and per-language quickstarts, plus three SDK READMEs covering full API surface, error types, and language-specific notes (BigInt for TS, Drop for Rust, no-CGo for Go).

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Root README | 72ef3bf | README.md |
| 2 | Per-language READMEs | 4d8fb93 | go/README.md, ts/README.md, rs/README.md |

## Decisions Made

1. **Root README structure**: Title + tagline, "Why cairn" (constraint is the product), quickstarts per language, storage guarantees, API overview table, SDK links. No badges or contributing section — library README, not product landing page.

2. **BigInt note placement**: In ts/README.md, the BigInt section is elevated to a top-level "Language-Specific Notes" section (not buried in API docs) because it's the #1 footgun. Includes concrete conversion recipe.

3. **Drop note structure**: In rs/README.md, Drop is explained with a code example showing the scoping pattern. Explicit `close()` is recommended only when checkpoint error handling is needed.

4. **Go SQLITE_DBCONFIG_DEFENSIVE**: Documented accurately — the Go SDK falls back to `PRAGMA trusted_schema = OFF` because `modernc.org/sqlite` does not expose the C-level config via `database/sql`. This matches the actual implementation in go/cairn.go.

5. **Install instructions**: Go uses actual module path `cairn.dev/sdk/go` from go.mod; TypeScript uses package name `cairn` from package.json; Rust references both crates.io future path and local path for development.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Created Files Exist
- [x] README.md — FOUND
- [x] go/README.md — FOUND
- [x] ts/README.md — FOUND
- [x] rs/README.md — FOUND

### Commits Exist
- [x] 72ef3bf — FOUND (docs(04-01): create root README with positioning and quickstarts)
- [x] 4d8fb93 — FOUND (docs(04-01): create per-language READMEs for Go, TypeScript, and Rust)

### Verification Criteria
- [x] README.md contains "append-only" — PASS
- [x] README.md contains "Open" — PASS
- [x] README.md contains "BigInt" — PASS
- [x] README.md contains "Drop" — PASS
- [x] go/README.md contains "modernc" — PASS
- [x] ts/README.md contains "BigInt" — PASS
- [x] rs/README.md contains "Drop" — PASS

## Self-Check: PASSED
