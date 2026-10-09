# R13b — Response-rule and template HTTP/preview boundaries

Status: locally validated on `refactor/response-policy-http`, incorporating accepted R13a main `aff9073` through a normal fast-forward. Characterization commit `a086689` precedes production separation; hosted/A04 acceptance remains.

Four new contract files use the actual registered routes. They protect template/rule create-update-list-delete success envelopes, scalar normalization, stable slugs, null slices, complete v1 error envelopes/request IDs, repeated corrupt-row GET failures and audit side effects, SQL list failure mapping with sessions still valid, five role states across nine endpoints, independent role expectations and CSRF precedence. Previews retain all five format content types and their actual sample output, malformed/unknown-field/invalid-input/render-error responses.

All three public delivery URLs retain block/not-found precedence and explicit browser dispatch while subbody keeps its forced body format. Stored legacy custom headers cannot override canonical metadata, safe root headers still apply, optional browser Happ crypto failure keeps the plain link, and deleting a referenced template uses the unchanged foreign-key SET NULL behavior and default delivery. Existing all-excluded 422, empty/integrity 503 and HTML/CSS/JSON/URL escaping suites remain required. No runtime data or foreign-key constraint was bypassed. The initial four files score 10.00; targeted registered-contract and existing delivery/escaping checks pass before extraction.

Continue by separating template HTTP, rule HTTP and the pure preview renderer without changing their bodies or route registrations. Require full local/hosted validation and actionable review resolution before requirement-level A04 acceptance. R14 queue/source lifecycle follows after acceptance; broader A01 and A05-A06/A08-A14 remain incomplete.

Template HTTP, rule HTTP and pure preview rendering now have three focused modules. All 17 original function bodies are byte-identical across the move; route registration is unchanged. All new modules, remaining core and four contract files score 10.00. Post-separation targeted administrative/public/store/format/escaping tests pass; vet/build/lint pass. Full uncached Go/atomic coverage and final strict/secret checks remain. No command, envelope, role, SQL, schema, formatter, template application, metadata, metric, fallback or third-party dependency changed.

## A04 requirement-level evidence

| Requirement | Evidence |
| --- | --- |
| Priority/tie, AND/OR/empty, case/absence/regex and normalization | R12 tests-first condition/selection/template corpus; historical regex folding and aliasing explicitly retained |
| Actual create/update/delete/list/preview endpoints and permissions | R13b four registered-route files; exact success/error envelopes, five role states across nine endpoints, CSRF and all five previews |
| Corrupt conditions/headers JSON, disable/audit/error and repeats | R13a single-connection/repeated/best-effort fixtures plus R13b repeated corrupt GET with no leaked SQL details |
| SQL reads/writes and rollback | R13a real SQLite rejecting triggers, retained rows/no success audits; R13b list-table failure with sessions still valid; actual autocommit/absence of an explicit commit seam recorded |
| Missing/disabled/mismatched/deleted templates | R12 compatibility cases, R13a load projections and R13b actual referenced delete/SET NULL/default delivery |
| Denial/browser precedence and canonical metadata | R13b actual three delivery URLs, root/subbody distinction and legacy stored-header protection; existing access/device denial suites retained |
| All-excluded 422 versus empty/integrity 503 | Existing StructuredDeliveryAllExcludedHandlerPolicy, StructuredDeliveryMixedAndNoEligiblePolicies, SubBodyAdapters and Stage7SubscriptionDeliveryCorruptedRowExclusion suites retained |
| Escaping and optional crypto network failure | Existing PublicPageEscapesScriptCSSAndUnsafeURLs plus R13b actual browser route and local 503 crypto fallback |
| Architectural/behavior criteria | Pure policy, SQL adapter and HTTP/preview boundaries; exact bodies/SQL and all full/format checks required; remaining core health 10.00 |

Accept A04 only after R13b full local/hosted checks and actionable review resolution; this table does not claim pending checks passed. Optional official-client/snapshot skips and unavailable local CGO remain limits, covered separately by existing hosted checks where configured.

Final local validation passes: 1,351 Go cases pass / five unchanged optional skips, no failures; uncached atomic coverage (server 65.2%), vet/build/lint, strict delta, body parity and secret scanning pass. All seven new production/test files and the remaining core score 10.00. Require the six final-head hosted checks and completed actionable review before accepting A04 and continuing R14.

PR25's initial six checks pass at 060a7fd. Automated review identified two stale machine-readable continuation fields that could repeat completed extraction. The prerequisite and critical-risk fields now explicitly state that R13b separation/local validation are complete and hosted/review acceptance remains, followed by R14. This documentation correction changes no source. Require final-head checks/review after the correction before accepting the batch.
