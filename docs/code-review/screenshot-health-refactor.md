# Screenshot code-health refactor — 2026-10-13

This batch follows the eleven selected or hand-marked files in the screenshots, on top of merged PR #46 (`79a5c28`). Scores below are historical screenshot evidence; they are not fresh measurements.

| Target | Screenshot score | Changes |
| --- | --- | --- |
| `frontend/e2e/admin-smoke.spec.ts` | 5.31 | Retain all nine scenarios in domain specs; give long flows named steps with explicit state dependencies; share layout/accessibility assertions |
| `frontend/src/components/ui/AppleEmojiInput.tsx` | 7.47 | Separate safe DOM encoding/decoding, UTF-16 selection handling, grapheme limits, and the editable lifecycle |
| `frontend/src/components/admin/KeyEditorModal.tsx` | 7.29 | Separate metadata, template, raw, connection, and protocol views; reuse the existing draft/session hooks; extract guarded loading and request results |
| `cmd/server/subscription_core_test.go` | 6.69 | Group subscription metadata, assignment, policy, response-rule and boundary contracts; check previously ignored fixture operations |
| `internal/storage/legacy_test.go` | 5.69 | Separate CRUD/order, credential lifecycle, category and rollback contracts; make multi-field assertions explicit |
| `internal/storage/migrations_test.go` | Marked | Group settings/profile migrations and historical-schema fixtures; preserve rollback, indexes, triggers and foreign-key checks |
| `internal/storage/profile_repository_test.go` | Marked | Separate credential and metadata contracts from profile lifecycle tests; use a typed keyring fixture descriptor |
| `internal/storage/profile_repository.go` | Marked | Separate clone and legacy adapters; extract transaction-local insert/update operations without changing transaction ownership |
| `internal/keymanagement/keymanagement_test.go` | 7.55 | Separate the shared repository fake and source-owned metadata contract; preserve the service scenarios |
| `internal/keymanagement/create.go` | Marked | Separate per-protocol URI construction, tri-state reads, canonicalization and creation metadata normalization |
| `internal/httpapi/key_profile_test.go` | 7.72 | Share request construction while preserving all 21 handler requests and their individual assertions |

## Preserved contracts

- All 44 original Go test names remain. An AST inventory maps 441 original failure guards to 620 explicit guard conditions; original expected values and diagnostics remain at their call sites. Additional checks fail immediately on fixture query, insert-ID, keyring and encryption errors.
- Tests still execute the real HTTP handlers, SQLite migrations, repository and encryption operations. Storage helpers use the caller's transaction; encryption still authenticates the same row ID. Source metadata updates preserve ciphertext, revision conflicts retain their precedence, and failed writes roll back.
- All nine original browser scenarios and viewport sequences remain. Extracted steps return shared locators and close over the same state. The routing fixture exposes its current delivery mode through a getter so assertions observe the completed PUT.
- The editor retains its guarded request sessions, reveal targets, authoritative raw representation, hidden structured/raw workspaces, dirty/discard handling, source ownership policy and existing DOM/classes.
- Emoji selection offsets remain UTF-16; length limits remain grapheme-based. Escaping, block line breaks, native image fallback and caret insertion retain their existing behavior. Mount initialization uses an explicit initial-value snapshot instead of a hooks lint exemption.
- URI output, omission rules, escaping and legacy fallback match the original builder in 1,600 differential cases. A committed regression distinguishes an omitted ALPN, an empty list, a non-empty list containing an empty entry, and a clear operation.

## Validation

- Uncached baseline: four affected Go packages pass; 4,236 frontend tests pass.
- Full uncached `go test -json -count=1 -timeout=20m ./...`: all 13 packages pass.
- Race detector: affected profile repository/lifecycle, key-management/source metadata and HTTP handler tests pass.
- Full Vitest: 4,238 tests pass, including the added markup-escaping and block/grapheme contracts.
- TypeScript, full ESLint and golangci-lint pass; Go lint reports zero issues.
- Fresh static production build succeeds. The complete browser suite passes 26 desktop/mobile cases; final confirmation uses the latest build after extraction.
- `git diff --check` passes.

Local inventories, assertion mappings, the original-builder differential overlay and detailed logs live under ignored `.cache/screenshot-health/`. Health scores and gate status must come from the new PR's CodeScene analysis. No CodeScene rules, test selections or quality thresholds were disabled.
