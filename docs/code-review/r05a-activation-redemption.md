# R05a — Atomic activation redemption boundary

R05a starts [PR #9](https://github.com/romanpodg/SubShare-Go/pull/9) from **d87b122**, the merged PR #8 base, at the user's request. The R03 registered-route, repeat/expiry and concurrent redemption fixtures are prerequisites. Assurance tests were committed first at **230540c**, and passed against the unchanged activation implementation; the extraction is **e361b18**.

## Changes and contracts

`redeemActivationCode` is now an HTTP-independent command. It loads the activation record, rejects missing/already-used codes, prepares the existing trimmed or newly generated subscription identity, and returns that identity only after the conditional claim confirms success. Command failures retain their underlying causes without serializing them into the public response.

`repository_activation.go` owns only the record read and conditional SQL claim. One UPDATE writes both `subscription_id` and `activation_used_at`, guarded by `id` and `activation_used_at IS NULL`. The initial read does not reserve a code: a concurrent winner makes a stale claim return zero affected rows. No read/write transaction, new repository interface, storage manager, schema, migration or retry policy is introduced.

Pure activation-code validation preserves whitespace trimming and empty/slash rejection, including the existing absence of an activation-endpoint length limit. The HTTP adapter maps missing codes to 404, used/stale claims to 403, validation failures to 400 and persistence/generation errors to the existing generic 500. Both public aliases keep their legacy error envelope, exact messages and two-field success response. URL presentation retains the configured base URL, Happ encrypted-link success and plaintext fallback on encryption errors.

Redemption continues to consume a code independently of access policy: paused/blocked, future-start and expired subscriptions can be redeemed, while subscription delivery still enforces their access restrictions. Request cancellation, rate limiting and success-audit behavior remain unchanged.

## Assurance

- Two real SQLite requests, one per public alias, are held immediately before their conditional UPDATE. Both have completed their reads and identity preparation. Tests cover missing and existing subscription IDs, prove exactly one successful response, compare its link to the persisted identity, withhold the loser link, and prove a subsequent claim cannot replace the winning identity or timestamp.
- The existing eight-request activation test, missing/duplicate code tests, null/blank/existing identity fixtures, SQL-trigger failures and access-policy fixtures remain. Direct command tests now assert domain outcomes rather than HTTP statuses; registered routes still protect exact HTTP behavior.
- A real uniqueness conflict in the trimmed identity fails the atomic claim and preserves the original unused code/identity. A synthetic affected-row confirmation error returns 500 without a link and preserves the command's error cause. That reporting failure occurs after the UPDATE: the claim may have committed, and rollback is not promised. This is assurance of existing behavior, not a corrective change.
- Both aliases exercise encrypted URL success and unavailable, empty-link and malformed-response fallbacks using disposable localhost servers. Code validation fixtures protect normalization and existing acceptance rules.
- The existing Linux CI race step now includes all `TestUserActivationConcurrent.*` cases, alongside the existing device/PATCH contention tests.

## Validation and health

- Full Windows backend suite at e361b18 passes with **860 test results, five existing optional skips**, using uncached tests and atomic coverage. Total statements are **55.8%**; redemption is **93.8%**, URL presentation **90.0%**, and code validation, HTTP error mapping, activation handler, record loading, identity preparation and CAS are **100.0%**. The remaining command gap is token-generation failure; the URL helper's empty-success fallback is unreachable through the current encryption client, which returns an error for an empty link.
- `go vet ./...`, server build, golangci-lint **2.13.2** against origin/main, actionlint and the uncached migration compatibility test pass. Strict CodeScene delta passes. Reports remain local under `.cache/r05a/` rather than being checked in.
- Hosted Linux race/backend/frontend/deployment results belong to [PR #9 checks](https://github.com/romanpodg/SubShare-Go/pull/9/checks), independently of PR #8's historical results.

CodeScene reviews report **10.00** for the command, policy, HTTP adapter, SQLite activation adapter and changed activation/fault test files. `handlers_subscription.go` improves **7.56 → 7.79**: activation's complex-method and nested-block findings are removed. Other delivery findings remain outside R05a. Strict delta against `origin/main` passes.

The Windows sandbox denies Go coverage metadata renames and localhost mock HTTP connections; final validation must use the approved unrestricted runner. Local race execution is unavailable because CGO is disabled and no C compiler is installed. Linux CI provides the race-enabled runner. Docker Desktop is not running locally; deployment validation belongs to hosted CI.

R05a's command/policy extraction is implemented and all six hosted checks passed at ddf7154. The later user-requested [R05b continuation](r05b-device-registration.md) owns current device policy/transaction evidence; R06 is now next and the broader A01 scope remains incomplete. The activation extraction can be reverted independently without converting stored data.
