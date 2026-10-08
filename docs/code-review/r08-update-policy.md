# R08 — Profile update planning and ownership phases

R08 follows the R06 patch/ownership characterization and the R07 persistence boundaries. The service retains fetch → revision check → ownership dispatch → persistence → safe response ordering. The existing repository commands, credential store, DTOs, SQL and revision behavior remain active.

## Scope and preserved behavior

`update_policy.go` prepares local persistence commands without I/O, normalizes metadata, resolves mode/payload compatibility and plans the updated URI. Source-owned planning uses named identity, category and metadata-only payload checks. Display-name normalization shares trim/length handling while callers explicitly supply their existing local-label or empty-source fallback. `update.go` retains orchestration and separates empty structured-patch detection, common endpoint fields and protocol patch dispatch.

Local display-name errors still precede invalid status/kind and mode errors. Source-owned rejection still precedes display-name errors; revision rejection still precedes ownership checks. Absent versus whitespace raw input retains implicit-mode behavior. Source category IDs, source-controlled fields, structured/raw exclusivity, informational keys, malformed stored URI fallback, protocol selection, explicit clears, false versus omitted booleans, plugin/query removal and unchanged-byte no-op patches keep their existing contracts. Mutation transaction ownership and post-commit response mapping are unchanged.

Pre-extraction commit `e880b15` freezes additional request/error contracts: local versus source display-name resets, long-name versus invalid-status precedence, source rejection versus long-name precedence and implicit whitespace raw input. It captures the command sent to the repository rather than relying on the fake repository's projected name. These fixtures and the complete key-management suite pass before production extraction.

## Validation

The complete key-management suite passes before extraction (0.505s), after added characterization (0.402s) and after final policy decomposition (0.443s). Existing R06 protocol tri-state, source/revision, raw-byte and persistence-failure matrices remain the primary compatibility evidence. Direct CodeScene reviews score `update.go`, `update_policy.go` and the changed service-contract test file **10.00**, without findings; the original update module scored **6.64**. Vet, lint (zero new issues), server build and strict delta against the R07b branch pass.

Full uncached Windows backend regression passes with **1,107 passed, five optional skips and no failures**; the server package completes in 310.389s. JSON evidence is under `.cache/r08`. Its own PR must pass hosted backend/frontend/deployment/coverage/CodeScene checks and final automated review before merge. No gate suppression or schema change is included.

## Next work

The original next batch is R09's frontend preservation corpus. A02 also retains R06's reproduced ignored create-assignment failure: handle it in a separately reviewed corrective batch (R08c) with rollback assertions, rather than changing it silently inside R08.

## Final hosted acceptance

[PR #13](https://github.com/romanpodg/SubShare-Go/pull/13) merged at **3e4c333** after all six checks passed at **9c5fc34**: backend, frontend, Docker deployment smoke, CodeScene, Codecov patch and bundles. [Actions run 37731061671](https://github.com/romanpodg/SubShare-Go/actions/runs/37731061671) includes full backend/frontend, migration, user/profile contention races, static and secret scans and populated restart checks. Final Codex review found no major issues, and no inline threads were unresolved. The separate R08c correction follows.
