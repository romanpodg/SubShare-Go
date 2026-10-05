# Analysis Evidence

Analyzed Git-tracked source at `3ab25a5d7aa4f2a28069330406d74f697ac1f1c3`, branch `main`, 2026-10-06. This appendix contains summarized measurements and analyzer findings, not raw API responses, credentials or application data. Read [repository-audit.md](repository-audit.md) for semantic interpretation and limitations; [refactoring-backlog.md](refactoring-backlog.md) owns priorities. A file allocation identifies scope, not a separate issue for each file.

## Reproducible collection method

Inventory: `git ls-files`; inspect generated markers only within tracked source. Git: `git log --format=@@%H|%aI|%s --numstat --no-renames`, current-path matches, all history and recent since 2026-04-06. Added+deleted counts include initial additions/splits; no rename lineage attribution. Recent frequency is 6-month touch count, not a prediction. Latest change is latest path touch, not a verified functional change. Bug-fix subject matching is a heuristic. Co-change excludes commits touching over 20 files.

Analyzer: `cs review --output-format json <file>` for each candidate, four CLI children, two transient failures retried. Version `1.0.47-SNAPSHOT (b23cf1c56f9b651dae4ab3ca900121b85c4e5376)`. All 211 production inputs succeeded. Additional 8 configuration/scripts were attempted; unsupported `.cjs` is the only final failure. `cs delta main` had no source changes.

Coverage: public read-only GETs to `https://api.codecov.io/api/v2/github/romanpodg/repos/SubShare-Go/` using `totals/?sha=<HEAD>`, `totals/?branch=main`, `report/?sha=<HEAD>`, `file_report/<path>?sha=<HEAD>`, daily `coverage/` with required interval and dates, and all pages of `test-analytics/?branch=main&commit_sha=<HEAD>`. API token absent; upload token never accessed. Two independent file reports confirmed handlers_users 2.14% and keymanagement/update 35.51%. No trailing slash after a greedy file path: the slash was interpreted as part of the path and returned a coverage-not-found error in initial verification. No authenticated-only result is assumed.

Local coverage: existing CI-style `go test -json -covermode=atomic -coverprofile=.cache/audit/go-coverage.out ./...`, then uncached `-count=1 -coverpkg=./...` to detect cross-package blind spots. Statement-block counts are deduplicated across package profiles for the supplemental measurement. CC = Codecov lines; local/cross = statements; these cannot be interchanged. Missing records are unavailable, not zero.

## Tracked classification

| Category | Count | Treatment |
| --- | --- | --- |
| docs-assets-other | 32 | Documentation/assets/deployment context; not imperative production code |
| configuration | 19 | Context review; supported operational TS/MJS configs additionally analyzed |
| production-go | 122 | All analyzed; includes historical migration implementation source |
| tests-fixtures-harness | 88 | Behavior/test architecture reviewed; no production health prioritization |
| production-frontend | 89 | All analyzed |
| scripts-tools | 5 | Context review; PowerShell analyzed, CJS attempted |

No tracked generated/vendor/build sources were found. Git discovery excludes untracked/ignored dependencies, runtime data, caches and output by construction. No production file was excluded due to poor health.

## Complete production metrics

| File | Health | Git lifetime/recent | Recent churn | Latest path touch | CC lines | Local statements | Cross statements | Backlog / batches |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `cmd/server/admin_credentials.go` | 9.24 | 1/1 | 128 | 2026-08-01 | 70.83% | 77.80% | 77.80% | no independent target |
| `cmd/server/api_tokens.go` | 9.68 | 2/2 | 210 | 2026-09-20 | 11.11% | 12.60% | 12.60% | no independent target |
| `cmd/server/api_v1.go` | 8.95 | 11/11 | 518 | 2026-09-20 | 46.73% | 39.50% | 39.50% | no independent target |
| `cmd/server/auth.go` | 8.24 | 12/8 | 245 | 2026-09-20 | 72.66% | 76.10% | 76.10% | P1/A06: R16a, R16b |
| `cmd/server/backup.go` | 9.61 | 5/5 | 260 | 2026-09-20 | 42.60% | 59.20% | 59.20% | P2/A13: R22a, R22b |
| `cmd/server/configuration.go` | 10.00 | 12/12 | 2305 | 2026-09-20 | 100.00% | 100.00% | 100.00% | no independent target |
| `cmd/server/external_subscriptions.go` | 9.40 | 14/14 | 6073 | 2026-09-20 | 35.40% | 32.80% | 32.80% | P1/A05: R14, R15a, R15b |
| `cmd/server/handlers.go` | 10.00 | 34/26 | 6473 | 2026-09-20 | 100.00% | 100.00% | 100.00% | no independent target |
| `cmd/server/handlers_admins.go` | 9.28 | 1/1 | 309 | 2026-09-20 | 19.14% | 23.70% | 23.70% | no independent target |
| `cmd/server/handlers_auth.go` | 9.68 | 1/1 | 90 | 2026-09-20 | 12.06% | 22.20% | 22.20% | no independent target |
| `cmd/server/handlers_hwid.go` | 10.00 | 1/1 | 63 | 2026-09-20 | 0.00% | 0.00% | 0.00% | no independent target |
| `cmd/server/handlers_settings.go` | 8.59 | 1/1 | 267 | 2026-09-20 | 41.53% | 47.00% | 47.00% | no independent target |
| `cmd/server/handlers_subscription.go` | 7.56 | 1/1 | 461 | 2026-09-20 | 42.39% | 48.30% | 48.30% | P1/A04: R12, R13a, R13b |
| `cmd/server/handlers_users.go` | 8.00 | 1/1 | 427 | 2026-09-20 | 2.14% | 3.00% | 3.00% | P1/A01: R03, R04, R05a, R05b |
| `cmd/server/happ_routing.go` | 9.24 | 2/2 | 212 | 2026-09-20 | 84.61% | 90.00% | 90.00% | no independent target |
| `cmd/server/helpers.go` | 9.31 | 20/14 | 483 | 2026-09-20 | 69.72% | 80.00% | 80.00% | no independent target |
| `cmd/server/hwid_parser.go` | 9.61 | 2/1 | 136 | 2026-09-20 | 56.56% | 53.90% | 53.90% | no independent target |
| `cmd/server/jobs.go` | 9.38 | 11/11 | 1024 | 2026-09-20 | 32.46% | 35.50% | 35.50% | P1/A05: R14, R15a, R15b |
| `cmd/server/keys.go` | 10.00 | 5/5 | 243 | 2026-09-20 | 81.44% | 84.00% | 84.00% | no independent target |
| `cmd/server/main.go` | 9.57 | 27/20 | 1311 | 2026-09-20 | 8.01% | 10.40% | 10.40% | P0/A00: R01, R02 |
| `cmd/server/openapi.go` | 10.00 | 1/1 | 16 | 2026-07-29 | 0.00% | 0.00% | 0.00% | no independent target |
| `cmd/server/repository.go` | 6.33 | 24/16 | 1609 | 2026-09-20 | 57.31% | 56.50% | 56.50% | P1/A01: R03, R04, R05a, R05b |
| `cmd/server/sources_v1.go` | 7.79 | 11/11 | 968 | 2026-09-20 | 57.18% | 58.00% | 58.00% | P1/A05: R14, R15a, R15b |
| `cmd/server/subscription_admin_v1.go` | 7.67 | 5/5 | 518 | 2026-09-20 | 39.51% | 48.50% | 48.50% | P1/A01: R03, R04, R05a, R05b |
| `cmd/server/subscription_crypto.go` | 9.68 | 2/1 | 4 | 2026-08-01 | 0.00% | 0.00% | 0.00% | no independent target |
| `cmd/server/subscription_delivery.go` | 8.57 | 3/3 | 281 | 2026-09-20 | 79.20% | 87.60% | 87.60% | P1/A04: R12, R13a, R13b |
| `cmd/server/subscription_delivery_settings.go` | 9.58 | 7/7 | 337 | 2026-09-20 | 77.86% | 82.50% | 82.50% | no independent target |
| `cmd/server/subscription_generation.go` | 9.38 | 9/9 | 3178 | 2026-09-20 | 85.71% | 90.80% | 90.80% | no independent target |
| `cmd/server/subscription_page.go` | 8.09 | 8/8 | 1306 | 2026-09-20 | 73.06% | 73.90% | 73.90% | P2/A14: R23 |
| `cmd/server/subscription_rules.go` | 6.50 | 10/10 | 1463 | 2026-09-20 | 40.40% | 44.20% | 44.20% | P1/A04: R12, R13a, R13b |
| `cmd/server/types.go` | 10.00 | 14/9 | 57 | 2026-09-20 | 100.00% | 100.00% | 100.00% | no independent target |
| `frontend/src/app/admin/admins/page.tsx` | 10.00 | 1/1 | 16 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/audit/page.tsx` | 9.31 | 4/4 | 150 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/keys/page.tsx` | 9.68 | 2/2 | 79 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/layout.tsx` | 10.00 | 1/1 | 11 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/login/page.tsx` | 10.00 | 8/5 | 258 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/overview/page.tsx` | 9.06 | 7/7 | 347 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/page.tsx` | 10.00 | 10/5 | 345 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/response-rules/page.tsx` | 9.68 | 4/4 | 497 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/settings/branding/page.tsx` | 10.00 | 4/4 | 52 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/settings/security/page.tsx` | 10.00 | 4/4 | 198 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/settings/subscription/page.tsx` | 8.08 | 6/6 | 644 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/sources/page.tsx` | 8.32 | 3/3 | 478 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/templates/page.tsx` | 9.68 | 4/4 | 276 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/admin/users/page.tsx` | 8.39 | 5/5 | 480 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/error.tsx` | 10.00 | 2/0 | 0 | 2026-02-14 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/layout.tsx` | 10.00 | 8/4 | 44 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/loading.tsx` | 10.00 | 2/0 | 0 | 2026-02-14 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/not-found.tsx` | 10.00 | 1/0 | 0 | 2026-02-14 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/page.tsx` | 10.00 | 1/0 | 0 | 2026-02-14 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/subscription/NumericGlobe.tsx` | 9.68 | 1/1 | 209 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/subscription/SubscriptionBlocks.tsx` | 8.45 | 5/5 | 778 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/app/subscription/page.tsx` | 9.53 | 7/3 | 628 | 2026-07-29 | unavailable | unavailable | unavailable | P2/A14: R23 |
| `frontend/src/app/subscription/pageConfig.ts` | null | 3/3 | 420 | 2026-07-30 | unavailable | unavailable | unavailable | P2/A14: R23 |
| `frontend/src/components/ErrorBoundary.tsx` | 10.00 | 2/0 | 0 | 2026-02-14 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/AddKeyModal.tsx` | 10.00 | 10/5 | 1306 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/AddUserModal.tsx` | 10.00 | 4/1 | 4 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/AdminsSection.tsx` | 10.00 | 4/4 | 405 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/BulkEditKeysModal.tsx` | 10.00 | 4/4 | 206 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/BulkEditUsersModal.tsx` | 9.68 | 2/2 | 151 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/CreateKeyCategoryModal.tsx` | 10.00 | 2/2 | 141 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/DashboardShell.tsx` | 8.68 | 3/3 | 449 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/EditKeyModal.tsx` | 10.00 | 9/5 | 1258 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/EditSubscriptionModal.tsx` | 10.00 | 4/2 | 43 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/GlobalSubscriptionSettingsModal.tsx` | 9.60 | 9/6 | 1287 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/HwidManager.tsx` | 8.28 | 6/2 | 31 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/KeyAssignerModal.tsx` | 10.00 | 6/3 | 34 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/KeyCategoryEditorModal.tsx` | 8.38 | 3/3 | 406 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/KeyEditorConflictDialog.tsx` | 10.00 | 1/1 | 50 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/KeyEditorModal.tsx` | 6.20 | 5/5 | 3694 | 2026-08-11 | unavailable | unavailable | unavailable | P1/A07: R11a, R11b |
| `frontend/src/components/admin/KeyHealthBadge.tsx` | 10.00 | 2/2 | 66 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/KeysSection.tsx` | 6.71 | 13/8 | 2650 | 2026-08-11 | unavailable | unavailable | unavailable | P1/A08: R17a, R17b |
| `frontend/src/components/admin/OutputCapabilitiesMatrix.tsx` | 10.00 | 1/1 | 93 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/PageHeader.tsx` | 10.00 | 3/3 | 41 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/PanelSettingsModal.tsx` | 8.70 | 3/2 | 33 | 2026-07-30 | unavailable | unavailable | unavailable | P2/A12: R21 |
| `frontend/src/components/admin/RoutingSettingsModal.tsx` | 9.68 | 3/3 | 888 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/SourceCreateDrawer.tsx` | 10.00 | 5/5 | 465 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/SourceDetailDrawer.tsx` | 10.00 | 4/4 | 305 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/StatCard.tsx` | 10.00 | 4/4 | 61 | 2026-08-09 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/SubscriptionPageConfigModal.tsx` | 10.00 | 2/2 | 214 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/UsersSection.tsx` | 8.54 | 10/4 | 604 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/keyCategoryColors.ts` | 10.00 | 1/1 | 27 | 2026-05-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/keyTemplateVariables.ts` | 10.00 | 2/1 | 37 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/protocol-editor-capabilities.ts` | 10.00 | 1/1 | 33 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/protocol-editors/Hysteria2Fields.tsx` | 9.36 | 1/1 | 147 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/protocol-editors/LegacyXrayFields.tsx` | 8.82 | 1/1 | 63 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/protocol-editors/ShadowsocksFields.tsx` | 9.24 | 1/1 | 114 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/protocol-editors/TuicV4ReadOnlyBanner.tsx` | 10.00 | 1/1 | 25 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/admin/protocol-editors/TuicV5Fields.tsx` | 8.59 | 1/1 | 220 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/AppleEmojiInput.tsx` | 7.47 | 7/5 | 334 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Button.tsx` | 10.00 | 6/3 | 22 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Card.tsx` | 10.00 | 3/2 | 4 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/ConfirmDialog.tsx` | 10.00 | 2/0 | 0 | 2026-02-17 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Drawer.tsx` | 9.68 | 3/3 | 96 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/EmojiPickerButton.tsx` | 8.65 | 6/4 | 407 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/EmojiText.tsx` | 10.00 | 5/3 | 79 | 2026-08-05 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Input.tsx` | 10.00 | 5/3 | 14 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/LoadingSpinner.tsx` | 10.00 | 1/0 | 0 | 2026-02-14 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Modal.tsx` | 9.84 | 8/3 | 68 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/PasswordInput.tsx` | 10.00 | 4/3 | 18 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/ResourceState.tsx` | 10.00 | 2/2 | 67 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Select.tsx` | 9.02 | 5/4 | 313 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/StatusBadge.tsx` | 10.00 | 5/3 | 24 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Technical.tsx` | 10.00 | 2/2 | 202 | 2026-07-30 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/components/ui/Toast.tsx` | 10.00 | 4/1 | 65 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/context/AuthContext.tsx` | 10.00 | 1/1 | 101 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/context/PanelSettingsContext.tsx` | 9.84 | 4/3 | 23 | 2026-07-30 | unavailable | unavailable | unavailable | P2/A12: R21 |
| `frontend/src/hooks/useAuth.ts` | null | 3/2 | 51 | 2026-07-29 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/adminPassword.ts` | 10.00 | 1/1 | 29 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/api.ts` | 9.15 | 12/9 | 615 | 2026-08-11 | unavailable | unavailable | unavailable | P2/A09: R18a, R18b |
| `frontend/src/lib/clipboard.ts` | 9.68 | 1/0 | 0 | 2026-02-17 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/configuration.ts` | 2.36 | 4/4 | 1830 | 2026-08-11 | unavailable | unavailable | unavailable | P1/A03: R09, R10a, R10b |
| `frontend/src/lib/datetime.ts` | 10.00 | 1/1 | 19 | 2026-04-25 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/emoji-assets.ts` | 9.68 | 1/1 | 21 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/emoji-popover.ts` | 10.00 | 1/1 | 47 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/external-import-messages.ts` | 10.00 | 1/1 | 60 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/externalSourceName.ts` | 10.00 | 1/1 | 14 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/response-rules.ts` | 8.95 | 1/1 | 90 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `frontend/src/lib/types.ts` | null | 14/10 | 665 | 2026-08-11 | unavailable | unavailable | unavailable | P2/A09: R18a, R18b |
| `frontend/src/lib/xray-json-document.ts` | 9.68 | 1/1 | 157 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `internal/delivery/capabilities.go` | 10.00 | 1/1 | 121 | 2026-09-20 | 0.00% | 0.00% | 100.00% | P2/A13: R22a, R22b |
| `internal/delivery/entry.go` | 9.68 | 1/1 | 168 | 2026-09-20 | 0.00% | 0.00% | 89.70% | P2/A13: R22a, R22b |
| `internal/delivery/headers.go` | 10.00 | 1/1 | 50 | 2026-09-20 | 0.00% | 0.00% | 100.00% | P2/A13: R22a, R22b |
| `internal/delivery/identity.go` | 10.00 | 1/1 | 99 | 2026-09-20 | 0.00% | 0.00% | 84.60% | P2/A13: R22a, R22b |
| `internal/delivery/informational.go` | 10.00 | 1/1 | 158 | 2026-09-20 | 0.00% | 0.00% | 77.40% | P2/A13: R22a, R22b |
| `internal/delivery/mihomo.go` | 10.00 | 1/1 | 231 | 2026-09-20 | 0.00% | 0.00% | 80.30% | P2/A13: R22a, R22b |
| `internal/delivery/plain.go` | 10.00 | 1/1 | 117 | 2026-09-20 | 0.00% | 0.00% | 74.20% | P2/A13: R22a, R22b |
| `internal/delivery/render.go` | 10.00 | 3/3 | 3693 | 2026-09-20 | 0.00% | 0.00% | 56.30% | P2/A13: R22a, R22b |
| `internal/delivery/shadowsocks.go` | 10.00 | 1/1 | 64 | 2026-09-20 | 0.00% | 0.00% | 76.00% | P2/A13: R22a, R22b |
| `internal/delivery/singbox.go` | 10.00 | 1/1 | 219 | 2026-09-20 | 0.00% | 0.00% | 78.40% | P2/A13: R22a, R22b |
| `internal/delivery/split.go` | 10.00 | 1/1 | 58 | 2026-09-20 | 0.00% | 0.00% | 85.70% | P2/A13: R22a, R22b |
| `internal/delivery/structured.go` | 10.00 | 1/1 | 144 | 2026-09-20 | 0.00% | 0.00% | 87.90% | P2/A13: R22a, R22b |
| `internal/delivery/validate.go` | 9.68 | 1/1 | 171 | 2026-09-20 | 0.00% | 0.00% | 90.20% | P2/A13: R22a, R22b |
| `internal/delivery/xray.go` | 10.00 | 1/1 | 228 | 2026-09-20 | 0.00% | 0.00% | 84.60% | P2/A13: R22a, R22b |
| `internal/httpapi/key_administration.go` | 8.61 | 4/4 | 298 | 2026-09-20 | 60.83% | 64.10% | 64.10% | no independent target |
| `internal/httpapi/key_categories.go` | 9.09 | 2/2 | 294 | 2026-09-20 | 51.74% | 46.70% | 46.70% | no independent target |
| `internal/httpapi/key_profile_errors.go` | 9.34 | 4/4 | 242 | 2026-09-20 | 33.92% | 54.50% | 54.50% | no independent target |
| `internal/httpapi/key_profile_headers.go` | 10.00 | 1/1 | 15 | 2026-08-06 | 100.00% | 100.00% | 100.00% | no independent target |
| `internal/httpapi/key_profiles.go` | 9.38 | 3/3 | 284 | 2026-09-20 | 81.61% | 84.90% | 84.90% | no independent target |
| `internal/httpapi/key_queries.go` | 9.24 | 3/3 | 133 | 2026-09-20 | 73.68% | 81.40% | 88.40% | no independent target |
| `internal/httpapi/legacy_keys.go` | 10.00 | 4/4 | 492 | 2026-09-20 | 81.69% | 80.40% | 80.40% | no independent target |
| `internal/httpapi/respond.go` | 9.68 | 1/1 | 112 | 2026-09-20 | 71.92% | 77.50% | 95.00% | no independent target |
| `internal/keymanagement/bulk_health.go` | 9.68 | 3/3 | 244 | 2026-09-20 | 58.90% | 62.50% | 91.10% | no independent target |
| `internal/keymanagement/categories.go` | 10.00 | 2/2 | 166 | 2026-09-20 | 78.82% | 82.30% | 88.70% | no independent target |
| `internal/keymanagement/clone.go` | 10.00 | 3/3 | 33 | 2026-09-20 | 84.61% | 87.50% | 87.50% | no independent target |
| `internal/keymanagement/commands.go` | null | 4/4 | 142 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `internal/keymanagement/configuration_validation.go` | 10.00 | 1/1 | 26 | 2026-08-11 | 80.00% | 88.90% | 88.90% | no independent target |
| `internal/keymanagement/create.go` | 7.84 | 7/7 | 580 | 2026-09-20 | 45.45% | 54.30% | 57.40% | no independent target |
| `internal/keymanagement/detail.go` | 9.31 | 4/4 | 250 | 2026-09-20 | 60.00% | 70.00% | 78.30% | no independent target |
| `internal/keymanagement/errors.go` | 10.00 | 5/5 | 121 | 2026-09-20 | 42.10% | 46.20% | 53.80% | no independent target |
| `internal/keymanagement/legacy.go` | 8.31 | 6/6 | 295 | 2026-09-20 | 22.22% | 31.30% | 56.30% | no independent target |
| `internal/keymanagement/repository.go` | null | 1/1 | 139 | 2026-09-20 | unavailable | unavailable | unavailable | no independent target |
| `internal/keymanagement/reveal.go` | 9.68 | 3/3 | 60 | 2026-09-20 | 58.06% | 66.70% | 75.00% | no independent target |
| `internal/keymanagement/schema.go` | 9.45 | 1/1 | 120 | 2026-08-06 | 100.00% | 100.00% | 100.00% | no independent target |
| `internal/keymanagement/service.go` | 10.00 | 3/3 | 56 | 2026-09-20 | 50.00% | 50.00% | 100.00% | no independent target |
| `internal/keymanagement/update.go` | 6.64 | 7/7 | 754 | 2026-09-20 | 35.51% | 36.00% | 52.30% | P1/A02: R06, R07a, R07b, R08 |
| `internal/keypersistence/errors.go` | 10.00 | 1/1 | 35 | 2026-08-09 | 0.00% | 0.00% | 0.00% | P2/A11: R20 |
| `internal/keypersistence/params.go` | null | 2/2 | 73 | 2026-08-11 | unavailable | unavailable | unavailable | P2/A11: R20 |
| `internal/keypersistence/repository.go` | null | 1/1 | 30 | 2026-08-09 | unavailable | unavailable | unavailable | P2/A11: R20 |
| `internal/middleware/middleware.go` | 8.47 | 8/7 | 328 | 2026-09-20 | 68.03% | 73.60% | 74.20% | no independent target |
| `internal/model/model.go` | 8.54 | 16/11 | 1097 | 2026-09-20 | 58.65% | 56.90% | 94.40% | P2/A09: R18a, R18b |
| `internal/platform/configuration/configuration.go` | 7.33 | 5/5 | 415 | 2026-09-20 | 80.00% | 82.60% | 82.60% | no independent target |
| `internal/profileconfig/drafts.go` | null | 2/2 | 94 | 2026-08-11 | unavailable | unavailable | unavailable | no independent target |
| `internal/profileconfig/hysteria2_probe.go` | 10.00 | 1/1 | 169 | 2026-08-11 | 55.88% | 60.00% | 60.00% | no independent target |
| `internal/profileconfig/parse.go` | 7.40 | 5/5 | 704 | 2026-09-20 | 69.30% | 74.40% | 85.70% | no independent target |
| `internal/profileconfig/probe.go` | 9.61 | 2/2 | 234 | 2026-08-11 | 46.26% | 52.50% | 52.50% | no independent target |
| `internal/profileconfig/render.go` | 9.38 | 3/3 | 444 | 2026-09-20 | 77.58% | 84.10% | 84.10% | no independent target |
| `internal/profileconfig/xray.go` | 5.48 | 3/3 | 1271 | 2026-09-20 | 43.78% | 58.00% | 82.10% | P2/A10: R19 |
| `internal/profilepersistence/errors.go` | null | 2/2 | 13 | 2026-08-09 | unavailable | unavailable | unavailable | P2/A11: R20 |
| `internal/profilepersistence/params.go` | null | 5/5 | 116 | 2026-08-11 | unavailable | unavailable | unavailable | P2/A11: R20 |
| `internal/profilepersistence/repository.go` | null | 5/5 | 25 | 2026-08-11 | unavailable | unavailable | unavailable | P2/A11: R20 |
| `internal/profiles/doc.go` | null | 1/1 | 20 | 2026-08-01 | unavailable | unavailable | unavailable | no independent target |
| `internal/profiles/errors.go` | 10.00 | 1/1 | 57 | 2026-08-01 | 68.42% | 73.30% | 73.30% | no independent target |
| `internal/profiles/fingerprint.go` | 9.92 | 1/1 | 112 | 2026-08-01 | 95.00% | 95.70% | 95.70% | no independent target |
| `internal/profiles/hysteria2.go` | 8.21 | 3/3 | 272 | 2026-09-20 | 74.52% | 83.20% | 84.00% | no independent target |
| `internal/profiles/legacy.go` | 9.11 | 1/1 | 325 | 2026-08-01 | 68.00% | 78.30% | 78.90% | no independent target |
| `internal/profiles/model.go` | 10.00 | 1/1 | 362 | 2026-08-01 | 77.61% | 71.90% | 71.90% | no independent target |
| `internal/profiles/registry.go` | 9.24 | 1/1 | 192 | 2026-08-01 | 68.22% | 78.50% | 78.50% | no independent target |
| `internal/profiles/shadowsocks.go` | 8.45 | 2/2 | 445 | 2026-09-20 | 75.86% | 85.20% | 85.20% | no independent target |
| `internal/profiles/tuic.go` | 5.59 | 3/3 | 1063 | 2026-09-20 | 74.82% | 83.00% | 83.00% | no independent target |
| `internal/profiles/uri.go` | 4.74 | 2/2 | 581 | 2026-09-20 | 88.62% | 93.20% | 93.90% | no independent target |
| `internal/security/password/hasher.go` | 6.35 | 1/1 | 299 | 2026-08-01 | 65.38% | 70.40% | 74.50% | no independent target |
| `internal/security/password/policy.go` | 10.00 | 1/1 | 45 | 2026-08-01 | 56.25% | 81.80% | 90.90% | no independent target |
| `internal/security/profilestorage/crypto.go` | 8.81 | 3/3 | 262 | 2026-09-11 | 55.17% | 67.30% | 74.50% | no independent target |
| `internal/security/profilestorage/errors.go` | null | 2/2 | 22 | 2026-08-09 | unavailable | unavailable | unavailable | no independent target |
| `internal/security/profilestorage/keyring.go` | 8.28 | 4/4 | 446 | 2026-09-20 | 50.48% | 62.40% | 67.10% | no independent target |
| `internal/security/profilestorage/permissions_unix.go` | 10.00 | 1/1 | 16 | 2026-08-09 | 100.00% | unavailable | unavailable | no independent target |
| `internal/security/profilestorage/permissions_windows.go` | 9.68 | 2/2 | 103 | 2026-09-11 | unavailable | 77.80% | 77.80% | no independent target |
| `internal/security/profilestorage/secret_url.go` | 10.00 | 1/1 | 55 | 2026-08-05 | 63.63% | 69.20% | 69.20% | no independent target |
| `internal/sources/decode.go` | 9.68 | 1/1 | 181 | 2026-09-20 | 32.32% | 37.50% | 93.80% | no independent target |
| `internal/sources/fetch.go` | 10.00 | 1/1 | 112 | 2026-09-20 | 64.61% | 74.00% | 74.00% | no independent target |
| `internal/sources/hwid.go` | 10.00 | 1/1 | 75 | 2026-09-20 | 75.00% | 77.30% | 81.80% | no independent target |
| `internal/sources/names.go` | 10.00 | 1/1 | 81 | 2026-09-20 | 18.18% | 16.70% | 80.60% | no independent target |
| `internal/sources/parse_json.go` | 9.61 | 1/1 | 290 | 2026-09-20 | 0.00% | 0.00% | 81.50% | no independent target |
| `internal/sources/parse_links.go` | 9.68 | 1/1 | 359 | 2026-09-20 | 32.47% | 42.90% | 88.90% | no independent target |
| `internal/sources/refs.go` | 10.00 | 1/1 | 120 | 2026-09-20 | 55.55% | 60.40% | 88.70% | no independent target |
| `internal/sources/ssrf.go` | 9.92 | 1/1 | 137 | 2026-09-20 | 37.97% | 37.10% | 56.50% | no independent target |
| `internal/sources/sync.go` | 9.53 | 1/1 | 458 | 2026-09-20 | 0.00% | 0.00% | 84.50% | P2/A13: R22a, R22b |
| `internal/sources/types.go` | 10.00 | 1/1 | 151 | 2026-09-20 | 9.09% | 13.30% | 66.70% | no independent target |
| `internal/sources/xray_hysteria2.go` | 9.61 | 1/1 | 438 | 2026-09-20 | 0.00% | 0.00% | 80.00% | no independent target |
| `internal/storage/category_store.go` | 9.68 | 1/1 | 72 | 2026-08-09 | 40.47% | 46.70% | 73.30% | no independent target |
| `internal/storage/credential_store.go` | 9.38 | 2/2 | 77 | 2026-08-09 | 58.97% | 75.00% | 75.00% | no independent target |
| `internal/storage/delivery.go` | 10.00 | 2/2 | 277 | 2026-09-20 | 0.00% | 0.00% | 71.10% | no independent target |
| `internal/storage/key_repository.go` | 8.81 | 3/3 | 379 | 2026-09-20 | 30.00% | 42.30% | 81.00% | no independent target |
| `internal/storage/migrations.go` | 9.33 | 3/3 | 3447 | 2026-09-20 | 54.43% | 73.10% | 73.10% | no independent target |
| `internal/storage/migrations_baseline.go` | 9.92 | 1/1 | 319 | 2026-09-20 | 62.16% | 79.40% | 79.40% | no independent target |
| `internal/storage/migrations_data.go` | 7.39 | 1/1 | 505 | 2026-09-20 | 55.28% | 66.50% | 66.50% | P2/A13: R22a, R22b |
| `internal/storage/profile_repository.go` | 5.67 | 10/10 | 1767 | 2026-09-20 | 64.07% | 75.50% | 76.50% | P1/A02: R06, R07a, R07b, R08 |
| `internal/storage/source_keys.go` | 10.00 | 2/2 | 293 | 2026-09-20 | 0.00% | 0.00% | 82.90% | no independent target |
| `internal/storage/sqlite.go` | 9.92 | 3/3 | 115 | 2026-09-20 | 60.78% | 67.50% | 67.50% | P0/A00: R01, R02 |
| `internal/storage/startup.go` | 9.61 | 1/1 | 65 | 2026-08-06 | 68.42% | 81.10% | 81.10% | no independent target |

## Supplemental analyzer inputs

| File | Result | Health | Reason/interpretation |
| --- | --- | --- | --- |
| `frontend/eslint.config.mjs` | complete | null | Valid null score; declaration/configuration/short script |
| `frontend/next.config.ts` | complete | 10.00 | Operational/configuration source, not production distribution |
| `frontend/playwright.config.ts` | complete | null | Valid null score; declaration/configuration/short script |
| `frontend/postcss.config.mjs` | complete | null | Valid null score; declaration/configuration/short script |
| `frontend/vitest.config.ts` | complete | null | Valid null score; declaration/configuration/short script |
| `scripts/backup.ps1` | complete | null | Valid null score; declaration/configuration/short script |
| `scripts/codecov-bundle.cjs` | failed | null | Unsupported .cjs; script still inspected contextually |
| `scripts/start.ps1` | complete | 8.03 | Operational/configuration source, not production distribution |

## Every tracked exclusion

The following 136 inputs were not submitted for production Code Health review. Exclusion from this metric does not exclude migration/deployment/tests/contracts from the semantic audit.

| File | Classification | Reason |
| --- | --- | --- |
| `.dockerignore` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `.env.example` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `.gitattributes` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `.github/workflows/ci.yml` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `.github/workflows/release-images.yml` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `.gitignore` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `.gitleaksignore` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `.golangci.yml` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `Dockerfile` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `LICENSE` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `Makefile` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `README.md` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `README_RU.md` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `cmd/server/admin_credentials_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/api_tokens_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/api_v1_integration_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/authorization_matrix_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/backup_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/characterization_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/configuration_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/external_profiles_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/external_subscriptions_security_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/happ_routing_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/helpers_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/jobs_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/key_routes_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/keys_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/migration_existing_db_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/openapi.yaml` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `cmd/server/runtime_configuration_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/server_encryption_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/sources_v1_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/subscription_core_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/subscription_delivery_settings_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/subscription_external_validation_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/subscription_generation_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `cmd/server/subscription_rules_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `codecov.yml` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `docker-compose.yml` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `docs/protocol-health-checks.md` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `docs/quality-tooling.md` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `docs/screenshots/dashboard.png` | docs-assets-other | Binary/vector/media asset |
| `docs/screenshots/external-sources.png` | docs-assets-other | Binary/vector/media asset |
| `docs/screenshots/users.png` | docs-assets-other | Binary/vector/media asset |
| `frontend/.dockerignore` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `frontend/.gitignore` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `frontend/Caddyfile` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `frontend/Dockerfile` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `frontend/e2e/admin-smoke.spec.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/e2e/run.mjs` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/e2e/static-server.mjs` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/package-lock.json` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `frontend/package.json` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `frontend/public/file.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/globe.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/icons/external-link.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/next.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/subscription/apple.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/subscription/appstore.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/subscription/googleplay.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/subscription/linux.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/subscription/logo.jpg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/subscription/windows.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/vercel.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/public/window.svg` | docs-assets-other | Binary/vector/media asset |
| `frontend/src/app/admin/overview/page.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/app/admin/response-rules/page.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/app/admin/settings/subscription/page.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/app/admin/sources/page.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/app/favicon.ico` | docs-assets-other | Binary/vector/media asset |
| `frontend/src/app/globals.css` | docs-assets-other | Stylesheet; declarative presentation |
| `frontend/src/app/subscription/NumericGlobe.module.css` | docs-assets-other | Stylesheet; declarative presentation |
| `frontend/src/app/subscription/NumericGlobe.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/app/subscription/SubscriptionBlocks.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/app/subscription/subscription-page.module.css` | docs-assets-other | Stylesheet; declarative presentation |
| `frontend/src/components/admin/GlobalSubscriptionSettingsModal.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/HwidManager.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/KeyEditorModal.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/KeyHealthBadge.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/KeysSection.health.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/KeysSection.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/RoutingSettingsModal.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/SourceCreateDrawer.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/StatCard.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/keyTemplateVariables.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/admin/protocol-editor-capabilities.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/AppleEmojiInput.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/EmojiPickerButton.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/EmojiText.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/Input.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/Modal.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/Technical.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/components/ui/Toast.test.tsx` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/adminPassword.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/api-keys-privacy.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/configuration.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/emoji-assets.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/emoji-popover.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/external-import-messages.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/externalSourceName.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/src/lib/response-rules.test.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `frontend/tsconfig.json` | configuration | Declarative manifest/schema/CI/deployment config; contextual review |
| `frontend/vitest.setup.ts` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `go.mod` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `go.sum` | docs-assets-other | Documentation, metadata or deployment/text asset; contextual review |
| `internal/httpapi/key_administration_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/httpapi/key_categories_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/httpapi/key_profile_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/httpapi/key_queries_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/httpapi/legacy_keys_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/keymanagement/boundaries_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/keymanagement/categories_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/keymanagement/create_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/keymanagement/keymanagement_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/keymanagement/legacy_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/middleware/middleware_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/model/model_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/platform/configuration/configuration_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profileconfig/profileconfig_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/adversarial_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/fuzz_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/profiles_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/testdata/fuzz/FuzzHysteria2Parse/b46bcc166214beb7` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/testdata/fuzz/FuzzHysteria2Parse/bff854553a2a403b` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/testdata/fuzz/FuzzRegistryParse/369cc0db252529a7` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/profiles/testdata/valid_profiles.json` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/security/password/password_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/security/profilestorage/permissions_windows_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/security/profilestorage/profilestorage_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/sources/fetch_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/storage/legacy_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/storage/migrations_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/storage/profile_repository_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `internal/storage/storage_test.go` | tests-fixtures-harness | Tests, fixtures or harness; reviewed for behavioral protection |
| `scripts/backup.sh` | scripts-tools | Shell operational script; CLI does not support .sh |
| `scripts/start.sh` | scripts-tools | Shell operational script; CLI does not support .sh |

## Git hotspot evidence

These counts describe activity, not causal defect probability. High-churn healthy parents matter for newly split files.

| File | Full/recent churn | Bug-fix-subject touches | Small-commit co-change leaders (count) |
| --- | --- | --- | --- |
| `cmd/server/main.go` | 1772/1311 | 3: cdc89a62 fix: validate runtime configuration strictly; 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc.; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/handlers.go (7); cmd/server/types.go (7); cmd/server/repository.go (7); cmd/server/auth.go (5); cmd/server/helpers.go (4) |
| `internal/storage/sqlite.go` | 115/115 | 1: 25cba98b fix: polish admin routing, health checks, and ui | cmd/server/main.go (2); cmd/server/repository.go (1); cmd/server/subscription_delivery_settings.go (1); internal/storage/migrations.go (1); internal/storage/startup.go (1) |
| `cmd/server/repository.go` | 2708/1609 | 3: 25cba98b fix: polish admin routing, health checks, and ui; 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc.; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/handlers.go (8); cmd/server/main.go (7); cmd/server/auth.go (5); cmd/server/helpers.go (5); cmd/server/types.go (5) |
| `cmd/server/handlers_users.go` | 427/427 | 0:  |  |
| `cmd/server/subscription_admin_v1.go` | 518/518 | 0:  | cmd/server/api_v1.go (1); cmd/server/backup.go (1); cmd/server/external_subscriptions.go (1); cmd/server/handlers.go (1); cmd/server/helpers.go (1) |
| `internal/storage/profile_repository.go` | 1767/1767 | 2: 25cba98b fix: polish admin routing, health checks, and ui; d4c2b0a5 fix: assign cloned keys to all-mode users | internal/keymanagement/create.go (2); internal/keymanagement/legacy.go (2); internal/keymanagement/commands.go (2); internal/keymanagement/errors.go (2); internal/profilepersistence/params.go (2) |
| `internal/keymanagement/update.go` | 754/754 | 1: 25cba98b fix: polish admin routing, health checks, and ui | internal/keymanagement/create.go (2); internal/keymanagement/errors.go (2); cmd/server/auth.go (1); cmd/server/handlers.go (1); cmd/server/main.go (1) |
| `frontend/src/lib/configuration.ts` | 1830/1830 | 1: 17224894 fix: harden JSON editing and admin UI |  |
| `cmd/server/subscription_rules.go` | 1463/1463 | 1: 25cba98b fix: polish admin routing, health checks, and ui | cmd/server/configuration.go (2); cmd/server/handlers.go (2); cmd/server/subscription_delivery_settings.go (2); cmd/server/subscription_generation.go (2); cmd/server/helpers.go (1) |
| `cmd/server/handlers_subscription.go` | 461/461 | 0:  |  |
| `cmd/server/subscription_delivery.go` | 281/281 | 0:  | cmd/server/handlers.go (1); cmd/server/helpers.go (1); cmd/server/subscription_generation.go (1) |
| `cmd/server/jobs.go` | 1024/1024 | 1: 25cba98b fix: polish admin routing, health checks, and ui | cmd/server/configuration.go (1); cmd/server/external_subscriptions.go (1); cmd/server/handlers.go (1); cmd/server/main.go (1); cmd/server/sources_v1.go (1) |
| `cmd/server/sources_v1.go` | 968/968 | 0:  | cmd/server/external_subscriptions.go (2); cmd/server/handlers.go (2); cmd/server/types.go (2); cmd/server/configuration.go (1); cmd/server/jobs.go (1) |
| `cmd/server/external_subscriptions.go` | 6073/6073 | 1: 25cba98b fix: polish admin routing, health checks, and ui | cmd/server/handlers.go (3); cmd/server/helpers.go (2); cmd/server/sources_v1.go (2); cmd/server/types.go (2); cmd/server/auth.go (1) |
| `cmd/server/auth.go` | 408/245 | 1: 5584ad09 fix: hash admin sessions at rest and deny unscoped token writes | cmd/server/handlers.go (7); cmd/server/main.go (5); cmd/server/repository.go (5); cmd/server/helpers.go (4); cmd/server/types.go (4) |
| `frontend/src/components/admin/KeyEditorModal.tsx` | 3694/3694 | 2: 25cba98b fix: polish admin routing, health checks, and ui; 17224894 fix: harden JSON editing and admin UI |  |
| `frontend/src/components/admin/KeysSection.tsx` | 3606/2650 | 3: 25cba98b fix: polish admin routing, health checks, and ui; 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc.; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/handlers.go (1); cmd/server/helpers.go (1); cmd/server/hwid_parser.go (1); cmd/server/main.go (1); cmd/server/repository.go (1) |
| `internal/model/model.go` | 1509/1097 | 3: 25cba98b fix: polish admin routing, health checks, and ui; 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc.; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/handlers.go (3); cmd/server/helpers.go (3); cmd/server/repository.go (3); cmd/server/main.go (2); cmd/server/types.go (2) |
| `frontend/src/lib/api.ts` | 761/615 | 2: 25cba98b fix: polish admin routing, health checks, and ui; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/handlers.go (2); frontend/src/lib/types.ts (2); cmd/server/configuration.go (1); cmd/server/subscription_delivery_settings.go (1); cmd/server/subscription_generation.go (1) |
| `frontend/src/lib/types.ts` | 748/665 | 2: 25cba98b fix: polish admin routing, health checks, and ui; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/handlers.go (3); frontend/src/lib/api.ts (2); cmd/server/helpers.go (2); cmd/server/repository.go (2); frontend/src/components/admin/HwidManager.tsx (2) |
| `internal/profileconfig/xray.go` | 1271/1271 | 0:  | cmd/server/configuration.go (1); internal/profileconfig/drafts.go (1); internal/profileconfig/parse.go (1); internal/profileconfig/probe.go (1); internal/profileconfig/render.go (1) |
| `internal/keypersistence/errors.go` | 35/35 | 0:  |  |
| `internal/keypersistence/params.go` | 73/73 | 0:  |  |
| `internal/keypersistence/repository.go` | 30/30 | 0:  |  |
| `internal/profilepersistence/errors.go` | 13/13 | 0:  | internal/keymanagement/clone.go (1); internal/keymanagement/commands.go (1); internal/keymanagement/create.go (1); internal/keymanagement/detail.go (1); internal/keymanagement/errors.go (1) |
| `internal/profilepersistence/params.go` | 116/116 | 1: 25cba98b fix: polish admin routing, health checks, and ui | internal/keymanagement/commands.go (2); internal/keymanagement/errors.go (2); internal/profilepersistence/repository.go (2); internal/storage/profile_repository.go (2); cmd/server/handlers.go (1) |
| `internal/profilepersistence/repository.go` | 25/25 | 1: 25cba98b fix: polish admin routing, health checks, and ui | internal/keymanagement/commands.go (2); internal/keymanagement/errors.go (2); internal/profilepersistence/params.go (2); internal/storage/profile_repository.go (2); cmd/server/handlers.go (1) |
| `frontend/src/context/PanelSettingsContext.tsx` | 250/23 | 1: 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc. |  |
| `frontend/src/components/admin/PanelSettingsModal.tsx` | 280/33 | 1: 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc. |  |
| `cmd/server/backup.go` | 260/260 | 1: cdc89a62 fix: validate runtime configuration strictly | cmd/server/api_v1.go (2); cmd/server/types.go (2); cmd/server/external_subscriptions.go (1); cmd/server/handlers.go (1); cmd/server/helpers.go (1) |
| `internal/storage/migrations_data.go` | 505/505 | 0:  |  |
| `internal/sources/sync.go` | 458/458 | 0:  |  |
| `internal/delivery/capabilities.go` | 121/121 | 0:  |  |
| `internal/delivery/entry.go` | 168/168 | 0:  |  |
| `internal/delivery/headers.go` | 50/50 | 0:  |  |
| `internal/delivery/identity.go` | 99/99 | 0:  |  |
| `internal/delivery/informational.go` | 158/158 | 0:  |  |
| `internal/delivery/mihomo.go` | 231/231 | 0:  |  |
| `internal/delivery/plain.go` | 117/117 | 0:  |  |
| `internal/delivery/render.go` | 3693/3693 | 0:  | cmd/server/configuration.go (1); cmd/server/handlers.go (1); cmd/server/helpers.go (1); cmd/server/keys.go (1); cmd/server/subscription_delivery_settings.go (1) |
| `internal/delivery/shadowsocks.go` | 64/64 | 0:  |  |
| `internal/delivery/singbox.go` | 219/219 | 0:  |  |
| `internal/delivery/split.go` | 58/58 | 0:  |  |
| `internal/delivery/structured.go` | 144/144 | 0:  |  |
| `internal/delivery/validate.go` | 171/171 | 0:  |  |
| `internal/delivery/xray.go` | 228/228 | 0:  |  |
| `cmd/server/subscription_page.go` | 1306/1306 | 0:  | cmd/server/main.go (1); cmd/server/subscription_generation.go (1); internal/profiles/hysteria2.go (1); internal/profiles/tuic.go (1); internal/security/profilestorage/keyring.go (1) |
| `frontend/src/app/subscription/pageConfig.ts` | 420/420 | 0:  |  |
| `frontend/src/app/subscription/page.tsx` | 763/628 | 1: 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc. |  |
| `cmd/server/handlers.go` | 9482/6473 | 4: 5584ad09 fix: hash admin sessions at rest and deny unscoped token writes; 25cba98b fix: polish admin routing, health checks, and ui; 9b44b2fe added panel settings modal and context for managing panel configurations, fix some docker bugs, ssl sert autorenew and etc.; 037674eb fix bugs + info-keys in subscription + changes UX\UI | cmd/server/helpers.go (9); cmd/server/repository.go (8); cmd/server/auth.go (7); cmd/server/main.go (7); cmd/server/types.go (7) |
| `internal/profiles/uri.go` | 581/581 | 0:  | internal/profiles/doc.go (1); internal/profiles/errors.go (1); internal/profiles/fingerprint.go (1); internal/profiles/hysteria2.go (1); internal/profiles/legacy.go (1) |
| `internal/profiles/tuic.go` | 1063/1063 | 0:  | internal/profiles/hysteria2.go (2); cmd/server/main.go (1); cmd/server/subscription_generation.go (1); cmd/server/subscription_page.go (1); internal/security/profilestorage/keyring.go (1) |
| `internal/security/password/hasher.go` | 299/299 | 0:  |  |

## Codecov intersections with high-priority functions

Counts below count reported lines in a finding/function range; unreported comments/declarations are not misses. Partial lines are kept distinct. No branch coverage was exposed (`branches: 0`). Frontend has no file/line coverage in the retrieved report. Gaps are interpreted with tests and local profiles in the backlog.

| File / function | Range | Hit / miss / partial | Missing line ranges |
| --- | --- | --- | --- |
| `cmd/server/main.go` / handleCLI | 353–386 | 9/15/1 | 355, 367–369, 371–373, 375–378, 380–381, 383, 385 |
| `cmd/server/main.go` / bootstrapKeyring | 390–421 | 8/9/7 | 395, 399, 402, 406–407, 410, 413–414, 417 |
| `cmd/server/main.go` / openDatabase | 86–117 | 0/19/0 | 86–87, 89–90, 93–95, 97–98, 101–103, 106–108, 110, 115–117 |
| `internal/storage/sqlite.go` / ConfigureSQLitePragmas | 55–84 | 10/6/2 | 60–61, 63–65, 79 |
| `internal/storage/sqlite.go` / CleanupSQLiteSidecars | 87–95 | 3/1/1 | 89 |
| `cmd/server/repository.go` / listUsers | 14–87 | 49/5/5 | 50, 58, 63, 67, 72 |
| `cmd/server/repository.go` / subscriptionAccessAllowed | 449–487 | 25/2/1 | 463, 472 |
| `cmd/server/repository.go` / redeemActivationCode | 489–535 | 0/34/0 | 489–499, 501, 504–505, 508–512, 516–524, 526–528, 530–531, 534 |
| `cmd/server/repository.go` / registerHWID | 537–652 | 53/29/9 | 541, 546, 575, 579–580, 582, 589, 594–609, 611–612, 614, 623, 645, 649 |
| `cmd/server/repository.go` / getSubscriptionSettings | 273–354 | 62/5/5 | 315, 340, 343, 346, 351 |
| `cmd/server/repository.go` / scanUserRow | 91–189 | 75/7/7 | 137, 142, 146, 159, 162, 173, 185 |
| `cmd/server/handlers_users.go` / apiCreateUser | 96–173 | 0/56/0 | 96–100, 103–106, 108, 110–113, 116–119, 122–123, 125–128, 130, 132–141, 144–149, 151, 153, 155–158, 161–164, 166–168, 171–172 |
| `cmd/server/handlers_users.go` / apiUpdateUserSubscription | 226–334 | 0/81/0 | 226–229, 232–235, 238–241, 244–247, 249–252, 255–257, 260–262, 265–268, 271–273, 275–277, 280–284, 286, 289–291, 293–295, 298–301, 304–330, 332–333 |
| `cmd/server/handlers_users.go` / validateCreateUserRequest | 53–94 | 0/28/0 | 53–58, 60–62, 64, 66–67, 69–70, 73–75, 78–79, 81–82, 84–85, 87–88, 90–91, 93 |
| `cmd/server/handlers_users.go` / apiUpdateUserSettings | 374–427 | 0/39/0 | 374–377, 380–383, 386–388, 390–392, 394–396, 399–401, 403–407, 410–414, 416–419, 422–426 |
| `cmd/server/handlers_users.go` / apiUpdateUserSubscription:255 | 255–255 | 0/1/0 | 255 |
| `cmd/server/subscription_admin_v1.go` / updateUserKeyAssignment | 219–271 | 21/8/8 | 228, 239, 244, 247, 250, 254, 260, 268 |
| `cmd/server/subscription_admin_v1.go` / apiV1PatchUserSubscription | 117–217 | 33/30/16 | 120, 124–125, 144–145, 148–149, 153–154, 158, 161–162, 165–166, 169–170, 174–175, 178–179, 182–183, 186–187, 190–191, 194–195, 212–213 |
| `cmd/server/subscription_admin_v1.go` / apiV1UpdateUserKeyAssignment | 273–306 | 0/28/0 | 273–276, 278–281, 283–286, 288–290, 292–304 |
| `cmd/server/subscription_admin_v1.go` / normalizeAbsoluteHTTPURL:30 | 30–30 | 0/1/0 | 30 |
| `cmd/server/subscription_admin_v1.go` / apiV1PatchUserSubscription:168 | 168–168 | 0/0/1 | none |
| `internal/storage/profile_repository.go` / ReorderKeys | 1026–1083 | 25/6/6 | 1029, 1037, 1042, 1066, 1072, 1078 |
| `internal/storage/profile_repository.go` / ListKeyCategories | 778–848 | 30/13/10 | 781, 793, 797, 800, 807, 817, 825, 829, 832–835, 840 |
| `internal/storage/profile_repository.go` / DeleteKeyCategory | 966–1002 | 15/5/5 | 972, 978, 986, 993, 998 |
| `internal/storage/profile_repository.go` / UpdateLegacy | 669–733 | 23/10/10 | 671, 675, 680, 692, 697, 703, 709, 714, 722, 729 |
| `internal/storage/profile_repository.go` / UpdateKeyCategory | 883–964 | 37/11/11 | 894, 900, 905, 910, 919, 925, 933, 941, 944, 949, 953 |
| `internal/storage/profile_repository.go` / ListLegacy | 528–606 | 52/7/7 | 538, 561, 570, 578, 583, 590, 593 |
| `internal/storage/profile_repository.go` / CloneLocal | 392–463 | 24/12/12 | 394, 397, 401, 412, 418, 423, 434, 439, 444, 448, 455, 459 |
| `internal/storage/profile_repository.go` / loadKeyByID | 74–164 | 59/8/8 | 115, 121, 126, 134, 139, 144, 147, 151 |
| `internal/storage/profile_repository.go` / UpdateSourceOwnedMetadata | 334–390 | 21/12/10 | 336, 340, 353, 356, 359, 362, 365–366, 368, 380, 384, 387 |
| `internal/storage/profile_repository.go` / UpdateLocal | 229–293 | 27/9/9 | 231, 235, 239, 245, 267, 272, 278, 284, 289 |
| `internal/storage/profile_repository.go` / CreateLegacy | 608–667 | 18/10/10 | 610, 614, 619, 625, 630, 638, 643, 648, 659, 663 |
| `internal/storage/profile_repository.go` / CreateLocal | 166–227 | 21/10/10 | 168, 172, 177, 183, 188, 200, 205, 210, 214, 223 |
| `internal/storage/profile_repository.go` / loadLocalKeyForUpdate | 298–329 | 19/3/3 | 313, 322, 326 |
| `internal/storage/profile_repository.go` / cloneClientDisplayName:517 | 517–517 | 1/0/0 | none |
| `internal/storage/profile_repository.go` / UpdateLegacy:701 | 701–701 | 1/0/0 | none |
| `internal/keymanagement/update.go` / UpdateLocal | 174–247 | 37/10/5 | 190–192, 194–195, 197, 204, 207, 212, 242 |
| `internal/keymanagement/update.go` / updateSourceOwnedMetadata | 291–334 | 26/3/3 | 316, 327, 330 |
| `internal/keymanagement/update.go` / ApplyStructuredPatchToURI | 12–50 | 3/24/0 | 16–18, 21–23, 25, 27–30, 32–35, 37–40, 42, 45–47, 49 |
| `internal/keymanagement/update.go` / resolvePatchMode | 251–270 | 3/7/4 | 254–257, 261, 264, 267 |
| `internal/keymanagement/update.go` / updateSourceOwnedMetadata:302 | 302–308 | 7/0/0 | none |
| `internal/keymanagement/update.go` / ApplyStructuredPatchToURI:13 | 13–13 | 1/0/0 | none |
| `internal/keymanagement/update.go` / UpdateLocal:203 | 203–203 | 0/0/1 | none |
| `internal/keymanagement/update.go` / resolveUpdatedURI | 274–289 | 7/2/2 | 278, 286 |
| `cmd/server/subscription_rules.go` / validateResponseRuleInput | 128–169 | 16/7/7 | 132, 135, 139, 143, 147, 150, 158 |
| `cmd/server/subscription_rules.go` / responseRuleMatches | 251–269 | 11/1/0 | 261 |
| `cmd/server/subscription_rules.go` / saveResponseRule | 521–578 | 9/36/3 | 524–525, 529–530, 536–537, 543–544, 546–555, 557–560, 562–570, 572–574, 576–577 |
| `cmd/server/subscription_rules.go` / ruleConditionMatches | 212–249 | 15/16/2 | 222–223, 226–235, 239, 243, 246–247 |
| `cmd/server/subscription_rules.go` / normalizeResponseRuleCondition | 173–195 | 11/4/2 | 179, 183–184, 187 |
| `cmd/server/subscription_rules.go` / validateTemplateInput | 108–126 | 7/4/3 | 111, 115, 119, 125 |
| `cmd/server/subscription_rules.go` / validateTemplateInput:121 | 121–121 | 1/0/0 | none |
| `cmd/server/subscription_rules.go` / safeCustomResponseHeader:198 | 198–198 | 0/0/1 | none |
| `cmd/server/subscription_rules.go` / normalizeResponseRuleCondition:177 | 177–177 | 1/0/0 | none |
| `cmd/server/subscription_rules.go` / saveResponseRule:543 | 543–543 | 0/1/0 | 543 |
| `cmd/server/handlers_subscription.go` / apiActivateSubscription | 20–58 | 0/28/0 | 20–24, 27–30, 33–36, 38–40, 42–43, 46–51, 54–57 |
| `cmd/server/handlers_subscription.go` / handleSubscription | 62–111 | 23/11/4 | 65–66, 71–73, 82–84, 104–106 |
| `cmd/server/handlers_subscription.go` / sanitizeSubscriptionFilenamePart | 169–196 | 13/6/2 | 172, 182–185, 193 |
| `cmd/server/handlers_subscription.go` / renderSubscriptionBrowserPage | 337–391 | 0/41/0 | 337–341, 343–345, 348–352, 356–358, 361–366, 369–372, 375–383, 385–390 |
| `cmd/server/handlers_subscription.go` / encodeSubscriptionBody | 116–140 | 11/11/0 | 123–125, 129–136 |
| `cmd/server/handlers_subscription.go` / apiGetSubscriptionInfo | 410–461 | 0/38/0 | 410–415, 417–420, 422–425, 427–429, 432–435, 438–443, 445–448, 450–452, 455–456, 459–460 |
| `cmd/server/handlers_subscription.go` / serveSubscriptionBody | 288–321 | 18/5/4 | 291–292, 296–297, 315 |
| `cmd/server/handlers_subscription.go` / applySubscriptionResponseHeaders | 206–242 | 17/3/3 | 229, 234, 237 |
| `cmd/server/handlers_subscription.go` / handleSubscription:81 | 81–81 | 0/0/1 | none |
| `cmd/server/handlers_subscription.go` / handleSubscription:98 | 98–98 | 1/0/0 | none |
| `cmd/server/subscription_delivery.go` / prepareSubscriptionDelivery | 156–219 | 29/10/6 | 164, 168, 174–175, 180–181, 188, 191, 194, 213 |
| `cmd/server/subscription_delivery.go` / subscriptionDeviceAllowed | 114–151 | 23/1/1 | 141 |
| `cmd/server/subscription_delivery.go` / loadSubscriptionContext | 43–89 | 31/3/3 | 47, 62, 76 |
| `cmd/server/jobs.go` / apiV1RetryJob | 524–572 | 0/40/0 | 524–527, 529–533, 535–537, 539–541, 543–551, 553–557, 559–563, 565–569, 571 |
| `cmd/server/jobs.go` / decodeKeyHealthCheckJobCounts:290 | 290–290 | 1/0/0 | none |
| `cmd/server/sources_v1.go` / apiV1UpdateSource | 414–503 | 30/28/14 | 417, 421–422, 425–426, 430–431, 440–441, 446–447, 451–452, 459, 463–464, 477–479, 481–482, 486–487, 491, 494–495, 498–499 |
| `cmd/server/sources_v1.go` / apiV1CreateSource | 295–412 | 62/25/11 | 298–299, 303–304, 308–309, 332–333, 339–340, 344–345, 349–350, 354–355, 367–369, 371–372, 396–397, 401–402 |
| `cmd/server/sources_v1.go` / maskExternalSourceURL:66 | 66–66 | 0/0/1 | none |
| `cmd/server/sources_v1.go` / parseOrFetchSource | 229–237 | 5/1/0 | 236 |
| `cmd/server/external_subscriptions.go` / listExternalSourceCategories | 70–139 | 0/49/0 | 70–73, 75, 77–82, 84–86, 88–89, 91–92, 95–101, 103, 105–109, 111–113, 115, 117–118, 121–123, 126–127, 130–134, 136, 138 |
| `cmd/server/external_subscriptions.go` / apiRenameExternalSourceCategory | 317–396 | 0/60/0 | 317–321, 324–328, 330–334, 337–341, 343–347, 349–351, 354–357, 359, 361–369, 372–381, 384–387, 390–392, 395 |
| `cmd/server/auth.go` / apiTokenAllows | 173–214 | 19/0/0 | none |
| `cmd/server/auth.go` / requireAdmin | 20–44 | 18/0/0 | none |
| `cmd/server/auth.go` / apiTokenSessionFromRequest:157 | 157–157 | 0/0/1 | none |
| `cmd/server/auth.go` / requireSuperAdmin | 46–70 | 9/6/3 | 50–51, 58–59, 64–65 |
| `cmd/server/auth.go` / writeAdminAuthError | 72–78 | 4/1/0 | 77 |

## Test inventory

48 Go test files, 32 Vitest files, one Playwright spec with 20 desktop/mobile results, additional fixtures/harnesses and Docker smoke scripts. Codecov Test Analytics: 803 records for the SHA, 798 pass/5 skip. Existing test source is listed so each backlog plan can be traced to protection. This is not a claim of complete branch or endpoint coverage.

### cmd/server/

- `cmd/server/admin_credentials_test.go`
- `cmd/server/api_tokens_test.go`
- `cmd/server/api_v1_integration_test.go`
- `cmd/server/authorization_matrix_test.go`
- `cmd/server/backup_test.go`
- `cmd/server/characterization_test.go`
- `cmd/server/configuration_test.go`
- `cmd/server/external_profiles_test.go`
- `cmd/server/external_subscriptions_security_test.go`
- `cmd/server/happ_routing_test.go`
- `cmd/server/helpers_test.go`
- `cmd/server/jobs_test.go`
- `cmd/server/key_routes_test.go`
- `cmd/server/keys_test.go`
- `cmd/server/migration_existing_db_test.go`
- `cmd/server/runtime_configuration_test.go`
- `cmd/server/server_encryption_test.go`
- `cmd/server/sources_v1_test.go`
- `cmd/server/subscription_core_test.go`
- `cmd/server/subscription_delivery_settings_test.go`
- `cmd/server/subscription_external_validation_test.go`
- `cmd/server/subscription_generation_test.go`
- `cmd/server/subscription_rules_test.go`

### internal/

- `internal/httpapi/key_administration_test.go`
- `internal/httpapi/key_categories_test.go`
- `internal/httpapi/key_profile_test.go`
- `internal/httpapi/key_queries_test.go`
- `internal/httpapi/legacy_keys_test.go`
- `internal/keymanagement/boundaries_test.go`
- `internal/keymanagement/categories_test.go`
- `internal/keymanagement/create_test.go`
- `internal/keymanagement/keymanagement_test.go`
- `internal/keymanagement/legacy_test.go`
- `internal/middleware/middleware_test.go`
- `internal/model/model_test.go`
- `internal/platform/configuration/configuration_test.go`
- `internal/profileconfig/profileconfig_test.go`
- `internal/profiles/adversarial_test.go`
- `internal/profiles/fuzz_test.go`
- `internal/profiles/profiles_test.go`
- `internal/security/password/password_test.go`
- `internal/security/profilestorage/permissions_windows_test.go`
- `internal/security/profilestorage/profilestorage_test.go`
- `internal/sources/fetch_test.go`
- `internal/storage/legacy_test.go`
- `internal/storage/migrations_test.go`
- `internal/storage/profile_repository_test.go`
- `internal/storage/storage_test.go`

### frontend/

- `frontend/e2e/admin-smoke.spec.ts`
- `frontend/src/app/admin/overview/page.test.tsx`
- `frontend/src/app/admin/response-rules/page.test.tsx`
- `frontend/src/app/admin/settings/subscription/page.test.tsx`
- `frontend/src/app/admin/sources/page.test.tsx`
- `frontend/src/app/subscription/NumericGlobe.test.tsx`
- `frontend/src/app/subscription/SubscriptionBlocks.test.tsx`
- `frontend/src/components/admin/GlobalSubscriptionSettingsModal.test.tsx`
- `frontend/src/components/admin/HwidManager.test.tsx`
- `frontend/src/components/admin/KeyEditorModal.test.tsx`
- `frontend/src/components/admin/KeyHealthBadge.test.tsx`
- `frontend/src/components/admin/KeysSection.health.test.tsx`
- `frontend/src/components/admin/KeysSection.test.ts`
- `frontend/src/components/admin/RoutingSettingsModal.test.tsx`
- `frontend/src/components/admin/SourceCreateDrawer.test.tsx`
- `frontend/src/components/admin/StatCard.test.tsx`
- `frontend/src/components/admin/keyTemplateVariables.test.ts`
- `frontend/src/components/admin/protocol-editor-capabilities.test.ts`
- `frontend/src/components/ui/AppleEmojiInput.test.tsx`
- `frontend/src/components/ui/EmojiPickerButton.test.tsx`
- `frontend/src/components/ui/EmojiText.test.tsx`
- `frontend/src/components/ui/Input.test.tsx`
- `frontend/src/components/ui/Modal.test.tsx`
- `frontend/src/components/ui/Technical.test.tsx`
- `frontend/src/components/ui/Toast.test.tsx`
- `frontend/src/lib/adminPassword.test.ts`
- `frontend/src/lib/api-keys-privacy.test.ts`
- `frontend/src/lib/configuration.test.ts`
- `frontend/src/lib/emoji-assets.test.ts`
- `frontend/src/lib/emoji-popover.test.ts`
- `frontend/src/lib/external-import-messages.test.ts`
- `frontend/src/lib/externalSourceName.test.ts`
- `frontend/src/lib/response-rules.test.ts`

## CodeScene category descriptions

The following are analyzer-provided descriptions, deduplicated verbatim by description text. Encoding replacement characters in CLI text are retained; they have no semantic significance. Indication values below are recorded tool values, not interpreted as a cross-category severity ranking.

### D01: Bumpy Road Ahead

A Bumpy Road is a function that contains multiple chunks of nested conditional logic inside the same function. The deeper the nesting and the more bumps, the lower the code health.

A bumpy code road represents a lack of encapsulation which becomes an obstacle to comprehension. In imperative languages there�s also an increased risk for feature entanglement, which leads to complex state management. CodeScene considers the following rules for the code health impact: 1) The deeper the nested conditional logic of each bump, the higher the tax on our working memory. 2) The more bumps inside a function, the more expensive it is to refactor as each bump represents a missing abstraction. 3) The larger each bump � that is, the more lines of code it spans � the harder it is to build up a mental model of the function. The nesting depth for what is considered a bump is 2 levels of conditionals.

### D02: Complex Method

A Complex Method has a high cyclomatic complexity. The recommended threshold for the Go language is a cyclomatic complexity lower than 9.

Severity: Brain Method - Complex Method - Long Method.

### D03: Excess Number of Function Arguments

Functions with many arguments indicate either a) low cohesion where the function has too many responsibilities, or b) a missing abstraction that encapsulates those arguments.

The threshold for the Go language is 4 function arguments.

### D04: Complex Conditional

A complex conditional is an expression inside a branch such as an <code>if</code>-statmeent which consists of multiple, logical operations. Example: <code>if (x.started() && y.running())</code>.Complex conditionals make the code even harder to read, and contribute to the Complex Method code smell. Encapsulate them.

### D05: Code Duplication

Duplicated code often leads to code that's harder to change since the same logical change has to be done in multiple functions. More duplication gives lower code health.

### D06: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 55 % of all function arguments are string types.

### D07: Overall Code Complexity

Overall Code Complexity is measured by the mean cyclomatic complexity across all functions in the file. The lower the number, the better.

Cyclomatic complexity is a function level metric that measures the number of logical branches (if-else, loops, etc.). Cyclomatic complexity is a rough complexity measure, but useful as a way of estimating the minimum number of unit tests you would need. As such, prefer functions with low cyclomatic complexity (2-3 branches).

### D08: Large Method

Overly long functions make the code harder to read. The recommended maximum function length for the Go language is 80 lines of code. Severity: Brain Method - Complex Method - Long Method.

### D09: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 41 % of all function arguments are string types.

### D10: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 65 % of all function arguments are string types.

### D11: Low Cohesion

Cohesion is a measure of how well the elements in a file belong together. CodeScene measures cohesion using the LCOM4 metric (Lack of Cohesion Measure). With LCOM4, the functions inside a module are related if a) they access the same data members, or b) they call each other. High Cohesion is desirable as it means that all functions are related and likely to represent the same responsibility. Low Cohesion is problematic since it means that the module contains multiple behaviors. Low Cohesion leads to code that's harder to understand, requires more tests, and very often become a coordination magnet for developers.

### D12: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 54 % of all function arguments are string types.

### D13: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 41 % of all functions have primitive types as arguments.

### D14: Complex Method

A Complex Method has a high cyclomatic complexity. The recommended threshold for the React language is a cyclomatic complexity lower than 10.

Severity: Brain Method - Complex Method - Long Method.

### D15: Large Method

Overly long functions make the code harder to read. The recommended maximum function length for the React language is 120 lines of code. Severity: Brain Method - Complex Method - Long Method.

### D16: Excess Number of Function Arguments

Functions with many arguments indicate either a) low cohesion where the function has too many responsibilities, or b) a missing abstraction that encapsulates those arguments.

The threshold for the React language is 4 function arguments.

### D17: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 63 % of all functions have primitive types as arguments.

### D18: Complex Method

A Complex Method has a high cyclomatic complexity. The recommended threshold for the TypeScript language is a cyclomatic complexity lower than 9.

Severity: Brain Method - Complex Method - Long Method.

### D19: Lines of Code in a Single File

This module has 1315 lines of code (comments stripped away). This puts the module at risk of evolving into a Brain Class. Brain Classes are problematic since changes become more complex over time, harder to test, and challenging to refactor. Act now to prevent future maintenance issues.

### D20: Deep, Nested Complexity

Deep nested logic means that you have control structures like if-statements or loops inside other control structures. Deep nested logic increases the cognitive load on the programmer reading the code. The human working memory has a maximum capacity of 3-4 items; beyond that threshold, we struggle with keeping things in our head. Consequently, deep nested logic has a strong correlation to defects and accounts for roughly 20% of all programming mistakes.

CodeScene measures the maximum nesting depth inside each function. The deeper the nesting, the lower the code health. The threshold for the TypeScript language is 4 levels of nesting.

### D21: Large Method

Overly long functions make the code harder to read. The recommended maximum function length for the TypeScript language is 70 lines of code. Severity: Brain Method - Complex Method - Long Method.

### D22: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 47 % of all functions have primitive types as arguments.

### D23: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 47 % of all function arguments are string types.

### D24: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 53 % of all function arguments are string types.

### D25: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 52 % of all function arguments are string types.

### D26: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 44 % of all function arguments are string types.

### D27: Deep, Nested Complexity

Deep nested logic means that you have control structures like if-statements or loops inside other control structures. Deep nested logic increases the cognitive load on the programmer reading the code. The human working memory has a maximum capacity of 3-4 items; beyond that threshold, we struggle with keeping things in our head. Consequently, deep nested logic has a strong correlation to defects and accounts for roughly 20% of all programming mistakes.

CodeScene measures the maximum nesting depth inside each function. The deeper the nesting, the lower the code health. The threshold for the Go language is 4 levels of nesting.

### D28: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 89 % of all functions have primitive types as arguments.

### D29: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 56 % of all function arguments are string types.

### D30: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 77 % of all functions have primitive types as arguments.

### D31: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 67 % of all function arguments are string types.

### D32: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 100 % of all functions have primitive types as arguments.

### D33: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 96 % of all function arguments are string types.

### D34: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 45 % of all functions have primitive types as arguments.

### D35: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 40 % of all function arguments are string types.

### D36: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 60 % of all function arguments are string types.

### D37: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 54 % of all functions have primitive types as arguments.

### D38: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 39 % of all function arguments are string types.

### D39: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 48 % of all function arguments are string types.

### D40: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 59 % of all function arguments are string types.

### D41: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 91 % of all functions have primitive types as arguments.

### D42: String Heavy Function Arguments

String is a generic type that fail to capture the constraints of the domain object it represents. In this module, 83 % of all function arguments are string types.

### D43: Primitive Obsession

Code that uses a high degree of built-in, primitives such as integers, strings, floats, lacks a domain language that encapsulates the validation and semantics of function arguments. Primitive Obsession has several consequences: 1) In a statically typed language, the compiler will detect less erroneous assignments. 2) Security impact since the possible value range of a variable/argument isn't retricted.

In this module, 58 % of all functions have primitive types as arguments.

### D44: Global Conditionals

There's global code outside of functions that contain complex constructs. Consider to encapsulate those business rules in named functions.

### D45: Deep, Global Nested Complexity

There's global code outside of functions that contain deep, nested complexity (4 levels).

## Complete CodeScene findings

All successfully reviewed files with findings appear below, including supplemental configuration/operational inputs. Functions are reported with one-based start/end lines at the audited SHA and the exact detail measurements. File-level categories may have no function range. Clean or null-scored inputs remain in the metrics tables. Low cohesion and repeated complexity require semantic review; acceptable grammar/security/framework cases are identified in the main audit.

### cmd/server/admin_credentials.go — 9.24

- **Bumpy Road Ahead**; indication 3; description D01.
  - ensureBootstrapOwner: 27–87; bumps = 2.
  - authenticateAdministrator: 92–128; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - ensureBootstrapOwner: 27–87; cc = 13.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - ensureBootstrapOwner: 27–87; Arguments = 5.

### cmd/server/api_tokens.go — 9.68

- **Complex Method**; indication 2; description D02.
  - apiV1CreateAPIToken: 96–153; cc = 12.

### cmd/server/api_v1.go — 8.95

- **Complex Method**; indication 2; description D02.
  - apiV1ListUsers: 171–205; cc = 11.
  - apiV1Dashboard: 76–156; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - apiV1ListUsers:184: 184–184; 3 complex conditional expressions.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - recordAuditEvent: 287–293; Arguments = 5.
  - recordAuditEventForActor: 300–312; Arguments = 6.

### cmd/server/auth.go — 8.24

- **Complex Method**; indication 2; description D02.
  - apiTokenAllows: 173–214; cc = 10.
  - requireAdmin: 20–44; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - apiTokenSessionFromRequest:157: 157–157; 2 complex conditional expressions.
- **Code Duplication**; indication 2; description D05.
  - requireAdmin: 20–44; no additional detail.
  - requireSuperAdmin: 46–70; no additional detail.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - writeAdminAuthError: 72–78; Arguments = 5.
- **String Heavy Function Arguments**; indication 2; description D06.

### cmd/server/backup.go — 9.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - performBackup: 34–79; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - performBackup: 34–79; cc = 13.

### cmd/server/external_subscriptions.go — 9.40

- **Bumpy Road Ahead**; indication 3; description D01.
  - listExternalSourceCategories: 70–139; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - apiRenameExternalSourceCategory: 317–396; cc = 14.
  - listExternalSourceCategories: 70–139; cc = 14.

### cmd/server/handlers_admins.go — 9.28

- **Bumpy Road Ahead**; indication 3; description D01.
  - apiUpdateAdmin: 100–157; bumps = 2.
  - apiDeleteAdmin: 241–309; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - apiDeleteAdmin: 241–309; cc = 12.
  - apiUpdateAdmin: 100–157; cc = 10.
  - apiCreateAdmin: 45–98; cc = 9.

### cmd/server/handlers_auth.go — 9.68

- **Complex Method**; indication 2; description D02.
  - apiLogin: 16–64; cc = 9.

### cmd/server/handlers_settings.go — 8.59

- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - apiUpdateSubscriptionSettings: 103–212; cc = 17.
- **Complex Conditional**; indication 2; description D04.
  - normalizeSettingsURL:70: 70–70; 2 complex conditional expressions.
- **Large Method**; indication 2; description D08.
  - apiUpdateSubscriptionSettings: 103–212; LoC = 101 lines.

### cmd/server/handlers_subscription.go — 7.56

- **Bumpy Road Ahead**; indication 3; description D01.
  - apiActivateSubscription: 20–58; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - handleSubscription: 62–111; cc = 16.
  - apiActivateSubscription: 20–58; cc = 12.
  - sanitizeSubscriptionFilenamePart: 169–196; cc = 11.
  - renderSubscriptionBrowserPage: 337–391; cc = 10.
  - encodeSubscriptionBody: 116–140; cc = 10.
  - apiGetSubscriptionInfo: 410–461; cc = 9.
  - serveSubscriptionBody: 288–321; cc = 9.
  - applySubscriptionResponseHeaders: 206–242; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - handleSubscription:81: 81–81; 3 complex conditional expressions.
  - handleSubscription:98: 98–98; 2 complex conditional expressions.

### cmd/server/handlers_users.go — 8.00

- **Bumpy Road Ahead**; indication 3; description D01.
  - apiCreateUser: 96–173; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - apiUpdateUserSubscription: 226–334; cc = 18.
  - apiCreateUser: 96–173; cc = 14.
  - validateCreateUserRequest: 53–94; cc = 11.
  - apiUpdateUserSettings: 374–427; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - apiUpdateUserSubscription:255: 255–255; 2 complex conditional expressions.
- **Large Method**; indication 2; description D08.
  - apiUpdateUserSubscription: 226–334; LoC = 98 lines.

### cmd/server/happ_routing.go — 9.24

- **Bumpy Road Ahead**; indication 3; description D01.
  - validateHappRoutingField: 78–108; bumps = 4.
  - validateHappRoutingConfig: 47–74; bumps = 3.
- **Complex Method**; indication 2; description D02.
  - validateHappRoutingField: 78–108; cc = 13.
  - validateHappRoutingConfig: 47–74; cc = 11.
- **Complex Conditional**; indication 2; description D04.
  - validateHappRoutingConfig:69: 69–69; 2 complex conditional expressions.

### cmd/server/helpers.go — 9.31

- **Bumpy Road Ahead**; indication 3; description D01.
  - requestValue: 27–39; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - resolveBaseURL: 155–185; cc = 12.
- **String Heavy Function Arguments**; indication 2; description D09.

### cmd/server/hwid_parser.go — 9.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - extractValue: 47–59; bumps = 2.
- **String Heavy Function Arguments**; indication 2; description D10.

### cmd/server/jobs.go — 9.38

- **Complex Method**; indication 2; description D02.
  - apiV1RetryJob: 524–572; cc = 12.
- **Complex Conditional**; indication 2; description D04.
  - decodeKeyHealthCheckJobCounts:290: 290–290; 2 complex conditional expressions.

### cmd/server/main.go — 9.57

- **Complex Method**; indication 2; description D02.
  - handleCLI: 353–386; cc = 13.
  - bootstrapKeyring: 390–421; cc = 10.

### cmd/server/repository.go — 6.33

- **Low Cohesion**; indication 3; description D11.
- **Bumpy Road Ahead**; indication 3; description D01.
  - listUsers: 14–87; bumps = 2.
  - subscriptionAccessAllowed: 449–487; bumps = 2.
  - redeemActivationCode: 489–535; bumps = 2.
  - registerHWID: 537–652; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - getSubscriptionSettings: 273–354; cc = 15.
  - registerHWID: 537–652; cc = 14.
  - scanUserRow: 91–189; cc = 13.
  - subscriptionAccessAllowed: 449–487; cc = 10.
  - listUsers: 14–87; cc = 10.
  - redeemActivationCode: 489–535; cc = 9.
- **Large Method**; indication 2; description D08.
  - registerHWID: 537–652; LoC = 105 lines.
  - scanUserRow: 91–189; LoC = 99 lines.
  - getSubscriptionSettings: 273–354; LoC = 80 lines.

### cmd/server/sources_v1.go — 7.79

- **Bumpy Road Ahead**; indication 3; description D01.
  - apiV1UpdateSource: 414–503; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - apiV1UpdateSource: 414–503; cc = 21.
  - apiV1CreateSource: 295–412; cc = 18.
- **Complex Conditional**; indication 2; description D04.
  - maskExternalSourceURL:66: 66–66; 2 complex conditional expressions.
- **Large Method**; indication 2; description D08.
  - apiV1CreateSource: 295–412; LoC = 117 lines.
  - apiV1UpdateSource: 414–503; LoC = 90 lines.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - parseOrFetchSource: 229–237; Arguments = 5.

### cmd/server/subscription_admin_v1.go — 7.67

- **Bumpy Road Ahead**; indication 3; description D01.
  - updateUserKeyAssignment: 219–271; bumps = 3.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - apiV1PatchUserSubscription: 117–217; cc = 20.
  - updateUserKeyAssignment: 219–271; cc = 16.
  - apiV1UpdateUserKeyAssignment: 273–306; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - normalizeAbsoluteHTTPURL:30: 30–30; 4 complex conditional expressions.
  - apiV1PatchUserSubscription:168: 168–168; 2 complex conditional expressions.
- **Large Method**; indication 2; description D08.
  - apiV1PatchUserSubscription: 117–217; LoC = 97 lines.

### cmd/server/subscription_crypto.go — 9.68

- **Complex Method**; indication 2; description D02.
  - encryptSubscriptionURL: 20–64; cc = 10.

### cmd/server/subscription_delivery_settings.go — 9.58

- **Bumpy Road Ahead**; indication 3; description D01.
  - validateSubscriptionDeliverySettings: 44–76; bumps = 3.
- **Complex Method**; indication 2; description D02.
  - apiV1UpdateSubscriptionDeliverySettings: 115–167; cc = 9.
  - validateSubscriptionDeliverySettings: 44–76; cc = 9.

### cmd/server/subscription_delivery.go — 8.57

- **Bumpy Road Ahead**; indication 3; description D01.
  - prepareSubscriptionDelivery: 156–219; bumps = 3.
  - subscriptionDeviceAllowed: 114–151; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - prepareSubscriptionDelivery: 156–219; cc = 19.
  - loadSubscriptionContext: 43–89; cc = 11.
  - subscriptionDeviceAllowed: 114–151; cc = 10.

### cmd/server/subscription_generation.go — 9.38

- **Complex Method**; indication 2; description D02.
  - selectSubscriptionEntries: 38–96; cc = 14.
- **String Heavy Function Arguments**; indication 2; description D12.

### cmd/server/subscription_page.go — 8.09

- **Bumpy Road Ahead**; indication 3; description D01.
  - activationCopyLabels: 485–505; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - safePageURL: 307–327; cc = 14.
- **Complex Conditional**; indication 2; description D04.
  - safePageURL:323: 323–323; 3 complex conditional expressions.
  - cssVar:299: 299–299; 2 complex conditional expressions.
  - safePageURL:319: 319–319; 2 complex conditional expressions.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - renderSubscriptionPageHTML: 930–959; Arguments = 7.
- **Primitive Obsession**; indication 2; description D13.

### cmd/server/subscription_rules.go — 6.50

- **Low Cohesion**; indication 3; description D11.
- **Bumpy Road Ahead**; indication 3; description D01.
  - validateResponseRuleInput: 128–169; bumps = 2.
  - responseRuleMatches: 251–269; bumps = 2.
  - saveResponseRule: 521–578; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - saveResponseRule: 521–578; cc = 15.
  - validateResponseRuleInput: 128–169; cc = 15.
  - ruleConditionMatches: 212–249; cc = 13.
  - normalizeResponseRuleCondition: 173–195; cc = 10.
  - validateTemplateInput: 108–126; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - validateTemplateInput:121: 121–121; 4 complex conditional expressions.
  - safeCustomResponseHeader:198: 198–198; 3 complex conditional expressions.
  - normalizeResponseRuleCondition:177: 177–177; 2 complex conditional expressions.
  - saveResponseRule:543: 543–543; 2 complex conditional expressions.

### frontend/src/app/admin/audit/page.tsx — 9.31

- **Complex Method**; indication 2; description D14.
  - AuditPage: 11–138; cc = 13.
- **Large Method**; indication 2; description D15.
  - AuditPage: 11–138; LoC = 124 lines.

### frontend/src/app/admin/keys/page.tsx — 9.68

- **Complex Method**; indication 2; description D14.
  - KeysPage: 11–71; cc = 10.

### frontend/src/app/admin/overview/page.tsx — 9.06

- **Complex Method**; indication 2; description D14.
  - OverviewPage: 44–223; cc = 27.
- **Large Method**; indication 2; description D15.
  - OverviewPage: 44–223; LoC = 173 lines.

### frontend/src/app/admin/response-rules/page.tsx — 9.68

- **Complex Conditional**; indication 2; description D04.
  - ResponseRulesPage.remove:122: 122–122; 2 complex conditional expressions.

### frontend/src/app/admin/settings/subscription/page.tsx — 8.08

- **Complex Method**; indication 2; description D14.
  - SubscriptionSettingsPage: 57–394; cc = 28.
- **Complex Conditional**; indication 2; description D04.
  - SubscriptionSettingsPage:135: 135–135; 2 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - SubscriptionSettingsPage: 57–394; LoC = 321 lines.

### frontend/src/app/admin/sources/page.tsx — 8.32

- **Complex Method**; indication 2; description D14.
  - SourcesPage: 51–400; cc = 32.
- **Large Method**; indication 2; description D15.
  - SourcesPage: 51–400; LoC = 325 lines.

### frontend/src/app/admin/templates/page.tsx — 9.68

- **Complex Conditional**; indication 2; description D04.
  - TemplatesPage.remove:87: 87–87; 2 complex conditional expressions.

### frontend/src/app/admin/users/page.tsx — 8.39

- **Complex Method**; indication 2; description D14.
  - UsersPage: 34–356; cc = 56.
- **Large Method**; indication 2; description D15.
  - UsersPage: 34–356; LoC = 310 lines.

### frontend/src/app/subscription/NumericGlobe.tsx — 9.68

- **Complex Conditional**; indication 2; description D04.
  - NumericGlobe (top-level context):83: 83–83; 2 complex conditional expressions.

### frontend/src/app/subscription/page.tsx — 9.53

- **Bumpy Road Ahead**; indication 3; description D01.
  - getDefaultActivationBlock: 41–64; bumps = 2.
- **Complex Method**; indication 2; description D14.
  - SubscriptionPage (top-level context): 264–325; cc = 10.

### frontend/src/app/subscription/SubscriptionBlocks.tsx — 8.45

- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D14.
  - detectDevicePlatform: 39–63; cc = 16.
  - LinkButtonsBlock.getButtonsForPlatform: 230–248; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - detectDevicePlatform:54: 54–54; 2 complex conditional expressions.
  - recommendedToneByButtonID:159: 159–159; 2 complex conditional expressions.

### frontend/src/components/admin/BulkEditUsersModal.tsx — 9.68

- **Complex Method**; indication 2; description D14.
  - BulkEditUsersModal.handleSubmit: 39–92; cc = 11.

### frontend/src/components/admin/DashboardShell.tsx — 8.68

- **Complex Method**; indication 2; description D14.
  - DashboardShell: 79–285; cc = 25.
- **Complex Conditional**; indication 2; description D04.
  - DashboardShell:92: 92–92; 2 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - DashboardShell: 79–285; LoC = 194 lines.

### frontend/src/components/admin/GlobalSubscriptionSettingsModal.tsx — 9.60

- **Complex Method**; indication 2; description D14.
  - GlobalSubscriptionSettingsModal.load: 144–192; cc = 14.

### frontend/src/components/admin/HwidManager.tsx — 8.28

- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D14.
  - HwidManager.inferDeviceKind: 90–99; cc = 14.
- **Complex Conditional**; indication 2; description D04.
  - HwidManager.inferDeviceKind:94: 94–94; 2 complex conditional expressions.
  - HwidManager.inferDeviceKind:95: 95–95; 2 complex conditional expressions.
  - HwidManager.inferDeviceKind:96: 96–96; 2 complex conditional expressions.

### frontend/src/components/admin/KeyCategoryEditorModal.tsx — 8.38

- **Complex Method**; indication 2; description D14.
  - KeyCategoryEditorModal: 36–362; cc = 51.
- **Large Method**; indication 2; description D15.
  - KeyCategoryEditorModal: 36–362; LoC = 312 lines.

### frontend/src/components/admin/KeyEditorModal.tsx — 6.20

- **Bumpy Road Ahead**; indication 3; description D01.
  - KeyEditorModal.handleSubmit: 489–693; bumps = 6.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D14.
  - KeyEditorModal.handleSubmit: 489–693; cc = 117.
- **Complex Conditional**; indication 2; description D04.
  - KeyEditorModal.handleSubmit:656: 656–656; 2 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - KeyEditorModal.handleSubmit: 489–693; LoC = 192 lines.

### frontend/src/components/admin/KeysSection.tsx — 6.71

- **Bumpy Road Ahead**; indication 3; description D01.
  - dragScrollTargetAt: 118–153; bumps = 2.
- **Complex Method**; indication 2; description D14.
  - KeysSection.renderKeyCard: 742–904; cc = 15.
  - KeysSection.renderKeyRows: 906–1030; cc = 12.
  - dragScrollTargetAt: 118–153; cc = 12.
- **Complex Conditional**; indication 2; description D04.
  - dragAutoScrollVelocity:99: 99–99; 3 complex conditional expressions.
  - dragScrollTargetAt:147: 147–147; 3 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - KeysSection.renderKeyCard: 742–904; LoC = 160 lines.
  - KeysSection.renderCategoryBlock: 1060–1190; LoC = 127 lines.
  - KeysSection.renderKeyRows: 906–1030; LoC = 122 lines.
- **Excess Number of Function Arguments**; indication 2; description D16.
  - dragAutoScrollVelocity: 91–116; Arguments = 5.
- **Primitive Obsession**; indication 2; description D17.

### frontend/src/components/admin/PanelSettingsModal.tsx — 8.70

- **Complex Method**; indication 2; description D14.
  - PanelSettingsModal: 45–250; cc = 18.
- **Complex Conditional**; indication 2; description D04.
  - PanelSettingsModal:69: 69–69; 2 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - PanelSettingsModal: 45–250; LoC = 189 lines.

### frontend/src/components/admin/protocol-editors/Hysteria2Fields.tsx — 9.36

- **Complex Method**; indication 2; description D14.
  - buildHysteria2Patch: 110–147; cc = 12.
- **Excess Number of Function Arguments**; indication 2; description D16.
  - buildHysteria2Patch: 110–147; Arguments = 10.

### frontend/src/components/admin/protocol-editors/LegacyXrayFields.tsx — 8.82

- **Complex Method**; indication 2; description D14.
  - LegacyXrayFields: 13–63; cc = 44.

### frontend/src/components/admin/protocol-editors/ShadowsocksFields.tsx — 9.24

- **Bumpy Road Ahead**; indication 3; description D01.
  - buildShadowsocksPatch: 76–114; bumps = 2.
- **Complex Method**; indication 2; description D14.
  - buildShadowsocksPatch: 76–114; cc = 10.
- **Excess Number of Function Arguments**; indication 2; description D16.
  - buildShadowsocksPatch: 76–114; Arguments = 7.

### frontend/src/components/admin/protocol-editors/TuicV5Fields.tsx — 8.59

- **Complex Method**; indication 2; description D14.
  - buildTUICPatch: 162–220; cc = 17.
  - TuicV5Fields: 33–160; cc = 10.
- **Large Method**; indication 2; description D15.
  - TuicV5Fields: 33–160; LoC = 127 lines.
- **Excess Number of Function Arguments**; indication 2; description D16.
  - buildTUICPatch: 162–220; Arguments = 18.

### frontend/src/components/admin/RoutingSettingsModal.tsx — 9.68

- **Complex Method**; indication 2; description D14.
  - normalizeRoutingConfig: 85–99; cc = 11.

### frontend/src/components/admin/UsersSection.tsx — 8.54

- **Bumpy Road Ahead**; indication 3; description D01.
  - UsersSection (top-level context): 56–103; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D14.
  - UsersSection (top-level context): 56–103; cc = 14.
- **Complex Conditional**; indication 2; description D04.
  - UsersSection (top-level context):85: 85–85; 3 complex conditional expressions.

### frontend/src/components/ui/AppleEmojiInput.tsx — 7.47

- **Bumpy Road Ahead**; indication 3; description D01.
  - boundaryOffset: 59–86; bumps = 3.
  - getText: 35–48; bumps = 2.
  - restoreCursor.walk: 110–138; bumps = 2.
  - AppleEmojiInput: 191–400; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D14.
  - AppleEmojiInput: 191–400; cc = 44.
  - getText: 35–48; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - AppleEmojiInput:353: 353–353; 2 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - AppleEmojiInput: 191–400; LoC = 190 lines.

### frontend/src/components/ui/Drawer.tsx — 9.68

- **Complex Method**; indication 2; description D14.
  - Drawer: 6–84; cc = 10.

### frontend/src/components/ui/EmojiPickerButton.tsx — 8.65

- **Complex Method**; indication 2; description D14.
  - EmojiPickerButton: 64–246; cc = 50.
- **Complex Conditional**; indication 2; description D04.
  - EmojiPickerButton:138: 138–138; 3 complex conditional expressions.
- **Large Method**; indication 2; description D15.
  - EmojiPickerButton: 64–246; LoC = 172 lines.

### frontend/src/components/ui/Modal.tsx — 9.84

- **Bumpy Road Ahead**; indication 3; description D01.
  - Modal.handleKeyDown: 73–102; bumps = 3.

### frontend/src/components/ui/Select.tsx — 9.02

- **Complex Method**; indication 2; description D14.
  - Select: 25–224; cc = 27.
- **Large Method**; indication 2; description D15.
  - Select: 25–224; LoC = 181 lines.

### frontend/src/context/PanelSettingsContext.tsx — 9.84

- **Bumpy Road Ahead**; indication 3; description D01.
  - PanelSettingsProvider: 128–224; bumps = 2.

### frontend/src/lib/api.ts — 9.15

- **Complex Method**; indication 2; description D18.
  - request: 78–138; cc = 21.
  - listSummaries: 209–217; cc = 10.

### frontend/src/lib/clipboard.ts — 9.68

- **Complex Method**; indication 2; description D18.
  - copyToClipboard: 1–53; cc = 10.

### frontend/src/lib/configuration.ts — 2.36

- **Lines of Code in a Single File**; indication 2; description D19.
- **Bumpy Road Ahead**; indication 3; description D01.
  - patchXrayJSONConfiguration: 742–1152; bumps = 10.
  - buildDefaultXrayJSONConfig: 490–672; bumps = 3.
  - patchEditableConfiguration: 1283–1337; bumps = 2.
- **Deep, Nested Complexity**; indication 3; description D20.
  - buildDefaultXrayJSONConfig: 490–672; Nesting depth = 4 conditionals.
  - patchXrayJSONConfiguration: 742–1152; Nesting depth = 4 conditionals.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D18.
  - patchXrayJSONConfiguration: 742–1152; cc = 133.
  - parseXrayJSONConfiguration: 312–473; cc = 67.
  - createConfigurationFromXrayJSON: 1339–1410; cc = 60.
  - patchEditableConfiguration: 1283–1337; cc = 47.
  - buildDefaultXrayJSONConfig: 490–672; cc = 45.
  - parseQueryParams: 202–220; cc = 22.
  - createXrayJSONFromConfiguration: 1154–1246; cc = 17.
  - buildConfiguration: 267–310; cc = 12.
  - parseConfiguration: 230–265; cc = 12.
- **Complex Conditional**; indication 2; description D04.
  - patchXrayJSONConfiguration:860: 860–860; 4 complex conditional expressions.
  - patchXrayJSONConfiguration:985: 985–985; 3 complex conditional expressions.
  - patchXrayJSONConfiguration:1006: 1006–1006; 3 complex conditional expressions.
  - asRecord:75: 75–75; 2 complex conditional expressions.
  - portValue:101: 101–101; 2 complex conditional expressions.
  - parseXrayJSONConfiguration:385: 385–385; 2 complex conditional expressions.
  - parseXrayJSONConfiguration:387: 387–387; 2 complex conditional expressions.
  - toPortNumber:484: 484–484; 2 complex conditional expressions.
  - patchXrayJSONConfiguration:919: 919–919; 2 complex conditional expressions.
  - patchXrayJSONConfiguration:965: 965–965; 2 complex conditional expressions.
- **Large Method**; indication 2; description D21.
  - patchXrayJSONConfiguration: 742–1152; LoC = 394 lines.
  - buildDefaultXrayJSONConfig: 490–672; LoC = 180 lines.
  - parseXrayJSONConfiguration: 312–473; LoC = 147 lines.
  - createXrayJSONFromConfiguration: 1154–1246; LoC = 88 lines.
- **Primitive Obsession**; indication 2; description D22.
- **String Heavy Function Arguments**; indication 2; description D23.

### frontend/src/lib/emoji-assets.ts — 9.68

- **Complex Conditional**; indication 2; description D04.
  - getEmojiAssetURL:16: 16–16; 2 complex conditional expressions.

### frontend/src/lib/response-rules.ts — 8.95

- **Complex Method**; indication 2; description D18.
  - parseResponseRuleDraftJSON: 55–90; cc = 26.
  - parseCondition: 29–42; cc = 13.
- **Complex Conditional**; indication 2; description D04.
  - parseResponseRuleDraftJSON:63: 63–63; 2 complex conditional expressions.
  - parseResponseRuleDraftJSON:73: 73–73; 2 complex conditional expressions.

### frontend/src/lib/xray-json-document.ts — 9.68

- **Complex Method**; indication 2; description D18.
  - collectDuplicateKeys: 62–93; cc = 10.

### internal/delivery/entry.go — 9.68

- **String Heavy Function Arguments**; indication 2; description D24.

### internal/delivery/validate.go — 9.68

- **String Heavy Function Arguments**; indication 2; description D25.

### internal/httpapi/key_administration.go — 8.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - CheckAllKeys: 135–188; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - CheckAllKeys: 135–188; cc = 14.
- **Complex Conditional**; indication 2; description D04.
  - CheckAllKeys:150: 150–150; 3 complex conditional expressions.

### internal/httpapi/key_categories.go — 9.09

- **Code Duplication**; indication 2; description D05.
  - CreateCategory: 34–59; no additional detail.
  - UpdateCategory: 61–95; no additional detail.
  - RenameCategory: 97–130; no additional detail.
  - DeleteCategory: 132–153; no additional detail.
  - ReorderCategories: 155–183; no additional detail.

### internal/httpapi/key_profile_errors.go — 9.34

- **Complex Method**; indication 2; description D02.
  - mapServiceError: 10–84; cc = 19.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - mapServiceError: 10–84; Arguments = 5.

### internal/httpapi/key_profiles.go — 9.38

- **Code Duplication**; indication 2; description D05.
  - CreateKeyProfile: 133–165; no additional detail.
  - UpdateKeyProfile: 167–204; no additional detail.

### internal/httpapi/key_queries.go — 9.24

- **Complex Method**; indication 2; description D02.
  - ListKeys: 49–79; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - ListKeys:62: 62–62; 3 complex conditional expressions.

### internal/httpapi/respond.go — 9.68

- **Excess Number of Function Arguments**; indication 2; description D03.
  - WriteV1Error: 57–59; Arguments = 5.
  - WriteFieldError: 63–79; Arguments = 6.

### internal/keymanagement/bulk_health.go — 9.68

- **Excess Number of Function Arguments**; indication 2; description D03.
  - SaveHealthCheckResult: 97–101; Arguments = 5.

### internal/keymanagement/create.go — 7.84

- **Bumpy Road Ahead**; indication 3; description D01.
  - BuildURIFromStructuredCreate: 27–105; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - BuildURIFromStructuredCreate: 27–105; cc = 20.
  - CreateLocal: 124–184; cc = 14.
  - resolveCreationMode: 188–207; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - BuildURIFromStructuredCreate:70: 70–70; 3 complex conditional expressions.
  - setString:108: 108–108; 2 complex conditional expressions.
  - CreateLocal:137: 137–137; 2 complex conditional expressions.
- **String Heavy Function Arguments**; indication 2; description D24.

### internal/keymanagement/detail.go — 9.31

- **Bumpy Road Ahead**; indication 3; description D01.
  - BuildKeyProfileDetailResponse: 115–189; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - BuildKeyProfileDetailResponse: 115–189; cc = 14.
  - SanitizeCheckError: 15–32; cc = 13.
- **Complex Conditional**; indication 2; description D04.
  - BuildKeyProfileDetailResponse:140: 140–140; 2 complex conditional expressions.

### internal/keymanagement/legacy.go — 8.31

- **Bumpy Road Ahead**; indication 3; description D01.
  - CreateLegacy: 27–83; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - CreateLegacy: 27–83; cc = 14.
  - UpdateLegacy: 85–143; cc = 13.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - buildVLESSURL: 180–201; Arguments = 5.
- **String Heavy Function Arguments**; indication 2; description D26.

### internal/keymanagement/reveal.go — 9.68

- **Complex Method**; indication 2; description D02.
  - Reveal: 10–56; cc = 12.

### internal/keymanagement/schema.go — 9.45

- **Large Method**; indication 2; description D08.
  - EditorSchema: 18–120; LoC = 98 lines.

### internal/keymanagement/update.go — 6.64

- **Bumpy Road Ahead**; indication 3; description D01.
  - UpdateLocal: 174–247; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - updateSourceOwnedMetadata: 291–334; cc = 18.
  - ApplyStructuredPatchToURI: 12–50; cc = 17.
  - UpdateLocal: 174–247; cc = 16.
  - resolvePatchMode: 251–270; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - updateSourceOwnedMetadata:302: 302–308; 10 complex conditional expressions.
  - ApplyStructuredPatchToURI:13: 13–13; 6 complex conditional expressions.
  - UpdateLocal:203: 203–203; 2 complex conditional expressions.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - resolveUpdatedURI: 274–289; Arguments = 5.

### internal/middleware/middleware.go — 8.47

- **Bumpy Road Ahead**; indication 3; description D01.
  - parseTrustedProxyNetworks: 171–191; bumps = 2.
- **Deep, Nested Complexity**; indication 3; description D27.
  - cleanup: 130–151; Nesting depth = 4 conditionals.
- **Complex Method**; indication 2; description D02.
  - evictLocked: 96–116; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - Allow:72: 72–72; 2 complex conditional expressions.
  - evictLocked:113: 113–113; 2 complex conditional expressions.

### internal/model/model.go — 8.54

- **Code Duplication**; indication 2; description D05.
  - NormalizeUserStatus: 595–606; no additional detail.
  - NormalizeKeyStatus: 618–632; no additional detail.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - EffectiveUserStatus: 125–140; Arguments = 6.
- **Primitive Obsession**; indication 2; description D28.
- **String Heavy Function Arguments**; indication 2; description D29.

### internal/platform/configuration/configuration.go — 7.33

- **Bumpy Road Ahead**; indication 3; description D01.
  - LoadFromMap: 77–147; bumps = 3.
  - loadProfileKeyring: 166–188; bumps = 2.
  - parseTrustedProxyNetworks: 293–318; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - LoadFromMap: 77–147; cc = 20.
  - parseHTTPURL: 263–272; cc = 11.
- **Complex Conditional**; indication 2; description D04.
  - parseHTTPURL:265: 265–265; 4 complex conditional expressions.
  - parseHTTPURL:268: 268–268; 4 complex conditional expressions.
  - RedactURL:240: 240–240; 2 complex conditional expressions.
  - listenAddressForPort:342: 342–342; 2 complex conditional expressions.
- **Primitive Obsession**; indication 2; description D30.
- **String Heavy Function Arguments**; indication 2; description D31.

### internal/profileconfig/parse.go — 7.40

- **Bumpy Road Ahead**; indication 3; description D01.
  - ClientDisplayNameFromKeyURL: 24–59; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - parseVMessDraft: 382–452; cc = 12.
  - parseUserInfoDraft: 323–380; cc = 12.
  - ClientDisplayNameFromKeyURL: 24–59; cc = 11.
- **Complex Conditional**; indication 2; description D04.
  - ParsePortNumber:214: 214–214; 2 complex conditional expressions.
  - parseUserInfoDraft:350: 350–350; 2 complex conditional expressions.
- **Code Duplication**; indication 2; description D05.
  - DecodeVMESSPayload: 152–170; no additional detail.
  - DecodeVMESSPayloadMap: 173–191; no additional detail.
- **Primitive Obsession**; indication 2; description D32.
- **String Heavy Function Arguments**; indication 2; description D33.

### internal/profileconfig/probe.go — 9.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - checkConfigurationAvailability: 38–97; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - checkConfigurationAvailability: 38–97; cc = 15.

### internal/profileconfig/render.go — 9.38

- **Complex Method**; indication 2; description D02.
  - BuildShareLinkFromDraft: 14–32; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - BuildShareLinkFromDraft:15: 15–15; 2 complex conditional expressions.

### internal/profileconfig/xray.go — 5.48

- **Bumpy Road Ahead**; indication 3; description D01.
  - ParseXrayJSONTarget: 69–130; bumps = 2.
  - xrayVnextDrafts: 242–282; bumps = 2.
  - ProjectXrayJSONDrafts: 310–361; bumps = 2.
  - projectXrayShadowsocks: 393–439; bumps = 2.
  - projectXrayHysteria2: 441–487; bumps = 2.
  - xrayOutboundSettings: 692–734; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - ParseXrayJSONTarget: 69–130; cc = 15.
  - projectXrayHysteria2: 441–487; cc = 14.
  - ProjectXrayJSONDrafts: 310–361; cc = 13.
  - applyXraySecuritySettings: 786–825; cc = 12.
  - xrayVnextDrafts: 242–282; cc = 12.
  - ParseXrayJSONDrafts: 133–173; cc = 12.
  - applyXrayTransportSettings: 744–784; cc = 11.
  - projectXrayShadowsocks: 393–439; cc = 11.
  - xrayStreamDraft: 177–223; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - xrayVnextDrafts:259: 259–259; 3 complex conditional expressions.
  - xrayTrojanDrafts:299: 299–299; 3 complex conditional expressions.
  - projectXrayShadowsocks:433: 433–433; 3 complex conditional expressions.
  - projectXrayTUIC:513: 513–513; 3 complex conditional expressions.
  - AnyToPort:62: 62–62; 2 complex conditional expressions.
  - ParseXrayJSONDrafts:149: 149–149; 2 complex conditional expressions.
  - projectXrayHysteria2:483: 483–483; 2 complex conditional expressions.
- **Large Method**; indication 2; description D08.
  - BuildXrayJSONFromLink: 593–688; LoC = 92 lines.
- **Primitive Obsession**; indication 2; description D34.
- **String Heavy Function Arguments**; indication 2; description D35.

### internal/profiles/fingerprint.go — 9.92

- **Bumpy Road Ahead**; indication 3; description D01.
  - fingerprintExtraParameterLines: 71–105; bumps = 2.

### internal/profiles/hysteria2.go — 8.21

- **Bumpy Road Ahead**; indication 3; description D01.
  - Validate: 104–141; bumps = 3.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - Parse: 20–76; cc = 15.
  - Validate: 104–141; cc = 14.
  - SerializeCanonical: 161–201; cc = 13.
- **Complex Conditional**; indication 2; description D04.
  - normalizeCertificatePin:225: 225–225; 3 complex conditional expressions.

### internal/profiles/legacy.go — 9.11

- **Complex Method**; indication 2; description D02.
  - Parse: 178–225; cc = 11.
  - parseUserInfoLink: 31–74; cc = 11.
  - validateUserInfoLink: 76–98; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - Validate:259: 259–259; 2 complex conditional expressions.

### internal/profiles/registry.go — 9.24

- **Complex Method**; indication 2; description D02.
  - Register: 37–60; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - Register:38: 38–38; 3 complex conditional expressions.

### internal/profiles/shadowsocks.go — 8.45

- **Bumpy Road Ahead**; indication 3; description D01.
  - splitShadowsocksURI: 148–191; bumps = 3.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - splitShadowsocksURI: 148–191; cc = 13.
  - Parse: 16–74; cc = 13.
  - Validate: 268–292; cc = 9.
- **String Heavy Function Arguments**; indication 2; description D36.

### internal/profiles/tuic.go — 5.59

- **Bumpy Road Ahead**; indication 3; description D01.
  - Validate: 502–545; bumps = 3.
  - classifySingleUserInfo: 262–286; bumps = 2.
  - v5Data: 299–324; bumps = 2.
  - tuicFieldObservations: 420–451; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - Validate: 502–545; cc = 16.
  - parseTUICOptions: 110–158; cc = 16.
  - isUUID: 398–414; cc = 14.
  - classifySingleUserInfo: 262–286; cc = 13.
  - parseTUICCredentials: 211–240; cc = 13.
  - Parse: 44–107; cc = 12.
  - v5Data: 299–324; cc = 10.
  - tuicFieldObservations: 420–451; cc = 9.
  - durationParameter: 346–366; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - isUUID:409: 409–409; 5 complex conditional expressions.
  - isUUID:403: 403–403; 3 complex conditional expressions.
  - tuicParameterProvenance:472: 472–472; 3 complex conditional expressions.
  - classifySingleUserInfo:266: 266–266; 2 complex conditional expressions.
  - classifySingleUserInfo:277: 277–277; 2 complex conditional expressions.
  - durationParameter:356: 356–356; 2 complex conditional expressions.
  - positiveIntegerParameter:374: 374–374; 2 complex conditional expressions.
- **Primitive Obsession**; indication 2; description D37.
- **String Heavy Function Arguments**; indication 2; description D38.

### internal/profiles/uri.go — 4.74

- **Bumpy Road Ahead**; indication 3; description D01.
  - duplicateWarnings: 349–398; bumps = 4.
  - parsePortSpec: 192–242; bumps = 3.
  - splitStandardURI: 53–90; bumps = 2.
  - parseAuthority: 92–142; bumps = 2.
  - canonicalQuery: 400–447; bumps = 2.
- **Deep, Nested Complexity**; indication 3; description D27.
  - canonicalQuery: 400–447; Nesting depth = 4 conditionals.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - parsePortSpec: 192–242; cc = 19.
  - parseAuthority: 92–142; cc = 15.
  - detectScheme: 36–51; cc = 13.
  - percentEncode: 449–463; cc = 12.
  - canonicalQuery: 400–447; cc = 12.
  - duplicateWarnings: 349–398; cc = 11.
  - splitStandardURI: 53–90; cc = 11.
  - parseQuery: 260–291; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - detectScheme:44: 44–45; 9 complex conditional expressions.
  - percentEncode:453: 453–454; 9 complex conditional expressions.
  - normalizeHost:182: 182–182; 2 complex conditional expressions.
  - parsePortSpec:215: 215–215; 2 complex conditional expressions.
  - parsePortNumber:254: 254–254; 2 complex conditional expressions.
  - validateKnownParameterValues:296: 296–296; 2 complex conditional expressions.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - canonicalURI: 480–498; Arguments = 5.
- **Primitive Obsession**; indication 2; description D37.
- **String Heavy Function Arguments**; indication 2; description D39.

### internal/security/password/hasher.go — 6.35

- **Bumpy Road Ahead**; indication 3; description D01.
  - Verify: 151–192; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - parseArgon2id: 218–261; cc = 14.
  - validateVerificationParameters: 282–291; cc = 11.
  - validateParameters: 271–280; cc = 11.
  - Verify: 151–192; cc = 10.
- **Complex Conditional**; indication 2; description D04.
  - validateParameters:272: 272–276; 9 complex conditional expressions.
  - validateVerificationParameters:283: 283–287; 9 complex conditional expressions.
  - parseArgon2id:220: 220–220; 3 complex conditional expressions.
- **Code Duplication**; indication 2; description D05.
  - validateParameters: 271–280; no additional detail.
  - validateVerificationParameters: 282–291; no additional detail.
- **String Heavy Function Arguments**; indication 2; description D40.

### internal/security/profilestorage/crypto.go — 8.81

- **Complex Method**; indication 2; description D02.
  - ParseEnvelope: 93–135; cc = 13.
- **Complex Conditional**; indication 2; description D04.
  - ParseEnvelope:123: 123–123; 2 complex conditional expressions.
- **Primitive Obsession**; indication 2; description D41.
- **String Heavy Function Arguments**; indication 2; description D06.

### internal/security/profilestorage/keyring.go — 8.28

- **Deep, Nested Complexity**; indication 3; description D27.
  - parseJSONValue: 277–307; Nesting depth = 4 conditionals.
- **Complex Method**; indication 2; description D02.
  - LoadKeyringJSON: 114–169; cc = 15.
- **Code Duplication**; indication 2; description D05.
  - GetEncryptionKey: 38–48; no additional detail.
  - GetActiveEncryptionKey: 50–60; no additional detail.
  - GetBlindIndexKey: 62–72; no additional detail.
  - GetActiveBlindIndexKey: 74–84; no additional detail.
  - decodeEncryptionKeys: 173–198; no additional detail.
  - decodeBlindIndexKeys: 200–225; no additional detail.

### internal/security/profilestorage/permissions_windows.go — 9.68

- **Complex Method**; indication 2; description D02.
  - firstBroadAllowTrustee: 66–103; cc = 9.

### internal/sources/decode.go — 9.68

- **String Heavy Function Arguments**; indication 2; description D42.

### internal/sources/parse_json.go — 9.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - parseExternalJSONBody: 142–171; bumps = 2.
- **Primitive Obsession**; indication 2; description D43.

### internal/sources/parse_links.go — 9.68

- **String Heavy Function Arguments**; indication 2; description D29.

### internal/sources/ssrf.go — 9.92

- **Bumpy Road Ahead**; indication 3; description D01.
  - ResolveHost: 69–94; bumps = 2.

### internal/sources/sync.go — 9.53

- **Bumpy Road Ahead**; indication 3; description D01.
  - indexExistingKeys: 268–295; bumps = 2.
  - matchExisting: 364–380; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - indexExistingKeys: 268–295; cc = 9.

### internal/sources/xray_hysteria2.go — 9.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - singleXrayHysteriaOutbound: 89–113; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - xrayHysteriaVersionRejection: 168–183; cc = 9.

### internal/storage/category_store.go — 9.68

- **Excess Number of Function Arguments**; indication 2; description D03.
  - resolveTx: 64–72; Arguments = 5.

### internal/storage/credential_store.go — 9.38

- **Code Duplication**; indication 2; description D05.
  - blindIndex: 28–37; no additional detail.
  - cloneBlindIndex: 39–48; no additional detail.
  - encrypt: 50–59; no additional detail.

### internal/storage/key_repository.go — 8.81

- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - BulkUpdateKeys: 32–71; cc = 11.
- **Complex Conditional**; indication 2; description D04.
  - GetLegacyByID:19: 19–19; 2 complex conditional expressions.

### internal/storage/migrations_baseline.go — 9.92

- **Bumpy Road Ahead**; indication 3; description D01.
  - applyBaselineSchema: 265–283; bumps = 2.

### internal/storage/migrations_data.go — 7.39

- **Bumpy Road Ahead**; indication 3; description D01.
  - migrateBackgroundJobWarningResults: 80–168; bumps = 3.
  - migrateVlessKeyBatch: 264–291; bumps = 2.
  - verifyVlessKeysMigration: 312–349; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - applyMigration13Rebuild: 416–482; cc = 18.
  - migrateBackgroundJobWarningResults: 80–168; cc = 15.
  - migrateCanonicalSubscriptionAnnouncement: 32–70; cc = 11.
  - verifyVlessKeysMigration: 312–349; cc = 9.
  - migrateVlessKeyBatch: 264–291; cc = 9.
  - migrateVlessKeysData: 170–202; cc = 9.
- **Large Method**; indication 2; description D08.
  - migrateBackgroundJobWarningResults: 80–168; LoC = 88 lines.
- **Excess Number of Function Arguments**; indication 2; description D03.
  - migrateVlessKeyBatch: 264–291; Arguments = 7.
  - insertVlessKeySecret: 295–308; Arguments = 6.

### internal/storage/migrations.go — 9.33

- **Bumpy Road Ahead**; indication 3; description D01.
  - MigrateWithKeyring: 504–556; bumps = 4.
  - applySchemaMigration: 558–601; bumps = 3.
- **Complex Method**; indication 2; description D02.
  - MigrateWithKeyring: 504–556; cc = 15.
  - applySchemaMigration: 558–601; cc = 13.

### internal/storage/profile_repository.go — 5.67

- **Low Cohesion**; indication 3; description D11.
- **Bumpy Road Ahead**; indication 3; description D01.
  - ReorderKeys: 1026–1083; bumps = 3.
  - ListKeyCategories: 778–848; bumps = 2.
  - DeleteKeyCategory: 966–1002; bumps = 2.
- **Overall Code Complexity**; indication 2; description D07.
- **Complex Method**; indication 2; description D02.
  - UpdateLegacy: 669–733; cc = 15.
  - ReorderKeys: 1026–1083; cc = 14.
  - UpdateKeyCategory: 883–964; cc = 14.
  - ListKeyCategories: 778–848; cc = 14.
  - ListLegacy: 528–606; cc = 14.
  - CloneLocal: 392–463; cc = 14.
  - loadKeyByID: 74–164; cc = 14.
  - UpdateSourceOwnedMetadata: 334–390; cc = 13.
  - UpdateLocal: 229–293; cc = 13.
  - CreateLegacy: 608–667; cc = 12.
  - CreateLocal: 166–227; cc = 11.
  - DeleteKeyCategory: 966–1002; cc = 9.
  - loadLocalKeyForUpdate: 298–329; cc = 9.
- **Complex Conditional**; indication 2; description D04.
  - cloneClientDisplayName:517: 517–517; 2 complex conditional expressions.
  - UpdateLegacy:701: 701–701; 2 complex conditional expressions.
- **Large Method**; indication 2; description D08.
  - loadKeyByID: 74–164; LoC = 88 lines.

### internal/storage/sqlite.go — 9.92

- **Bumpy Road Ahead**; indication 3; description D01.
  - ConfigureSQLitePragmas: 55–84; bumps = 2.

### internal/storage/startup.go — 9.61

- **Bumpy Road Ahead**; indication 3; description D01.
  - VerifyStartupEnvelopesAndInvariants: 11–65; bumps = 2.
- **Complex Method**; indication 2; description D02.
  - VerifyStartupEnvelopesAndInvariants: 11–65; cc = 13.

### scripts/start.ps1 — 8.03

- **Global Conditionals**; indication 2; description D44.
- **Deep, Global Nested Complexity**; indication 2; description D45.
- **Complex Conditional**; indication 2; description D04.
  - Get-DotEnvExactValue:44: 44–44; 2 complex conditional expressions.

## WAL experiment reproduction and limits

A disposable Go subprocess uses modernc SQLite with WAL and automatic checkpointing disabled. It creates a one-row table, truncating the schema WAL before the measured insert. It commits an insert and exits via `os.Exit` without clean close. Copy the closed-process DB/WAL/SHM twice. Open one preserved copy and count rows; invoke the actual `storage.CleanupSQLiteSidecars` on the other before opening. Recorded result: preserved **1**, deleted **0** committed rows. Only ignored disposable analysis paths were touched. This isolates the helper consequence; it does not simulate every initializer I/O failure or establish trigger frequency. Future R01 needs deterministic initializer/failure and lock/checkpoint cases. Do not experiment on runtime databases.
