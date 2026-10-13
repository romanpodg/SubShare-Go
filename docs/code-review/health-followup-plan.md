# Code Health follow-up plan — 2026-10-13

Status: **analysis and proposal only**. No production or test source changes, commits, PRs, goal restart or heartbeat changes are part of this request. This proposal is separate from the accepted A00–A14 backlog.

## Evidence and confidence

Working-tree basis: `main` at `4f8a2b3d41800b4d5569b5a20961a9c5972dabe9`; clean before this document was added. The attached screenshots identify these three files.

| File | Score | Verification and role |
| --- | --- | --- |
| `frontend/src/lib/configuration.ts` | **3.58** | Refreshed successfully with the installed CodeScene CLI; production editor parsing/building/conversion/patching |
| `cmd/server/subscription_generation_test.go` | **2.69** | Screenshot score; current blob `8351bb02b4939cf866a645abc9f6db2b56c7e480` exactly matches audit baseline `3ab25a5`; integration regression suite |
| `cmd/server/external_profiles_test.go` | **1.66** | Screenshot score; current blob `28ea02de3ec7799242214cea194252d3aeacfd1c` exactly matches audit baseline `3ab25a5`; import/sync/persistence/privacy regression suite |

The Go CLI refreshes failed during license/authentication checks: one `ClassCastException` and one explicit401/reauthorization response. Their scores are attributed to the attachments; fresh Go smell categories are unavailable. Go structural measurements below come from local AST inspection, **not** an imitation of CodeScene's scoring or cyclomatic-complexity formula. Restore authorized CodeScene access before measuring implementation deltas.

The previous R10 work addressed token-preserving Xray patch planning. Its [accepted record](r10c-shared-boundaries.md) explicitly leaves parser/default-builder/legacy complexity visible, explaining the remaining3.58 score. R22 added direct package assurance while retaining the original Go integration suites; those suites are now separate maintainability targets.

## File analysis

### configuration.ts: concentrated production decision logic

The refreshed report counts **1,134 code lines** excluding comments and identifies eight complex methods. Principal findings:

| Function / current line | CodeScene finding | Maintenance concern |
| --- | --- | --- |
| `parseXrayJSONConfiguration:314` | CC67,147 code lines | Root validation, outbound selection, protocol credentials, transport projection, security inference and display fields share one mutable draft-building procedure |
| `buildDefaultXrayJSONConfig:486` | CC45,180 code lines, three bumps, nesting depth4 | Protocol/transport/security construction and the static document envelope are coupled |
| `createConfigurationFromXrayJSON:1163` | CC60 | VMess payload and URI query serialization contain repeated field/default/omission policy |
| `patchEditableConfiguration:1107` | CC47, two bumps | VMess object mutation and URI query mutation share a large branch; explicit set/clear and aliases add substantial decision load |
| `createXrayJSONFromConfiguration:978` | CC17,88 code lines | Link parsing, alias projection and JSON construction share one function |
| `parseQueryParams:204` | CC22 | A mostly declarative alias/default projection accumulates fallback decisions |

The file's public API is widely reused. Preserve exported functions/types through a stable facade while extracting responsibilities. Reduce complexity inside the extracted functions and measure all resulting modules; facade size alone does not establish completion.

### subscription_generation_test.go: multiple assurance domains in one suite

Local AST inspection: **1,006 source lines,23 top-level tests**. It mixes capability/version contracts, request adaptation, raw/base64 delivery, Xray projection, read-only deduplication, safe exclusions, target-specific mappings and template security.

Concrete hotspots include:

- `TestDeliveryDedupIsSemanticCurrentKeyAndPersistenceReadOnly:472`:85 physical lines, including an inline persistence snapshot reader; fixture setup, winner selection, read-only assertions and identity comparisons share one test.
- `TestSubscriptionCapabilityMatrixIsCompleteAndVersionPinned:171`:81 physical lines and16 `if` statements, including nested output/version traversal and an inline capability lookup.
- `TestStructuredDeliveryAllExcludedHandlerPolicy:642`:75 physical lines; table fixtures and repeated HTTP/body/header/privacy assertions share one procedure.
- `insertAssignedDeliveryKey:128`:nine named parameters and ignored keyring/identity/encryption setup errors. A typed fixture descriptor and checked setup would make failures easier to locate.

These are important regression contracts. Keep actual handler/SQLite/encryption integration, pinned-version/runtime-interoperability distinctions, ordering, exact delivered bytes and non-mutation/privacy checks.

### external_profiles_test.go: long state-transition narratives

Local AST inspection: **1,506 source lines,36 top-level tests**. Recognition/base64/Xray parsing, semantic identity, persistence, historical repair, synchronization, source lifecycle, concurrency/history and secrecy share the file.

Concrete hotspots include:

- `TestExternalProfileCrossSourceAndTUICV4Compatibility:1132`:90 physical lines,17 `if` statements and nine direct SQL calls.
- `TestExternalProfileSyncPreservesLocalStatusAndClientDisplayName:681`:83 physical lines,21 `if` statements and nine SQL calls; initial import, rename, connection change, override reset and delivery checks form one story.
- `TestExternalProfileSynchronizationIsIdempotentAndSourceScoped:940`:79 physical lines; initial/repeated/reordered/renamed/added/partial/complete and cross-source transitions are coupled.
- `TestExternalProfilePersistenceAndSynchronization:871`:68 physical lines with14 repeated fatal-guard assertions and multi-field persistence predicates.

The improvements should expose each transition and its expected state. Keep the intentional historical/corrupt fixtures, encrypted-row identity, assignment merges, local metadata, source scope, partial-refresh retention, key rotation and concurrent history accounting.

## Proposed batches, in execution order

Each batch should be independently reviewable. Keep expected results independent of the production algorithm under test.

| Batch | Scope | Concrete change | Acceptance |
| --- | --- | --- | --- |
| H00 | All three files | Refresh licensed Go reports; inventory every existing scenario, assertion and CI-selected test name; capture uncached targeted baseline | Actual baseline findings and scenario mapping recorded; no test deletion |
| H01 | Shared Go test fixtures | Move only genuinely shared synthetic profile/source constants and fixture helpers into focused `_test.go` files; use typed seed/snapshot structures; check keyring/ID/encryption/query errors | Same setup semantics, intentional invalid-row admission, NULL/precision and secret boundaries; failures point to the setup step |
| H02 | `external_profiles_test.go` | Group recognition, identity, persistence, repair, sync, lifecycle and privacy suites; extract typed row/secret/count/history assertions; separate independent cases and name sequential phases | Every original state transition and assertion retained; fresh fixture per independent case; retained concurrency actually executes under race |
| H03 | `subscription_generation_test.go` | Group capabilities, raw delivery, Xray projection, structured renderers, handler exclusions and templates; extract capability lookup/version assertions, persistence snapshots and safe-response checks | Same bytes/order/winner/exclusion/error/header/privacy outputs; actual route/DB assertions remain |
| H04 | TS default builder | Extract protocol connection builders, transport builders, security builders and static envelope assembly into pure modules | Exact default/conversion goldens; fresh nested objects per build; `none`/TLS/Reality, TCP/raw, VMess alter-ID and omission behavior unchanged |
| H05 | TS parser/projection | Separate root/outbound selection from protocol, transport, security and display readers; centralize compatible coercions/types; retain public facade/re-exports | First supported outbound, primary user/server, alias precedence, dormant-security inference, raw/tcp precedence, malformed-shape/error messages and permissive port behavior unchanged |
| H06 | TS URI conversion/legacy patching | Separate URI/VMess codecs, conversion bridges and per-format patch policy; use field specifications only where set/clear/alias rules are identical; retain special network/security policies explicitly | Existing represented-field-only changes, false/zero/empty/omitted distinctions, unknown/duplicate fields, encoding/query order and existing lossy conversion boundaries preserved |

H02/H03 file organization should accompany function simplification. Keep an evolving case-to-assertion inventory so increased helper reuse or new subtests cannot silently reduce assurance. Maintain current CI regex coverage, or update filters explicitly when names change.

The existing security-plan, patch-value and token-document modules remain the implementation seams. Move type imports into the types boundary where appropriate to keep dependencies directed. JSON patching must continue to use the current ordered token editor; original large numbers, escaped tokens, unknown siblings, extra outbounds/users and byte-idempotence remain protected. Preserve generated object/query ordering; the full-document projection and conversion contracts have different preservation requirements.

## Validation and completion criteria

- **Go batches:** existing top-level/subtest mapping plus all assertions/transitions retained; focused uncached suites; full backend suite, vet/lint and appropriate hosted checks. Compare equivalent coverage runs as a regression signal, not proof of assertion equivalence. Preserve source/assignment/envelope snapshots and rollback checks. Add the retained `TestExternalProfileConcurrentRetryDoesNotDuplicateAndHistoryCountsAreAccurate` to explicit race execution: the current job/source race selector does not include this name.
- **TS batches:** retain the existing R09 **3,792-case** preservation corpus and later UTF-8/shared/security supplements. Add only missing builder/parser contracts before extraction; verify literal expected values independently of production conversion helpers. Run affected/full Vitest, TypeScript, eslint, static build and key-editor browser smoke; shared Go fixtures cover their supported intersection with deliberate differences retained.
- **Health:** record before/after scores and actual named smells for every touched/new module. Aim for focused helpers/modules at10 and reduce the identified large/complex/nested functions. Choose the remaining aggregate target from refreshed reports; no numerical score guarantee is made. Track complexity moved into helpers, module dependencies and diagnostic clarity alongside file scores.
- **Behavior:** no schema/API/runtime/editor protocol expansion, fixture access to operational data, dropped integration coverage, weaker goldens or health-rule suppression. Any separately reproduced correctness change should have its own failing regression and explicit review scope.

Recommended start: **H00/H01, then the external-profile suite (H02)**, followed by delivery tests (H03). Production TS changes follow in small H04–H06 batches once the behavior baseline is stable. This sequence improves test readability before touching the highest-risk production interpretation rules.

Local analysis artifacts: `.cache/health-configuration.json` and `.cache/health-go-shape.go`. Failed Go refresh output is not a successful baseline report. No new baseline test run or implementation validation is claimed by this analysis-only pass.
