package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
)

// A rebuild temporarily suspends enforcement on one reserved connection. Never
// return that connection to the pool if restoring or proving enforcement fails.
func restoreMigrationForeignKeys(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		discardMigrationConnection(conn)
		return fmt.Errorf("restore foreign_keys: %w", err)
	}
	var enabled int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		discardMigrationConnection(conn)
		return fmt.Errorf("verify foreign_keys status: %w", err)
	}
	if enabled != 1 {
		discardMigrationConnection(conn)
		return fmt.Errorf("foreign_keys failed to re-enable (status: %d)", enabled)
	}
	return nil
}

func discardMigrationConnection(conn *sql.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
}
