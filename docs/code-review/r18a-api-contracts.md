# R18a — Shared safe API contract fixtures

Status: tests-first baseline on `test/api-contract-fixtures`, based on accepted A08 merge `52d99e9`. Twenty-one frozen cases cover actual user/source/settings/profile reads, public panel GET, legacy/v1 aliases, empty arrays, v1/legacy-shaped errors and omitted/null/empty/rejected-null subscription patches. Registered-route and persisted PATCH assertions pass against unchanged production. Frontend request assertions pass; declared-type and OpenAPI assertions reproduce the mismatches below. No R18 PR or acceptance is claimed.

`testdata/api/contracts.json` is shared by Go route/DTO/schema tests and frontend actual request/TypeScript compiler tests. The TypeScript test compiles fixture literals with `satisfies` against the current declarations rather than casting responses. Panel branding's private wire interface is read from its source declaration. Synthetic identifiers/URLs and disposable encrypted profiles keep the corpus safe. V1 and legacy user responses retain the redacted empty token field; the TypeScript User projection deliberately excludes only that known server-only field.

Go checks exact response objects, including omitted safe_structured for informational profiles, nullable category/source IDs, non-null empty arrays, masked source URLs and sanitized detail/error payloads. Source timestamps retain the historical host-local wall-clock format: only their two expected timestamp fields derive the exact local conversion of the fixed UTC seed; no actual response is normalized. Shared patches assert both request bytes and persisted override/null behavior. Public branding aliases execute without a session in Go.

## Reproduced declaration/documentation drift

- Actual key detail ownership is the string `local`; OpenAPI declares an object.
- Actual informational detail has protocol `""`; the TypeScript detail declaration excludes it. The informational fixture is not a real-profile input and must not widen protocol creation inputs.
- OpenAPI omits response content schemas for subscription settings, public panel settings and successful PATCH, plus the actual invalid-ID 400 response.

Artifacts: `.cache/r18a/openapi-before.log` contains the failing documentation cases; `types-confirmed-before.log` contains the precise empty-protocol type diagnostic. Initial TypeScript virtual-file lookup failed because Windows path separators differed; the repaired harness reproduces the actual declaration mismatch. An initial informational discovery SQL row lacked its required encrypted storage unit and returned integrity error; discovery now creates the unit through the real storage seam. Those discovery/harness failures are not acceptance evidence.

All five new test files score 10.00 after the schema helper was separated into named shape checks. The helper validates the fixture-used response subset (refs/allOf/required/primitive-null/array/enum), not the entire OpenAPI specification. Legacy aliases absent from the versioned OpenAPI document are protected by registered-route fixtures. No generation tooling or runtime response validation is introduced.

Commit this baseline before correcting only response declarations/documentation. Then require the complete Go/frontend/static/quality/secret/hosted/review gates before R18a acceptance. Request failure consolidation remains R18b; branding acknowledgment/reopen behavior remains R21. A09 and broader A01/A09-A14 remain incomplete. Goal active; heartbeat paused per user.
