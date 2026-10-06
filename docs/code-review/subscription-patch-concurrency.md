# Subscription PATCH concurrency correction

This corrective change continues in [PR #8](https://github.com/romanpodg/SubShare-Go/pull/8), following R03's deterministic lost-update characterization. The original regression failed when changed to require both disjoint updates to survive; the corrected implementation passes it.

## Behavior and implementation

Each PATCH reads a subscription snapshot, applies the existing validation/clearing rules to a copy, then updates only if all subscription fields and the original timezone still match. Comparison and update are one SQLite statement. A stale request reloads and reapplies its original input, up to five attempts. No schema migration, client revision field, process-local mutex or global connection-pool change is introduced.

Dates are compared using their exact original stored text, including NULL and fractional precision, so Go's timestamp formatting cannot create false conflicts. Timezone comparison forces local dates to be reinterpreted if settings change during the request. Status, date, metadata and delivery validation retain their existing order and response codes.

Valid disjoint changes and explicit clears survive concurrent requests. Dependent date edits are validated against the latest winning state: an invalid combined range returns the existing HTTP 400 `date_range_invalid`. A deleted user returns 404. Sustained contention returns HTTP 409 `subscription_conflict` with `subscription changed; retry the request`, documented in OpenAPI. Successful writes alone produce success audit events; failed validation, exhausted retries and unknown affected-row results do not publish success.

The HTTP adapter delegates to three small implementation files: request orchestration/error mapping, pure patch policy, and snapshot persistence. `apiV1PatchUserSubscription` cyclomatic complexity drops **20 → 4**. `subscription_admin_v1.go` improves **7.67 → 8.34**; all three new implementation files and both new test files score **10.00**. Existing assignment/URL findings remain outside this fix.

## Verification

- The former characterization now requires simultaneous title/status changes to survive, with blocked-reason clearing and both audit events preserved.
- Controlled overlap covers explicit NULL clearing and conflicting date changes. Interleaving fixtures cover timezone changes, an actual legacy PUT, deletion, four stale writes followed by success, and five stale writes returning conflict.
- An independent SQLite handle cannot overwrite a stale snapshot. NULL dates, blank timezone fallback, literal/driver timestamps and nanosecond precision are preserved. Cancellation and affected-row errors retain safe diagnostics and avoid false success.
- Windows full Go/atomic-coverage snapshot passes: **821 results, five existing optional skips**. The final legacy PUT fixture passes separately; repeated contention cases, vet, build, lint, migration compatibility and strict CodeScene delta also pass.
- Linux CI's required race pattern now includes every `TestSubscriptionMutationConcurrentPatch` fixture. Local race execution remains unavailable with `CGO_ENABLED=0`. Current hosted results are in [PR #8 checks](https://github.com/romanpodg/SubShare-Go/pull/8/checks); the PR description records verified CI results without rewriting this pre-publication local snapshot.

An affected-row reporting error can occur after the database has applied a write; the handler returns a safe 500 rather than claiming success or automatically retrying an unknown outcome. The fault-driver test verifies response/audit handling, not rollback of an already applied driver operation. Ordinary SQL failure atomicity remains covered by R03.

The reproduced PATCH lost-update defect is resolved. The later [R04 user create/delete command extraction](r04-user-commands.md) is now implemented in the same PR; R05 and other A01 work remain separate.
