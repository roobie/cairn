# Phase 1: Spec and Schema - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Create the cross-language contract: API spec document (api.md), shared test vectors (JSON), and schema DDL (schema.sql). All three SDK implementations (Go, TypeScript, Rust) will implement against these artifacts. No SDK code in this phase.

</domain>

<decisions>
## Implementation Decisions

### Spec document structure
- Organized by operation: Concepts section up front, then one section per operation (Open, Close, Append, AppendBatch, Query), then Storage Invariants, then Error Catalog
- Pseudocode signatures for all operations (e.g., `Append(topic: string, payload: bytes) -> EventID | PayloadTooLarge | StoreNotOpen`)
- Formal type definitions in Concepts section: EventID (uint64), Timestamp (int64 nanos), Topic (string), Payload (bytes), Event struct
- Each operation section includes: signature, preconditions, behavior, errors

### Error contract
- Named error codes defined in the spec: PayloadTooLarge, StoreNotOpen, ImmutabilityViolation, EmptyTopic, EmptyPayload
- SDKs must use these names in idiomatic form (Go: ErrPayloadTooLarge, TS: PayloadTooLargeError, Rust: Error::PayloadTooLarge)
- Error Catalog section with table: Code, Operation, Trigger

### Schema DDL
- Single canonical `spec/schema.sql` file that all SDKs execute verbatim via exec()
- schema.sql contains only CREATE TABLE/INDEX/TRIGGER — no PRAGMAs
- INTEGER PRIMARY KEY (no AUTOINCREMENT) as decided in STATE.md
- Composite index on (topic, ts)
- Immutability triggers with descriptive messages: `'cairn: updates not allowed'` and `'cairn: deletes not allowed'`

### Connection PRAGMAs
- Documented in api.md under Open behavior, not in schema.sql
- PRAGMAs are per-connection, set every time: journal_mode=WAL, busy_timeout=5000, foreign_keys=ON, SQLITE_DBCONFIG_DEFENSIVE=ON

### Claude's Discretion
- Test vector JSON structure and file organization
- Exact wording of spec prose between signatures
- Whether to include a versioning header in api.md
- Appendix content (if any)

</decisions>

<specifics>
## Specific Ideas

- Pseudocode style inspired by Protocol Buffers / LSP spec — clean, language-neutral
- Error catalog as a table for quick scanning
- Type definitions block at the top of Concepts so SDK authors see the data model first

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield project, no existing code

### Established Patterns
- None yet — this phase establishes the patterns all SDKs follow

### Integration Points
- spec/API.md will be the single source of truth for Phases 2-3
- spec/schema.sql will be executed verbatim by all three SDKs
- spec/vectors/*.json will be the acceptance test suite for all SDKs

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-spec-and-schema*
*Context gathered: 2026-03-18*
