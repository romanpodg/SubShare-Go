# R05b — Device registration transaction and policy

R05b continues [PR #9](https://github.com/romanpodg/SubShare-Go/pull/9) at the user's request, after R05a was validated at **ddf7154**. Assurance tests were committed first at **9d745e8** and passed against the existing implementation. The extraction is **a8d6582**.

## Boundaries and retained behavior

`registerHWID` owns normalization, one SQLite transaction, commit/rollback and the accepted result. `registerDeviceInTransaction` first updates an existing normalized identity, then loads capacity and inserts a new device only when allowed. SQL reads/writes live in `device_registration_repository.go`; `repository_devices.go` now owns only connected-device read projections.

The initial UPDATE remains before the capacity read. It obtains SQLite's write reservation even when no existing device matches, serializing competing registration transactions before their count decisions. Moving that count outside the transaction or ahead of the first write would change contention behavior. The extraction preserves statement order, existing lock/busy-timeout configuration and the absence of application retries.

Pure policy owns raw HWID trimming, explicit normalized-identity precedence, lowercase normalization, nonpositive/unlimited capacity and required/oversized HWID decisions. Capacity still counts distinct identities using the existing normalized-or-legacy raw fallback; unlimited users still skip the count query. Existing identities remain eligible for metadata refresh at capacity, retain their original raw HWID/creation timestamp, and preserve stored fields when new values are empty.

`subscription_devices.go` adapts requests to the policy and command. HWID precedence stays query `hwid`, `X-HWID`, then `X-Device-ID`. The 128-byte limit, mandatory-HWID provider-plus-flag condition, metadata parsing/merging, optional empty-identity bypass, custom/default limit text and delivery-remark overrides are unchanged. Registered subscription/subbody/plain routes retain 403 plus `Subscription-Status: limited` for device denials and a generic 500 for persistence errors. The general subscription status/date and response-rule ordering remain outside this extraction.

No schema, normalized-identity index, storage manager, repository interface, DTO, request-cancellation behavior or admin HWID endpoint change is introduced. Existing direct-command persistence errors remain discoverable, including wrapped commit errors. HTTP does not expose private driver diagnostics.

## Assurance and validation

- Real SQLite transactions are held immediately before their first write, ensuring both registration commands have begun. Distinct IDs compete for one remaining slot, while differently cased/trimmed versions of the same ID both succeed with one stored identity. The existing repeated last-slot fixture remains.
- Begin, update, affected-row confirmation, insert and rollback-modeled commit failures deny registration, preserve metadata/creation/last-seen timestamps and device counts, and release the transaction so another registration can succeed. Confirmation failures occur inside the transaction and are rolled back; R05a's post-autocommit unknown-outcome semantics do not apply to this command.
- Explicit normalized identity takes precedence over a different raw HWID; refresh retains the original raw ID and avoids consuming another slot. Empty optional identities bypass persistence. Existing unlimited/negative/finite limits, legacy distinct counting and read-projection fixtures pass unchanged.
- Request tests cover HWID-source precedence, ASCII and Unicode byte boundaries, provider/mandatory combinations, all three public delivery routes' persistence failures, and default/custom/overridden limit remarks.
- The full Windows backend suite passes with **887 test results and five existing optional skips**, plus focused checks after the final policy readability adjustment. The atomic-coverage snapshot reports **55.9%** total statements and **100.0%** for device normalization, capacity/access policy, transaction orchestration, SQL update/capacity/insert functions and request adaptation. These are statement measurements, not a claim that every semantic failure is exercised. Vet, build, golangci-lint 2.13.2, actionlint and uncached migration compatibility pass. Local artifacts remain under `.cache/r05b/`.
- CodeScene reports **10.00** for the command, SQL adapter, policy, request adapter, device projections and new/fault test files. `subscription_delivery.go` improves **8.57 → 9.37**, removing device access's complex-method and nested-block findings. Strict delta against ddf7154 passes; remaining general delivery findings are outside R05b.
- The Linux CI race step now selects `TestUserDeviceConcurrent.*` alongside activation/PATCH contention tests. Hosted validation belongs to [PR #9 checks](https://github.com/romanpodg/SubShare-Go/pull/9/checks). The established Windows CGO/compiler and local Docker limitations remain; passing hosted checks provide that evidence.

R05b's device command/policy boundary is implemented. R06's encrypted-profile fault and tri-state characterization is next. A01 still includes other user/subscription mutation boundaries; completion of R05a/R05b does not close the broader audit item. This extraction can be reverted without stored-data conversion.
