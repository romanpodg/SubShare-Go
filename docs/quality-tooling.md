# Quality tooling

## CodeScene

CodeScene CLI authentication is local. No CodeScene token or paid MCP setup is
stored in this repository. Review a tracked source file and compare work to main:

```sh
cs version
cs review --output-format json internal/keymanagement/create.go
cs delta main
```

`cs delta main` includes uncommitted work. It reports findings but exits zero by
default; use `cs delta main --error-on-warnings` when a regression gate is wanted.
On main with no source changes, an empty delta is expected. Audit inputs are Git
history, CodeScene reviews/deltas, Codecov reports, source code and tests.

## Codecov

The existing main push / main PR CI generates Go Coverage Analytics and JUnit
Test Analytics without a second full Go test run:

```sh
go install gotest.tools/gotestsum@v1.13.0
gotestsum --junitfile .cache/junit/go.xml -- -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
gotestsum --junitfile .cache/junit/migration.xml -- ./internal/storage -run TestMigrateAppliesVersionedMigrationsIdempotently -count=1
cd frontend
npm test -- --reporter=default --reporter=junit --outputFile.junit=../.cache/junit/vitest.xml
npm run test:e2e
```

In PowerShell, quote `'-coverprofile=coverage.out'` and `'-func=coverage.out'`.
Playwright keeps its existing console reporter and writes
`frontend/test-results/playwright.xml` with desktop/mobile project names.
The dedicated uncached migration run is retained. Docker deployment smoke checks
and operational shell scripts do not produce JUnit and are not given synthetic
reports. External-client/snapshot Go tests retain their existing environment-based
skips when their optional fixtures are absent.

Official `codecov/codecov-action@v7` uploads explicit files, disables discovery,
and fails CI on upload errors. JUnit upload conditions allow uploads after test
failures, unless cancelled or no report exists; test failures remain failures.
Flags distinguish `backend`, `migration`, `frontend-unit` and `frontend-e2e`.
Coverage currently measures the Go backend; frontend test counts are not coverage.

`codecov.yml` makes project and patch coverage informational with automatic
baselines (project tolerance: one percentage point). No production code is
excluded. Annotations are enabled, with one updating PR comment only when coverage
changes. Revisit blocking thresholds after the first main baseline and audit.

Bundle Analysis is configured for the real Next.js 16 Turbopack static export.
The supported `@codecov/bundle-analyzer@2.0.1` asset analyzer reads
`frontend/out/_next/static` after the existing production build; it does not
switch bundlers or build again. The analyzer is installed separately in the CI
runner's temporary directory because its package supports Linux/macOS only.
For a local dry run with an isolated tool installation:

```sh
CODECOV_BUNDLE_ANALYZER_DIR=/path/to/tool-install node scripts/codecov-bundle.cjs --dry-run
```

The dry-run report is `.cache/bundle-report.json`; it performs no upload.
Asset analysis shows sizes/compression without bundler module/import attribution.
The pre-existing E2E command still builds its own export; no additional build is
introduced for Codecov.

### Authentication and future API audits

`CODECOV_TOKEN` is a GitHub Actions repository secret, used only by upload steps.
It is never embedded in a browser bundle. Fork PRs still run checks, but all
Codecov uploads are skipped because repository secrets are unavailable. This also
requires provisioning the token as a Dependabot secret for same-repository
Dependabot PR uploads; otherwise those uploads fail visibly.
Do not use privileged PR triggers to expose secrets.

`CODECOV_API_TOKEN` is an optional local environment variable for Codex read-only
API analysis, distinct from the Actions upload token. Never write its value into
repository files or logs. With it available, use authenticated HTTP GET requests
to the Codecov v2 API at `https://api.codecov.io/api/v2/github/romanpodg/repos/SubShare-Go/`
for repository totals, coverage reports, file reports, coverage trends and pull
coverage, selecting branch `main`. Use `Authorization: Bearer` authentication;
report only status and relevant coverage data. A missing report before the first
successful CI upload is expected. No permanent API wrapper is required.

Official references: [Codecov Action](https://github.com/codecov/codecov-action),
[asset bundle analyzer](https://docs.codecov.com/docs/bundle-analyzer-quick-start),
[Codecov API](https://docs.codecov.com/reference/overview).
