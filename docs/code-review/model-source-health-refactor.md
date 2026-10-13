# Model and source API health refactor — 2026-10-13

Basis: `main` at `d71f611`. The supplied CodeScene PDF reviews report **8.55** for `internal/model/model.go` and **8.15** for `cmd/server/sources_v1.go` (the screenshots display 8.54 and 8.14).

## Findings and changes

- **Model duplication and primitive arguments:** user/key status, key kind, check status, subscription format and assignment mode now have distinct domain types with normalization/label methods. One private vocabulary lookup owns trimming, case normalization, accepted values, defaults and aliases. Existing DTO fields, JSON tags and canonical string results remain unchanged. Callers explicitly select the appropriate domain at validation boundaries.
- **Six status arguments:** `UserStatusSnapshot` groups stored status, optional expiry and device usage. The clock remains an explicit second argument. Blocked/paused status still precedes expiry, expiry still precedes device limits, and expiration requires strictly passing the expiry timestamp.
- **Source update complexity and nesting:** request preparation, HWID resolution, SQL settings writes and linked key status updates have separate responsibilities. The handler loads the current source before decoding the body and audits only after the transaction commits. Shared source failures preserve HTTP status, code, message, field attribution and request ID.
- **Source creation complexity:** URL/duplicate/name preflight and parsing/selection are separate from the HTTP response and existing atomic creation command. Duplicate detection still precedes name validation and fetching. Concurrent duplicate conflicts and post-commit response-read failures retain their existing treatment.
- **Five parsing arguments and masking condition:** pasted body, content type and final URL travel together as `sourceBodyInput`; parsing still takes precedence over fetching when the body is nonblank. URL masking uses sequential guards and retains the original result.

## Preserved boundaries

All original tests and route registrations remain. Update category upserts intentionally remain outside the settings/key transaction, including on a later URL conflict. No schema or wire contract changes were introduced. Omitted/null/empty/whitespace HWID values retain the current secret; a nonblank replacement is trimmed, and explicit clearing takes precedence. Update still trims version/model fields without applying import-time HWID clamping. Source settings and linked key status commit together; failures roll back both and publish no update audit. SQL errors stay behind safe client messages.

## Validation

- Uncached baseline model/source/external-profile suites pass; the same refactored suites pass.
- New actual-route source update checks cover seven HWID cases, disable/re-enable publication, write/disappearance/begin/commit failures, conflict/category lifecycle, category failures, JSON rejection and validation precedence. Those checks also pass against the **original update handler**, using an ignored Go overlay with its original route binding.
- Existing model tables remain; added contracts cover expiry equality/precedence, absent expiry, unknown status, negative limits, format/kind/assignment defaults and rejection of values from the wrong domain.
- Repository lint: **0 issues**. Full backend and affected race validation are in progress at PR creation; results will be recorded before handoff.

Detailed validation logs and the original-handler overlay are under ignored `.cache/final-health/`. Fresh health scores must come from the PR's CodeScene analysis. No health rules, thresholds, test selections or secret scanning were disabled.
