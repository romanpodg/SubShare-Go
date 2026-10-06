# R02 — SQLite startup data safety

Base: `main` at `267811b`, after PR #7 merged R01. [PR #8](https://github.com/romanpodg/SubShare-Go/pull/8) completes the next P0 batch, A00/R02, with source validated at **a896708**. User-mutation work (R03) remains a separate follow-up.

## Decision and changes

Fail closed when SQLite initialization returns an error. The server creates the database directory and invokes initialization once; it does not infer that an I/O error makes WAL files disposable. SQLite still owns crash recovery, checkpointing and the existing journal-mode fallback. Preserve the original initialization error for diagnostics and explicit later retries.

Remove `CleanupSQLiteSidecars` and `IsRecoverableSQLiteIO`, which have no remaining production consumers. Their destructive-behavior tests are replaced by preservation regressions; the original R01 evidence remains in the audit history. Extract journal-mode negotiation into a small private function, separating it from connection pragmas without changing fallback statements, ordering or errors. No migrations, schema, encryption format or frontend behavior change.

## Evidence

- Before the correction, the new failure regression failed in four I/O/checkpoint/close cases: the original recovery path deleted sidecars and called the initializer twice.
- A subprocess checkpoints the schema, disables automatic checkpoints, commits an administrator, user and encrypted profile, then exits without closing SQLite. Each injected initialization failure must return its original error, make one attempt and leave the database, WAL, SHM and keyring bytes unchanged. An explicit later startup must recover accounts, migrations and decryptable profile data without replacing the administrator.
- Injected cases cover generic disk I/O, SHM resize code 4874, checkpoint error, joined checkpoint/close errors, permission, lock, corruption and migration errors. These exercise errors returned by the initializer; they do not simulate actual kernel checkpoint/close/permission failures.
- A separate process holds a real write lock while startup attempts a required migration write. The startup fails with a lock diagnostic; database/WAL/keyring bytes survive, and a later startup succeeds after lock release. SHM reader bookkeeping may legitimately change in this real SQLite case.
- The storage crash fixture proves the row exists only in the WAL and remains recoverable through SQLite initialization and a subsequent reopen. Existing WAL/DELETE restart, directory failure, migration retry and bootstrap tests remain.

## Validation

Windows targeted startup/storage tests, journal-mode fallback/diagnostic tests and the uncached migration compatibility test pass. The full gotestsum/atomic-coverage run passes: **708 test results, five existing optional skips, no failures**. `go vet ./...`, `go build -o .cache/r02/server.exe ./cmd/server`, golangci-lint 2.13.2 with `--new-from-rev=origin/main`, JSON validation and `git diff --check` pass. The initial sandboxed full run failed on temporary-file renames and loopback restrictions; the unsandboxed rerun passed without weakening tests.

With user-authorized CodeScene analysis, `internal/storage/sqlite.go` improves from **9.92 to 10.00**, resolving its Bumpy Road finding. `cmd/server/main.go` remains **9.57**; its existing CLI findings are outside R02. The new failure and journal test files and storage crash test score **10.00**. The strict delta against `origin/main` passes. Local startup functions and journal-mode negotiation have **100% statement coverage**; overall local statements remain **52.4%**.

[Linux Actions run 37439646348](https://github.com/romanpodg/SubShare-Go/actions/runs/37439646348) passes backend/migration/vet/lint/build/secret scanning, frontend type/lint/Vitest/build/Playwright, and Docker deployment/restart checks for a896708. [CodeScene review 7827467](https://codescene.io/projects/84883/delta/results/7827467) passes all three gates and also reports `storage_test.go` improving from 8.37 to 8.91 after obsolete cleanup tests were removed. [Codecov](https://app.codecov.io/gh/romanpodg/SubShare-Go/pull/8) reports **85.71% patch coverage** and a passing bundle check with **0.0% size change**. All six PR checks pass for the validated source commit.

Local Docker is unavailable because its daemon is stopped; Linux CI supplies deployment evidence. Local Go has `CGO_ENABLED=0`, so race execution is unavailable here. Optional existing snapshot/client fixtures retain their normal skips. The tests do not establish production trigger frequency or fix underlying filesystem faults; they prevent application-side destructive recovery. See the [operator recovery guide](../sqlite-recovery.md).
