# Go code-health follow-up: first implementation batch — 2026-10-13

Basis: `main` at `4f8a2b3d41800b4d5569b5a20961a9c5972dabe9`, plus the separately supplied [follow-up plan](health-followup-plan.md). This record covers H01 and the first selected H02/H03 hotspots. H00's licensed-report requirement remains pending. It does not close the entire follow-up plan or the accepted A00–A14 backlog.

The subsequent H02–H06 implementation and current remaining gates are recorded in [Code-health refactoring execution](health-refactoring-execution.md); the results below describe this first batch at its validation point.

## Analysis and scope

The plan accurately identifies two different problems: shared integration setup is embedded in large scenario files, and some tests combine repeated persistence checks with state transitions or capability policy. The first batch fixes these seams before production interpretation changes. The TypeScript findings still require independent parser/builder/conversion contracts and the existing preservation corpus; no frontend or production source is changed here.

Direct inspection confirms that the default Xray builder still combines credential construction, seven-network selection, security omission/aliases and the static document envelope. In particular, TCP and raw settings share header construction, Reality emits both `publicKey` and `password`, VMess alter-ID retains a numeric-prefix/string fallback, and false `allowInsecure` is omitted. Existing `configuration.test.ts`, `configuration-preservation.test.ts`, `configuration-shared.test.ts`, and `xray-interpretation.contract.test.ts` are the next production-refactor assurance boundaries. Extracting a facade alone would leave these policy branches intact; H04 must address them while retaining literal values and property order.

The 36 external-profile and 23 delivery tests retain their top-level names. CI selectors retain their previous names and additionally include `TestExternalProfileConcurrentRetryDoesNotDuplicateAndHistoryCountsAreAccurate` in the job/source lifecycle race step. Independent table cases continue to create fresh integration apps. The concurrent synchronization test's goroutines and history assertions are unchanged.

## Implementation and boundaries

- `profile_uri_fixture_test.go`: shared synthetic profile constants, fingerprint key and body parsing fixture; original bytes and key material retained.
- `external_profile_fixture_test.go`: source/user setup, typed historical-row seeding and source counts. Historical rows retain schema version zero, legacy compatibility, raw key references, and nullable blank fingerprints.
- `subscription_delivery_fixture_test.go`: typed assigned-key descriptor replaces positional setup arguments across all callers. Optional source IDs use `*int64`, preserving SQL NULL for manually owned keys. Malformed stored URIs remain deliberately admissible.
- `profile_storage_fixture_test.go`: checked inserted IDs, encryption-key loading/encryption/secret insertion, assignment insertion, count queries and raw-secret reads. Fixtures use the actual keyring, SQLite and row-bound encryption.
- `subscription_delivery_snapshot_test.go`: extracted persistence snapshot preserves row ordering, nullable source IDs, encrypted envelope bytes, labels/status, the original fingerprint coalescing policy, and timestamp text precision.
- `subscription_capability_assertions_test.go`: matrix lookup and official-binary version assertions. Expected targets remain the independent existing minimum/current constants; syntax validation still cannot imply runtime interoperability.
- `external_profile_assertions_test.go`: checked synchronization and typed local-metadata assertions. The metadata story uses named sequential phases and stops after a failed predecessor. Failure output no longer includes decrypted configuration bytes.

The four other modified test files only migrate assigned-key call sites to the descriptor: `happ_routing_test.go`, `repository_test.go`, `response_policy_public_test.go`, and `subscription_delivery_settings_test.go`.

## Explicit assertion mapping for changed tests

### Local status and client-display-name synchronization

`TestExternalProfileSyncPreservesLocalStatusAndClientDisplayName` retains initial import and row-ID lookup, local inactive status/custom-name setup, source rename, connection update, override reset, another source rename, effective display-name selection, actual selected-subscription generation and inactive-profile exclusion.

Each phase calls checked synchronization and reads metadata by the original row ID, so a row replacement still fails. Rename retains label `Source B`, inactive status and valid `CUSTOM`; connection update retains inactive status/custom override and exact decrypted new bytes; reset retains label `Source C`, inactive status and NULL override, with effective name `Source C`. Metadata and exact raw checks now run for every phase, strengthening rename/reset assurance. Assignment still uses the real table, and delivery still rejects any `vless.example` substring. Query/decryption errors fail at their helper step.

### Capability matrix

`TestSubscriptionCapabilityMatrixIsCompleteAndVersionPinned` retains matrix size seven, every protocol/generation, all eight output claims, minimum/current pinning for the three validated structured targets, the `official_binary`/`not_tested` distinction, Hysteria 2 conditional Xray support, TUIC v5 unsupported Xray reason, TUIC v4 compatibility/raw-only reason, serialized target versions and credential exclusion.

The previous inline lookup becomes `findDeliveryCapability`. The official-binary branch becomes `assertOfficialDeliveryCapability`: non-official claims remain outside that branch; missing runtime disclaimer fails; only the same three targets are version-pinned. No expected capability is computed by the production matrix algorithm.

### Read-only delivery deduplication

`TestDeliveryDedupIsSemanticCurrentKeyAndPersistenceReadOnly` retains reversed insertion/source-priority ordering, deliberately mismatched stored fingerprints, exact delivered winner comparison, both stored rows and assignments, full before/after snapshot equality, TUIC alias equality, and all four credential/connectivity distinctions.

The snapshot's query/scan/iteration guards move intact to `loadPersistedDeliveryState`; both count queries now check their errors. The original snapshot values and `reflect.DeepEqual` comparison are retained. Existing winner serialization expectations are unchanged in this setup-focused batch; further conversion goldens belong to H03/H06.

## Validation and evidence

Validation results and inventory follow below. Local artifacts are ignored under `.cache/health-followup/`, including source snapshots, Go AST inventories, before/after JSON test events, coverage profiles and a baseline overlay. The overlay restores the six original test files from HEAD and hides the seven new helpers for an authentic before run without altering the working tree.

CodeScene refresh was explicitly approved by the user after automatic review flagged source uploads. The approved retry still fails because the CLI requires authentication (`CS_ACCESS_TOKEN`). No new score, delta or smell category is claimed for either original file or any helper. The supplied 2.69/1.66 scores remain screenshot-attributed historical observations. This batch introduces no health-rule suppression.

Initial local race attempts failed with CGO disabled and then with a missing C compiler. A checksum-verified portable LLVM-MinGW compiler in the ignored workspace cache resolved this limitation; the full updated CI job/source selector now passes locally under the race detector, including the retained external-profile concurrency/history test. Execution on hosted Linux CI remains pending. Optional external-binary tests retain their existing skip policy.

## Remaining follow-up

- H00: authenticated before/after CodeScene reports for original and new modules; hosted CI/race evidence.
- H02: domain grouping and remaining persistence, idempotence, cross-source, lifecycle/privacy hotspots; continue the case-to-assertion inventory.
- H03: remaining delivery grouping, all-excluded safe-response helpers and independent byte goldens where missing.
- H04–H06: default builder, parser/projection and URI/legacy patch extraction, with the existing TypeScript preservation corpus and editor smoke validation.

## Final local validation

Both uncached focused runs pass all 59 original tests: 118 baseline and 122 final test/subtest pass events. The four extra final pass events are the named sequential metadata phases. Coverage definitions and covered/uncovered flags match for every production block, including 597 of 4224 instrumented statements (14.1%). Coverage is a regression signal, not proof of assertion equivalence.

The uncached full backend run (`go test ./... -count=1 -timeout=20m -json`) passes 13 packages with 4626 test/subtest pass events. The first attempt reached Go's default ten-minute timeout near the end with no assertion failures; the successful retry uses a longer timeout. Existing optional skips: `TestExistingDatabaseSnapshotMigration`, `TestOfficialMihomoValidation`, `TestOfficialSingBoxValidation`, `TestOfficialXrayMinimumValidation`, `TestOfficialXrayCurrentValidation`.

The updated CI job/source lifecycle selector also passes locally under `go test -race`, with 90 test/subtest pass events. The retained `TestExternalProfileConcurrentRetryDoesNotDuplicateAndHistoryCountsAreAccurate` executes and passes under the race detector. This is actual local Windows race execution; hosted Linux CI remains pending.

Local race tooling: Go 1.26.0 windows/amd64, command-scoped `CGO_ENABLED=1` and `CC` pointing into `.cache/health-followup/portable-cc/`. The compiler is [LLVM-MinGW 20261006 / LLVM 23.1.3](https://github.com/mstorsjo/llvm-mingw/releases/tag/20261006), downloaded from the official release into the ignored workspace cache. Archive SHA-256 matches the published release-asset digest: `317492c456aa27ee607a5919f1d2d38dcdc1112516a24d0bf4b00d078f52d17a`.

Additional passing checks: `go vet ./...`; backend build to the ignored cache directory; repository-configured `golangci-lint run --new-from-rev=HEAD ./...` (zero new issues); `actionlint .github/workflows/ci.yml`; `gofmt` inspection; `git diff --check`. Lint and race compilation were run outside the filesystem sandbox because its restrictions blocked ancestor path resolution and compiler subprocess execution. Hosted checks are not claimed.

Equivalent commands use a writable local build cache (`$env:GOCACHE = Join-Path (Get-Location) ".cache/go-build"`). In PowerShell, quote native Go arguments containing relative file paths, e.g. `'-overlay=.cache/health-followup/baseline-overlay.json'` and `'-coverprofile=.cache/health-followup/after.cover'`. The final evidence uses these quoted flags.

### Structural observations

Local Go AST measurements count physical function lines and `if` statements; these are not CodeScene scores or its complexity formula. The extracted helper functions span 8–23 lines and contain 1–3 `if` statements each. This explicitly accounts for logic moved outside the scenario files.

| Test | Physical lines before / after | `if` statements before / after |
| --- | ---: | ---: |
| `TestExternalProfileSyncPreservesLocalStatusAndClientDisplayName` | 83 / 63 | 21 / 10 |
| `TestSubscriptionCapabilityMatrixIsCompleteAndVersionPinned` | 81 / 49 | 16 / 9 |
| `TestDeliveryDedupIsSemanticCurrentKeyAndPersistenceReadOnly` | 85 / 52 | 9 / 6 |

| Extracted helper | Physical lines | `if` statements |
| --- | ---: | ---: |
| `parseExternalTestBody` | 8 | 1 |
| `seedExternalProfileSource` | 9 | 1 |
| `seedExternalRepairUser` | 13 | 1 |
| `insertHistoricalExternalProfile` | 16 | 1 |
| `sourceProfileCount` | 8 | 1 |
| `profileFixtureInsertID` | 8 | 1 |
| `insertProfileFixtureSecret` | 14 | 3 |
| `assignProfileFixtureUsers` | 8 | 1 |
| `profileFixtureCount` | 8 | 1 |
| `profileFixtureRaw` | 12 | 2 |
| `insertAssignedDeliveryKey` | 21 | 2 |
| `loadPersistedDeliveryState` | 23 | 3 |
| `findDeliveryCapability` | 14 | 2 |
| `assertOfficialDeliveryCapability` | 17 | 3 |
| `syncExternalFixtureBody` | 8 | 1 |
| `assertExternalFixtureMetadata` | 11 | 2 |

### Retained test-to-assertion inventory

For the 56 tests without explicit mappings above, normalized AST assertion calls, conditional expressions and subtest-name expressions match the original versions. Fixture arguments migrate field-for-field, with the same SQL constants and encryption/assignment semantics. Assertion-call counts include error guards; they do not estimate scenario coverage. Original table cases and ordered state transitions remain in those tests.

| Original test name (retained) | Original direct failure/error calls | Mapping |
| --- | ---: | --- |
| `TestExternalProviderAnnouncementMetadataRemainsIndependent` | 7 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileInputRecognitionAndMixedPartialSuccess` | 4 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileBase64AliasCompatibilityAndSafePreview` | 5 | Same assertion calls, conditions and subtest expressions |
| `TestExternalImporterBase64DetectionPolicy` | 7 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSchemeRecognitionIsCaseInsensitiveAndExplicit` | 2 | Same assertion calls, conditions and subtest expressions |
| `TestExternalImporterDoesNotGuessProviderJSONOrSIP008` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestExternalImporterExistingXrayJSONCompatibility` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExternalImporterRecognizesXrayHysteria2AndPreservesSupportedFields` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestExternalImporterClassifiesHysteriaV1AndIdentifiableXrayFailures` | 8 | Same assertion calls, conditions and subtest expressions |
| `TestExtractJSONSubscriptionLabelRejectsGenericRoutingTags` | 2 | Same assertion calls, conditions and subtest expressions |
| `TestExternalImporterRuntimeXrayHysteria2ShapeAndCandidateAccounting` | 8 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSemanticDeduplication` | 1 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileConnectivityChangesRemainDistinct` | 2 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileFingerprintKeyAndSelectionIntegrity` | 7 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSyncRepairsHistoricalDuplicatesAndMergesAssignments` | 8 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSyncPreservesLocalStatusAndClientDisplayName` | 21 | Explicit mapping below |
| `TestExternalProfileSyncRepairsBlankDuplicatesDuringPartialImport` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSyncLeavesRawXrayJSONOnExactRawIdentity` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSyncRepairsEightyRowsToFortyAndRemainsStable` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileDuplicateWithinSourcePersistsOnce` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfilePersistenceAndSynchronization` | 14 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSynchronizationIsIdempotentAndSourceScoped` | 12 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileConcurrentRetryDoesNotDuplicateAndHistoryCountsAreAccurate` | 5 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileLegacyRowWithoutFingerprintKeepsIDOnRename` | 4 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileKeyRotationMatchesPreviousFingerprint` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileCrossSourceAndTUICV4Compatibility` | 16 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileWarningOnlyRefreshPreservesRow` | 5 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSourceSpecificWarningsAndLifecycle` | 9 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfilePreviewApplyIsAuthoritativeAndSecretSafe` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfilePreviewAndAuditNeverExposeSecrets` | 8 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileCreateRollsBackOnPersistenceFailure` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileSynchronizationRollsBackOnPersistenceFailure` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileProbePolicyMarksOnlyUnimplementedProtocolsUnsupported` | 1 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileUnsupportedProbeDoesNotAccumulateHealthFailures` | 5 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileErrorsDoNotContainSecrets` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestExternalProfileLoggingStyleFormattingIsSecretSafe` | 2 | Same assertion calls, conditions and subtest expressions |
| `TestShareURIWithDisplayNameCoversAllSupportedProtocols` | 2 | Same assertion calls, conditions and subtest expressions |
| `TestSourceOwnedClientDisplayNameOverridesURIAndJSONProjection` | 4 | Same assertion calls, conditions and subtest expressions |
| `TestSourceOwnedXrayProjectionUsesHumanLabelInsteadOfRoutingTag` | 6 | Same assertion calls, conditions and subtest expressions |
| `TestSubscriptionCapabilityMatrixIsCompleteAndVersionPinned` | 15 | Explicit mapping below |
| `TestCapabilityAPIUsesBackendMatrixAndRejectsClientOverride` | 4 | Same assertion calls, conditions and subtest expressions |
| `TestOpenAPIDeliveryContractsRemainParseableAndReadOnly` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestPlainDeliveryPreservesExactMixedRawAndBase64WrapsFinalBody` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestLinkModeProjectsRawXrayJSONAtResolverBoundaryWithoutMutatingProfiles` | 11 | Same assertion calls, conditions and subtest expressions |
| `TestLinkModeProjectsMultipleXrayOutboundsWithClientNameAndKeepsStoredBytes` | 10 | Same assertion calls, conditions and subtest expressions |
| `TestDeliveryDedupIsSemanticCurrentKeyAndPersistenceReadOnly` | 9 | Explicit mapping below |
| `TestUnsafeAndMalformedStoredRowsAreSafelyExcluded` | 9 | Same assertion calls, conditions and subtest expressions |
| `TestSubscriptionControlValidationRejectsCRLFNULAndDEL` | 1 | Same assertion calls, conditions and subtest expressions |
| `TestStructuredDeliveryWithOnlyExcludedProfilesReturnsAllExcludedFailure` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestStructuredDeliveryAllExcludedHandlerPolicy` | 5 | Same assertion calls, conditions and subtest expressions |
| `TestStructuredDeliveryMixedAndNoEligiblePolicies` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestExclusionHeadersAreAggregatedDeterministicallyAndBounded` | 4 | Same assertion calls, conditions and subtest expressions |
| `TestMihomoMappingsAndSafeExclusions` | 13 | Same assertion calls, conditions and subtest expressions |
| `TestSingBoxMappingsProvenanceAndExclusions` | 7 | Same assertion calls, conditions and subtest expressions |
| `TestXrayPartialOutputPreservesLegacyAndSupportsPluginFreeShadowsocks` | 7 | Same assertion calls, conditions and subtest expressions |
| `TestXrayRejectsRemovedInsecureTLSField` | 4 | Same assertion calls, conditions and subtest expressions |
| `TestStructuredNamesAreSafeUniqueAndSerializationValidationRejectsInjection` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestFinalStructuredTemplateValidationRejectsUnsafeTransforms` | 3 | Same assertion calls, conditions and subtest expressions |
| `TestStructuredTemplateRequiresSubscriptionPlaceholder` | 1 | Same assertion calls, conditions and subtest expressions |
