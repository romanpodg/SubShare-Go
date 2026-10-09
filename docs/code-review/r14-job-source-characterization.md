# R14 — Job/source lifecycle, failure and overlap characterization

Status: initial focused characterization on `test/job-source-lifecycle`, based on accepted A04 main `817107b`. Production jobs/source code remains unchanged; R14/A05 are not complete.

Initial cases pass for queued→running→terminal state/timestamps, conditional running transition, existing unconditional terminal overwrite, queue/start zero sentinels on rejected inserts, best-effort ignored transition/finish writes, zero-ID no-ops and idempotent interrupted-job/source recovery. Actual registered source queue returns 202/job ID and reaches a controlled fetch only after running/syncing writes; success retains imported encrypted profiles/run/audit, fetch 503 becomes failed job/run/source, and real queue insert failure withholds 202 and creates no run. Three new files score 10.00.

The existing source-client replacement seam avoids external network requests. Release channels and bounded polling observe the terminal write/audit boundary before database cleanup. Current statuses and error handling are characterization, not approval to silently change best-effort writes, retries or races.

Remaining before R14 acceptance: retry eligibility/deleted/repeated targets, fetch size/status/timeout/cancellation, start/run/status/audit DB failure outcomes, synchronized reverse-order same-source fetches, URL/category/enable changes and delete during fetch, assignment/secret rollback, restart failure classifications and tracked-completion versus interrupted-recovery shutdown evidence. Complete those cases and full/hosted checks/review before R15 extraction. Any corrected semantic defect must remain separately identified and reviewed.
