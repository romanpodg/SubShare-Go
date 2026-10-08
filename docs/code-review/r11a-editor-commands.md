# R11a — Pure editor profile commands

Status: implemented in [PR #19](https://github.com/romanpodg/SubShare-Go/pull/19); initial functional checks/review pass, quality correction locally validated; final hosted acceptance pending. Characterization commit `7705487` precedes production changes on `refactor/editor-profile-commands`, based on accepted R10c main.

Sixteen new mounted-modal contracts freeze native create defaults and metadata trimming, untouched/hidden credential omission, explicit empty credential sets, optional SNI/plugin clears, unrevealed raw-save errors and the exact source-owned metadata payload. All 16 existing modal regressions remain. The baseline preserves empty native-create port input, untrimmed structured display-name input and omitted source template text; these are characterized without adding corrective behavior.

Pure create and update builders consume an explicit form snapshot and existing safe profile detail. Named protocol patch phases reuse the original patch helpers. Raw validation and legacy creation share one internal policy implementation. Source-owned updates return before configuration planning. Legacy, Xray JSON and native raw/structured content decisions retain their original precedence, errors, revision, tri-state values and optional wire fields.

The modal retains requests, toast/conflict/saving behavior, refresh/close order and all draft/reveal state. The initial source comparison verified 70 other component statements unchanged. The quality follow-up also extracts detail and raw-display projection; its final comparison verifies 68 other statements, including all JSX, plus the moved legacy constructor body, unchanged. No state library, endpoint, DTO, parser or lifecycle policy was introduced.

All 4,005 frontend tests, nonincremental TypeScript, lint, exported build and 20 desktop/mobile Playwright cases pass. All four new executable planning modules and the new contract test score 10.00; the type-only snapshot file has no executable score. Strict local delta exits 1: submit's former CC 117 / 192 lines / six bumps is removed, but the analyzer now reports the remaining component as CC 152 / 828 lines / five bumps / depth four, changing file health from 6.20 to 5.62. This remaining component finding is visible and unresolved; no gate or threshold is changed. Hosted CodeScene must pass before merge, or the finding must be addressed in a further reviewed change. Local evidence is retained in ignored `.cache/r11a/`.

Publish separately, resolve actionable automated findings and require all hosted checks. R11b remains the next distinct draft/session/reveal-lifetime batch. A03 is accepted through R09/R10a/R10b/R10c; the broader A01 and A04-A14 backlog remains.

## Quality follow-up

Hosted CodeScene rejected initial head `b84cd3b` for hotspot decline and newly reported depth four. Its automated functional review found no major issues. Pure detail-to-form projection now has named connection/legacy/native phases, retaining conditional application on reload; missing sub-projections leave existing fields unchanged. One shared initial-value implementation serves both detail projection and update planning. The unchanged legacy constructor moved alongside these defaults.

Raw reveal display formatting is now a pure projection using the existing duplicate-aware formatter, warnings and size fallback. Authoritative raw storage, reveal payloads, errors, revision and request timing remain in the modal. This extraction removes the reported deep nesting; it adds no close/reopen or request-generation policy. R11b remains separate.

All six executable planning/default modules score 10.00. The modal improves to 6.30, with three bumps and no depth-four finding; component CC 126 / 804 lines remains visible. Strict delta still reports its aggregate complex-method warning, while the old submit findings are removed. Final functional and hosted checks, review, replies and thread resolution are required before merge. No gate, rule or threshold is changed.

Final local source state passes all 4,005 tests, types/lint/export and all 20 browser cases. Staged/history scans precede the follow-up push.

## Final-source review and queued check checkpoint

Automated functional review at `2562dbed33ffb7ec360f268331a2d2338c6a807b` reports no major issues. Backend, frontend, Docker and both Codecov checks pass in Actions run `37794276181`; CodeScene check `113369486969` remains queued without diagnostics after service interruptions. A direct check re-request returned HTTP 404. A documentation-only checkpoint triggers a fresh normal head check; production source remains identical to the validated/reviewed source. Require the new final head's full checks and review disposition before merging. The Complex Method thread remains open pending analysis; the removed deep-nesting finding is resolved.

The separate request-session correction in [PR #20](https://github.com/romanpodg/SubShare-Go/pull/20) is stacked on this branch. Its CodeScene analysis `7867309` passes all three gates at source `5db7328`, and its initial finding was removed by the service. Retarget that PR to main only after this PR is accepted, then rerun main's full required gates. Neither stacked work nor service interruption waives a merge gate.

## Explicit rendering boundary

Fresh hosted analysis at documentation head `0e6301e` still rejects the aggregate Complex Method category, despite file health improving to 6.30. The final correction names the existing JSX presentation as `renderEditor` inside the modal, leaving all state/hooks, event handlers and rendered JSX byte-identical. This makes command/state orchestration and presentation distinct phases without new props, components or public API.

Code Health improves to 7.29. The remaining presentation method is explicitly measured at CC 49 / 452 lines, compared with the original submit's CC 117 / 192 lines. Complex Method pressure and overall complexity improve; mean CC falls 16.20 to 10.44, the old submit bumps and deep nesting are removed. Strict delta retains the visible existing-file presentation-method warning; no gate or rule changes. Final local/browser and hosted checks/review must pass before merge. The presentation length is retained as remaining debt, not hidden or claimed eliminated.

The final renderer boundary passes all 4,005 unit cases, types/lint/export and 20 desktop/mobile browser cases. Source parity verifies 67 unrelated component statements plus byte-identical rendered JSX and the moved constructor body.
