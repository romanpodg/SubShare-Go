# R24b — Remaining A01 assignment and user-settings boundaries

Status: accepted in PR44 at head `63a0c0624761947780cb49d62461c322a31f61b3`, merge `be4ee583a405d8db9a465fa50779eb9d3a36310b`, after tests-first `d268c04`, explicit correction `3df1219` and extraction `63a0c06`. Earlier checkpoints below are historical; all A00–A14 code requirements are accepted.

Assignment already owns one transaction but normalization and persistence share the v1 HTTP module. User settings still normalize and write directly in HTTP. Extract their pure policy and command boundaries, preserving SQL/arguments/background context/error precedence/defaults/aliases/deduplicated order and a single assignment transaction. Keep legacy and v1 error contracts distinct, including the characterized selected nonpositive-ID500. No generic repository, read-projection rewrite or schema change.

Baseline uses actual registered routes and synthetic encrypted profile fixtures. Nineteen assignment events pass: all/selected/deduplication/empty, both keys aliases, invalid mode/all IDs/nonpositive/missing key, missing user with other assignments intact, and real begin/query/delete/insert/mode/commit rollback. Ten settings events pass: aliases/defaults/trim/lower, byte limit and validation precedence, missing user and SQL failure/no audit/state retention. All new/extended test files score10.

One desired settings regression fails against production: the real UPDATE completes on an existing user, then the synthetic driver fails affected-row reporting. HTTP incorrectly claims404/user not found because the error is discarded. Independent rerun first verifies persisted UTC/en and logs actual404, then fails desired500 assertion; no harness/import failure. Save this tests-first baseline. The separate correction must return500/no success audit for unknown reporting outcome and preserve the actual performed write; do not claim rollback. Confirmed zero rows still mean404, successful settings updates retain their responses/defaults/audits.

After correction, extract settings validation/command/HTTP and assignment normalization/named transaction phases/HTTP mapping. Add direct command audit/failure tests, retain R03/R04/R05/R24a and PATCH CAS/UTC/DST/NULL/precision/timezone/concurrency corpus. Require focused/full/static/build/migration/strict/secret and six final-head hosted gates/five races/populated Docker/completed clear review. Then complete the A01 requirement matrix and persist final backlog acceptance through normal protected review. Goal active; heartbeat paused per user.

## Implemented correction and command boundaries

Tests-firstd268c04 precedes the separately saved correction3df1219: affected-row reporting failure returns500/no success audit, with the performed update explicitly retained in assertions; known zero still404. All11 settings cases pass after correction. No rollback or generic success is inferred from an unknown outcome.

Settings validation/command/HTTP and assignment normalization/transaction phases/HTTP mapping are separate. Seven retained HTTP functions are AST-identical; all four settings/assignment SQL literals byte-identical. Original statement/argument/context/order/transaction semantics are retained, including first-occurrence ID deduplication, default/localized settings, legacy/v1 aliases and selected nonpositive-ID500. PATCH/replacement command/policy/repository bodies are unchanged.42focused route/rollback/timezone/concurrency and5direct command tests pass. Direct commands retain validation/reporting/commit causes, return no provisional success, preserve subscription/assignment state and leave audits to HTTP.

All six new modules and new/extended tests score10; handlers_users9.09→10 and subscription_admin_v1 9.46→10. Strict gate empty/pass. Static62353 completed vet/lint0/build; its initial guessed server idempotence filter matched no tests and is not counted as migration proof. Correct storage TestMigrateAppliesVersionedMigrationsIdempotently is run separately. Full uncached Go96974 is live; observe actual completion before publication. Frontend source/schema/routing/authentication/coverage upload are unchanged; final hosted gates remain.

## A01 requirement reconciliation (final hosted acceptance pending)

| Original requirement | Evidence |
| --- | --- |
| User create defaults/custom/duplicate codes and input/duration limits | R03 registered legacy/v1 corpus; R04 pure policy/command; false success after exhausted identity retries separately corrected |
| Atomic create identity/assignment and failure outcomes | R03 real SQLite triggers plus begin/commit faults; R04 direct commands withhold provisional ID and preserve rollback/audit boundary |
| Delete missing/existing target and cascading state | R03 target-only profile/device assignment cases; R04 one DELETE command and separately corrected unknown reporting outcome |
| Subscription PUT versus PATCH clearing/default/byte/rune policy | R03 shared fixtures and R24a14 extra route contracts; omitted/null/empty, refresh12 versus clear, unlimited reason versus255rune and missing-target differences retained |
| URL/date/timezone/DST/equal-boundary semantics | Existing R03/core fixtures, R24a pure endpoint-specific policy and unchanged date helpers; PATCH uses current user timezone, PUT server-local |
| Paused/blocked/future/expired/unknown access | R03/core registered delivery status/date/header cases and existing model tests retained |
| Activation exactly one winner/identity/crypto/error safety | R05a command/policy/CAS and synchronized alias/failure cases; original crypto fallback and post-autocommit reporting policy retained |
| Device normalized identity/capacity/last slot/failure rollback | R05b policy and one transaction, synchronized distinct/same identity and begin/update/confirmation/insert/commit cases; zero/negative/unlimited and existing-at-capacity contracts |
| Concurrent PATCH safety | Saved correction corpus protects disjoint/clear/date revalidation, budget, timezone/deletion/intervening PUT, independent DB handles and raw NULL/date precision; command/repository bodies unchanged |
| Assignment all/selected/future/deduplication/rollback | Existing core future-key fixtures plus R24b19 registered alias/validation/begin/query/delete/insert/mode/commit cases; one transaction and original SQL/context preserved |
| User settings defaults/validation/persistence/audit safety | R24b11 route events and direct commands; ordered byte/timezone/language policy; actual false404 reporting regression corrected explicitly |
| HTTP adapters contain no mutation SQL | R04/R05/R24a/R24b adapters decode, map and audit; commands own create/delete/CAS/device/replacement/assignment/settings writes; remaining read projections remain explicit |
| Architectural maintainability | Existing repository/projection/settings decomposition retained; legacy large/complex PUT and assignment/settings coupling removed through focused policy/commands; new modules and final handlers10; no generic repository/schema rewrite |
| Integration and review | Full local backend/static/strict/secret and final six hosted gates/five Linux races/populated Docker/clear review remain required before accepting this final row |

The complete backlog is not accepted until final hosted gates/review and persisted final documentation. Optional real snapshot/four pinned client validators retain their deliberate skips/policy; no external compatibility claim is added.

## Final local validation

Full uncached Go/atomic coverage exec96974 exitszero:4,622passes/five unchanged optional skips in528.663s; server77.8%. Final production/test source stayed unchanged throughout that run.47focused/direct events, actual storage idempotence, vet/configured lint0/build, allnewmodule/test/handler health10, strict empty/pass, protected function/SQL/PATCH/replacement parity and staged-secret gates pass. No local process remains. Frontend/schema/routing/coverage config unchanged; final exact-head hosted frontend/Docker/backend/health/patch/bundle, five races and automated review remain mandatory. A01 and entire backlog acceptance await those gates plus persisted completion records.

## Final acceptance

[PR44](https://github.com/romanpodg/SubShare-Go/pull/44) merged at `be4ee583a405d8db9a465fa50779eb9d3a36310b` after all six gates at `63a0c0624761947780cb49d62461c322a31f61b3`. [Run38296550395](https://github.com/romanpodg/SubShare-Go/actions/runs/38296550395) proves all five Linux races, authentic populated Docker upgrade/matched restore and ordinary restart smoke. Exact-head review completed with a positive reaction/no findings/threads; normal expected-head merge respected protections. The A01 matrix above is accepted. All original A00–A14 code requirements are accepted, with no remaining implementation batch; [completion record](refactoring-completion.md) persists the full reconciliation. Earlier pending checkpoints are historical. Heartbeat remains paused; persistent-goal closure follows final documentation publication.
