---
phase: 1
slug: spec-and-schema
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | None — Phase 1 produces no executable code |
| **Config file** | N/A |
| **Quick run command** | `python3 -m json.tool spec/vectors/*.json > /dev/null` |
| **Full suite command** | Manual checklist review against Success Criteria |
| **Estimated runtime** | ~1 second (JSON syntax only) |

---

## Sampling Rate

- **After every task commit:** Run `python3 -m json.tool spec/vectors/*.json > /dev/null`
- **After every plan wave:** Manual checklist review against Success Criteria
- **Before `/gsd:verify-work`:** All 4 Success Criteria from ROADMAP.md verified
- **Max feedback latency:** 1 second

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | SPEC-01 | manual | `test -f spec/api.md` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | SPEC-04 | grep | `grep -c AUTOINCREMENT spec/schema.sql` (expect 0) | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 1 | STOR-01 | grep | `grep -c journal_mode spec/api.md` | ❌ W0 | ⬜ pending |
| 01-01-04 | 01 | 1 | STOR-02 | grep | `grep -c 'BEFORE DELETE' spec/schema.sql` | ❌ W0 | ⬜ pending |
| 01-01-05 | 01 | 1 | STOR-03 | grep | `grep -c SQLITE_DBCONFIG_DEFENSIVE spec/api.md` | ❌ W0 | ⬜ pending |
| 01-01-06 | 01 | 1 | STOR-04 | grep | `grep -c nanosecond spec/api.md` | ❌ W0 | ⬜ pending |
| 01-01-07 | 01 | 1 | STOR-05 | grep | `grep -c BLOB spec/schema.sql` | ❌ W0 | ⬜ pending |
| 01-01-08 | 01 | 1 | STOR-06 | grep | `grep -c 1048576 spec/api.md` | ❌ W0 | ⬜ pending |
| 01-01-09 | 01 | 1 | STOR-07 | grep | `grep -c wal_checkpoint spec/api.md` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | SPEC-02 | json | `python3 -m json.tool spec/vectors/*.json > /dev/null` | ❌ W0 | ⬜ pending |
| 01-02-02 | 02 | 1 | SPEC-03 | grep | `grep -r '"ts":' spec/vectors/ \| grep -v '"ts": "'` (expect empty) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `spec/api.md` — covers SPEC-01, STOR-01, STOR-03, STOR-04, STOR-05, STOR-06, STOR-07
- [ ] `spec/schema.sql` — covers SPEC-04, STOR-02, STOR-05
- [ ] `spec/vectors/append.json` — covers SPEC-02, SPEC-03 (partial)
- [ ] `spec/vectors/batch.json` — covers SPEC-02 (batch case)
- [ ] `spec/vectors/query.json` — covers SPEC-02, SPEC-03 (timestamp strings)
- [ ] `spec/vectors/immutability.json` — covers SPEC-02 (immutability rejection cases)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| api.md defines all 5 operations with full error semantics | SPEC-01 | Semantic review — grep can confirm existence but not completeness | Review each operation section for: signature, preconditions, behavior, errors |
| Test vectors cover all 6 required cases | SPEC-02 | Structural completeness — need human judgment | Verify append single, append batch, query range, query empty, reject UPDATE, reject DELETE |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 1s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
