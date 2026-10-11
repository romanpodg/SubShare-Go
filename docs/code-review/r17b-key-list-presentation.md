# R17b — Key-list card, aligned rows and category presentation

Status: locally validated presentation extraction on `refactor/key-list-presentation`, based on accepted R17a merge `516e977` and tests-first parity commit `e725609`. Hosted checks and final automated review remain before R17b/A08 acceptance. No R17b PR exists at this checkpoint.

The matrix covers both subscription formats, real/informational kinds, active/non-active status and local/source profiles; effective/fallback client names, safe literal labels/source fallback, health/action availability, one shared subscription position across opposite-column gaps, and pane hiding/restoration. Accepted R17a action/selection/category/creation/drag fixtures remain the action authority; desktop/mobile geometry and normal/reduced-motion autoscroll remain in browser tests.

`KeyListCard`, `KeyListRows` and `KeyListCategoryBlock` now share explicit typed data/action contexts from the accepted controller. Private card/category headers and named render sections reduce renderer complexity without adding DOM wrappers. Pure alignment/client-name helpers move to `key-list-presentation.ts`; Main reexports their public declarations. Category component keys match the prior root keys, and ref registration, complete order positions, request targets and selection/confirmation ownership remain unchanged.

## Local validation

- All 53 focused cases pass: 19 new parity cases, 29 accepted mounted action/placement cases, four public helper cases and one health case. All 4,099 Vitest cases in 41 files pass; TypeScript, full ESLint and static export pass.
- All 22 desktop/mobile Playwright cases pass, including geometry and normal/reduced-motion edge autoscroll.
- Four non-event markup trees (card, rows, category and Main root) are identical after expanding private headers/named render calls. Both public projection function bodies are identical. Event behavior is protected by the mounted and browser contracts.
- KeysSection Code Health improves from 8.04 to 9.68. New presentation components, pure projections and parity tests score 10.00; the declaration-only context file is unscored. The pre-existing five-argument public drag helper finding remains visible. Final strict delta and staged whitespace checks pass; no rule suppressions were added.
- Initial render-call replacement produced invalid JSX and ran no tests. Those failed artifacts remain under `.cache/r17b`; the repaired candidate passed the final checks above. Backend production is unchanged; hosted backend/race checks remain required.

Local artifacts: `.cache/r17b/focused-repaired-after.json`, `full-after.json`, `full-after.log`, `lint-repaired-after.log`, `build-after.log`, `browser-after.log`, `presentation-delta.json`, `prove-markup.mjs` and `prove-projections.mjs`. Final staged/history secret scans precede publication.

## A08 requirement evidence (acceptance pending)

| Requirement | Evidence |
| --- | --- |
| Full-ID order, filtered view, unsaved refetch and failed-save retry | Accepted R17a action fixtures retain exact complete request payloads and local order |
| Selection/filter/deletion, cancellation/failure/partial count and repeat guard | Accepted mounted action cases, including permanent pruning when removed IDs reappear |
| Category order/moves/uncategorized and new-key placement | Accepted category/error/payload and five placement cases, including optional post-create failures |
| Health errors and drag end/unmount cleanup | Accepted mounted action/health cases verify failure state and matched listener removal |
| Card/row parity and action sharing | Nineteen tests-first matrix cases, public projection/body proof, typed contexts and unchanged four markup trees |
| Layout, near-edge scrolling and reduced motion | All 22 desktop/mobile browser cases pass locally |
| Quality and final integration | Local strict delta passes; all six final-head hosted checks and resolved actionable automated review remain required |

Broader A01/A09-A14 and the whole goal remain incomplete. After R17b acceptance, record the final head/merge/run and accept A08 against this matrix, then continue R18a. Heartbeat stays paused per user; foreground goal stays active.
