# R03 — User mutation contracts and retry correction

R03 continues in [PR #8](https://github.com/romanpodg/SubShare-Go/pull/8), as explicitly requested by the user. The test-only characterization commit is `5d758a6`; the subsequent corrective change fixes exhausted create retries and isolates the checked insertion phase. R04's full transaction-owned create/delete command boundary remains future work.

## Test matrix

| Boundary | Added protection |
| --- | --- |
| Registered legacy/v1 create routes | Normalization, active/default/custom statuses, zero/negative/minimum/maximum durations, byte/Unicode field limits, required/custom/duplicate activation codes, malformed/unknown/trailing/oversized JSON, exact success and error envelopes, initial key assignment and audit effects |
| Create persistence | Parent/assignment SQL failure, rollback and successful later retry, injected begin/commit failure, four subscription-ID collision retries followed by success, exhausted retries with/without existing profiles |
| Registered delete routes | Invalid/missing/repeated IDs, target-only device/assignment cascades, retained other users and encrypted profiles, cascade failure rollback and audit effects |
| Activation | Both public aliases and their legacy error shape, trimmed codes, single use, unknown/invalid codes, update failure, existing delivery identity, separate access denial for paused/blocked/future/expired users, crypto-service fallback, eight simultaneous requests with exactly one winner |
| Devices | Repeated last-slot contention on real SQLite connections; normalized existing device at capacity; commit failure retains previous metadata. Existing repository fixtures retain zero/negative/unlimited limits, legacy duplicate normalization, insert rollback and missing-user/database errors |
| Subscription updates | PUT replacement versus PATCH omission/null/empty clearing, refresh defaults, explicit overrides, Unicode byte-versus-rune validation, invalid URL/date/range/status/timezone, UTC/offset/local/user-zone and DST behavior, equal start/expiry, SQL failure atomicity, legacy PUT alias |
| Concurrent PATCH | A bounded execution barrier makes both requests read before either writes, deterministically reproducing the existing disjoint-update loss |

The SQL fault driver wraps real disposable SQLite connections. Its commit failure explicitly rolls back before returning the injected error; it tests application response/audit behavior, not every real driver's failed-commit cleanup or kernel I/O scenario. No runtime databases, accounts or keys are used.

## Separate corrective change

Previously, regenerating a subscription ID overwrote the last insertion error. Five failed inserts could reach commit with user ID zero, report HTTP 200 and publish a success audit event even though no user existed. The characterization reproduced that behavior with real SQLite constraint errors from a disposable trigger. No production collision frequency is claimed.

`insertUserWithSubscriptionRetries` now retains insertion failures, stops after five attempts, checks the returned identity and leaves assignment/commit in the original caller-owned transaction. Exhaustion returns the existing conflict envelope and rolls back every fixture side effect without a success audit event. This extraction reduces `apiCreateUser` cyclomatic complexity **14 → 9** and removes its Bumpy Road finding. `handlers_users.go` improves **8.00 → 8.14**; the new insertion module and all six test files score **10.00**. Other handler findings remain outside this correction.

## Validation and follow-up

Local Windows full Go suite with atomic coverage passes: **793 results, five existing optional skips**. Final focused fixtures, repeated contention cases, migration compatibility, vet, build, golangci-lint 2.13.2 and strict CodeScene delta are checked separately. The full-suite coverage snapshot measures overall statements **52.4% → 55.2%**, create **88.6%**, delete **100%**, activation **91.7%**, device registration **100%**, and checked insertion **93.3%**. Additional focused cases do not overwrite that snapshot or masquerade as a new aggregate profile.

Linux race verification is now a required backend CI step for the three contention fixtures; local race execution is unavailable with `CGO_ENABLED=0`. Current Linux/frontend/deployment/CodeScene/coverage status is available in [PR #8 checks](https://github.com/romanpodg/SubShare-Go/pull/8/checks). This record captures local evidence before publication; the PR description records observed hosted results.

**Confirmed P1 — now corrected in the same PR:** R03 reproduced concurrent disjoint PATCH requests returning HTTP 200 while losing one change. The subsequent [PATCH correction](subscription-patch-concurrency.md) replaces that characterization with preservation regressions and conditional writes, preserving clearing/date/status contracts and documenting bounded conflict handling. This section's R03 metrics remain historical evidence. A01 remains open; R04/R05 production command/policy work is incomplete.
