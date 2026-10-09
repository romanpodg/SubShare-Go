# R11b — Credential provenance and revision-owned reveal correction

Status: locally validated on `fix/editor-reveal-draft-preservation`, based on accepted typed-draft main `1d73705`; hosted acceptance remains. Corrective behavior is explicitly distinguished from the earlier draft refactoring.

Regression commit `511568a` first reproduces thirteen reachable failures: pending reveals overwrite connection/credential edits, read-only reveal values become credential patches during metadata saves, unknown plugin options cannot be explicitly cleared, and revision refresh retains stale revealed data. Four further cases reproduce old-revision completion after a newer read, and an edit-load failure reproduces an unintended create request. Tests precede correction commit `aa6dc7f`; raw failing and corrected results remain ignored under `.cache/r11b-reveal/`.

The typed draft now distinguishes user-entered native secrets from revealed display values. Commands include only explicitly edited secret fields. Explicit empty credential sets retain their previous shape; unknown plugin-option clears use an unknown initial baseline instead of assuming empty. Native reveal cannot replace a touched field. Legacy edits recorded before or during reveal overlay the returned configuration through the existing loss-preserving patcher; original authoritative bytes remain intact.

Loading a new profile revision invalidates read-only credentials/raw baselines while retaining explicit native or raw user edits. Reveal leases bind to open profile, session generation and loaded revision; atomic draft updates also check revision ownership. Load and committed-mutation callbacks retain session ownership so loading does not cancel itself and committed changes still refresh the list. Missing edit detail is rejected before either create or update through the pure save-intent builder.

All 22 new cases and all 4,048 full frontend cases pass, together with nonincremental TypeScript, lint without new warnings, exported build and 20 desktop/mobile browser cases. New transition/save/test files and changed draft/session hooks score 10.00. Strict delta against accepted main passes; rendering is byte-identical. No endpoint, wire DTO, parser, state package, rule or threshold changed.

## A07 requirement-level evidence

| Requirement | Evidence across accepted R11a/R11b and this correction |
| --- | --- |
| Payload/raw-byte parity and reduced submit complexity | Original 16 regressions, 16 command goldens, R09 corpus; named pure builders; forty initializers/reset values and JSX unchanged |
| Reveal failure, 409 and fresh revision | Lifecycle failure/retry/conflict cases plus four stale-cache and four pending-old-revision cases |
| Submit validation and server field errors | Raw malformed/duplicate rejection, unrevealed/divergent drafts, server 422/network/409 and missing-detail no-write cases |
| Clone/network and post-commit refresh failure | Lifecycle cases retain current-session behavior; closed-session mutation/clone completion cannot close a new profile and still refreshes committed changes |
| Key/mode changes and late request isolation | Nine same/different reopen/direct-switch cases; connection edits survive reveal with and without a raw-mode switch; loading hides editor controls |
| Source metadata and native absent/empty/clear | Exact source payload cases; native untouched/empty password, SNI/plugin-name, plugin-option, TUIC UUID and Hysteria obfuscation clear cases; metadata after reveal sends no credential patch |
| Dirty cancel/confirm and repeated actions | Mounted lifecycle dialog, repeated submit and repeated pending reveal cases |
| Explicit draft/session lifetime and validation | Typed forty-field draft/reset hook, session/profile leases and secret provenance; full frontend/browser/strict checks pass |

Require all six final-head hosted checks and clear actionable automated review before accepting this correction and A07. Presentation length remains visible (the existing renderer is not claimed decomposed). Then continue R12/R13 response policy and the unchanged broader backlog; the persistent goal is not complete.

## Retained raw edit review correction

PR22 review identified that a revision refresh retains explicit legacy raw edits but clears their reveal baseline and returns to structured mode. A subsequent Save could omit those retained bytes and close the editor. Regression commit `a1741e8` reproduces this for VLESS, VMess and Trojan before the correction.

The legacy update policy now rejects that Save until the current revision is revealed. The entered raw bytes remain intact; after a fresh reveal, the same tests verify submission of those exact bytes with revision 9. Untouched metadata updates retain their structured payload. The policy is isolated in a pure legacy-content module, keeping both changed production modules and the mounted regression file at Code Health 10.00. All 4,051 unit tests pass; final mounted command/reveal tests, nonincremental TypeScript, lint, production export and strict delta pass. The previous 20 browser cases remain the presentation baseline; hosted browser checks will run on the final head. No gate, threshold or unrelated work changed. Recheck final-head CI and automated review before acceptance.
