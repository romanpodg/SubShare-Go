# R07a — Category and ordering persistence

R07a continues R06 characterization commit **6a12236**, merged through PR #10 at **ebd427f**. The initial extraction was implemented on 2026-10-07 and published in [PR #11](https://github.com/romanpodg/SubShare-Go/pull/11). The R06 storage, key-management and HTTP suites passed uncached before extraction. This stage separates category and ordering responsibilities within the existing SQLite adapter; it introduces no behavior correction. The follow-up below records the additional decomposition required by hosted review.

## Initial extraction boundaries and scope

`cmd/server` composes routes and services. `internal/httpapi` maps key requests and responses, `internal/keymanagement` owns commands and validation, and `storage.Repository` implements its active persistence interface. Profile parsing, configuration, credential encryption, source reconciliation and delivery already have separate packages. R07a keeps these boundaries and the existing `categoryStore` used inside profile transactions.

- `internal/storage/key_category_repository.go` owns category normalization, ensure/list/color lookup and create/update/delete operations.
- `internal/storage/key_order_repository.go` owns category ordering and complete-key ordering.
- `internal/storage/profile_repository.go` retains encrypted profile commands, legacy CRUD and profile projections. Its size falls from **1,083 to 754 lines**.
- `internal/storage/key_repository.go` retains bulk and health operations; `EnsureKeyCategory` moves into the category module.

The same `Repository` receiver, constructor and `keymanagement.Repository` interface remain active. All moved helpers and method bodies match the R06 source byte-for-byte after newline normalization. SQL statement order, transaction boundaries, error strings, ignored-error compatibility and DTOs are unchanged. There is no schema, encryption format, revision, blind-index or source-ownership change.

## Retained contracts

The existing R06 fixtures protect rename/merge across key and source references, legacy names without category IDs, destination category identity/order, rollback after a source-reference failure, and both category deletion modes. They assert ciphertext and user-assignment preservation for retained keys and cascades for deleted keys.

Ordering fixtures protect the full key-ID set, missing/duplicate/unknown IDs, rollback after an earlier successful write, and resulting list order. Existing service and HTTP fixtures retain partial category ordering with unknown names, empty-key ordering, normalized colors, the legacy lowercase default and zero category IDs in mutation responses. No new validation rule or failure acknowledgment is added.

## Initial extraction validation

The full uncached Windows backend suite passes on Go **1.26.0**: **1,106 test results, 1,101 passed and five optional skips**, with no failures. Atomic statement coverage is **57.1%** overall, **63.9%** for storage, **68.3%** for key management and **68.2%** for HTTP adapters. Category/key reorder functions have **83.3% / 86.1%** package-local statement coverage. These measurements do not imply complete semantic coverage; adapter calls made by HTTP/server tests are not included in storage's package-local instrumentation.

Focused category/order contracts, the pre-extraction storage/service/HTTP baseline, uncached migration compatibility, `go vet ./...`, `go build -trimpath -o .cache/r07a/server.exe ./cmd/server`, formatting checks and `golangci-lint run --new-from-rev=HEAD ./...` pass. The lint comparison reports **zero new issues**. The five skips require the optional existing-database snapshot and official Mihomo, Sing-box and Xray binaries.

Sandboxed coverage runs encountered temporary-file rename restrictions, followed by blocked local HTTP/UDP connections after moving temporary files into the workspace. Those incomplete runs were interrupted. The successful full run used workspace temporary directories outside the sandbox, with disposable databases and local test servers. Its JSON/JUnit reports, `coverage-validated.out`, function coverage, source reviews and extraction-parity evidence remain under `.cache/r07a/`.

CodeScene source reviews report:

| File | Before | After | Interpretation |
| --- | --- | --- | --- |
| `profile_repository.go` | 5.67 | 6.35 | Reported responsibilities fall from six to three; general read/command findings remain |
| `key_repository.go` | 8.81 | 8.81 | Bulk/health operations retain their implementation |
| `key_category_repository.go` | Extracted | 8.62 | Existing category loops and transaction branches remain |
| `key_order_repository.go` | Extracted | 9.61 | Existing full-order validation branches remain |

The advisory `cs delta main` exits 0; the delta against R06 confirms the same R07a findings. **Strict `cs delta main --error-on-warnings` exits 1:** moving simple methods raises average complexity in the remaining profile file from **7.74 to 8.00** and in the bulk/health file from **5.27 to 5.70**. Individual method complexity is unchanged. The delta examines the two modified tracked files; the new files are separately reviewed directly. Findings reported as removed from the original file have moved into the extracted modules, rather than being eliminated from the repository. No CodeScene configuration or suppression changes are introduced.

## Initial review boundary

R07a is implemented and locally validated for review, with the strict CodeScene result explicitly non-passing. R07b is next: separate safe read/projection mapping from encrypted commands, using the existing R06 corrupt/missing-secret and source-projection fixtures. R08's patch/ownership predicates remain separate. Broader A01 work, hosted CI, Linux race validation and optional external snapshot/client fixtures are not completed by this local extraction. Local race tests were not run with `CGO_ENABLED=0` and no GCC on PATH. R07a can be reverted without a stored-data conversion.

## 2026-10-08 hosted review follow-up

At **642fec7**, hosted CodeScene rejected the inherited complex/nested methods in the two new files. No gate configuration or suppression was changed. Category definition scanning, count accumulation, rename preflight, transactional reference reassignment and deletion-mode SQL selection now have named phases. Full-key ordering separately loads the complete ID set, validates count/membership/duplicates and persists the order in one transaction. Read rows close on return from their loading helper; SQL ordering, validation precedence, error wrapping, transaction ownership and ignored-error compatibility remain unchanged.

Direct CodeScene source reviews now score both `key_category_repository.go` and `key_order_repository.go` **10.00**, with no findings. The strict delta still reports the two historical average-complexity warnings in the remaining profile/bulk files; this is distinct from the hosted new-file and critical-rule failures addressed here. R07b remains a separate batch.

A fresh uncached storage/key-management/HTTP baseline passes (78.894s / 0.448s / 19.394s). The final phase-extraction category/order contracts pass (storage 10.651s, HTTP 4.540s), as does the category route suite (1.605s). `go vet ./...` and lint against `origin/main` pass with zero issues. Hosted checks and automated review must pass at the final PR head before merge. Copilot reported its review quota exhausted; Codex completed review of the initial commit with no major findings and is requested again for the phase decomposition.
