# R16a — Actual registered-route authorization contracts

Status: accepted in [PR30](https://github.com/romanpodg/SubShare-Go/pull/30), merge `4b6cf82d815b52f317845a7b542afed764eb6af2`. All six checks and the new actual-route authorization race pass at `01e6785a455f9258bb47adb7735a15970cd19962` in [run 38117356256](https://github.com/romanpodg/SubShare-Go/actions/runs/38117356256). Final automated review completed with a positive reaction and no threads. Production guards/registrations remain unchanged in R16a; R16b and A06 acceptance remain.

The initial matrix passes 2,907 cases across 116 registered route patterns, 18 anonymous/session/token principals and GET/HEAD behavior. It covers owner/operator/viewer, legacy role aliases, API token creator roles, read/users/keys/settings/wildcard/empty scopes, valid/missing/invalid CSRF, owner-only reads/writes, interactive token mint/revoke and four public configuration aliases. Permitted handler statuses/codes are frozen from a safe valid-owner probe using malformed write payloads and absent IDs. Runtime tests use the actual registered ServeMux, match its pattern and compare independent frozen permissions; they never call production guard or scope helpers to obtain expected decisions. Accepted asynchronous health jobs are joined before fixture cleanup.

The source inventory separately scans every non-test Go file for API registrations and compares the exact alias set against the frozen fixture. New or missing endpoints therefore require an explicit reviewed contract; expected guard/scope data is never regenerated during tests. Login/activation/public subscription delivery retain their existing rate-limit/access contracts and are excluded from this administration matrix.

Additional actual-route cases pass for expired/revoked credentials, deleted administrator, malformed/unknown/null scopes, session/token/role lookup SQL failures, cookie-plus-bearer precedence, cookie-required CSRF and whitespace normalization. Independent SHA256 checks protect hashed cookies/tokens; denied responses/logs must contain no raw credentials. The empty-cookie case sends an actual empty cookie. The final combined characterization passes all 2,930 cases. All seven new test files score 10.00 after typed fixtures and named checks remove quality findings without suppressions. Vet/build/lint, actionlint and strict delta pass. Full uncached Go/atomic coverage exits zero: 4,370 pass / five unchanged optional skips, no failures; server coverage 75.4%.

The workflow adds a dedicated Linux race run for these actual-route cases because local CGO is unavailable. Require final combined characterization, uncached full Go/coverage/vet/build/lint, actionlint, strict quality/secret gates, all six final-head checks and race, and resolved actionable review before R16a acceptance. R16b then shares guard evaluation while retaining explicit routes, error/denial precedence, constant-time CSRF, hashed credentials, per-handler principal queries and current scope policy. A06 and the entire goal remain incomplete. Heartbeat stays paused per the user; the persistent goal remains active.

| Required authorization characterization | Evidence |
| --- | --- |
| Registered endpoints and all administration aliases | Frozen 116-pattern fixture, live ServeMux pattern checks and source-wide API inventory |
| Anonymous/viewer/operator/owner and legacy aliases | Eighteen session/token principal variants over every route and GET/HEAD |
| Scope, CSRF, owner-only and interactive token boundaries | Full matrix with scoped/wildcard/empty tokens, creator roles and valid/missing/invalid CSRF |
| Expired/revoked/deleted principal, invalid scopes and DB errors | Twelve actual-route credential cases and real SQLite query faults |
| Cookie-plus-bearer precedence and empty/expired cookies | Seven actual-route precedence cases, including trimmed CSRF |
| Unknown write denial | Retained `TestAPITokenAllowsDeniesUnknownWritePaths` plus actual logout denial for every token, including wildcard |
| Hashed storage and secret-free denied output/logs | Independent SHA256 assertions and actual denied-request capture |
| Principal downgrade/removal while handler runs | R16b retains per-handler queries and introduces no principal context cache; this conditional requirement stays explicit |
