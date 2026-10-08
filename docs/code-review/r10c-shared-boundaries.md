# R10c — Shared Xray fixtures and size-boundary assurance

Status: accepted in [PR #18](https://github.com/romanpodg/SubShare-Go/pull/18), merged at `087e873`. This closes two explicit A03 prerequisites after R09/R10a/R10b. Production parsers, patch planning, token editing and limits are unchanged.

The single `testdata/configuration/xray-shared.json` corpus is consumed by both Go and TypeScript. Its 63 cases cover VLESS/VMess/Trojan, seven supported networks and none/TLS/Reality. Each has an unsupported first outbound and an extra supported outbound. Independent expected values cover protocol, server, port, identifier, network, security and SNI. Go checks both first draft and connection target; the editor checks its selected outbound and draft. Go deliberately expands both supported outbounds while the editor selects the first. Agreement on these supported fields does not claim identical acceptance or conversion semantics.

Six frontend cases exercise 65,534/65,535/65,536 UTF-8 bytes, including literal Unicode, an escaped Unicode token and an integer beyond JavaScript precision. Exactly sized formatted documents freeze the inclusive formatting threshold. Compact documents freeze the original-text fallback when formatting expands past the limit. Same-width connection edits must retain every other token exactly, and repeating the edit must be byte-idempotent. The patcher itself continues its existing formatting behavior; these tests introduce no new rejection policy.

All 69 new frontend cases and 3,989 full tests pass, with nonincremental TypeScript, lint and exported build. All 63 shared Go cases pass uncached. Both new test files score 10.00 after separating fixture loading, draft assertions and target assertions; all original assertions remain. Full Go suite, vet, build and strict CodeScene delta against accepted R10b main pass. Existing optional fixture/race limitations remain unchanged. Final secret scans and hosted checks/review must finish before acceptance. Evidence remains in ignored `.cache/r10c/`.

After R10b and this supplement have hosted acceptance, reconcile A03's completion criteria against their combined evidence, record acceptance and proceed to R11a. Keep the unrelated parser/default-builder complexity visible and retain all existing corpus cases.

## Hosted acceptance and A03 reconciliation

All six checks pass at `ea0273625325e122860660dd8b6926ea26bf7adb` in [Actions run 37790369799](https://github.com/romanpodg/SubShare-Go/actions/runs/37790369799). Automated review reports no major issues and no inline threads remain. Merge `087e873b1a45e1eb8d0461cb1b8efaf90df5239f` uses the normal GitHub merge path.

| A03 completion criterion | Accepted evidence |
| --- | --- |
| Corpus and preservation invariants unchanged | All 3,792 R09 goldens remain and pass; six UTF-8/byte-idempotence cases supplement them |
| Patch CC/bumps/depth visibly reduced | R10a replaces CC 133 / ten bumps / depth four with named planning/application phases below the complex-method threshold; R10b isolates security phases at 10.00 |
| No new parser, generated semantics or format behavior | Existing token editor/parser/public API remain; R10a verifies 37 unrelated functions and R10b verifies 60 plus three moved helper bodies |
| Shared supported fixture agreement | Identical 63-case JSON consumed by Go and TypeScript; common-field agreement and intentional selection/expansion differences are explicit |
| Frontend validation passes | 3,989 full tests, types/lint/export and hosted frontend checks pass |

A03 is accepted. Core health improves from the audit's 2.36 to 3.58. Remaining parser/default-builder/legacy complexity is recorded, not claimed removed. Continue to R11a/R11b without redefining the unfinished broader backlog.
