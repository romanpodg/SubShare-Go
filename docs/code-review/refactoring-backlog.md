# Refactoring Backlog

Basis: `main` at `3ab25a5d7aa4f2a28069330406d74f697ac1f1c3`, 2026-10-06. The original audit backlog is retained below; current progress is summarized below, with detailed execution records at the end. Ranking combines source semantics, CodeScene categories/functions, Git activity, Codecov lines, local statement profiles, tests, architectural role, and regression blast radius. Evidence tables and full per-file findings are in [repository-audit.md](repository-audit.md) and [analysis-evidence.md](analysis-evidence.md).

Coverage labeled **CC** is Codecov line coverage for this SHA. **Local** is CI-style Go statement coverage; **cross** is supplemental uncached cross-package statement coverage. Frontend percentages are unavailable, not zero. Git entries are lifetime/recent touches, then recent added+deleted lines; recent starts 2026-04-06. File splits can hide historical activity. Completion requires behavior parity, not a specific score increase.

## Current implementation status — 2026-10-06

**2026-10-08 continuation:** R06 is merged in PR #10; R07a, R07b, R08 and the separately reviewed R08c correction are merged in PRs #11-#14 after all six hosted checks and final automated reviews passed. A02 is accepted. R09 merged through PR #15 at 01f92aa with all six checks and all 27 review findings resolved. R10a merged in PR #16 at 8952b75 after all six checks and clear automated review. R10b merged in PR #17 at 1809f80 after all six final-head checks and clear review. R10c merged in PR #18 at 087e873 after all six checks and clear review. A03 is accepted. R11a/R11b and their corrections are accepted in PR19-PR22; A07 is complete at merge d65519b after all six final-head checks and clear review. R12 pure response policy is in local validation. See [R09](r09-configuration-preservation.md) and [autonomous continuation](autonomous-continuation.md). **Historical 2026-10-07 update:** the checked-out history includes R05a/R05b merged at **dc91072**, R06 characterization at **6a12236**, and the local R07a category/order extraction. See the [R07a execution record](r07a-category-ordering.md) for scope, validation and strict CodeScene warnings. R07b is next. The historical audit measurements and earlier CI evidence below are retained.

The R01 prerequisite was merged through [PR #7](https://github.com/romanpodg/SubShare-Go/pull/7) at **267811b**. R02 is complete in [PR #8](https://github.com/romanpodg/SubShare-Go/pull/8), with source validated at **a896708** and all six checks passing; see the [R02 execution record](r02-startup-safety.md) for the recovery contract, evidence and limits. Baseline measurements below still describe main at 3ab25a5; the R01 results below are historical evidence for a438377.

Completed work:

- **R01 complete:** portable startup/WAL characterization, WAL/DELETE restart, account/user/encrypted-profile/keyring retention, migration failure and retry, sidecar failures and I/O classification.
- **R02 complete:** fail closed on initialization errors; remove destructive cleanup/retry and its obsolete error classifier; separate SQLite journal negotiation from connection pragmas. Crash-WAL, injected failure and second-process lock regressions protect accounts, encrypted profiles and keyring state. Windows checks and Linux/deployment CI pass.
- **R03 implemented in the same PR, per user request:** registered user creation/deletion/activation routes, failure/rollback contracts, subscription PUT/PATCH compatibility and deterministic contention fixtures. Separate corrections prevent false create success after five collisions and the reproduced PATCH lost update. See the [R03 record](r03-user-mutations.md) and [PATCH correction](subscription-patch-concurrency.md) for evidence and bounded conflict handling.
- **R04 implemented in PR #8:** transaction-owned create and atomic delete commands, pure input validation and HTTP error mapping, with R03 parity tests and direct command assurance. Unknown deletion affected-row outcomes now return 500 instead of a false 404. See the [R04 record](r04-user-commands.md); R05 remains separate.
- **PR #8 merged at d87b122; R05a implemented separately:** HTTP-independent activation redemption and pure input validation delegate to the existing conditional SQLite claim. Synchronized alias requests prove one winner and retained identity/timestamp; confirmation failures withhold the link, while existing access rules and crypto fallbacks remain compatible. See the [R05a record](r05a-activation-redemption.md); all six hosted checks passed at ddf7154.
- **R05b implemented as the user-requested PR #9 continuation:** registration transaction ownership, pure normalization/capacity/access policy, SQL commands and request adaptation are separate from device projections. Synchronized final-slot/same-identity transactions and rollback/route fixtures preserve existing contracts. See the [R05b record](r05b-device-registration.md); the activation/device work is included in merge dc91072.
- **R06 characterization committed at 6a12236:** encrypted profile mutation/read, service patch/ownership, category/order atomicity and HTTP contracts. The uncached storage/service/HTTP baseline passes before R07a extraction.
- **R07a merged in PR #11 at 1a7968b:** category/order methods and named phases retain SQL, transactions, DTOs and the active adapter. All six hosted checks pass at 33631dd and final automated review is clear. See the [execution record](r07a-category-ordering.md); R07b reads are implemented and R08 remains.
- **Existing repository refactoring retained:** e3ceaba separated user projection, settings, activation, subscription access and device persistence, factored transaction helpers and propagated RowsAffected errors. Repository Code Health is 10.00; helpers improved from 9.31 to 9.61. This is partial A01 progress, not completion of its mutation/fault/concurrency plan.
- **Repository test maintainability fixed:** a438377 reorganized regression cases while retaining every assertion and state transition. Local Code Health improved from 4.05 to 10.00; the remote gate passes.
- **Audit-prose scan false positive resolved:** 235408c documents one exact historical fingerprint exception; the scan and rules remain enabled.

| Audit item | Status | Remaining work |
| --- | --- | --- |
| A00 / P0 | R01/R02 complete; PR #8 merged | No production I/O trigger frequency is claimed |
| A01 / P1 | R03/R04 merged; R05a/R05b included in dc91072 merge | Other A01 mutation boundaries remain |
| A02 / P1 | Completed; R06/R07a/R07b/R08/R08c merged with hosted checks/review clear | Retain encrypted command, projection, ownership and rollback contracts |
| A03 / P1 | Completed; R09/R10a/R10b/R10c merged | Retain preservation/shared/UTF-8 contracts; remaining parser complexity stays visible |
| A04 / P1 | R12 accepted in PR23 at d5d9616; R13a persistence extraction locally validated | R13a hosted acceptance and R13b HTTP/delivery acceptance |
| A05 / P1 | Not started | R14/R15a/R15b queue/source lifecycle, overlap and source-create transaction |
| A06 / P1 | Not started | R16a/R16b actual-route authorization matrix and guard evaluation |
| A07 / P1 | Completed; R11a/R11b and corrections merged in PR19-PR22 | Final six checks/review clear at b6fb366; retain [requirement evidence](r11b-reveal-provenance.md) |
| A08 / P1 | Not started | R17a/R17b ordering/actions and shared presentation |
| A09 / P2 | Not started | R18a/R18b Go/TS/OpenAPI contracts and request errors |
| A10 / P2 | Not started | R19 backend Xray interpretation |
| A11 / P2 | Not started | R20 supported-consumer check and unused declarations |
| A12 / P2 | Not started | R21 branding save acknowledgment and reopen state |
| A13 / P2 | Not started | R22a/R22b package measurement and backup/upgrade/restore failures |
| A14 / P2 | Not started | R23 public-page renderer/config contracts |

All six checks passed for a438377: backend, frontend, Docker smoke, CodeScene, Codecov patch and Codecov bundles. [Actions run 37429826449](https://github.com/romanpodg/SubShare-Go/actions/runs/37429826449) includes Go/migration/vet/lint/build/secret scanning, TypeScript/lint/Vitest/build/Playwright, restart persistence and configured uploads. [CodeScene analysis 7826153](https://codescene.io/projects/84883/delta/results/7826153) passed all three gates; both local deltas pass.

Codecov: project **48.02%** versus main **46.89%**; main.go **18.46%** versus **8.01%**; sqlite.go **64.70%** versus **60.78%**; whole-PR patch **80.09%**. Test Analytics: **848 passed, 5 skipped, no failures/errors**. Bundle upload/check passed; size/delta figures were unavailable in inspected results. Local statements remain **52.4%**, with unchanged coverage across 140 repository blocks. Optional snapshot/client fixtures and local race execution remain unverified. Passing frontend CI does not complete its planned refactoring.

**Current review boundary:** R01-R11b and separate correctness fixes are merged through PR22. A00/A02/A03/A07 are accepted. R12 response policy is in local validation; broader A01 and A04-A06/A08-A14 remain incomplete. Historical audit measurements remain distinct from current validation.

## P0

### A00 — Preserve committed SQLite WAL data on failed startup

- **Module/file:** `cmd/server/main.go:86–117`, `internal/storage/sqlite.go:87–103`; associated startup/storage tests.
- **Problem:** automatic recovery deletes `-wal` and `-shm` after a broadly classified I/O error, without demonstrating a successful checkpoint or retaining recoverable data.
- **Evidence:** `openDatabase` calls the deletion helper before retrying. `IsRecoverableSQLiteIO` accepts generic `disk i/o error` or `(4874)`. A disposable subprocess committed a row without checkpointing; preserved-sidecar copy read 1 row, copy passed through the actual helper read 0. The test `TestCleanupSQLiteSidecars` only verifies removal of arbitrary files. Production trigger frequency was not reproduced.
- **CodeScene findings:** main 9.57, SQLite 9.92; only CLI/pragmas complexity is reported. This is a semantic correctness risk independent of Code Health.
- **Git evidence:** main 27/20 touches, 1,311 recent churn; SQLite 3/3, 115. Startup helpers introduced/extracted by `c2be9f5`, later updated by `25cba98` and `c32a805`.
- **Coverage evidence:** CC main 8.01%, SQLite 60.78%; `openDatabase` 0% local. CC misses include main 101–103, 106–108 and 110. Helper coverage cannot establish recovery safety.
- **Regression risk:** critical; losing committed user/profile/migration state after restart. A metric-only refactor would miss the hazard.
- **Required tests:** subprocess crash leaving a committed WAL; recoverable and nonrecoverable startup failures with WAL unchanged; simulated failed checkpoint/close; I/O or permission failure removing SHM; second-process lock; normal WAL/DELETE startup; migration failure and retry; existing database/keyring retained; bootstrap does not overwrite accounts. Use disposable DB files and an injected initializer/filesystem seam, never runtime data. Explicitly distinguish current destructive characterization from a future data-preservation regression assertion.
- **Proposed direction:** first commit tests/characterization alone (R01). Then a **separate corrective behavior change**, R02: fail closed or use SQLite-supported recovery with preserved DB/WAL and an explicit operator recovery path. Do not treat a valid WAL as cache. No architecture rewrite or schema migration is required.
- **Dependencies:** R02 requires R01 and a reviewed recovery contract. This audit does not authorize implementation.
- **Behavior to protect:** normal startup/migration order, usable database state, encryption keyring, owner/bootstrap behavior and diagnostic errors. Destructive loss is not a compatibility requirement.
- **Completion criteria:** failing startup cannot remove committed records; crash/failure tests pass on Linux and Windows; no source/secret bytes in errors; Go/storage/migration checks and populated restart validation pass. An unchanged CodeScene score is acceptable.

## P1

### A01 — Stabilize user, activation, subscription and device persistence boundaries

- **Module/file:** `cmd/server/handlers_users.go`, `repository.go`, `subscription_admin_v1.go`; read/policy/delivery callers remain explicit.
- **Problem:** user HTTP validation and writes, status/date rules, global setting projection, activation CAS, assignment and HWID registration are coupled to SQL and mixed in a broad `App` boundary. Legacy PUT and v1 PATCH have distinct validation/timezone/optional-field handling that could drift.
- **Evidence (original audit ranges):** create 96–173 and subscription PUT 226–334 in handlers; access 449–487, redemption 489–535, HWID 537–652 in repository; PATCH read current fields then wrote the whole set at 117–216 without revision comparison. R03 reproduced disjoint PATCH loss deterministically. The subsequent correction conditionally compares/writes the snapshot and reapplies stale patches, replacing the defect assertion with preservation regressions. R03 also fixes the characterized false create success after retry exhaustion.
- **CodeScene findings:** 8.00/6.33/7.67; repository Low Cohesion; `getSubscriptionSettings` CC 15, `registerHWID` CC 14/105 LoC, `scanUserRow` CC 13/99 LoC; PUT CC 18/98 LoC; repeated Bumpy Road.
- **Git evidence:** handlers 1/1, 427; repository 24/16, 1,609; v1 subscription 5/5, 518. Handlers inherit behavior from `handlers.go` with 34 lifetime touches; their new path count is a lower-bound lineage signal.
- **Coverage evidence:** CC 2.14%, 57.31%, 39.51%. Local create, legacy subscription PUT and activation redemption are 0%; HWID 59%, PATCH 50%. These are same-package gaps, not cross-package artifacts.
- **Regression risk:** high: access dates/status, bearer identity, activation single use, assignments, device quotas and deletion of user data.
- **Existing tests:** `subscription_core_test.go` covers state/device policy, all/selected assignment, omitted/null PATCH and timezone handling; characterization status/envelopes; helper/model tests. These do not cover normal user creation/redemption routes.
- **Required tests:** create with default/custom/duplicate activation code, malformed/oversized input, duplicate subscription ID retry, zero/negative/max durations, assignment insert failure and transaction rollback; delete missing/existing user with device/key cascades; legacy PUT versus PATCH fixtures including omitted/null/empty fields, invalid URL/timezone, DST and equal start/expiry; unknown/paused/blocked/future/expired status; duplicate/concurrent activation exactly one winner; existing device at capacity, normalized duplicate HWID, unlimited/zero/negative limit, last-slot concurrent requests, DB/commit failure. Characterize concurrent disjoint PATCH behavior and separately decide whether to fix it.
- **Proposed direction:** R03 tests, R04 one user-create/delete command seam, R05 activation/device policy and persistence extraction. Keep read projections/settings apart; do not build a generic user repository with every `App` operation. Preserve current endpoint-specific timezone/clearing behavior until separately approved changes.
- **Dependencies:** own tests first; no dependency on editor refactoring. Shared fixtures support A09 later.
- **Completion criteria:** exact response/status/assignment/access outputs preserved; invalid/failure cases have deterministic tests; no SQL in the extracted command's HTTP adapter; atomic CAS/device transaction boundaries retained; targeted CodeScene large-method/low-cohesion findings reduced without hiding logic.

### A02 — Separate credential-safe profile persistence and patch policy

- **Module/file:** `internal/storage/profile_repository.go`, `internal/keymanagement/update.go`, active repository seam, HTTP profile callers.
- **Problem:** one adapter handles modern encrypted mutations, legacy CRUD, categories, ordering and projection; update policy mixes ownership, revisions, raw/structured mode and tri-state protocol patching. Independent responsibilities are hard to modify safely.
- **Evidence:** 1,084-line adapter contains `loadKeyByID`, Create/Update/Clone, legacy operations and category/reorder methods. Source-owned metadata guard at update 302–308 rejects many source-controlled changes. Adapter `CreateLocal:215–218` ignores assignment insert errors, unlike CloneLocal: partial-success possibility if SQL fails. Metadata-only updates intentionally preserve the encrypted secret and blind index.
- **CodeScene findings:** adapter 5.67 (Low Cohesion, 13 complex methods); update 6.64 (`UpdateLocal` CC 16, patch CC 17, source metadata CC 18, 10-expression ownership conditional).
- **Git evidence:** adapter 10/10, 1,767 churn; update 7/7, 754. New interface itself has one commit, which says little about mutation stability.
- **Coverage evidence:** CC 64.07%/35.51%; local 75.5%/36.0%; cross 76.5%/52.3%. Key-service tests cover ownership better than SS/Hy2/TUIC tri-state patches; adapter read/normal cases do not prove rollback.
- **Regression risk:** high: credential changes, profile identity/revision conflicts, source ownership, user assignment and encrypted storage integrity.
- **Existing tests:** storage `Create_Update_Clone`, key separation, `ClientDisplayNameMetadataDoesNotRewriteSecret`, `SourceLocalMetadataDoesNotRewriteSecret`, error classification; service source ownership and raw Xray preservation; HTTP FullSuite and reveal authorization.
- **Required tests:** table per protocol/tri-state field for absent/set/clear, clearing required secrets, plugin clear and query removal, false versus omitted boolean, malformed stored URI and unknown params; two requests with same revision (one conflict); source-controlled field mutations rejected; local/source name/status-only updates retain ciphertext/blind index and increase revision exactly once; DB failure after parent/secret/user-assignment write rolls back the intended unit; duplicate blind index; clone identity/user all-mode; post-commit reload failure/retry classification. Reproduce ignored assignment failure with a disposable trigger before choosing a correction.
- **Proposed direction:** R06 tests, R07 extract category/read responsibility inside the current SQLite adapter, R08 named patch/ownership policy phases. Preserve one credential store and one transaction per command. Essential assignment-error handling, if corrected, belongs in a separately reviewed corrective change, not silent refactoring.
- **Dependencies:** R07/R08 require R06; A07 UI command extraction reuses this contract, not a new backend DTO.
- **Completion criteria:** no raw secret in summaries/logs; ownership/revision and tri-state fixtures unchanged; rollback tests pass; low cohesion and update conditional complexity improve through meaningful boundaries; no duplicate persistence manager.

### A03 — Decompose the frontend configuration document/URI editor

- **Module/file:** `frontend/src/lib/configuration.ts`, using `xray-json-document.ts` and existing tests.
- **Problem:** one module mixes parsing, wire-format conversion, defaults, Xray outbound selection, patch planning and token-preserving document mutation. This is the worst Code Health concentration, with a real combinatorial compatibility burden.
- **Evidence:** `patchXrayJSONConfiguration:742–1152` maintains mutable `nextRaw`, closure flags and protocol/transport/security branches; conversion functions additionally duplicate related mappings. Unknown fields, extra outbounds/users, dormant branches and query order are intentional preservation behavior.
- **CodeScene findings:** 2.36; patch CC 133, 394 executable LoC, 10 bumps/depth 4; parse CC 67; reverse conversion CC 60; default builder CC 45.
- **Git evidence:** 4/4 touches, 1,830 churn; low count is offset by large changes and central editor responsibility.
- **Coverage evidence:** no frontend coverage uploaded or local instrumentation configured. `configuration.test.ts` protects many crucial cases; 128 suite-wide tests do not establish this module's percentage.
- **Regression risk:** high: silently altering VPN connectivity, credentials, duplicate/unknown configuration or raw JSON bytes.
- **Existing tests:** duplicate-key formatting guard, first-node-only edits, unknown fields/extra users, explicit field clear, transport/security independence, dormant branches, protocol conversions, VLESS duplicate query and VMess extra payload preservation.
- **Required tests:** protocol × network × security matrix; absent/null/false/numeric/alias forms; repeated duplicate/escaped query fields; extra Trojan server/VMess user/outbound entries; represented-field-only patches, repeated no-op patch, near 65,535-byte boundary and Unicode; unsupported/malformed/outbound-not-found input; raw token preservation and harmless formatting normalization; fixture agreement with Go for shared supported inputs, without assuming UI conversion equals backend validation.
- **Proposed direction:** R09 fixtures, R10 separate pure patch planning from document edits and extract one coherent transport/security phase at a time. Keep duplicate-aware JSON token editing. Avoid reserialize-whole-object conversion or automatic protocol normalization changes.
- **Dependencies:** R10 requires R09; A07 follows this stabilized command/config seam.
- **Completion criteria:** corpus/property invariants unchanged; CC/bumps/depth of patch method visibly reduced; no new parser, generated semantics, or format behavior introduced; Vitest/type/lint/export pass.

### A04 — Separate response-rule policy from CRUD and public delivery

- **Module/file:** `cmd/server/subscription_rules.go`, `handlers_subscription.go`; selected `subscription_delivery.go`/generation callers and delivery package tests.
- **Problem:** pure rule/template validation/matching, persistence CRUD, invalid-row repair, preview/rendering, headers and metrics share one file and request flow.
- **Evidence:** rules 128–269 contain policy, 271–350 SQL loading/self-disabling invalid persisted JSON, 521–578 save/lookup/write logic. A read can disable a corrupt rule and still return an error; preserve this unusual contract until explicitly changed. Delivery has separate browser/direct/subbody behavior and canonical metadata protection.
- **CodeScene findings:** 6.50/7.56; rules Low Cohesion; save/validation CC 15, condition matching CC 13, repeated Bumpy Road.
- **Git evidence:** rules 10/10, 1,463 churn; handler 1/1, 461 after split. Public delivery is central regardless of current filename churn.
- **Coverage evidence:** CC 40.40%/42.39%; save local 28.2%. Pure rule tests are stronger than CRUD/persisted corruption/error flows. Extracted delivery's CC zero reports are instrumentation blind spots, not absence of server tests.
- **Regression risk:** high: wrong clients/formats, accidental unblock/fallback, header injection, expired/device-limited subscriptions, unsafe public rendering.
- **Existing tests:** matching/unsafe headers/template format mismatch/preview, `subscription_core_test` denial precedence and missing keys, large generation suite for format exclusions and metadata, public escaping test.
- **Required tests:** actual create/update/delete/template preview endpoints; priority/tie and AND/OR/empty conditions, regex compilation/invalid regex, case/header absence; corrupt conditions/headers JSON with disable+audit+error and repeated requests; DB reads/writes/commit failure; missing/disabled/deleted/mismatched templates; block/not-found/browser precedence across all delivery URLs; metadata cannot be overwritten by stored custom headers; all-excluded 422 versus empty/integrity 503; HTML/CSS/JSON/URL escaping; network failure for optional Happ crypto with existing fallback behavior.
- **Proposed direction:** R12 pure matcher/validation extraction with tests, R13 rule/template persistence and HTTP adapters. Keep policy order and fail-closed output. Put format-only fixture cases near delivery while retaining end-to-end handler tests.
- **Dependencies:** focused contract tests before each extraction; no dependency on front-end reorganization.
- **Completion criteria:** recorded response body/status/headers and rule side effects preserved; SQL and pure matching separated; low cohesion/complex functions improve; Go and format validation pass.

### A05 — Make queued job/source orchestration explicit and testable

- **Module/file:** `cmd/server/jobs.go`, `sources_v1.go`, `external_subscriptions.go`, with existing `sources.Sync`/storage seam.
- **Problem:** persistent job/run/source state, detached goroutines, fetches, reconciliation and HTTP retry semantics are distributed; essential state writes sometimes fail silently. Source handlers leak category/transaction responsibilities.
- **Evidence:** jobs 191–213 startup recovery ignores errors; queue/transition 235–267, workers 469–522, retry 524–572. `syncExternalSource:248–274` locks reconciliation, but fetched data was read earlier from a source URL. Editing that URL or overlapping fetches can leave a stale result applied to current source state; no scenario was reproduced. Source create 295–412 uses an explicit transaction while update creates categories before its later transaction.
- **CodeScene findings:** jobs healthy 9.38 (retry CC 12); sources 7.79 with create large/complex method. Semantic lifecycle risk is greater than the job score suggests; sync 9.53 is already decomposed.
- **Git evidence:** jobs 11/11, 1,024 churn; sources 11/11, 968; external adapters 14/14, 6,073 including major extraction.
- **Coverage evidence:** CC jobs 32.46%, sources 57.18%; queued workers, queue persistence and retry 0% local. `sources.Sync` CC 0% but cross statements 84.5% distinguishes exercised reconciliation from unexercised worker lifecycle.
- **Regression risk:** high: duplicated/out-of-order destructive reconciliation, wrong run state, retry reporting, source deletion/assignments, shutdown consistency.
- **Existing tests:** worker aggregation/persistence warnings/concurrency cap; atomic source create/duplicate, masked details/HWID update, source RBAC/delete and terminal polling; external source fingerprint/duplicate/encryption integration cases.
- **Required tests:** actual queue 202/job ID and queue DB failure; queued→running→terminal success/error; retry allowed only for failed/known valid target, deleted target, repeated retries; fetch size/status/timeout/cancellation failure; start/run/status/audit DB failure classification; blocking fake fetches released in reverse order, URL/category/enable update during fetch, delete during fetch; assignments and secrets rolled back together; restart recovery idempotence. Record current same-source overlap semantics before considering serialization/CAS behavior changes. Shutdown test must distinguish tracked work completion from interrupted-state recovery.
- **Proposed direction:** R14 lifecycle/race/fault characterization; R15 queue/retry/transition seam followed by separately reviewable source-create transaction boundary. Retain existing source reconciliation mutex and DB constraints. No distributed queue or scheduler replacement.
- **Dependencies:** R15 requires R14; source contract fixtures can reuse A09 later but are not blocked by it.
- **Completion criteria:** terminal/error/count behavior and source transactions match fixtures; bounded concurrency/network behavior preserved; required versus best-effort state writes documented; any changed race/retry semantics separately identified; worker bodies have focused test evidence.

### A06 — Consolidate authorization evaluation after real-route coverage

- **Module/file:** `cmd/server/auth.go`, route registrations in `main.go`/`keys.go`; auth/token/route tests.
- **Problem:** admin/owner wrappers duplicate principal resolution, token scope and CSRF checks; handlers often repeat principal queries; string-based endpoint scope policy needs explicit coverage when adding routes.
- **Evidence:** wrappers 20–70 share the same scope/CSRF logic; `apiTokenAllows:173–214` is deny-by-default but tied to path substrings. The matrix test directly invokes wrappers with a supplied `ownerOnly` flag, so cannot catch all registration mistakes.
- **CodeScene findings:** 8.24, Code Duplication, guard CC 9, scope CC 10; common HTTP helper argument count is not itself debt.
- **Git evidence:** 12/8 touches, 245 churn; `5584ad09` fixed hashed sessions and token write restrictions. Frequent co-change with old handlers/main increases blast radius.
- **Coverage evidence:** CC 72.66%, local/cross 76.1%; sampled role tests do not establish every route's correctness.
- **Regression risk:** high security boundary; no bypass established by this audit.
- **Existing tests:** authorization matrix, API token tests and unsafe/deny-default cases, real reveal routes, auth/legacy envelopes, admin password upgrade tests.
- **Required tests:** table generated from registered actual endpoints across anonymous/viewer/operator/owner, legacy role aliases, session/API token scopes; valid/missing/invalid CSRF and owner-only reads/writes; all aliases; expired/revoked/deleted-admin/invalid scopes/DB errors; cookie-plus-bearer precedence; unknown write route denial; token mint/revoke requires interactive session; no raw cookie stored or logged. Test principal downgrade/removal between guarded handler steps if moving principal data into request context.
- **Proposed direction:** R16 first expands route tests, then shares policy evaluation/CSRF enforcement while preserving route declarations and error shapes. Avoid replacing explicit routes with a generic permissions dispatcher.
- **Dependencies:** authorization characterization is mandatory before production extraction; can proceed independently of editor/backend persistence work.
- **Completion criteria:** actual-route matrix passes, duplicated guard logic decreases, no wider token scope or owner privilege change, constant-time comparison and hashed-session behavior retained.

### A07 — Extract editor command construction and draft/secret lifecycle

- **Module/file:** `frontend/src/components/admin/KeyEditorModal.tsx` and focused tests; use existing protocol field components.
- **Problem:** a large modal controls many independent async and draft states, secret reveal/reset, source ownership, raw/structured divergence and protocol-specific submission payloads.
- **Evidence:** states/load/reveal 110–488; `handleSubmit:489–693` branches over modes/protocols/ownership. Load/reveal promises are not cancelled or guarded by a session generation; late responses after close/reopen are a risk, not a demonstrated leak.
- **CodeScene findings:** 6.20, submit CC 117, 192 LoC and 6 bumps. This is command/state complexity, not merely JSX length.
- **Git evidence:** 5/5 touches and 3,694 churn: unusually large changes per modification.
- **Coverage evidence:** unavailable frontend percentage; 16 focused test cases protect important modes/metadata, but not all asynchronous failure paths.
- **Regression risk:** high: replacing/clearing credentials, stale revision payloads, revealed-secret lifetime and source configuration ownership.
- **Existing tests:** six protocols, secret/raw preservation, duplicate JSON, conflicting drafts, name/template metadata, source-owned restrictions/status and clone behavior; browser responsive editor smoke.
- **Required tests:** reveal failure/409/revision refresh, submit validation and server field errors, clone/network failure; switching key/mode while load/reveal pending; close/reopen and late result never populating wrong session; source metadata sends no credential patch; absent versus clear password/plugin/TUIC fields; successful refresh failure after saved mutation; dirty confirm/cancel; double submit/repeated reveal. Assert contracts and observable state, not incidental hook call counts.
- **Proposed direction:** R11 extract pure create/update command builders, then a small draft/session hook with explicit secret state transitions. Continue using current endpoints; no new state library.
- **Dependencies:** stabilized A03 fixtures/seam (R09/R10) and A02 patch contract; add modal cases before moving state.
- **Completion criteria:** exact payload and raw-byte fixtures pass; close/reopen protections established; submit CC/bumps decrease; no unintended secret disclosure or metadata rewrite; Vitest/type/lint/E2E/export pass.

### A08 — Separate key-list ordering/actions from card and row presentation

- **Module/file:** `frontend/src/components/admin/KeysSection.tsx`, current unit/health/browser tests.
- **Problem:** selection/filtering, drag/autoscroll listeners, local order, bulk deletion/categories, health actions and duplicate card/row representation share one component.
- **Evidence:** drag listeners/state around 243–450; order/actions 535–697; cards 742–904, rows 906–1030 and category blocks 1060–1190. Failure/refetch can affect pending local order; not all of this should be forced into one hook.
- **CodeScene findings:** 6.71, card CC 15/160 LoC, row CC 12/122 LoC, category 127 LoC; drag target CC 12; some extra drag arguments represent geometry and are acceptable.
- **Git evidence:** 13/8 touches, 2,650 churn and three heuristic bug-fix-subject touches.
- **Coverage evidence:** no frontend percentage. Pure ordering/display tests and one health-launch component test are complemented by browser geometry/autoscroll coverage.
- **Regression risk:** high when changing order/selection/destructive requests across views and filters.
- **Existing tests:** align/order/client-label helper tests, asynchronous health launch, responsive key-list controls and drag autoscroll E2E.
- **Required tests:** full ID order submitted regardless of filtered view, unsaved order/refetch/failed save, selection after deleting or filtering, category moves/uncategorized/new key placement, card/row parity; bulk delete cancelled/failed/partially reported response, repeat request guard; health errors; drag end/unmount listener cleanup, reduced motion and near-edge scrolling. Keep actual layout/drag checks in Playwright.
- **Proposed direction:** R17 add focused action/order cases, extract an order/selection controller and action helpers, then reuse tested row/card subcomponents. Do not combine business actions with visual restyling.
- **Dependencies:** own prerequisite tests; independent of A07 except the existing editor open/refresh callback contract.
- **Completion criteria:** visible order/payload/selection/confirmation semantics unchanged; browser drag checks pass; renderer length/branching drops without duplicating action logic.

## P2

### A09 — Protect frontend/backend/API contracts and consolidate request errors

Go DTOs, OpenAPI, TS types and handwritten calls can drift (`model.go` health 8.54, `api.ts` 10, TS types unscored). Git: model 16/11 touches, 1,097 recent churn; API 12/9, 615; types 14/10, 665. Tests currently check key route/OpenAPI surface and mocked payloads; whole API compatibility is incomplete. R18 adds shared safe JSON fixtures for user/source/settings/detail/errors, verifies legacy/v1 aliases and omitted/null fields, then routes branding/selected direct calls through the existing request boundary. Preserve public GET loading, credentials, CSRF and 401 handling. Do not mandate code generation until fixture evidence shows an appropriate seam. Completion: contract drift fails a focused test; no endpoint or envelope changes.

### A10 — Clarify backend Xray JSON interpretation and traversal

`internal/profileconfig/xray.go` scores 5.48 (Bumpy Road, complex traversal/build methods); 3/3 touches, 1,271 churn, CC 43.78%, local 58.0%, cross 82.1%. Distinct target selection/build/projection and dynamic-map traversal are local design issues mixed with real protocol complexity. R19 extracts one outbound interpretation helper after golden fixtures for first usable outbound, missing/null/wrong types, default ports, generic versus display tags, extra users/outbounds, unsafe TLS and unsupported transport combinations. Coordinate A03/A09 fixtures but do not force UI loss-preserving editing through backend JSON reserialization. Preserve existing acceptance/default/exclusion behavior; evaluate stricter port validation separately.

### A11 — Remove unused persistence interface/type packages

`internal/keypersistence/{repository,params,errors}.go` and `internal/profilepersistence/{repository,params,errors}.go` have no tracked Go imports; the active seam is `keymanagement.Repository`. Mostly null CodeScene results; declarations are not a quality failure. Their sole/no recent consumers and duplicate parameter/error concepts justify small cleanup, not new abstractions. R20 verifies `go list`/imports and external compatibility policy, then removes only dead definitions. Prerequisite: compile/test boundary checks; preserve the active adapter and legacy routes. Completion: no missing internal/external supported consumers and no runtime differences. Do not split the broad active interface speculatively.

### A12 — Correct branding persistence acknowledgment and modal reopening

`PanelSettingsContext.tsx` 9.84; modal 8.70. Context 4/3 touches, only 23 recent churn; modal 3/2, 33. Source-established impossible reopen condition at modal 69 and ignored `saveToAPI` success/failure in setters deserve localized attention, not a new frontend state framework. R21 first characterizes mounted reopen/external settings updates, cold-cache/default backend, failed/denied PUT and slow GET race; then separately fixes acknowledgment/resync behavior before any extraction. Corrections are explicit behavior changes. Preserve public branding, fallback cache and server truth. Completion: a denied/failed save cannot be silently presented as remotely persisted; a reopened editor reflects agreed current data.

### A13 — Add focused delivery/sync measurement and operational failure assurance

This is test/quality debt with two independent scopes, not another copy of A04/A05's production refactoring. `internal/delivery` and `sources/sync.go` report 0% CC because tests mainly run in `cmd/server`; cross coverage proves they are exercised. R22a adds package-local semantic fixtures and a reviewed instrumentation proposal, retaining handler integration tests. Separately, R22b adds backup publication failure/restore, custom `BACKUP_PATH` export contract, matching keyring, envelope authentication versus startup shape checks, and interrupted encryption/table-rebuild tests (`backup.go` health 9.61, CC 42.60%; `migrations_data.go` 7.39/local 66.5%). The optional real snapshot and external-client validators need a deliberate fixture/runner policy. Keep historical migration identities, row IDs/sequence, FK restoration, credentials and rollback intact. No old migration rewrite is proposed. Completion: failures are proven safe on populated disposable databases; coverage shows actual package execution without claiming line/statement equivalence.

### A14 — Stabilize the public subscription page configuration across renderers

Go `subscription_page.go` 8.09, CC 73.06%, 8/8 touches, 1,306 churn; frontend page config/blocks and modal repeat related schema/default/layout choices. Backend HTML contains substantial CSS/script constants that should not be mistaken for procedural code. Existing escaping and responsive tests are valuable. R23 adds common JSON config/default/unsafe URL/theme fixtures and renderer-equivalence assertions for supported block semantics before extracting normalization/default definitions. Preserve browser activation flow, link schemes, escaping, localized defaults and static deployment. Avoid imposing pixel equality or replacing the Go page with a Next runtime service.

## P3

No independent cosmetic backlog is proposed. Naming, redundant directives, small dead branches and helper arguments should be handled only inside an already justified batch when directly relevant. The impossible modal condition is already grouped under A12; it is not a duplicate P3 item.

## Deliberately deferred score-only candidates

Do not start with `profiles/uri.go` (4.74, CC 88.62%), `profiles/tuic.go` (5.59), password hasher (6.35), crypto/keyring argument/duplication warnings or the migration SQL lists. Those largely represent tested grammar, explicit security roles or ordered compatibility work. Their precise semantic decisions and limits appear in the audit's acceptable-complexity table. If they need future functional edits, protect the existing corpus and failure cases first.

## Ordered shortlist

1. A00 startup WAL/data preservation.
2. A01 user/activation/subscription/device mutation boundaries.
3. A02 encrypted profile persistence/patch policy.
4. A03 frontend configuration document editing.
5. A04 response-rule persistence and public delivery.
6. A05 job/source lifecycle and transaction boundary.
7. A06 authorization guard consolidation with actual-route tests.
8. A07 key editor command/draft lifecycle.
9. A08 key-list ordering/actions/presentation.
10. A09 contracts/common request errors.
11. A10 backend Xray interpretation.
12. A12 branding save/reopen correctness.
13. A13 focused package tests and operational failure assurance.
14. A11 unused persistence declarations.
15. A14 public-page renderer/config consistency.

Priorities describe future engineering attention. P0/R02 and the identified A02/A12 corrections must be labeled corrective changes in future review; behavior-preserving refactoring must not silently change validation, defaulting, roles, outputs, scheduling or data semantics.

## Execution status — R01 / PR #7 (2026-10-06)

R01 (P0/A00, no prerequisites) is complete and Windows/Linux validated at a438377. The former repository-test CodeScene failure is resolved; all six PR checks pass. See the current status above and the R01 execution record for tests, coverage and limits. Startup production health remains 9.57/9.92; both new test files score 10.00. Existing initializer, migration and bootstrap tests are retained.

Added characterization protects normal WAL/DELETE startup and restart, existing accounts/users/encrypted profiles/keyring, migration failure and later retry, directory errors, sidecar absence/junk, partial filesystem removal, and wrapped I/O classification. A disposable crash-WAL reference proves that the current cleanup helper removes a committed row; this is labeled a destructive observation to replace in R02, not a desired invariant.

**A00 remains open.** R02's corrective recovery policy, fault-injection seam, checkpoint/close/lock/permission/automatic-retry scenarios and fail-closed contract are deliberately deferred. The R01 tests do not establish production trigger frequency or repair the loss hazard. Proposed contract: retain committed DB/WAL/keyring on failure; use demonstrably safe SQLite recovery or return an error without deleting recoverable state. No production, migration, security or API behavior changed.

**PR-specific follow-up under A01 — resolved:** the earlier test-health gate failed at local 4.05/service 4.06. The requested test-only correction in a438377 raises local health to 10.00 and passes the remote gate without suppression. A01 remains partial because mutation-route and concurrency/fault work is unfinished.

**Linux CI follow-up — passed:** early run 37364737957 was blocked by an audit-prose scan false positive. The exact-fingerprint correction in 235408c resolved it; run 37429826449 at a438377 passes backend, frontend, Docker and all uploads/quality checks.

**User-requested CodeScene follow-up (2026-10-06):** the reported PR #7 test-health failure is resolved by reorganizing only `cmd/server/repository_test.go`. Its score rose from 4.05 (service baseline 4.06) to 10.00, with no findings; both ordinary and strict `cs delta main` pass. Existing projection, access, activation, HWID and database-failure assertions are retained, including empty-to-populated reads and valid-to-invalid time-zone updates. This is a focused test-maintainability follow-up explicitly requested after R01, not implementation of A01 production work or another batch. No CodeScene suppression or production change is included; all six remote checks pass at a438377.
