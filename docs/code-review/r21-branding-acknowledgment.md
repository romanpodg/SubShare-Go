# R21 — Branding persistence acknowledgment and editor reopening

Status: locally validated correction on `fix/branding-save-acknowledgment`, after tests-first `1f2e1a0`, based on accepted A11 merge `a0120f2`. The baseline's thirteen mounted cases produced ten desired failures/three compatibility passes; the correction and expanded retry/recovery/favicon/browser matrix pass. No R21 PR or A12 acceptance exists at this checkpoint.

Desired failures protect opening after backend load, cancelling/reopening, later acknowledged external data, denied/server/network/invalid-response saves, pending/double-save acknowledgment, late successful GET and late failed GET versus a saved update. Passing compatibility checks retain cold-cache public defaults, fallback cache with missing defaults, and sequential panel/page-title payload merging. Artifacts: `.cache/r21/complete-desired-before.json`; the original five-case discovery and landed baseline remain separate.

Correction policy: context/cache represent acknowledged backend data, with fallback cache retained only when public loading fails. Mutations return an awaitable outcome; failed saves remain visible in the editor and do not close it or commit an unsaved title/cache. Serialize or otherwise preserve sequential partial updates, and keep late initial loading from overwriting newer mutations. Each fresh modal opening begins from current agreed settings and discards cancelled draft data. Preserve wire mappings, dedicated subscription-page configuration, image handling/favicon behavior and public branding.

Use local React state/refs and the existing API boundary; no new state framework. Replace R18b's explicitly temporary optimistic-denied characterization with the corrected acknowledgment contract. Require full frontend/types/lint/export/browser/strict/secret and all six final-head hosted/race checks plus resolved actionable review before A12 acceptance. Broader A01/A12-A14 remain incomplete; goal active and heartbeat paused.

## Implemented behavior and local validation

Context/cache commit only the acknowledged PUT response. Promise-returning partial updates serialize through the existing context ref, so later edits merge with the latest acknowledged state and a failed request does not poison retries. A successful-mutation revision prevents late initial GET success/failure from replacing newer agreed data; failed concurrent edits do not suppress initial loading. Public GET defaults/fallback cache and existing wire mappings remain.

Each opened editor mounts from current agreed settings after initial loading; cancelling discards its local draft. Save awaits persistence before closing, retains draft/error on failure, guards repeated submission and closing while pending, and disables pending form controls. The old optimistic-denied characterization now asserts preserved agreed data. Favicon publication was separated into named DOM phases to resolve newly moved nesting while retaining normal/shortcut alias behavior.

Sixteen corrective state/modal cases, two favicon cases and three request-compatibility cases pass. Full frontend passes 4,205 cases in 50 files; TypeScript/full lint/static export and all 26 desktop/mobile Playwright cases pass (22 existing plus four new actual-branding cases). Context/modal/new suites/browser spec score 10.00; strict delta passes. Four mapping/cache bodies and defaults are identical under the TypeScript printer. No endpoint, backend handler or dedicated page-config behavior changed.

Artifacts remain under `.cache/r21`, including complete-desired-before.json, focused-final.json, full-final.json, build-final.log, browser-final.log, branding-browser-after.log, final-correction-delta.json and prove-mappings.mjs. The initial ten failing assertions are retained as desired-regression evidence. Finish secret scans, commit/push/open an independent main PR and require all six final-head checks/race plus resolved actionable automated review before A12 acceptance.

## A12 requirement evidence (acceptance pending)

| Requirement | Evidence |
| --- | --- |
| Failed/denied/network/invalid-response saves are not acknowledged | Four mounted failure cases retain editor/agreed title/cache and visible error |
| Retry and pending/repeat/close guard | Retry, pending/double-save cases and actual desktop/mobile denial/pending scenarios |
| Fresh opening/cancel/reopen/external data | Mounted initial-load, cancelled draft, acknowledged external-update cases and browser reopen |
| Late loading versus mutations | Successful/failed GET after acknowledgment and initial load after failed edit |
| Cache/default/server truth and partial merging | Cold-cache defaults/fallback and sequential/failed-queue partial-update tests |
| UI/default/mapping/favicon compatibility | Four unchanged bodies/defaults, favicon alias tests, full frontend and browser corpus |
| Final integration/review | Local strict/validation passes; final secret and six exact-head hosted/race/review gates remain |
