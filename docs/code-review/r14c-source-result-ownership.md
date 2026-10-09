# R14c — Bind source results to their owning fetch

Status: three desired regressions fail on `fix/source-sync-stale-results`, based on accepted R14 main `1785f86`. No production correction exists at this checkpoint.

The separately accepted R14 characterization is replaced by protection expectations: the newer started fetch retains its result when an older one finishes later; a changed source URL rejects old fetched bytes; and an older failed request cannot replace a newer successful source status. All three fail against unchanged production. Raw failing results stay ignored under `.cache/r14c/stale-before.json`. The original R14 history retains the prior characterization, so the corrective behavior is explicit rather than a silent refactor.

Implement fetch ownership and a transactional current-source check, preserving current category/enable target projection, profile/secret/assignment atomicity, public job/source shapes and existing best-effort tracking classification. No schema, scheduler or credential manager replacement is needed. Require no URL/secret values in stale-result errors, all focused/full/race/hosted checks and resolved actionable review before acceptance. Contended tracking is recorded separately for lifecycle command work; do not silently redesign it here. R15 and the broader backlog remain incomplete.
