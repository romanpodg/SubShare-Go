# R23 — Shared public-page contracts

Status: characterization in progress from PR41 merge `5a88693`, on `test/public-page-contracts`.

Use common JSON fixtures in both Go and frontend tests before extracting defaults/normalization. Assert supported hero/steps/linkButtons/activation/text/footer semantics, localized defaults, explicit branding substitution, escaped text/attributes, valid relative/HTTP links and Happ activation URLs, unsafe URLs and custom/legacy theme keys. Exercise the actual frontend page and Go renderer rather than an independent reimplementation.

Existing renderer differences are compatibility boundaries: Go serves an existing subscription with inline delivery/copy script; React exposes an activation form and dark theme. Their default branding/text, missing-template behavior, theme alias precedence and URL handling are not assumed identical. Record observed differences explicitly; do not silently change admission, default text, activation or static deployment to create pixel equality.

After a saved tests-first baseline, extract a focused config/default boundary from the Go renderer and pure normalization from the React page. Retain CSS/script constants and HTTP/runtime behavior. Require focused/full Go and frontend checks, existing browser smoke, strict health/secret gates and all six hosted checks/five race steps/completed actionable review before acceptance. Broader A01 remains next; goal active, heartbeat paused per user.

## Executed characterization

Shared corpus executes against actual Go HTML and React page. Twenty-two Go parent/subtest events (including retained escaping regression) and twenty frontend cases pass; initial incorrect fixture import/CSS-variable test spelling were harness errors corrected before this evidence. TypeScript/eslint pass; both new test files score10. Config admission, all supported block/step/default/download types, text/attribute escaping, branding override, HTTP/relative links, Happ activation loading/success/failure/copy, custom/unsafe CSS properties and URL policies are protected. Existing URL/hover/unknown-template differences are explicit expected values, not alleged equivalence. Production code is unchanged.
