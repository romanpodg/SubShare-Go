# R23 — Shared public-page contracts

Status: accepted in PR42 at head `de6baf1571a0f7d9cf30a0a6af473ea4d6d742e9`, merge `0dfdba1010a27f5a60a7b807f96e197fb6222fed`. Tests-first `f5c8601` precedes extraction; earlier checkpoints below are historical.

Use common JSON fixtures in both Go and frontend tests before extracting defaults/normalization. Assert supported hero/steps/linkButtons/activation/text/footer semantics, localized defaults, explicit branding substitution, escaped text/attributes, valid relative/HTTP links and Happ activation URLs, unsafe URLs and custom/legacy theme keys. Exercise the actual frontend page and Go renderer rather than an independent reimplementation.

Existing renderer differences are compatibility boundaries: Go serves an existing subscription with inline delivery/copy script; React exposes an activation form and dark theme. Their default branding/text, missing-template behavior, theme alias precedence and URL handling are not assumed identical. Record observed differences explicitly; do not silently change admission, default text, activation or static deployment to create pixel equality.

After a saved tests-first baseline, extract a focused config/default boundary from the Go renderer and pure normalization from the React page. Retain CSS/script constants and HTTP/runtime behavior. Require focused/full Go and frontend checks, existing browser smoke, strict health/secret gates and all six hosted checks/five race steps/completed actionable review before acceptance. Broader A01 remains next; goal active, heartbeat paused per user.

## Executed characterization

Shared corpus executes against actual Go HTML and React page. Twenty-two Go parent/subtest events (including retained escaping regression) and twenty frontend cases pass; initial incorrect fixture import/CSS-variable test spelling were harness errors corrected before this evidence. TypeScript/eslint pass; both new test files score10. Config admission, all supported block/step/default/download types, text/attribute escaping, branding override, HTTP/relative links, Happ activation loading/success/failure/copy, custom/unsafe CSS properties and URL policies are protected. Existing URL/hover/unknown-template differences are explicit expected values, not alleged equivalence. Production code is unchanged.

## Implemented normalization boundary

Tests-first f5c8601 precedes extraction. Go subscription_page_config.go owns the unchanged default JSON, admission, template variables and a fixed ordered theme argument list. Thirty-three fallback values formerly repeated in the renderer now come from its existing canonical default JSON through a private read-only map. The explicit parity probe checks all33 original ordered keys and fallback bytes; no theme alias precedence changes. Twenty-four original Go functions/constants remain AST-identical, excluding this deduplicated assembly. Renderer8.09→8.34; new config boundary10.

React pageNormalization.ts owns pure config/default/activation hydration/template normalization. Eight original function bodies remain AST-identical; only default activation lookup is flattened with the same first-match order and fallback. The actual page continues to own fetch/effects/activation/copy and dark-theme assembly. Page9.53→9.68; new normalization10; tests10. Existing security/URL/layout differences remain characterized. CSS/script template and endpoint behavior are unchanged; no new dependency, Next runtime service, schema or coverage-upload change.

Full frontend validation exits zero: 4,225 tests/51files, TypeScript, eslint, static build and26 browser tests including desktop/mobile public-page accessibility/responsiveness. Backend vet/lint(0issues)/build pass, strict delta empty/pass. Full uncached Go exec41544 is still running; do not claim full pass or publish acceptance until observed. Final secret and hosted six/five-race/completed-clear-review gates remain.

## Final local validation / A14 requirement evidence (hosted acceptance pending)

Full Go exec41544 exits zero: 4,570 passes/five unchanged optional skips in528.387s; server statement coverage76.4%. Go and frontend production code stayed unchanged throughout final full runs. Final static/lint/build, TypeScript/eslint/static frontend build,4,225 frontend tests and26 browser smoke cases pass. Staged diff/secret and strict gates pass. Hosted six gates/five Linux races and completed clear exact-head review remain mandatory before normal merge; no local CGO race or external optional-validator pass is claimed.

| A14 requirement | Evidence |
| --- | --- |
| Common config/block/default contract | Same JSON corpus drives22 Go events and20 actual React-page tests; all six supported block/step forms and seven default download IDs protected |
| Supported renderer semantics | Shared titles/Unicode/branding/text/links/footer; explicit existing delivery-versus-activation, dark-theme, URL-filtering, alias-precedence and missing-template differences |
| Escaping and safe boundaries | Go text/attributes/inline JSON/CSS/raster/URL regression plus shared URL/theme cases; React text cannot create script markup and built-in JavaScript URL blocking retained |
| Activation and copying | Pending request disables button, code sent intact, success/failure text preserved, Happ import scheme/manual link/clipboard/custom and hydrated default labels protected |
| Focused normalization/default extraction | Go config/default/template boundary and canonical33 theme fallbacks; pure TS normalization;24 Go declarations/eight TS bodies AST-identical with two explicit equivalent structural changes |
| Localized/default/static deployment behavior | Existing language/brand/layout/download semantics captured; static Next build and desktop/mobile public-page browser smoke pass; Go CSS/script and HTTP bodies identical |
| Quality and integration | New modules/tests10; renderer8.34/page9.68; full local gates pass, final hosted/review pending |

A14 remains unaccepted until final hosted gates/review. Broader A01 remains; no goal completion claim.

## Final acceptance

[PR42](https://github.com/romanpodg/SubShare-Go/pull/42) merged at `0dfdba1010a27f5a60a7b807f96e197fb6222fed` after all six gates at `de6baf1571a0f7d9cf30a0a6af473ea4d6d742e9`. [Run38290705750](https://github.com/romanpodg/SubShare-Go/actions/runs/38290705750) confirms all five Linux race steps, authentic populated Docker upgrade/matched restore and retained clean-stack/restart smoke. Exact-head automated review completed with positive reaction and no findings/threads; normal expected-head merge respected protections. The requirement matrix above is accepted; earlier pending checkpoints are historical. A14 is complete. Broader A01 remains, with R24a/R24b closing its remaining boundaries; goal active, heartbeat paused per user.
