# Code-health refactoring execution — 2026-10-13

This continues the [first Go batch](health-followup-go-batch.md) and implements the remaining H02–H06 refactoring from the [follow-up plan](health-followup-plan.md). It preserves the working-tree changes from the first batch. The change contains no deployment, database migration, protocol expansion, or health-rule suppression.

## Result

The external-profile and delivery integration suites are organized by assurance domain, with all 36 + 23 original top-level test names retained. Persistence, repeated synchronization, cross-source ownership, safe exclusion responses and synchronization history now have focused fixture/assertion boundaries. The production configuration API remains at `frontend/src/lib/configuration.ts`; its implementation is split into pure construction, interpretation, codec, conversion and token-editing modules.

CodeScene source uploads were approved for both Go and TypeScript. The approved refresh still requires CLI authentication (`CS_ACCESS_TOKEN`), so this record makes no fresh score/delta claim. Numeric health verification and hosted Linux CI remain pending. Local structural measurements and executed behavior checks are recorded below.

## Go responsibilities

| Domain | Suite |
| --- | --- |
| Recognition, base64 and safe previews | `external_profiles_test.go` |
| Xray parsing and candidate accounting | `external_profile_xray_test.go` |
| Semantic identity and selection integrity | `external_profile_identity_test.go` |
| Historical duplicate repair and key rotation | `external_profile_repair_test.go` |
| Persistence, local metadata, synchronization and concurrency | `external_profile_sync_test.go` |
| Source lifecycle, rollback and health/probe policy | `external_profile_lifecycle_test.go` |
| Preview/apply authority, audit and secrecy | `external_profile_privacy_test.go` |
| Raw delivery and read-only semantic deduplication | `subscription_generation_test.go` |
| Capability matrix and API/OpenAPI contracts | `subscription_capabilities_test.go` |
| Source display names and Xray projection | `subscription_projection_test.go` |
| Exclusion, request/response policy and privacy | `subscription_exclusions_test.go` |
| Target renderer mappings and serialization | `subscription_renderers_test.go` |
| Final template validation | `subscription_templates_test.go` |

`external_profile_state_fixture_test.go` holds typed persisted rows, source identities, selective count expectations, named phase execution, checked decrypted-row counting and typed history loading. `subscription_exclusion_assertions_test.go` keeps the original actual-handler status/body/header/privacy checks together. `subscription_render_fixture_test.go` holds shared generated-object/exclusion inspection. Original synthetic fixtures and encryption/assignment helpers from H01 remain in use.

## Assertion and transition mappings

Normalized Go AST assertion-call, conditional-expression and subtest-name lists match the preceding batch for 53 of the 59 original tests. The six intentional changes are mapped here; every original top-level name still executes in the focused run. The earlier batch's 59-test inventory remains the baseline, with the domain table above giving the new locations.

- **Persistence and synchronization:** initial import still asserts two imported profiles; actual encrypted Shadowsocks bytes, protocol, `pf1_` fingerprint, schema version, full compatibility and `[]` warnings are checked. Tag refresh retains updated/unchanged counts, row ID, raw bytes and stable reference. Partial refresh retains two rows; complete refresh removes the missing row. Changed credentials import once and change the fingerprint. Named phases stop after a failed predecessor. Previously ignored updated-secret decryption errors now fail explicitly.
- **Idempotence and source scope:** the initial three added rows, two repeated refreshes, reorder, rename/stable ID, added profile, partial retention, complete removal and second-source ownership all remain. Selective count maps deliberately leave originally unasserted counters unasserted; zero remains an explicit expected value. Source row counts and the actual cross-source SQL count remain checked. Stable row ID is read after reorder, as before.
- **TUIC/cross-source compatibility:** the first v4 compatibility count and actual decrypted raw/fingerprint/read-only metadata, second source compatibility/import counts, two encrypted matching rows, distinct row IDs/equal stable references, real selected assignments, both unchanged refreshes, source-specific deletion/retained compatibility/assignment, and byte-different v5 ownership all remain. Five sequential phases expose the story. Query/scan/decryption/iteration/count errors now fail at the relevant setup/read step; original fingerprint coalescing is preserved.
- **Concurrent synchronization/history:** both goroutines, waits/channels, added and unchanged totals and actual source-row count are retained. Actual start/finish history persistence remains; its JSON read/decode guards move into a typed helper, and the independent expected added/updated/unchanged counters stay `0/0/3`. The final test executes under the race detector.
- **All-excluded handler policy:** all five independent table cases still create fresh integration apps and invoke the real subscription handler. The extracted assertion retains HTTP 422, all-excluded code, eligible/excluded/reason counts, all three exclusion headers, and every forbidden raw/profile/secret fragment. Original subtest names and fixture inputs are retained.
- **Delivery deduplication:** source priority, reversed insertion order, rotated fingerprints, rows/assignments, full encrypted-envelope/source/metadata/timestamp snapshots, TUIC alias identity and all connectivity distinctions remain. The expected winner is now the literal canonical URI `hysteria2://auth@hy.example:443/?sni=tls.example#first`, removing dependence on the production display-name serializer when asserting exact delivered bytes.

## TypeScript boundaries

| Responsibility | Modules |
| --- | --- |
| Stable API and directed type/value imports | `configuration.ts`, `configuration-types.ts`, `configuration-values.ts` |
| Protocol credentials, transport, security and fresh envelope assembly | `configuration-default-connection.ts`, `configuration-default-transport.ts`, `configuration-default-security.ts`, `configuration-default-envelope.ts`, `configuration-xray-builder.ts` |
| Root/outbound selection and connection/transport/security/display projection | `configuration-xray-reader.ts`, `configuration-xray-connection-reader.ts`, `configuration-xray-transport-reader.ts`, `configuration-xray-security-reader.ts` |
| UTF-8 URI/VMess codecs and conversion bridges | `configuration-link-codec.ts`, `configuration-link-projection.ts`, `configuration-link-serialization.ts`, `configuration-conversion.ts` |
| Ordered represented-field/alias policy for legacy patches | `configuration-link-fields.ts`, `configuration-link-patch.ts` |
| Existing ordered Xray token patching and security/value seams | `configuration-xray-patch.ts`, `configuration-security-plan.ts`, `configuration-patch-values.ts` |

The moved token-patch implementation is unchanged after normalizing source line endings. It still uses the original ordered token editor; none of the new construction/projection code replaces token edits with a full JSON rewrite. The 21 configuration modules have no imports back into the public facade. Security-plan and patch-value type imports now use the types boundary.

Protocol credentials, defaults, VMess numeric-prefix/string alter-ID behavior, TCP/raw header aliases, per-build object independence, TLS false omission and Reality `publicKey`/`password` ordering are retained. URI aliases select the first raw truthy value before trimming; VMess aliases select the first coerced nonempty text, with boolean aliases handled separately. Network/security mutation, VMess and URI alias sets, omitted/empty/false/zero values and query/property order remain explicit per-format policies. URI authority is still discarded on link-to-JSON conversion while VMess authority is retained; the existing lossy conversion boundary is deliberately unchanged.

## Validation

- **Go:** an uncached full backend run passed with 4,638 test/subtest pass events and the same five optional fixture/client-binary skips. After the final test-only phase/history/helper extractions, focused suites were rerun: all 59 original tests passed with 139 pass events, followed by final hotspot and history/template checks. Production coverage definitions and covered/uncovered block flags match the preceding batch exactly (597 / 4,224 instrumented statements, 14.1%). Final concurrency/history race execution passes. Vet, backend build, configured lint with zero new issues, workflow syntax and diff checks pass.
- **TypeScript:** all 3,916 pre-existing affected case names are retained. Nine new literal construction/conversion contracts passed before extraction; two additional direct factory tests verify actual nested object independence and within-build TCP/raw header identity. The full final frontend suite passes **4,236 tests across 52 files**, including the original 3,792-case preservation corpus and its supplements.
- **Supplemental API comparison:** **7,606** old/new comparisons pass for exact builder/conversion/patch strings, parsed draft order/values and errors. The reference is the saved pre-refactor module with equivalent unchanged security/value helpers. Cases cover all three protocols, seven networks, three security modes, eight port forms, patch omission/set/clear, malformed root/payload shapes, Unicode, alias precedence and unknown/duplicate query data. This supplements the retained independent goldens; it does not replace them or establish an analyzer score.
- **Frontend integration:** nonincremental TypeScript, full eslint with no warnings/errors and the static production build pass. The actual fresh `.next-validation` static export was served for the existing key-editor smoke; **desktop and mobile Chromium both pass**. The temporary server adapter is an ignored cache artifact, avoiding the stale `out` export.

Evidence is under ignored `.cache/health-refactor/`: pre-refactor sources, Go/TypeScript inventories, JSON test events, coverage, API comparisons, dependency/function observations, build/lint logs and browser smoke output. Source tests and fixtures contain only synthetic data; operational databases, subscriptions and credentials were not used.

## Local structural observations

These are physical function-line and `if` counts from the Go/TypeScript ASTs, not CodeScene's cyclomatic-complexity formula or scores. The moved token-plan functions are counted rather than excluded. The largest configuration function is the pre-existing token patch context at 52 lines; newly split interpretation/construction/codec helpers are at most 35 lines. The facade is 81 lines, including its literal placeholder examples.

| Go hotspot | Physical lines before / after | `if` statements before / after |
| --- | ---: | ---: |
| Persistence/synchronization | 68 / 25 | 14 / 2 |
| Idempotence/source scope | 79 / 35 | 12 / 3 |
| Cross-source/TUIC v4 | 90 / 52 | 17 / 5 |
| Concurrent history | 54 / 50 | 6 / 5 |
| All-excluded handler policy | 75 / 57 | 5 / 0 |

Source imports, all resulting modules and helper observations are recorded in the local inventories. Fresh named CodeScene findings for every affected/new module, a score target based on those reports, and hosted CI are the remaining measurement/acceptance work. None is represented as completed here.
