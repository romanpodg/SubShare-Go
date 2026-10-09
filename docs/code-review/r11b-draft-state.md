# R11b — Typed draft state and action completion

Status: locally validated on `refactor/editor-draft-state`, based on accepted request-session main `93d8488`; hosted acceptance remains. This continues A07 without claiming its entire matrix complete.

Characterization commit `53780c6` freezes nine existing reveal/update/clone/refresh/dirty-close contracts and separately reproduces three desired failures: duplicate legacy reveal requests, and old update/clone completion closing a newly opened profile. Correction `4d6ab09` guards mutation/clone notifications, conflict, close and busy completion with their session owner, while still refreshing committed mutations. Session transitions reset saving/cloning as well as loading/revealing/conflict; repeated pending reveal is withheld. All 53 mounted editor cases pass after correction.

The separate refactoring commit `c37571a` moves 40 draft fields and their reset into `useEditorDraft`. Its typed field writers retain functional update semantics and stable identity. Initialization/reset share one constructor, without new packages, endpoints or wire DTOs. All 40 initializers and reset values match the previous source exactly; `renderEditor` remains byte-identical. Explicit callback dependencies preserve stable load and projection behavior.

All 4,026 unit cases, nonincremental TypeScript, lint without new warnings, exported build and 20 desktop/mobile browser cases pass. The hook, lifecycle test file and expanded request-session hook score 10.00; strict delta against accepted main passes. Existing modal health remains 7.29 and its presentation findings stay visible. Evidence is in ignored `.cache/r11b-draft/`.

Publish/review, scan staged/history additions and require all six hosted checks before merge. Then reconcile A07's remaining pending mode/reveal/edit and native absent/clear credential matrix, correct any separately demonstrated defects and record requirement-level acceptance. The broader backlog and persistent goal remain unfinished.
