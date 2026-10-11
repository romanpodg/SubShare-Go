# R18b — Branding through the existing request/error boundary

Status: accepted in [PR35](https://github.com/romanpodg/SubShare-Go/pull/35), merge `56c5b244677d0cb691713383da8f07647ba7eab6`, after tests-first `5c9374f` / review regression `1da8102`. All six checks pass at `ab39494e11f86fde1c485feaedc616ed5d2066e2` in [run 38159109104](https://github.com/romanpodg/SubShare-Go/actions/runs/38159109104), including all four race steps and Docker validation. P2 is verified/replied/resolved; exact-head final review completed with a positive reaction and no actionable threads. A09 is accepted against the matrix below.

The canonical request suite protects GET credentials without mutation CSRF, exact mutation JSON/current CSRF, 401 clearing/notification across structured/empty/malformed responses, 403 field errors/request IDs without logout, header-ID fallback, network rejection identity, invalid successful JSON and empty success/204. Mounted context cases retain public branding load/flat-to-nested title mapping, cache/default fallback and PUT mapping/CSRF. Current local optimistic state after a denied PUT is characterization, not proof of remote persistence; acknowledgment/reopening correction remains R21.

The wire declaration now lives in shared `types.ts`; typed panel GET/PUT methods use `api.ts`, and the context's two direct fetch calls delegate to them. Public GET, endpoint paths, mappings/defaults/cache/favicon behavior and caught save boolean remain. R18a's compiler fixture check uses the moved declaration. Eight additional actual-panel-method cases protect success/payload bytes, credentials/CSRF, 401 clearing/notification, denied fields/request IDs, network rejection and invalid successful JSON. No generation tooling or second request/state framework was added.

## Local validation

All 25 focused cases pass: fourteen prerequisites, eight panel methods and three shared declaration/admission checks. Full frontend passes 4,142 cases in 46 files; TypeScript, full lint, static export and all 22 desktop/mobile browser cases pass. Seven mapping/cache/local-update bodies are identical under the TypeScript printer. Existing emitted API runtime is identical after excluding the new panel methods; the central request helper is the only remaining production fetch caller. Proof scripts/results remain in `.cache/r18b`.

Both baseline suites, new panel suite and updated shared type tests score 10.00. API health remains 9.15 with the same CC 21/10 findings; context remains 9.84 with the same provider finding. These are unchanged current-baseline measurements, distinct from the historical audit. Types remain unscored. Final strict delta and staged/history secret scans pass. Backend production is unchanged; final hosted backend/race checks remain required.

## Accepted A09 requirement evidence

| Requirement | Evidence |
| --- | --- |
| Shared Go/TS/OpenAPI user/source/settings/detail/error contracts | Accepted R18a/PR34, 23 safe fixtures, exact routes/aliases/envelopes/null/omitted/bounds and discriminated create inputs |
| Canonical request errors/credentials/CSRF/401/network | Eleven baseline request and eight actual panel cases pass through the unchanged existing request helper |
| Public GET and existing branding mapping/fallback | R18a anonymous actual-route cases and three mounted context baseline cases pass |
| Integration/quality/final review | Full frontend/types/lint/export/browser, retained-body proofs, strict/secret and all six final-head hosted/race checks pass; final exact-head review is clear |

Baseline 5c9374f is committed/pushed and the integration is accepted. Continue R19 Xray interpretation/traversal. A12/R21 and broader A01/A10-A14 remain incomplete. Goal active; heartbeat paused per user.

## PR35 review correction

P2 `4242453821` / `PRRT_kwDORQFS886rPqus` identifies an unsupported `subscriptionPageConfig` field in the new branding DTO. The actual Go model marks it `json:"-"`, and the branding update SQL does not persist it. Tests-first `1da8102` reproduces TypeScript admission and OpenAPI-versus-actual-Go wire-field drift. Remove the field from the branding type and response schema; page configuration remains on its existing dedicated GET/PUT API. Request/context emitted runtime and all seven retained bodies remain identical.

Full frontend now passes 4,143 cases; types/lint, focused Go/OpenAPI/DTO and static Go checks pass, regression files score 10.00. This type/documentation-only correction does not change the runtime already validated by static export and 22 browser cases. Strict/secret and all six final-head checks pass; the finding is verified/replied/resolved and final exact-head review is clear.
