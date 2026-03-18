# Stack Research

**Domain:** Append-only event storage SDK (SQLite wrapper, three-language implementation)
**Researched:** 2026-03-18
**Confidence:** HIGH (primary choices pre-decided in PROJECT.md, versions verified against live sources)

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `modernc.org/sqlite` (Go) | v1.47.0 | SQLite driver for Go | Pure Go (no CGo), cross-compiles to any GOOS/GOARCH from a single machine. SQLite 3.51.3 bundled. Pre-decided in PROJECT.md. |
| `better-sqlite3` (TypeScript) | 12.8.0 | SQLite driver for Node.js | Synchronous API, fastest Node SQLite library, simpler mental model for an append-only SDK where every write either succeeds or errors — no async error-swallowing. Pre-decided in PROJECT.md. |
| `rusqlite` (Rust) | 0.39.0 | SQLite driver for Rust | Idiomatic Rust bindings, `bundled` feature statically compiles SQLite 3.51.3 — eliminates system library dependency, simplifies cross-platform builds. Pre-decided in PROJECT.md. |

### Supporting Libraries — Go

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | v1.11.1 | Assertions and test helpers | All tests. `assert` for non-fatal checks, `require` for fatal (stop on first failure). Standard in Go SDK libraries. |
| stdlib `testing` | built-in | Test runner, table-driven tests | Used alongside testify. Go test runner is `go test`; no additional runner needed. |
| stdlib `database/sql` | built-in | SQL interface | `modernc.org/sqlite` implements `database/sql`, so no extra abstraction layer required. |

### Supporting Libraries — TypeScript

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `vitest` | 4.1.0 | Test runner and assertions | All tests. Native TypeScript, no transpilation step, 10-20x faster than Jest in watch mode. Supersedes Jest for new projects in 2025+. |
| `tsdown` | 0.20.3 | Library bundler (ESM + CJS + `.d.ts`) | Build step for npm publish. Outputs dual ESM/CJS with type declarations out of the box. Powered by Rolldown (Rust). Replaces tsup (unmaintained as of 2025). |
| `@types/better-sqlite3` | latest | TypeScript types for better-sqlite3 | Dev dependency; required for typed SQLite calls. |

### Supporting Libraries — Rust

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `cargo test` + stdlib | built-in | Test runner | All tests. No third-party test framework needed for an SDK of this size; `#[cfg(test)]` module pattern covers unit + integration. |
| `cargo-release` | latest | Crate versioning and publishing | Release workflow: dry-run validation, version bump, git tag, `cargo publish`. Use instead of manual `cargo publish`. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test ./...` | Run all Go tests | Standard; no extra runner. `-race` flag recommended for WAL concurrency tests. |
| `go vet` | Static analysis | Bundled with Go toolchain. Run in CI. |
| `golangci-lint` | Go linting | Widely adopted meta-linter. Use default config for an SDK. |
| `vitest run` | TypeScript test execution (CI) | Non-watch mode for CI. `vitest` (watch mode) for development. |
| `tsc --noEmit` | TypeScript type-checking | Run separately from `tsdown` build; tsdown does not fail on type errors. |
| `cargo clippy` | Rust linting | Standard; treat warnings as errors in CI (`-D warnings`). |
| `cargo fmt` | Rust formatting | `rustfmt` via cargo. Enforce in CI with `cargo fmt --check`. |
| `cargo nextest` | Faster Rust test runner | Optional but worthwhile for larger test suites; parallel test execution with better failure output. |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Go SQLite | `modernc.org/sqlite` | `mattn/go-sqlite3` | Requires CGo — breaks cross-compilation. For a SDK targeting IoT/edge, CGo adds toolchain complexity. |
| Go SQLite | `modernc.org/sqlite` | `ncruces/go-sqlite3` (wasm) | WASM-based, more experimental, fewer production users. `modernc` is battle-tested. |
| TS SQLite | `better-sqlite3` | `node:sqlite` (built-in, Node 22+) | Still experimental as of Node 22-24; requires `--experimental-sqlite` flag; not production-ready; API is async-leaning which fights the synchronous SDK model. |
| TS SQLite | `better-sqlite3` | `node-sqlite3` (async) | Async API adds complexity for an append-only SDK where operations are inherently sequential. better-sqlite3 is faster in benchmarks. |
| TS bundler | `tsdown` | `tsup` | tsup is no longer actively maintained (maintainer confirmed abandonment, 2025). tsdown is the direct successor, uses same mental model, built on Rolldown. |
| TS bundler | `tsdown` | plain `tsc` | tsc does not bundle, does not output CJS+ESM simultaneously. Dual-format publishing requires a bundler. |
| TS test | `vitest` | `jest` | Jest requires `ts-jest` or Babel transform for TypeScript; vitest supports TypeScript natively. Vitest 4.x is the current standard for new TS projects. |
| Rust SQLite | `rusqlite` (bundled) | `rusqlite` (system-linked) | System-linked requires SQLite to be installed on the target machine. `bundled` feature statically links SQLite 3.51.3, producing a self-contained binary — critical for edge/IoT targets. |
| Rust test | `cargo test` | `rstest` | `rstest` adds parameterized tests but is unnecessary complexity for this SDK size. Stdlib `#[test]` + manual table-driven patterns suffice. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `mattn/go-sqlite3` | CGo dependency breaks cross-compilation. Non-negotiable for IoT/edge targets. | `modernc.org/sqlite` |
| `tsup` | No longer actively maintained as of 2025. Maintainer redirects users to tsdown. | `tsdown` |
| `node:sqlite` (built-in) | Experimental flag required, not stable in any current LTS Node release, async-leaning API contradicts better-sqlite3's synchronous model. | `better-sqlite3` |
| `sqlx` (Rust) | ORM-level abstraction; adds async runtime dependency (tokio/async-std). Cairn is synchronous by design (SQLite WAL single-writer model). | `rusqlite` |
| `diesel` (Rust) | Full ORM, migration framework, code-gen — massive overkill for two SQL operations (`INSERT`, `SELECT`). | `rusqlite` |
| Async Go SQLite wrappers | SQLite's WAL model is single-writer; async wrappers add queue complexity with no benefit. Keep synchronous with connection-level mutex. | Synchronous `modernc.org/sqlite` via `database/sql` |
| Jest | Requires ts-jest/Babel, slower, no longer the default choice for new TS projects in 2025+. | `vitest` |

---

## Installation

### Go

```bash
go get modernc.org/sqlite@v1.47.0
go get github.com/stretchr/testify@v1.11.1
```

### TypeScript

```bash
npm install better-sqlite3
npm install -D @types/better-sqlite3 vitest tsdown typescript
```

### Rust (`Cargo.toml`)

```toml
[dependencies]
rusqlite = { version = "0.39.0", features = ["bundled"] }

[dev-dependencies]
# none needed beyond cargo test
```

---

## Version Notes

All versions verified against live package registries on 2026-03-18:

- `modernc.org/sqlite` v1.47.0 — wraps SQLite 3.51.3 (released 2026-03-17)
- `better-sqlite3` 12.8.0 — requires Node >= 20, wraps SQLite 3.51.3 (released ~2026-03-14)
- `rusqlite` 0.39.0 — released 2026-03-15, bundles SQLite 3.51.3
- `github.com/stretchr/testify` v1.11.1 — released 2025-08-27
- `vitest` 4.1.0 — released ~2026-03-14
- `tsdown` 0.20.3 — released ~2026-02-18

All three SQLite wrappers currently ship SQLite 3.51.3, meaning test vector behavior will be consistent across all three implementations — a meaningful alignment property for a shared-spec project.

---

## Sources

- [modernc.org/sqlite on pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) — v1.47.0, SQLite 3.51.3, pure Go
- [better-sqlite3 on npm](https://www.npmjs.com/package/better-sqlite3) — v12.8.0
- [better-sqlite3 vs node:sqlite discussion](https://github.com/WiseLibs/better-sqlite3/discussions/1245) — production readiness comparison
- [rusqlite on docs.rs](https://docs.rs/crate/rusqlite/latest) — v0.39.0, bundled feature
- [stretchr/testify releases](https://github.com/stretchr/testify/releases) — v1.11.1
- [vitest on npm](https://www.npmjs.com/package/vitest) — v4.1.0
- [Vitest 4.0 announcement](https://vitest.dev/blog/vitest-4) — current stable
- [tsup README — no longer maintained](https://github.com/egoist/tsup) — maintainer directs users to tsdown
- [tsdown introduction](https://tsdown.dev/guide/) — Rolldown-based, tsup migration guide
- [tsdown on npm](https://www.npmjs.com/package/tsdown) — v0.20.3
- [Switching from tsup to tsdown](https://alan.norbauer.com/articles/tsdown-bundler/) — practical migration comparison
- [TypeScript in 2025: ESM and CJS publishing](https://lirantal.com/blog/typescript-in-2025-with-esm-and-cjs-npm-publishing) — dual-format context

---

*Stack research for: Cairn — append-only event storage SDK (Go, TypeScript, Rust)*
*Researched: 2026-03-18*
