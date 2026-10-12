# R22a — Package-local delivery and source-sync contracts

Status: locally validated in [PR39](https://github.com/romanpodg/SubShare-Go/pull/39), initially at `f8e9c1b`, based on accepted R21 main `dabf134`; final corrected-head hosted/review gates pending. This is the measurement/test-locality part of A13; R22b operational backup/migration/restore assurance remains separate.

Keep all existing handler integration tests. Add direct semantic assertions for delivery format selection, credential/name projection, mixed exclusions, informational entries, safe failure/header publication and structured-output validation. Source-sync fixtures use populated disposable migrated SQLite databases and a fresh fixture keyring, exercising encrypted persistence, idempotence, stable identity, local metadata, partial-import removal policy, source scope, disabled insertion, user assignments, admission and publication rollback.

The first fixture run exposed incorrect test assumptions about the assignment column and Xray legacy tag policy. Correct the fixtures to actual frozen behavior; no production correction is proposed. Package and cross-package atomic statement profiles will be compared before/after. Cross-package instrumentation is a reviewed proposal; this batch does not change CI upload/configuration. Coverage percentages describe statements, not lines or test completeness.

Require focused/full Go, migrations, vet/lint/build, strict CodeScene, secret scans and all six exact-head hosted gates/four race steps plus resolved actionable review. A13 cannot be accepted until R22b also satisfies its independent requirements. Goal remains active; heartbeat paused per user; broader A01/A13-A14 remain.

## Direct-package statement measurement

| Scope | Statements | Existing direct tests | Added direct contracts |
| --- | ---: | ---: | ---: |
| internal/delivery | 781 | 0 / 0.00% | 289 / 37.00% |
| internal/sources | 1,076 | 232 / 21.56% | 394 / 36.62% |
| sources/sync.go | 206 | 0 / 0.00% | 134 / 65.05% |

The existing-direct profile executes the unchanged pre-existing tests with `-skip '^TestLocal'`; the after profile executes all package tests. The new suites add 35 executed parent/subtest events; the direct after run passes 38 events including three existing fetch cases. These counts are not independent end-to-end scenarios. They pass against unchanged production. All three new test files score 10.00 and strict delta passes.

The existing cross profile executes cmd/server plus both packages and covers delivery 645/781 (82.59%), sources 897/1,076 (83.36%), and sync.go 175/206 (84.95%). The completed full uncached after profile covers delivery 659/781 (84.38%), sources 904/1,076 (84.01%), and sync.go 181/206 (87.86%). This proves existing integration execution that ordinary per-package measurement misses and additional executed blocks from direct fixtures. Repeated block entries contributed by multiple test executables are unioned by file/range; a block is covered when any counter is positive. Go's coverage tool agrees with the aggregate 84.2% after union. Do not multiply statement denominators or read an in-progress profile as final evidence. Artifacts: `.cache/r22a/{local-before,local-after,cross-before,cross-after}.out`, coverage-comparison.json, cross-after-functions.txt and full-cross-after-tests.json.

## Reviewed instrumentation proposal

Retain the current CI `gotestsum -- -covermode=atomic -coverprofile=coverage.out ./...` and all integration tests in this batch. A separate instrumentation-only PR can propose adding `-coverpkg=./...` to that same test invocation, keeping the actual test selection and race gates. Before changing the coverage upload, compare unique statement-block denominators, runtime/memory and Codecov project/patch effects on the same revision; cross-package execution changes attribution and must not be presented as new tests or line coverage.

This batch measures a focused `-coverpkg=./internal/delivery,./internal/sources` diagnostic instead: before on those packages plus cmd/server, after on the full Go suite. Other packages have no test imports of either target; report the actual different test selections rather than claiming identical harnesses. The pre-existing integration diagnostic takes 426.8 seconds locally; direct after fixtures take 16.5 seconds. Hosted Linux timings and repository-wide instrumentation cost are not inferred from these Windows figures. Do not duplicate the full test suite as a permanent second CI job merely to inflate coverage, and do not upload the focused denominator under the current whole-backend flag. Proposal review is part of this PR's automated review; no CI or coverage-upload mutation is included.

## Local gate evidence and pending acceptance

Full uncached instrumented Go passes 4,511 cases with five unchanged optional snapshot/official-client skips in 474.2 seconds. Required idempotent migration, vet, configured golangci-lint (zero issues), server build, three test-health scores 10.00 and strict delta pass. Existing handler integration tests and all production/CI/frontend files remain unchanged. Staged/history secret scans and final six exact-head hosted gates/four race steps plus actionable-review resolution are required before this sub-batch is accepted.

| R22a requirement | Evidence |
| --- | --- |
| Direct delivery semantics | Format precedence, actual credential/name maps (including distinct legacy Xray tag policy), mixed exclusions, informational/blank entries, safe headers/failures, broken structured output |
| Direct source-sync semantics | Encrypted raw equality, idempotence/reorder, semantic rename, unique-label rotation, conservative ambiguous replacement, duplicate incoming refs, partial rejected/unsupported feeds, scoped disabled insertion, metadata and user assignments |
| Failure safety | Populated full-row snapshots retain keys/secrets/categories/assignments/source metadata after empty-feed, missing fingerprint key and injected publication failure |
| Measurement and proposal | Unique statement-block profiles above and separately reviewed `-coverpkg=./...` proposal; current uploads unchanged |
| Integration and final review | Local full/static/quality evidence above; exact-head hosted/review acceptance pending |

R22a alone does not accept A13: matching-keyring restore, backup publication faults, custom path export, encryption/table-rebuild interruptions, Docker populated upgrade/restore and optional runner policy remain R22b requirements. Broader A01/A13-A14 remain; goal active, heartbeat paused.

## PR39 review correction

P2 comment 4247753351 correctly identified stale current-boundary prose contradicting the A12/R21 acceptance table. The correction aligns the paragraph with PR38/A12 acceptance, current R22a PR39 and remaining R22b/R23/A01 scope. Backlog table, batch table, audit next-batch state and latest continuation now agree. This correction changes only records; the locally validated Go fixtures and production baseline remain identical. Require all six checks/four race steps and completed actionable-findings-free review at the corrected head, then reply/resolve the verified finding before acceptance.
