# Autonomous refactoring continuation

User authorization on 2026-10-08 covers completing the remaining refactoring plan, editing code, running checks, committing/pushing branches, creating PRs, requesting automated reviews, posting/replying to PR comments, fixing findings and merging after checks pass and actionable findings are resolved. Preserve unrelated work and existing behavior; respect branch protections. Update the backlog after each batch and proceed to the next. Notify at milestones or when input is required.

The persistent goal is attached to this chat. Active heartbeat `continue-subshare-go-refactoring` wakes this chat every 30 minutes and stays quiet for unchanged/non-actionable state. Disable it only after the full backlog is completed or the user changes the instruction.

## Current handoff

- R06 is merged in PR #10 at `ebd427f`.
- R07a is merged in [PR #11](https://github.com/romanpodg/SubShare-Go/pull/11) at `1a7968b`; all six hosted checks passed at `33631dd` and final Codex review reported no major issues. CodeScene approved; its earlier inline findings were removed after the fixes. No gates or rules were suppressed.
- R07b is merged in [PR #12](https://github.com/romanpodg/SubShare-Go/pull/12) at `e9643e1`; all six hosted checks pass at `bb98e95`, final Codex review is clear and no inline threads remain. Full Windows regression passes (1,103 passed, five optional skips), along with hosted user/profile races. The two narrow historical latency scan false-positive fingerprints are documented; all scanning stays enabled.
- R08 is merged in [PR #13](https://github.com/romanpodg/SubShare-Go/pull/13) at `3e4c333`; all six checks pass at `9c5fc34`, final Codex review is clear and no inline threads remain. Both source files score 10.00; full Windows regression passes (1,107 passed, five optional skips).
- R08c is merged in [PR #14](https://github.com/romanpodg/SubShare-Go/pull/14) at `2d55c03`; all six checks pass at `e35e49e`, final Codex review is clear and no inline threads remain. The required assignment failure now rolls back the full unit, with storage and secret-free HTTP failure/no-audit/retry regressions. Full Windows regression passes (1,108 passed, five optional skips). A02 is accepted.
- R09 is implemented on `test/configuration-preservation-corpus`: 63 protocol/transport/security combinations and 388 new cases; all 516 frontend tests, types/lint/build and direct new-file CodeScene (10.00) pass. Publish its own test-only PR, request review and resolve hosted gates before merging. Records/evidence are in `r09-configuration-preservation.md` and `.cache/r09`.
- Check the actual PR head, all CI/check results and automated review before merging. Resolve review threads only after verified fixes and replies. No bypass or suppression of gates is authorized.
- R10a pure patch planning/ordered application is next, then R10b security phase extraction. Retain R09's no-op/whole-document/alias-clear corpus and both documented baseline quirks; use the existing token editor and parser. Reconcile broader A01 mutation boundaries against their acceptance criteria before declaring the whole backlog complete. Remaining scopes/dependencies are in `refactoring-batches.md`; unfinished audit items are in `refactoring-backlog.md`.
- The initial workspace changes were all R07a work and were preserved in its own commit. Recheck the working tree before further branch operations; do not overwrite new unrelated changes.

## Local validation constraints

GitHub CLI and test/local-network tools need `require_escalated` on this Windows host. Sandboxed `gh auth status` falsely appeared invalid; the escalated check succeeded using the existing keyring. The connected GitHub integration can read PR data, but lacks branch-protection administration access; the CLI confirmed main was unprotected and the ruleset listing was empty. Recheck before merging if configuration changes.

Use disposable test databases and local servers. Workspace Go temporaries are under `.cache/r07a/tmp`. Go 1.26.0, CodeScene and `.cache/bin/golangci-lint.exe` are available. Local race execution lacks a CGO compiler; hosted Linux CI owns race/lint/deployment checks. Never count optional skips or incomplete runs as passes.
