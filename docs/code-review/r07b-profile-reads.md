# R07b — Profile read and projection boundary

R07b follows R06 characterization (PR #10, `ebd427f`) and R07a category/order persistence (PR #11, merge `1a7968b`). It keeps the existing SQLite adapter, active repository interface, schema and credential store.

## Scope and contracts

`profile_read_repository.go` owns by-ID reads, legacy list/by-ID reads, nullable row scanning and endpoint-specific mapping. A shared database row projection maps common metadata; explicit detail/legacy phases retain their different revision handling, error whitespace, status labels, timestamp locality and raw URI placement. Legacy list scanning still uses a non-nullable revision destination, while detail scanning retains the nullable destination. SQL columns, joins, ordering, fail-closed decryption and error classification are preserved. Safe detail/summary/reveal output still belongs to the existing key-management and HTTP layers.

`profile_repository.go` retains encrypted and legacy mutation commands. `key_repository.go` retains bulk and health operations. No duplicate persistence manager, parser, credential store or DTO is introduced.

Before extraction, commit `930e107` adds passing characterization for the observable detail/list differences, RFC3339 timestamps, empty-list nil behavior and missing IDs. Existing R06 fixtures retain missing/corrupt/wrong-row secrets, unavailable keys, source joins/fallbacks, safe detail and explicit reveal, metadata-only cipher preservation and revision conflicts. The mapping tests assert reads leave the full database snapshot unchanged.

## Validation

Initial targeted storage/HTTP contracts pass (19.677s / 4.986s); the two new characterization tests pass before extraction (3.539s). Direct CodeScene source review scores the new read adapter **10.00** without findings, compared with the inherited complex mapping/conditional logic. The remaining mutation adapter improves from **6.35 to 7.71** and bulk/health file from **8.81 to 9.09**. Remaining command complexity is reported, not suppressed.

Vet, lint (zero issues), server build, migration compatibility (3.438s), workflow validation and strict CodeScene delta against `origin/main` pass. The full uncached backend suite is running with saved JSON evidence under `.cache/r07b`. CI adds the focused existing same-revision profile contention test under Linux's race detector, since this Windows host lacks a CGO compiler. Hosted checks and final automated review must pass at the actual PR head before merge.

The R07a follow-up Windows full-suite attempt used a 180-second per-package bound and timed out in the server package after progressing through unrelated route fixtures; affected storage/service/HTTP suites passed. This was not counted as a full-suite pass. R07a's full hosted backend/frontend/deployment validation passed at `33631dd`. R07b uses a 10-minute bound for its local full suite.

## Next batch

R08 separates patch/ownership phases in `internal/keymanagement/update.go` against the retained R06 contracts. The characterized ignored create-assignment error remains an explicitly separate corrective change; R07b preserves current mutation semantics.
