# Autonomous refactoring continuation

User authorization on 2026-10-08 covers completing the remaining refactoring plan, editing code, running checks, committing/pushing branches, creating PRs, requesting automated reviews, posting/replying to PR comments, fixing findings and merging after checks pass and actionable findings are resolved. Preserve unrelated work and existing behavior; respect branch protections. Update the backlog after each batch and proceed to the next. Notify at milestones or when input is required.

The persistent goal is attached to this chat. Active heartbeat `continue-subshare-go-refactoring` wakes this chat every 30 minutes and stays quiet for unchanged/non-actionable state. Disable it only after the full backlog is completed or the user changes the instruction.

## Current handoff

- R06 is merged in PR #10 at `ebd427f`.
- R07a is merged in [PR #11](https://github.com/romanpodg/SubShare-Go/pull/11) at `1a7968b`; all six hosted checks passed at `33631dd` and final Codex review reported no major issues. CodeScene approved; its earlier inline findings were removed after the fixes. No gates or rules were suppressed.
- R07b is in [PR #12](https://github.com/romanpodg/SubShare-Go/pull/12), branch `refactor/profile-safe-reads`; pre-extraction characterization is `930e107`. Full local tests, static/migration/strict CodeScene checks pass. Hosted tests and user/profile contention races pass, but Gitleaks flagged the ordinary nullable-integer latency assignment in `1e3b6c3`. The exact historical fingerprint is documented in its execution record and narrowly excluded; recheck all hosted gates/review at the correction head before merge.
- R08 is implemented on `refactor/profile-update-policy`; passing update request/precedence characterization is `e880b15`. Pure local planning, named source-ownership checks and structured patch phases score 10.00 in both source files; service contracts, vet, build, lint and strict delta pass. The full suite is running under `.cache/r08/go-tests.json`. Publish its own PR after incorporating R07b acceptance/history; do not conflate the two batches.
- Check the actual PR head, all CI/check results and automated review before merging. Resolve review threads only after verified fixes and replies. No bypass or suppression of gates is authorized.
- R08 patch/ownership phases are next after R07b. Remaining scopes/dependencies are in `refactoring-batches.md`; unfinished audit items are in `refactoring-backlog.md`.
- The initial workspace changes were all R07a work and were preserved in its own commit. Recheck the working tree before further branch operations; do not overwrite new unrelated changes.

## Local validation constraints

GitHub CLI and test/local-network tools need `require_escalated` on this Windows host. Sandboxed `gh auth status` falsely appeared invalid; the escalated check succeeded using the existing keyring. The connected GitHub integration can read PR data, but lacks branch-protection administration access; the CLI confirmed main was unprotected and the ruleset listing was empty. Recheck before merging if configuration changes.

Use disposable test databases and local servers. Workspace Go temporaries are under `.cache/r07a/tmp`. Go 1.26.0, CodeScene and `.cache/bin/golangci-lint.exe` are available. Local race execution lacks a CGO compiler; hosted Linux CI owns race/lint/deployment checks. Never count optional skips or incomplete runs as passes.
