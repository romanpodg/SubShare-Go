//go:build dockerfixture

package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

// Explicitly selected fixture tooling; absent from ordinary test execution.
// Inputs are generated in the ignored workspace, never operational data.
func TestWriteDockerUpgradeFixture(t *testing.T) {
	directory := assuranceArtifactPath(t, os.Getenv("SUBSHARE_DOCKER_FIXTURE_DIR"))
	f := newMigrationAssuranceFixture(t, 11, 3)
	profileContractSuccess(t, f.db.Close())
	profileContractSuccess(t, os.MkdirAll(directory, 0700))
	data, err := os.ReadFile(f.path)
	profileContractSuccess(t, err)
	profileContractSuccess(t, os.WriteFile(filepath.Join(directory, "app.db"), data, 0600))
	profileContractSuccess(t, os.WriteFile(filepath.Join(directory, "keyring.json"), f.keyringJSON, 0600))
	lines := []string{migrationAssuranceURI(1), migrationAssuranceURI(2), migrationAssuranceURI(3)}
	profileContractSuccess(t, os.WriteFile(filepath.Join(directory, "expected-body.txt"), []byte(strings.Join(lines, "\n")), 0600))
}

func TestVerifyDockerRestoredFixture(t *testing.T) {
	path := assuranceArtifactPath(t, os.Getenv("SUBSHARE_DOCKER_RESTORE_DB"))
	keyringPath := assuranceArtifactPath(t, os.Getenv("SUBSHARE_DOCKER_RESTORE_KEYRING"))
	data, err := os.ReadFile(keyringPath)
	profileContractSuccess(t, err)
	keyring, err := profilestorage.LoadKeyringJSON(data)
	profileContractSuccess(t, err)
	db := assuranceReadOnlyDB(t, path)
	f := migrationAssuranceFixture{db: db, path: path, keyring: keyring, keyringJSON: data}
	f.assertUpgraded(t, 3)
}

func assuranceArtifactPath(t *testing.T, path string) string {
	t.Helper()
	if path == "" {
		t.Fatal("explicit generated fixture path is required")
	}
	root, err := filepath.Abs("../..")
	profileContractSuccess(t, err)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.Abs(path)
	profileContractSuccess(t, err)
	relative, err := filepath.Rel(filepath.Join(root, ".cache"), path)
	profileContractSuccess(t, err)
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatal("fixture artifacts must stay in the ignored workspace cache")
	}
	return path
}

func assuranceReadOnlyDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_pragma=foreign_keys%3DON")
	profileContractSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
