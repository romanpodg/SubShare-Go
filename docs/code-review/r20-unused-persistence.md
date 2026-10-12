# R20 — Remove unused persistence declarations

Status: locally validated six-file cleanup on `refactor/remove-unused-persistence-declarations`, based on accepted A10 merge `4a8c833`. Live direct/test/external-test imports are identical after excluding the two unused package roots. Full Go/atomic coverage, static/build/strict and secret checks pass. Hosted/review acceptance remains; no R20 PR exists at this checkpoint.

The exact scope is `internal/keypersistence/{repository,params,errors}.go` and `internal/profilepersistence/{repository,params,errors}.go`, 202 declaration lines. No runtime implementation, active storage/keymanagement interface, handler, migration, route or frontend file changed. The removed packages had no imports from other tracked Go packages or tests; similarly named active storage tests are not consumers.

Supported-consumer decision: SubShare-Go is documented as an application, not a Go SDK; these unused internal declarations have no documented compatibility promise. Independent external-module compiler probes reject imports of each package with `use of internal package ... not allowed`. Same-module/fork consumers would require their own source changes; none exist in the authoritative tracked import graph. Remove the dead definitions without introducing replacements or splitting `keymanagement.Repository`.

Evidence remains in `.cache/r20/import-graph-before.txt`, `import-graph-after.txt`, `external-consumer-before.log` and `profile-external-consumer-before.log`. External probe failures are the expected compiler-boundary proof, not failed application validation. Current root packages' direct/test/external-test imports compare exactly; only the two unused roots disappear. Go vet/lint/build and strict delta pass. No new tests were added for a declaration-only removal.

## A11 requirement evidence (acceptance pending)

| Requirement | Evidence |
| --- | --- |
| No supported tracked consumers | Exact before/after direct/test/external-test import graph and source/docs searches |
| External compatibility policy | No SDK promise; both isolated external imports rejected by the actual compiler |
| Only dead definitions removed | Six exact tracked paths, 202 deleted lines; no active adapter/interface edits |
| Compile/runtime parity | Full 4,476 Go passes/five unchanged optional skips, atomic coverage, static/build/strict pass; final hosted/race/review gates remain |

Final full uncached/atomic-coverage run 1788 exits zero with 4,476 passes / five unchanged optional skips and server statements 75.8%, unchanged from accepted R19. Staged/history secret scans pass. Artifacts remain under `.cache/r20`. Commit/push/open the independent main PR and require all six final-head checks/race plus resolved actionable review before A11 acceptance. Next is R21's separate branding acknowledgment/reopen correction. Whole goal remains incomplete; heartbeat paused per user.
