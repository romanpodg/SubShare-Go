# R04 — Transaction-owned user mutation commands

R04 continues in [PR #8](https://github.com/romanpodg/SubShare-Go/pull/8) at the user's request. The R03 route/fault/cascade corpus is the behavior reference, including the separately corrected create retry and subscription PATCH defects.

## Changes and boundaries

`createUser` owns identity preparation, UTC access dates, the single SQLite transaction, bounded insertion retries, initial profile assignment, commit and rollback. It returns an identity only after commit. `deleteUser` owns the existing one-statement DELETE and atomic foreign-key cascades, distinguishing a confirmed missing user from a persistence error.

The create/delete HTTP handlers decode/validate inputs, invoke commands, map errors and record success audit events. They contain no SQL, identity generation or transaction management. Pure normalization/validation is separated into named required-field/length phases, preserving status and duration error precedence, trim/default behavior and byte-length limits. Commands do not depend on HTTP requests or response writers. No generic user repository, new storage manager, schema migration or request DTO change is introduced.

Legacy and v1 responses are retained for ordinary success, validation, conflicts, missing users and SQL/transaction failures. Nested token-generation errors keep their original 500 response even when also tagged as insertion failures. Existing background-context persistence behavior is retained; request-cancellation policy is not changed by this extraction. Initial assignment remains inside the creation transaction, and profile/device cascading deletion remains one SQLite statement.

## Explicit failure correction

Before extraction, a synthetic affected-row reporting error caused deletion to return 404 because the handler discarded the error and treated the zero count as proof of a missing user. The new regression failed against that behavior. The checked command now returns an error mapped to a safe 500, without a success audit event.

This does not promise rollback of an operation already performed before a driver's reporting failure: the DELETE may have committed. The driver fault test establishes response/audit correctness for an unknown outcome; ordinary SQLite SQL/cascade failure atomicity remains protected by the real trigger tests.

## Verification and health

- Registered legacy/v1 create/delete fixtures pass unchanged for defaults, status/duration/length validation, duplicate codes, subscription-ID retries, initial assignment, transaction rollback, target-only cascades, missing IDs and success audit timing.
- Direct command tests prove committed identities and assigned profiles are returned together, failed begin/insert/assignment/commit operations return identity zero, and commands leave auditing to HTTP. Validation priority and private nested-error serialization also pass.
- Windows full Go/atomic-coverage run passes: **832 results, five existing optional skips**. Vet, build, golangci-lint 2.13.2, migration compatibility, JSON/diff checks and strict CodeScene delta pass. Hosted Linux/race/frontend/deployment results are tracked in [PR #8 checks](https://github.com/romanpodg/SubShare-Go/pull/8/checks); the PR description records verified hosted results.
- `apiCreateUser` complexity improves **9 → 4**, deletion **4 → 3**, and `handlers_users.go` Code Health **8.14 → 8.44**. The creation helper, command, validation, HTTP mapping and new command-test files score **10.00**. Legacy subscription/settings findings remain outside R04.

R04's create/delete command boundary is implemented. A01 remains incomplete: R05 activation/device policy acceptance and the other mutation boundaries retain their own audit scopes. R05a is the next planned batch; inspect its existing partial extraction before changing it.
