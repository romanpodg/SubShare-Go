# R10a — Patch planning and ordered document application

Status: discovery/implementation plan only. No R10a production changes have been made. R09 PR #15 is the prerequisite; its head `c427496` passes all six hosted checks, and final automated review is still running after twenty resolved findings. Do not merge R09 or treat it as accepted until that review completes and any new findings are addressed.

## Scope and approach

Start in `frontend/src/lib/configuration.ts`, retaining its public exports and existing parser/token editor. Separate normalized input/current projection, pure planning of ordered path/value edits, and application through `modifyXrayJSONPath`. Named protocol/connection/user/stream/transport/security phases should keep meaningful responsibilities and avoid introducing new complex/nested methods. R10b's later security extraction remains independently reviewable.

Record edits only for represented fields. Preserve original operation order, one-time settings/connection/stream initialization, absent/invalid array first-entry behavior and the exact `ensureJSONObject`/`ensureFirstJSONObject` semantics. An empty/non-array first-entry input currently becomes `[{}]` at that path; an existing array with a non-object first entry replaces only index zero. Preserve insertion order and explicit set/clear/omitted handling. Never serialize the parsed document or selected subtree wholesale: large numbers and escaped-string lexemes must survive.

Keep current mode inference, port fallback, required-field errors, dormant branch/alias policy, raw-TCP precedence, H2/QUIC ignored hints and none-to-Reality/TLS projection. Do not add a second parser, DTO or generic editor framework. If shared private helpers move, retain one implementation and avoid runtime import cycles or public API changes.

## Validation and publication

Use the R09 3,672-case targeted corpus and all 3,800 frontend tests, TypeScript, lint, exported build, direct CodeScene source review and strict delta against the accepted base. Add only meaningful regressions where a new discrepancy is discovered. Preserve exact normalized goldens, absent/partial/non-object parent behavior, independent and coupled edits, nested numeric/string lexemes and active-source conversion values. New source files must meet the hosted new-file gate; no suppressions or gate changes are planned.

Commit each coherent phase separately, publish its own PR after R09 acceptance, attach it to this chat, request automated review, address/reply/resolve actionable findings and merge only after all final-head checks pass. Update the backlog/continuation record, then proceed to R10b and the remaining batches. The persistent goal and 30-minute heartbeat remain active.
