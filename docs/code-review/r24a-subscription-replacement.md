# R24a — Remaining A01 subscription replacement boundary

Status: tests-first on `test/subscription-replacement-boundaries`, from accepted PR42 merge `0dfdba1`. A00/A02–A14 are accepted; A01 remains incomplete.

The original A01 requirement leaves legacy subscription PUT validation and SQL coupled to its HTTP handler. PATCH already has pure tri-state policy, snapshot comparison and five-attempt reload/revalidation. Finish the legacy replacement policy/command/adapter seam; relocate PATCH field normalization and response mapping into focused files while retaining CAS/retry/query bodies. Keep assignment/user-settings work separate as R24b. No generic repository or schema rewrite.

Existing committed R03 and PATCH-correction tests protect omission/null/empty, default refresh, byte-versus-rune lengths, URL/status/date/timezone/DST/equal-boundary behavior, SQL failures and audits; concurrency protects disjoint updates, explicit clear, revalidated date ranges, exhausted retries, timezone changes, deletion, independent DB writers and intervening PUT. The R23 full run executes this corpus unchanged.

Additional registered-route characterization executes14 parent/subtest events against unchanged subscription production code: missing target retains legacy/v1 PUT200+success audit versus PATCH404/no audit; legacy unlimited trimmed blocked reason versus PATCH255rune bound; validation precedence and no-write/no-audit failures. All pass in10.279s; new test health10. These are deliberate compatibility assertions, not corrections.

After saving baseline, introduce pure legacy replacement validation with the same ordered phases and original server-local date parsing/default12/byte limits. A command executes the exact existing UPDATE and arguments/background context without new affected-row or CAS checks; HTTP decodes/maps errors/audits after success. Direct command tests must protect no audit in persistence and error/state retention. Preserve PATCH's current caller context, SQL/NULL/date-precision comparisons and retry budget; no policy unification.

Require focused/full/static/migration/build, strict/secret and six exact-head hosted gates/five Linux races/completed actionable-findings-free review before acceptance. Then R24b and complete A01's requirement matrix. Goal active; heartbeat paused per user.
