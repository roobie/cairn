---
phase: 2
slug: go-sdk
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` package |
| **Config file** | None — `go test` requires no config file |
| **Quick run command** | `cd go && go test ./...` |
| **Full suite command** | `cd go && go test -v -count=1 ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd go && go test ./...`
- **After every plan wave:** Run `cd go && CGO_ENABLED=0 go build ./... && go test -v -count=1 ./...`
- **Before `/gsd:verify-work`:** All vector tests green + `CGO_ENABLED=0 go build ./...` passes
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | GO-01 | unit | `cd go && go test -run TestOpen ./...` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | GO-02 | unit | `cd go && go test -run TestClose ./...` | ❌ W0 | ⬜ pending |
| 02-01-03 | 01 | 1 | GO-07 | build | `cd go && CGO_ENABLED=0 go build ./...` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 2 | GO-03 | vector | `cd go && go test -run TestAppendVectors ./...` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 2 | GO-04 | vector | `cd go && go test -run TestBatchVectors ./...` | ❌ W0 | ⬜ pending |
| 02-02-03 | 02 | 2 | GO-05 | vector | `cd go && go test -run TestQueryVectors ./...` | ❌ W0 | ⬜ pending |
| 02-02-04 | 02 | 2 | GO-06 | vector | `cd go && go test -run TestImmutabilityVectors ./...` | ❌ W0 | ⬜ pending |
| 02-02-05 | 02 | 2 | GO-06 | integration | `cd go && go test -v -count=1 ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go/go.mod` — declares module and modernc.org/sqlite dependency
- [ ] `go/cairn.go` — Store type, Open, Close stubs
- [ ] `go/errors.go` — sentinel error variables
- [ ] `go/cairn_test.go` — test vector harness stubs
- [ ] Vector file access resolved (go:embed vs os.ReadFile)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SQLITE_DBCONFIG_DEFENSIVE enabled | GO-01 | May require raw conn introspection | Verify via code review that `Open()` calls defensive config or documents fallback |

*All other behaviors have automated verification via test vectors.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
