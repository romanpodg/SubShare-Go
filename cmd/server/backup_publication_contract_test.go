package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var errBackupPublicationFixture = errors.New("fixture publication failure")

func (f backupAssuranceFixture) publicationPaths() sqliteBackupPaths {
	return sqliteBackupPaths{backup: f.path, candidate: f.path + ".tmp", previous: f.path + ".previous"}
}

func (f backupAssuranceFixture) candidate(t *testing.T) {
	t.Helper()
	backupAssuranceExec(t, f.app.db, `VACUUM INTO ?`, f.path+".tmp")
	backupAssuranceSuccess(t, verifySQLiteBackup(f.path+".tmp"))
}

func TestBackupAssurancePublicationFailureRestoresPreviousBytes(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	before := backupAssuranceBytes(t, f.path)
	backupAssuranceExec(t, f.app.db, `UPDATE users SET name='New candidate' WHERE id=41`)
	f.candidate(t)
	rename := func(from, to string) error {
		if from == f.path+".tmp" {
			return errBackupPublicationFixture
		}
		return os.Rename(from, to)
	}
	backupAssuranceError(t, publishSQLiteBackup(f.publicationPaths(), rename), errBackupPublicationFixture)
	backupAssuranceUnchanged(t, f.path, before)
	backupAssuranceMissing(t, f.path+".tmp")
	backupAssuranceMissing(t, f.path+".previous")
	backupAssuranceSuccess(t, runValidateBackup(f.path, f.exportedKeyring(t)))
}

func TestBackupAssurancePublicationFailureWithoutPreviousKeepsLiveCredentials(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	backupAssuranceSuccess(t, os.MkdirAll(filepath.Dir(f.path), 0700))
	f.candidate(t)
	rename := func(string, string) error { return errBackupPublicationFixture }
	backupAssuranceError(t, publishSQLiteBackup(f.publicationPaths(), rename), errBackupPublicationFixture)
	backupAssuranceMissing(t, f.path)
	backupAssuranceMissing(t, f.path+".tmp")
	backupAssuranceMissing(t, f.path+".previous")
	backupAssuranceEqual(t, "live credentials retained", backupAssuranceRaw(t, f.app.db, f.app.profileKeyring, f.keyID), backupAssuranceURI)
}

func TestBackupAssuranceFailedRollbackRetainsRecoverablePreviousArtifact(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	before := backupAssuranceBytes(t, f.path)
	f.candidate(t)
	rename := func(from, to string) error {
		if from != f.path {
			return errBackupPublicationFixture
		}
		return os.Rename(from, to)
	}
	backupAssuranceError(t, publishSQLiteBackup(f.publicationPaths(), rename), errBackupPublicationFixture)
	backupAssuranceMissing(t, f.path)
	backupAssuranceMissing(t, f.path+".tmp")
	backupAssuranceUnchanged(t, f.path+".previous", before)
	backupAssuranceSuccess(t, runValidateBackup(f.path+".previous", f.exportedKeyring(t)))
}
