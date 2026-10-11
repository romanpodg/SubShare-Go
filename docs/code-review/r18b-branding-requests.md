# R18b — Branding through the existing request/error boundary

Status: tests-first continuation on `test/branding-request-contracts`, based on accepted R18a merge `eab5d8b`. Fourteen landed prerequisite cases pass against unchanged branding production, with TypeScript/lint and CodeScene 10.00. No R18b extraction or PR exists at this checkpoint.

The canonical request suite protects GET credentials without mutation CSRF, exact mutation JSON/current CSRF, 401 clearing/notification across structured/empty/malformed responses, 403 field errors/request IDs without logout, header-ID fallback, network rejection identity, invalid successful JSON and empty success/204. Mounted context cases retain public branding load/flat-to-nested title mapping, cache/default fallback and PUT mapping/CSRF. Current local optimistic state after a denied PUT is characterization, not proof of remote persistence; acknowledgment/reopening correction remains R21.

Move the existing wire declaration to shared types, add typed panel GET/PUT methods to `api.ts`, and replace the two remaining direct fetch calls in `PanelSettingsContext`. Keep public GET, endpoint paths, mappings/defaults/cache/favicon behavior and caught save boolean. Reuse the existing credentials/CSRF/401/error parser; add actual panel method failure/success cases before acceptance. Update R18a's wire-type check to use the moved declaration. Do not introduce generation tooling or a second request/state framework.

## A09 acceptance evidence (pending)

| Requirement | Evidence |
| --- | --- |
| Shared Go/TS/OpenAPI user/source/settings/detail/error contracts | Accepted R18a/PR34, 23 safe fixtures, exact routes/aliases/envelopes/null/omitted/bounds and discriminated create inputs |
| Canonical request errors/credentials/CSRF/401/network | Eleven tests-first request cases pass; actual panel integration remains |
| Public GET and existing branding mapping/fallback | R18a anonymous actual-route cases and three mounted context baseline cases pass |
| Integration/quality/final review | Full frontend/types/lint/export/browser/strict/secret and all six final-head hosted checks/review remain |

Commit the tests-first baseline before integration. Accept A09 only after the completed matrix and final-head gates; then proceed R19 Xray interpretation/traversal. A12/R21 and broader A01/A09-A14 remain incomplete. Goal active; heartbeat paused per user.
