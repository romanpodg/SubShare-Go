# R15b — Source-create transaction command

Status: locally validated extraction on `refactor/source-create-command`, based on accepted R15a merge `e52f3a4`, after tests-first commit `9e42a08`. Hosted/race/review acceptance remains; no R15b PR exists at this checkpoint.

Sixteen actual registered-route cases pass against unchanged source creation: source/category/key-category write and resolution errors, second profile/credential/all-mode assignment failure, source completion failure, rejected begin/commit, duplicate URL committed while an earlier fetch waits, shared category reuse and encrypted assignment success, and failed response read after a committed full unit. The read-fault seam extends the existing disposable SQLite driver. Its first test-only matcher failed to recognize multiline SQL; the corrected matcher is proven and the failed result stays ignored under `.cache/r15b/before.json`, with the passing baseline at `baseline.json`. All five fixture/contract files score 10.00. No production behavior was changed to pass these tests.

One typed command now owns the category/source/profile/secret/assignment transaction and returns identity/result only after commit. Validation and duplicate preflight ordering, fetch/selection/empty errors, normalized metadata/headers, unique-conflict classification, all 11 SQL literals and existing ignored LastInsertId handling remain. Thirteen other function bodies, including the moved category helper, are byte-identical. Response loading and audit remain outside the transaction: a failed post-commit read still returns the existing safe 500 while retaining committed data and withholding the audit/DTO. The existing source-sync adapter and credential manager remain active. Source update retains its audited category preparation before its later transaction.

All 31 new/existing source cases pass after extraction. Both new modules score 10.00; source adapter health improves 7.79 to 8.14. Final strict delta, vet/build/lint, actionlint and SQL/body parity pass. Full uncached Go/atomic coverage exits zero: 1,440 pass / five unchanged optional skips, no failures, server coverage 69.0%. Hosted race remains required because local CGO is unavailable.

The workflow's existing lifecycle race step includes `TestSourceCreate` cases. Require targeted/full uncached Go/coverage, vet/build/lint, actionlint, strict quality and secret gates, all six final-head checks with dedicated race, and resolved actionable review before acceptance. A05 requires R14/R14c/R15a plus this independent command batch; the wider backlog and broader A01 remain incomplete.

| A05 completion requirement | Evidence and acceptance boundary |
| --- | --- |
| Actual queue/202, retry, terminal/error/count and restart recovery | Accepted R14 matrix and R15a PR28; retry edges and unchanged worker bodies |
| Bounded fetch/concurrency and detached shutdown versus recovery | Accepted fetch/timeout/cancel/cap/shutdown fixtures and hosted lifecycle race |
| Required versus best-effort tracking/status/audit | Accepted real-SQL fault fixtures and documented SQLite contention; tracking policy unchanged |
| Reverse completion and mid-fetch URL/HWID/category/enable/delete | Separate accepted R14c corrective PR27, transactional snapshot and owner/status regressions |
| Category/source/profile/credential/assignment atomic create | Sixteen R15b actual-route cases, real statement triggers and wrapped begin/commit failures; full local validation passes, hosted acceptance pending |
| Duplicate URL, post-commit response failure, safe projections | R15b gated duplicate and committed-read failure plus retained source HTTP/legacy contracts |
| Meaningful lifecycle/transaction boundaries with existing scheduler/store | Accepted R15a typed queue/dispatch/HTTP/tracking seams; R15b single command and SQL/body proof |

A05 is unaccepted until R15b's final-head hosted/race/review gates pass. R16a is next after this batch; broader A01/A06/A08-A14 remain.
