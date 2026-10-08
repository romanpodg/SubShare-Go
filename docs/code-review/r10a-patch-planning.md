# R10a — Pure patch planning and ordered token application

R10a follows R09's accepted preservation corpus, merged in PR #15 at `01f92aa`. It changes only the Xray JSON patch path inside `frontend/src/lib/configuration.ts`; all public exports, existing parsers, builders and token-editor APIs remain compatible.

## Implementation and protected behavior

`buildXrayPatchPlan` constructs ordered path/value edits without changing the raw document or input projection. Named protocol, connection, user, stream, transport and security phases append only represented field changes and required parent initialization. `applyXrayDocumentEdits` applies that order through the existing token-aware modifier and final formatter. No parsed document or selected subtree is serialized wholesale.

Parent initialization retains the original empty/non-array first-entry `[{}]` rule and replacement of only index zero for an existing non-object entry. One-time settings/connection/stream flags remain scoped to one plan. Protocol replacement precedes connection initialization; user defaults and all field phases retain original order. Port fallback/prefix behavior, clear/omitted semantics, alias casing/shape, dormant data, raw-TCP precedence, ignored H2/QUIC hints and security projection remain unchanged.

The private old string-initialization helpers are replaced by equivalent operation recording. No second parser, DTO or editor framework is introduced. A source comparison verifies all **37 functions outside the targeted patch path are byte-for-byte unchanged**.

## Validation

All **3,792 accepted golden cases** and **3,920 full frontend tests** pass. TypeScript, lint without new warnings, exported build, formatting and strict CodeScene delta against accepted main pass. The module's Code Health improves **2.36 → 3.58**; no new planner phase has a complexity finding. The old patch function's CC 133 is replaced by named phases below the complex-method threshold. Remaining parser/default-builder/legacy findings are reported separately and not claimed resolved.

Evidence is saved under `.cache/r10a`: corpus/full-suite JSON, direct source/delta reviews, build output and unrelated-function parity. Hosted checks and automated review of this production change must be addressed before merge. Staged/history secret checks run before publication. No gate or rule is suppressed.

## Next batch

R10b isolates the security phase behind a focused internal module using the accepted alias/field/parent/Unicode corpus. Keep one normalization implementation and avoid runtime import cycles. Later original scopes and dependencies remain in the backlog.

## Final hosted acceptance

[PR #16](https://github.com/romanpodg/SubShare-Go/pull/16) merged at **8952b75** after all six final-head checks passed at **d225402**: backend, frontend, Docker deployment smoke, CodeScene, Codecov patch and bundles. [Actions run 37766659388](https://github.com/romanpodg/SubShare-Go/actions/runs/37766659388) includes full frontend/backend, migrations, races, static/secret checks and restart validation. Automated review at the same head reported no major issues; no inline threads were unresolved. R10b begins separately from this accepted main.
