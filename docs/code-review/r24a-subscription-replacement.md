# R24a — Remaining A01 subscription replacement boundary

Status: implemented on `refactor/subscription-replacement-command`, after tests-first `90b8fa6`, from accepted PR42 merge `0dfdba1`. Local validation passed; hosted acceptance pending. A00/A02–A14 are accepted; A01 remains incomplete.

The original A01 requirement leaves legacy subscription PUT validation and SQL coupled to its HTTP handler. PATCH already has pure tri-state policy, snapshot comparison and five-attempt reload/revalidation. Finish the legacy replacement policy/command/adapter seam; relocate PATCH field normalization and response mapping into focused files while retaining CAS/retry/query bodies. Keep assignment/user-settings work separate as R24b. No generic repository or schema rewrite.

Existing committed R03 and PATCH-correction tests protect omission/null/empty, default refresh, byte-versus-rune lengths, URL/status/date/timezone/DST/equal-boundary behavior, SQL failures and audits; concurrency protects disjoint updates, explicit clear, revalidated date ranges, exhausted retries, timezone changes, deletion, independent DB writers and intervening PUT. The R23 full run executes this corpus unchanged.

Additional registered-route characterization executes14 parent/subtest events against unchanged subscription production code: missing target retains legacy/v1 PUT200+success audit versus PATCH404/no audit; legacy unlimited trimmed blocked reason versus PATCH255rune bound; validation precedence and no-write/no-audit failures. All pass in10.279s; new test health10. These are deliberate compatibility assertions, not corrections.

After saving baseline, introduce pure legacy replacement validation with the same ordered phases and original server-local date parsing/default12/byte limits. A command executes the exact existing UPDATE and arguments/background context without new affected-row or CAS checks; HTTP decodes/maps errors/audits after success. Direct command tests must protect no audit in persistence and error/state retention. Preserve PATCH's current caller context, SQL/NULL/date-precision comparisons and retry budget; no policy unification.

Require focused/full/static/migration/build, strict/secret and six exact-head hosted gates/five Linux races/completed actionable-findings-free review before acceptance. Then R24b and complete A01's requirement matrix. Goal active; heartbeat paused per user.

## Implemented subscription boundary

Tests-first90b8fa6 precedes extraction. Legacy replacement has pure ordered access/metadata/delivery validation, a single-statement command and HTTP-only error mapping/audit. Server-local date parsing, byte limits/default12/unlimited blocked reason, omitted/null replacement and missing-user success remain. Exact UPDATE bytes and positional conversions/order/context are retained; no new affected-row/CAS check. PATCH field normalizers and response mapping move without changing command/CAS/repository bodies. The absolute HTTP URL condition is factored into an equivalent host/userinfo/scheme predicate to avoid carrying a complex condition into the new module.

Twenty retained functions are AST-identical; replacement SQL bytes identical; PATCH snapshot/policy files have no diff. Sixty-four retained/new route/concurrency events and three direct command cases pass. Direct tests prove persistence returns normalized committed state without auditing, validation and SQL failure preserve stored rows/no audit, and no provisional result is published. All new modules/tests score10. handlers_users8.44→9.09; subscription_admin_v1 8.34→9.46; remaining assignment/settings complexity remains R24b. Backend vet/configured lint(0issues)/build and strict gate pass. Full uncached Go exec4737 is running; static exec75103 completed zero. No local frontend rerun is claimed because frontend source is unchanged; final hosted frontend/Docker/all six gates/five races/review remain mandatory.

## Final local validation

Full uncached Go/atomic coverage exec4737 completed zero:4,587passes/five unchanged optional skips in494.003s,server76.6%. The final source is unchanged from this run. Static/migration corpus/configured lint0/build,67focused/direct events,20retained-function/exact-SQL proof and strict/staged-secret gates pass. Frontend source is unchanged; the prior accepted R23 local frontend evidence is not relabeled as a new R24a run. Require the final six hosted gates, five Linux races, populated Docker and completed clear exact-head review before normal merge. R24b and final A01 matrix remain.
