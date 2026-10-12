# R19 — Xray outbound interpretation and traversal

Status: accepted in [PR36](https://github.com/romanpodg/SubShare-Go/pull/36), merge `4a8c833eeeafbcf132a685c6a715f2483b89d74f`, after tests-first `219a0b4` / `f9a2cd0`. All six final-head checks pass at `ace692166f503fe34d2326e2ca09b7a6b8578758` in [run 38197040947](https://github.com/romanpodg/SubShare-Go/actions/runs/38197040947), including four race steps and Docker validation. Exact-head automated review completed with a positive reaction and no threads. A10 is accepted against the evidence below.

`testdata/configuration/xray-interpretation.json` freezes separate outputs for target selection, strict Go draft enumeration, lenient delivery projection/rejection and frontend selected draft/index. It covers null/wrong root/outbound/settings/node/user values, invalid first proxy with usable later proxy, extra nodes/users/outbounds, missing/null/zero/negative/oversized/prefix/wrong-type ports, generic/display tags, bool-versus-string unsafe TLS and unsupported network/security normalization. Compact Go expected drafts decode into the actual struct so omitted zero fields remain asserted; frontend drafts compare complete selected objects and exact boundary errors.

Keep existing policies distinct. Target reads first node/user per supported outbound and can skip unusable entries. Strict Go drafts enumerate every node/user and stop on ordinal errors. Projection counts invalid supported outbounds and continues. The frontend editor selects the first supported outbound even when it has an empty editable identifier, retaining the rest of the document. Both observed prefix-port parsers return 1234 for `1234suffix`; stricter validation remains separate. The accepted shared corpus retains all supported transport/security combinations.

The extended matrix adds standalone/before-native SS/Hysteria2/TUIC, multiple servers, plugin object/options/missing cipher, Hysteria alias/auth/version, port-hop/salamander/ambiguous/wrong/unsupported masks, missing auth, TUIC aliases/flags and missing password. Existing full-package tests protect share-link/build projection and all supported protocols. Discovery scripts/results remain ignored under `.cache/r19`; they are observational scaffolding, not production code.

Save this complete tests-first baseline, then separate one narrow outbound view/traversal boundary across target/strict/project loops without merging their admission policies. Keep native projection's existing JSON reparse and all protocol-specific stream/validation/build bodies unchanged; do not reserialize loss-preserving frontend edits through Go. Require Go/frontend/full/static/quality/secret/hosted/review gates before A10 acceptance. Broader A01/A10-A14 remain incomplete; goal active and heartbeat paused.

## Implemented boundary and validation

`xray_outbound.go` owns the interpreted object/protocol/settings/stream view, original ordinal index, first-endpoint target selection and shared strict/delivery traversal. The private boundary mode explicitly retains first-error versus count-and-continue behavior. Native delivery keeps its existing JSON reparse and local error index. The unchanged public validator stays with its target parser; all exported names/signatures remain. An unvalidated named document type distinguishes internal parsing input without changing runtime validation.

All 44 edge cases, 63 shared cases and the full profileconfig package pass. Twenty-five retained Go function bodies, including protocol stream/extended projection/build/default helpers and relocated validator, are identical under the Go AST printer. Frontend projection/loss-preservation subset passes 3,847 cases; full frontend passes 4,187 cases in 47 files, TypeScript and full lint pass. Go vet/build/lint pass. Frontend production is unchanged.

New module/tests score 10.00; remaining xray.go improves 5.48 to 6.38, with existing protocol/build complexity still visible. Strict delta passes. Intermediate introduced duplication/cohesion/nesting/argument warnings were resolved through the shared traversal, validator placement and explicit document boundary; no suppressions were added. Failed intermediate delta artifacts remain under `.cache/r19` and are not passes. Automatic approval review initially rejected formatting/test execution due to account allowance exhaustion; no command ran, and verification resumed normally after the confirmed reset.

Final uncached full Go/atomic coverage run 22765 exits zero: 4,476 passes / five unchanged optional skips, server statements 75.8%, no failures. Full frontend/static/strict and staged/history secret scans pass. Artifacts remain in `.cache/r19/go-full.log`, `go-events.json`, `go-junit.xml`, `coverage.out`, `frontend-full.json` and `final-extraction-delta.json`. Commit/push/open its independent main PR and require all six final-head checks/race and resolved actionable automated review before A10 acceptance. R20 read-only import-graph evidence is ignored under `.cache/r20`; no declarations have been removed. Whole goal remains incomplete; heartbeat stays paused.

## Accepted A10 requirement evidence

| Requirement | Evidence |
| --- | --- |
| First usable endpoint/default/wrong-type semantics | 44 boundary fixtures assert exact target/default/error outputs |
| Strict extra nodes/users and original ordinal errors | Exact draft arrays/errors in the same corpus; original indices retained in the view |
| Extended protocols and delivery rejection | SS/Hysteria/Hysteria2/TUIC version/plugin/mask/alias/server/error cases and full package builders pass |
| Frontend preservation and supported transport/security | Separate frontend selected-draft/index expectations plus accepted 63-case shared corpus and 3,847 preservation/configuration checks |
| Generic/display tags, TLS/default/exclusion behavior | Corpus and 25 byte-equivalent retained AST bodies; no numeric-validation or capability policy change |
| Reduced local complexity and stable public API | New module 10.00, old file 5.48 to 6.38, target/strict/project complexity reduced, unchanged exported signatures and strict delta pass |
| Final integration/review | All six final-head hosted/race gates pass; exact-head review is clear |

R19/A10 is accepted. Continue R20's narrow unused-interface cleanup. Broader A01/A11-A14 and the whole goal remain incomplete; heartbeat remains paused.
