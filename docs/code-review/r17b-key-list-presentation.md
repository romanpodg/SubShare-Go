# R17b — Key-list card, aligned rows and category presentation

Status: accepted in [PR33](https://github.com/romanpodg/SubShare-Go/pull/33), merge `52d99e90818f88c08a49f490616aab19589a96ea`, after tests-first parity `e725609`. All six final-head checks pass at `37157f5aeebdf0e26ee54a257626f6ef2223cf6d` in [run 38138897545](https://github.com/romanpodg/SubShare-Go/actions/runs/38138897545). Automated review completed with a positive reaction and no review threads. A08 is accepted against the matrix below.

The matrix covers both subscription formats, real/informational kinds, active/non-active status and local/source profiles; effective/fallback client names, safe literal labels/source fallback, health/action availability, one shared subscription position across opposite-column gaps, and pane hiding/restoration. Accepted R17a action/selection/category/creation/drag fixtures remain the action authority; desktop/mobile geometry and normal/reduced-motion autoscroll remain in browser tests.

`KeyListCard`, `KeyListRows` and `KeyListCategoryBlock` now share explicit typed data/action contexts from the accepted controller. Private card/category headers and named render sections reduce renderer complexity without adding DOM wrappers. Pure alignment/client-name helpers move to `key-list-presentation.ts`; Main reexports their public declarations. Category component keys match the prior root keys, and ref registration, complete order positions, request targets and selection/confirmation ownership remain unchanged.

## Local validation

- All 53 focused cases pass: 19 new parity cases, 29 accepted mounted action/placement cases, four public helper cases and one health case. All 4,099 Vitest cases in 41 files pass; TypeScript, full ESLint and static export pass.
- All 22 desktop/mobile Playwright cases pass, including geometry and normal/reduced-motion edge autoscroll.
- Four non-event markup trees (card, rows, category and Main root) are identical after expanding private headers/named render calls. Both public projection function bodies are identical. Event behavior is protected by the mounted and browser contracts.
- KeysSection Code Health improves from 8.04 to 9.68. New presentation components, pure projections and parity tests score 10.00; the declaration-only context file is unscored. The pre-existing five-argument public drag helper finding remains visible. Final strict delta and staged whitespace checks pass; no rule suppressions were added.
- Initial render-call replacement produced invalid JSX and ran no tests. Those failed artifacts remain under `.cache/r17b`; the repaired candidate passed the final checks above. Backend production is unchanged; hosted backend/race checks remain required.

Local artifacts: `.cache/r17b/focused-repaired-after.json`, `full-after.json`, `full-after.log`, `lint-repaired-after.log`, `build-after.log`, `browser-after.log`, `presentation-delta.json`, `prove-markup.mjs` and `prove-projections.mjs`. Final staged/history secret scans pass. Hosted CodeScene reports 8.05 to 9.69, distinct from local 8.04 to 9.68; bundle increase is 1.71 kB (0.08%) within its configured gate. All four hosted race steps, frontend browser checks and Docker persistence validation pass. Copilot quota failure is not counted as review approval; the configured automated review completed cleanly at the exact final head.

## Accepted A08 requirement evidence

| Requirement | Evidence |
| --- | --- |
| Full-ID order, filtered view, unsaved refetch and failed-save retry | Accepted R17a action fixtures retain exact complete request payloads and local order |
| Selection/filter/deletion, cancellation/failure/partial count and repeat guard | Accepted mounted action cases, including permanent pruning when removed IDs reappear |
| Category order/moves/uncategorized and new-key placement | Accepted category/error/payload and five placement cases, including optional post-create failures |
| Health errors and drag end/unmount cleanup | Accepted mounted action/health cases verify failure state and matched listener removal |
| Card/row parity and action sharing | Nineteen tests-first matrix cases, public projection/body proof, typed contexts and unchanged four markup trees |
| Layout, near-edge scrolling and reduced motion | All 22 desktop/mobile browser cases pass locally |
| Quality and final integration | Local strict delta and all six final-head hosted checks pass; exact-head automated review completed with a positive reaction and no threads |

Broader A01/A09-A14 and the whole goal remain incomplete. Next is R18a shared safe API contract fixtures. Heartbeat stays paused per user; foreground goal stays active.
