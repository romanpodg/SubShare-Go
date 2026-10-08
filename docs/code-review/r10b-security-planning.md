# R10b — Focused security planning module

Status: implemented and locally validated; publication and hosted acceptance remain. R10a is accepted in PR #16 at `8952b75`, with all six checks passing at `d225402` and automated review reporting no major issues.

Extract the security planning phase from `configuration.ts` into a focused internal module. Keep ordered path/value operations and the existing token application boundary. Pass original TLS/Reality projection and selected mode explicitly; preserve alias shapes, omitted/clear/false handling, missing/non-object parents, coupled mode/field behavior and none projections. Type-only imports may reuse existing draft/patch types; avoid a runtime cycle or new public configuration API.

Share normalization helpers through one internal implementation when needed; do not duplicate `optionalText` or ALPN splitting. Keep parent initialization semantics and lazy field normalization unchanged. The new module must meet the 10.00 new-file CodeScene gate without suppressions. All R09 goldens, the full frontend suite, types/lint/exported build and strict delta remain required. Publish/review/resolve/merge as its own batch, update the backlog, then continue the original remaining scopes.

## Implementation and validation

`configuration-security-plan.ts` records the TLS and Reality operations in their original order. Its explicit input carries selected mode, represented fields, original alias projection, destination path and operation callbacks. The caller retains stream/branch initialization and token application. None mode, lazy normalization, absent/clear/false behavior and modern-only/legacy-only/both/neither Reality key shapes remain protected by the accepted corpus.

`configuration-patch-values.ts` provides the single normalization implementation used by core and security planning. All three moved helper bodies are identical to the accepted source. Both internal modules import configuration types only, so their imports create no runtime cycle. Source comparison verifies 60 other functions are byte-identical.

All 3,792 golden cases and 3,920 full frontend tests pass, together with TypeScript, lint and exported build. Both new source modules score 10.00. Core Code Health remains 3.58 and executable lines decrease from 1,189 to 1,134. Strict delta exits 1 solely because removing simpler functions increases the remaining file's mean cyclomatic complexity from 7.17 to 7.45. No method became more complex; no gate, rule or threshold was changed. Hosted CodeScene acceptance remains required. Local evidence is retained in ignored `.cache/r10b/`.

Automatic approval review initially prevented the full validation command from executing because its account allowance was exhausted. After the reset and a positive ordinary-usage check, the same approval path executed successfully. No rejected command was rerouted.

## Remaining acceptance

Publish this batch, request automated review, fix actionable findings and require all six hosted checks before merging. Reconcile A03's near-size-boundary and shared Go fixture requirements before declaring the audit item complete; the existing corpus is not evidence for cases it does not contain. Then proceed to R11a and the remaining backlog.
