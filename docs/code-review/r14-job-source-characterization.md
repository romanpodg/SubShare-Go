# R14 — Job/source lifecycle, failure and overlap characterization

Status: locally validated on `test/job-source-lifecycle`, based on accepted A04 main `817107b`. Production jobs/source code remains unchanged; hosted/race R14 acceptance and A05 completion remain.

Initial cases pass for queued→running→terminal state/timestamps, conditional running transition, existing unconditional terminal overwrite, queue/start zero sentinels on rejected inserts, best-effort ignored transition/finish writes, zero-ID no-ops and idempotent interrupted-job/source recovery. Actual registered source queue returns 202/job ID and reaches a controlled fetch only after running/syncing writes; success retains imported encrypted profiles/run/audit, fetch 503 becomes failed job/run/source, and real queue insert failure withholds 202 and creates no run. Three new files score 10.00.

The existing source-client replacement seam avoids external network requests. Release channels and bounded polling observe the terminal write/audit boundary before database cleanup. Current statuses and error handling are characterization, not approval to silently change best-effort writes, retries or races.

The initial baseline is committed at `d51bed1`. Further cases characterize retry rejection for queued/running/succeeded jobs, unknown kinds and malformed/zero targets; deleted-source retry still returns 202 then fails asynchronously, while repeated retries create independent jobs and preserve one imported profile. Real triggers protect existing best-effort running/start-run/finish-job/finish-run/audit failure outcomes, including successful import with missing/stuck tracking or missing audit. All five files score 10.00 and the combined focused set passes against unchanged production.

The remaining focused matrix now passes: missing/read/queue/key-health retries; empty/unsupported/oversized/503 fetches, safe timeout/cancellation; source syncing/error write failures and independent restart-recovery failures; synchronized reverse-order fetches; URL/category/enable update and delete during fetch; rollback after parent/secret/assignment/source completion and injected begin/commit failures; HTTP shutdown returning before the detached worker, followed by its actual completion before fixture cleanup. All ten files score 10.00. The workflow adds a dedicated Linux race run for these cases because local CGO is unavailable. Full Go/coverage and final gates are running; no R14 PR or acceptance is claimed yet.

## Reproduced risks and correction boundary

The existing reconciliation mutex serializes writes after fetching; it does not bind a result to a fetch generation or source URL. Releasing the second request first and the first request last leaves the older payload as the final source profile. Changing the source URL while a request is blocked still applies the old payload to the current source. These two expectations explicitly characterize defects and must be replaced by protection assertions in a separate corrective change; they are not safety requirements to preserve forever. Category/enable changes intentionally use current target metadata, and deletion fails without recreating profiles.

Best-effort tracking/state/audit writes are recorded separately from required reconciliation writes. A rejected tracking write can leave a successful import with missing/stuck history or audit. Reconciliation itself retains the original encrypted envelope, profile and user assignments on statement/begin/commit failure. HTTP shutdown does not join detached jobs; restart recovery marks interrupted state. No durable queue, scheduler, schema or lifecycle policy is silently changed.

The first full run failed because the repeated-retry fixture assumed two uncontrolled simultaneous jobs both succeed. A bounded twenty-run diagnosis reproduced `SQLITE_BUSY` in three runs: tracking/status writes outside reconciliation can contend. The repeated-acceptance case now joins each terminal/audit boundary before the next request, independently proving repeated retry IDs and original failed-job eligibility; its twenty-run recheck passes. Concurrency remains covered by the separate gated reverse-order cases and the required race run. The failed full run is retained under `.cache/r14/uncontrolled-retry-before.*`; a normal full retry is running. No production retry/locking behavior or test timeout was changed. Contended tracking failure remains a recorded risk for the subsequent command/lifecycle work, not a concurrent-success guarantee.

| Requirement | Evidence |
| --- | --- |
| Actual 202, queue failure and queued/running/terminal | Lifecycle and source worker contracts |
| Retry eligibility/missing/deleted/repeated/key-health/read/queue | Retry contracts, with independent accepted jobs and legacy target text recorded |
| Fetch bounds/status/timeout/cancellation | Fetch contracts; safe failure messages omit URL tokens |
| Required/best-effort writes and recovery | Fault/recovery contracts and real SQLite triggers |
| Overlap and mid-fetch source mutations | Controlled request gates and exact retained encrypted-feed comparison |
| Profiles/secrets/assignments atomic rollback | Rollback contracts plus wrapped real SQLite begin/commit failures |
| Shutdown versus interrupted recovery | Actual HTTP server shutdown with blocked detached fetch, then completion; recovery idempotence/faults |

Require full local/hosted checks, including the new race step, and actionable review resolution before R14 acceptance. Then implement a separate reviewed stale-result correction before R15 extraction, preserving current category/enable target projection and transactional reconciliation. A05 and the overall goal remain incomplete.

Final normal full retry exits 0: 1,403 Go cases pass / five unchanged optional skips, no failures, atomic coverage (server 68.0%). Vet/build/lint, actionlint, strict delta and secret checks pass; all ten new test files score 10.00. The earlier invalid concurrent-success expectation and its failed run remain separately recorded. Race execution remains a required hosted Linux gate; local CGO is unavailable. Publish this tests/workflow-only batch and require all six final-head checks and actionable review resolution before acceptance.
