# R20 — Remove unused persistence declarations

Status: accepted in [PR37](https://github.com/romanpodg/SubShare-Go/pull/37), merge `a0120f25c0dde602e0d35e564c8095a45b016b4a`. All six final-head checks pass at `3f3c1c24ad68c6d43e4abb7341e7262a6aa20c40` in [run 38199337238](https://github.com/romanpodg/SubShare-Go/actions/runs/38199337238), including four race steps and Docker validation. Exact-head automated review completed with a positive reaction and no threads. A11 is accepted against the matrix below.

The exact scope is `internal/keypersistence/{repository,params,errors}.go` and `internal/profilepersistence/{repository,params,errors}.go`, 202 declaration lines. No runtime implementation, active storage/keymanagement interface, handler, migration, route or frontend file changed. The removed packages had no imports from other tracked Go packages or tests; similarly named active storage tests are not consumers.

Supported-consumer decision: SubShare-Go is documented as an application, not a Go SDK; these unused internal declarations have no documented compatibility promise. Independent external-module compiler probes reject imports of each package with `use of internal package ... not allowed`. Same-module/fork consumers would require their own source changes; none exist in the authoritative tracked import graph. Remove the dead definitions without introducing replacements or splitting `keymanagement.Repository`.

Evidence remains in `.cache/r20/import-graph-before.txt`, `import-graph-after.txt`, `external-consumer-before.log` and `profile-external-consumer-before.log`. External probe failures are the expected compiler-boundary proof, not failed application validation. Current root packages' direct/test/external-test imports compare exactly; only the two unused roots disappear. Go vet/lint/build and strict delta pass. No new tests were added for a declaration-only removal.

## Accepted A11 requirement evidence

| Requirement | Evidence |
| --- | --- |
| No supported tracked consumers | Exact before/after direct/test/external-test import graph and source/docs searches |
| External compatibility policy | No SDK promise; both isolated external imports rejected by the actual compiler |
| Only dead definitions removed | Six exact tracked paths, 202 deleted lines; no active adapter/interface edits |
| Compile/runtime parity | Full 4,476 Go passes/five unchanged optional skips, atomic coverage, static/build/strict and all six final-head hosted/race checks pass; final review clear |

Final full uncached/atomic-coverage run 1788 exits zero with 4,476 passes / five unchanged optional skips and server statements 75.8%, unchanged from accepted R19. Staged/history secret scans pass. Artifacts remain under `.cache/r20`. R20/A11 is accepted. Next is R21's separate branding acknowledgment/reopen correction. Broader A01/A12-A14 and the whole goal remain incomplete; heartbeat paused per user.
