package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
)

type administratorPasswordHasher interface {
	Hash(string) (string, error)
	RehashVerified(string) (string, error)
	Verify(string, string) (adminpassword.Verification, error)
}

func (a *App) passwordHasher() *adminpassword.Hasher {
	if a.adminPasswordHasher != nil {
		return a.adminPasswordHasher
	}
	return adminpassword.NewDefault()
}

// ensureBootstrapOwner serializes the empty-administrator check and insert
// with BEGIN IMMEDIATE so parallel startup attempts cannot create two owners.
func ensureBootstrapOwner(
	ctx context.Context,
	db *sql.DB,
	username, suppliedPassword string,
	hasher administratorPasswordHasher,
) (bool, error) {
	connection, err := db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("open bootstrap database connection")
	}
	defer connection.Close()

	if _, err := connection.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
		return false, fmt.Errorf("configure bootstrap database connection")
	}
	if _, err := connection.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return false, fmt.Errorf("begin bootstrap administrator check")
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = connection.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	var administratorCount int
	if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&administratorCount); err != nil {
		return false, fmt.Errorf("count bootstrap administrators")
	}
	if administratorCount > 0 {
		if _, err := connection.ExecContext(ctx, `COMMIT`); err != nil {
			return false, fmt.Errorf("complete bootstrap administrator check")
		}
		committed = true
		return false, nil
	}

	if suppliedPassword == "" {
		return false, fmt.Errorf("ADMIN_PASSWORD is required when creating the initial owner")
	}
	passwordHash, err := hasher.Hash(suppliedPassword)
	if err != nil {
		if adminpassword.IsPolicyError(err) {
			return false, fmt.Errorf("ADMIN_PASSWORD: %w", err)
		}
		return false, fmt.Errorf("hash bootstrap administrator password")
	}
	if _, err := connection.ExecContext(
		ctx,
		`INSERT INTO admins (username, password_hash, role) VALUES (?, ?, 'owner')`,
		username,
		passwordHash,
	); err != nil {
		return false, fmt.Errorf("create bootstrap administrator")
	}
	if _, err := connection.ExecContext(ctx, `COMMIT`); err != nil {
		return false, fmt.Errorf("commit bootstrap administrator")
	}
	committed = true
	return true, nil
}

// authenticateAdministrator verifies both legacy bcrypt and Argon2id hashes.
// Successful hashes that need upgrading are replaced with a compare-and-swap
// update; a rehash failure is operationally visible but never breaks login.
func (a *App) authenticateAdministrator(ctx context.Context, username, suppliedPassword string) (int64, bool, error) {
	var administratorID int64
	var encodedHash string
	if err := a.db.QueryRowContext(
		ctx,
		`SELECT id, password_hash FROM admins WHERE username = ?`,
		username,
	).Scan(&administratorID, &encodedHash); err != nil {
		if err == sql.ErrNoRows {
			a.passwordHasher().VerifyUnknown(suppliedPassword)
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("load administrator credentials")
	}

	verification, err := a.passwordHasher().Verify(suppliedPassword, encodedHash)
	if err != nil || !verification.Valid {
		return 0, false, nil
	}
	if verification.NeedsRehash {
		replacement, hashErr := a.passwordHasher().RehashVerified(suppliedPassword)
		if hashErr != nil {
			slog.Warn("administrator password hash migration failed", "admin_id", administratorID)
			return administratorID, true, nil
		}
		if _, updateErr := a.db.ExecContext(
			ctx,
			`UPDATE admins SET password_hash = ? WHERE id = ? AND password_hash = ?`,
			replacement,
			administratorID,
			encodedHash,
		); updateErr != nil {
			slog.Warn("administrator password hash migration write failed", "admin_id", administratorID)
		}
	}
	return administratorID, true, nil
}
