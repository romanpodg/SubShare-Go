package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBackupAssuranceNativeExportScriptContracts(t *testing.T) {
	cases := []struct {
		name, configured, failure string
		wantError                 bool
		copies                    int
	}{
		{"default", "/app/data/backups/app.db", "", false, 2},
		{"custom", "/app/data/custom/provider.db", "", false, 2},
		{"spaces", "/app/data/custom archive/provider.db", "", false, 2},
		{"padded custom", " \t/app/data/custom/provider.db\t ", "", false, 2},
		{"whitespace disabled", " \t ", "", true, 0},
		{"unicode padded custom", "\u00a0\u3000/app/data/custom/provider.db\u2003", "", false, 2},
		{"unicode whitespace disabled", "\u00a0\u2003", "", true, 0},
		{"unicode interior", "/app/data/备份/provider.db", "", false, 2},
		{"disabled", "", "", true, 0},
		{"configuration unavailable", "/app/data/backups/app.db", "lookup", true, 0},
		{"file keyring unavailable", "/app/data/backups/app.db", "keyring", true, 0},
		{"database copy fails", "/app/data/backups/app.db", "database-copy", true, 1},
		{"keyring copy fails", "/app/data/backups/app.db", "keyring-copy", true, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, err := filepath.Abs("../..")
			backupAssuranceSuccess(t, err)
			destination := t.TempDir()
			capture := filepath.Join(destination, "copies.txt")
			command := backupExportFixtureCommand(t, root)
			command.Env = append(os.Environ(), "SUBSHARE_BACKUP_TEST_ROOT="+root, "SUBSHARE_BACKUP_TEST_DESTINATION="+destination, "SUBSHARE_BACKUP_TEST_CAPTURE="+capture, "SUBSHARE_BACKUP_TEST_PATH="+c.configured, "SUBSHARE_BACKUP_TEST_FAILURE="+c.failure)
			output, err := command.CombinedOutput()
			backupAssuranceEqual(t, "native export outcome", err != nil, c.wantError)
			copies := backupExportCapturedCopies(t, capture)
			backupAssuranceEqual(t, "copy admission", len(copies), c.copies)
			if c.copies > 0 {
				backupAssuranceEqual(t, "configured database copied", copies[0], "backend:"+strings.TrimSpace(c.configured))
			}
			if c.copies > 1 {
				backupAssuranceEqual(t, "paired keyring copied", copies[1], "backend:/app/data/keyring.json")
			}
			backupAssuranceEqual(t, "success feedback", strings.Contains(string(output), "Backup exported to:"), !c.wantError)
		})
	}
}

func backupExportFixtureCommand(t *testing.T, root string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, backupExportPowerShell(t), "-NoProfile", "-File", filepath.Join(root, "testdata", "backup", "export-powershell-fixture.ps1"))
	}
	path, err := exec.LookPath("bash")
	backupAssuranceSuccess(t, err)
	return exec.CommandContext(ctx, path, filepath.Join(root, "testdata", "backup", "export-bash-fixture.sh"))
}

func backupExportPowerShell(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"pwsh", "powershell"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Fatal("a native PowerShell runtime is required for the Windows export contract")
	return ""
}

func backupExportCapturedCopies(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	backupAssuranceSuccess(t, err)
	return strings.FieldsFunc(string(data), func(char rune) bool { return char == '\n' || char == '\r' })
}
