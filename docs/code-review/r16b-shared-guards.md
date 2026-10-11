# R16b — Shared administrative access evaluation

Status: locally validated on `refactor/shared-admin-guards`, based on accepted R16a merge `4b6cf82`. Hosted/race/review acceptance remains; no R16b PR exists at this checkpoint.

`requireAdmin` and `requireSuperAdmin` retain the explicit route-facing API. They delegate principal resolution, role denial, API token scope and CSRF enforcement to one guard. A typed role/method descriptor carries no credentials to the pure role callback. Denial order, legacy/v1 envelopes, bearer/cookie precedence, interactive token management, constant-time CSRF and SHA256 storage remain. Handler principal queries retain their current timing; no request-context principal cache or generic route permissions dispatcher is introduced.

All 2,949 focused cases pass after application: the complete 2,930 accepted real-route characterization plus existing wrapper/CSRF/reveal cases. Eleven other authentication function bodies and six SQL literals are byte-identical; route/handler declarations are unchanged. New guard health is 10.00 and auth improves 8.24 to 8.81, removing guard duplication. Remaining scope complexity, token expiry conditional and common error-helper findings are unchanged. Final vet/build/lint, strict delta and parity pass. Full uncached Go/atomic coverage exits zero: 4,370 passes / five unchanged optional skips, no failures, server coverage 75.3%. Hosted authorization race remains required because local CGO is unavailable.

| A06 completion requirement | Evidence and acceptance boundary |
| --- | --- |
| Actual registered routes, aliases, roles/scopes/CSRF and public configuration | Accepted R16a PR30 fixture/inventory, 2,930-case characterization and dedicated hosted race |
| Expired/revoked/deleted/invalid/DB error and cookie/bearer precedence | Accepted credential and seven precedence cases, rerun unchanged after guard extraction |
| Interactive token mint/revoke and unknown write denial | Full actual-route matrix and retained deny-default token tests; token policy body unchanged |
| Hashing, constant-time comparison and secret-safe output/logs | Independent stored-hash/output/log fixtures; credential/hash/error helper bodies and SQL unchanged |
| Reduced duplicated guards with explicit routes and unchanged privileges | One shared guard with two named role policies; protected route declarations unchanged and 2,949 focused cases passing |
| Principal downgrade/removal condition | No principal context caching introduced; handler principal queries retain their original timing |

Require full local/secret checks, all six final-head hosted gates including authorization race, and resolved actionable review before A06 acceptance. Then proceed R17a ordering/selection/action characterization and controller extraction. Broader A01/A08-A14 and the entire goal remain incomplete; heartbeat stays paused per the user.
