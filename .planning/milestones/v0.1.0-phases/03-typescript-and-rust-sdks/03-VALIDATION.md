---
phase: 3
slug: typescript-and-rust-sdks
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

**TypeScript:**

| Property | Value |
|----------|-------|
| **Framework** | vitest (latest stable) |
| **Config file** | `ts/vitest.config.ts` |
| **Quick run command** | `cd ts && npx vitest run --reporter=verbose` |
| **Full suite command** | `cd ts && npx vitest run` |
| **Estimated runtime** | ~3 seconds |

**Rust:**

| Property | Value |
|----------|-------|
| **Framework** | Rust built-in test harness (`cargo test`) |
| **Config file** | `rs/Cargo.toml` |
| **Quick run command** | `cd rs && cargo test` |
| **Full suite command** | `cd rs && cargo test` |
| **Estimated runtime** | ~10 seconds (includes bundled SQLite compile on first run) |

---

## Sampling Rate

- **After every task commit:** Run relevant SDK test suite
- **After every plan wave:** Both full suites: `cd ts && npx vitest run` and `cd rs && cargo test`
- **Before `/gsd:verify-work`:** Both suites green + `cd ts && npx tsdown` build succeeds
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | TS-01, TS-02 | unit | `cd ts && npx vitest run -t "Open\|Close"` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | TS-03, TS-04, TS-05 | unit+vector | `cd ts && npx vitest run -t "Append\|Query"` | ❌ W0 | ⬜ pending |
| 03-01-03 | 01 | 1 | TS-06, TS-07 | vector | `cd ts && npx vitest run` | ❌ W0 | ⬜ pending |
| 03-01-04 | 01 | 1 | TS-07 | build | `cd ts && npx tsdown` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | RS-01, RS-02 | integration | `cd rs && cargo test test_open test_drop` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | RS-03, RS-04, RS-05 | integration+vector | `cd rs && cargo test append query` | ❌ W0 | ⬜ pending |
| 03-02-03 | 02 | 1 | RS-06, RS-07 | vector+build | `cd rs && cargo test` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

**TypeScript:**
- [ ] `ts/package.json` — better-sqlite3, vitest, tsdown devDeps
- [ ] `ts/tsconfig.json` — target ESNext, moduleResolution bundler
- [ ] `ts/tsdown.config.ts` — dual ESM+CJS with dts:true
- [ ] `ts/vitest.config.ts` — minimal vitest config
- [ ] `ts/src/index.ts` — Store class, open, close stubs

**Rust:**
- [ ] `rs/Cargo.toml` — rusqlite with bundled feature
- [ ] `rs/src/lib.rs` — Store struct, open, close stubs

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| BigInt timestamps round-trip without precision loss | TS-07 | Automated via safeIntegers, but spot-check with known nanosecond value | Insert event with ts=1700000000123456789n, query back, assert exact match |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
