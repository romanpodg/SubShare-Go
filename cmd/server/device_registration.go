package main

import (
	"database/sql"
	"fmt"
)

// registerHWID accepts a device only after its registration or metadata refresh
// commits. The first transactional UPDATE serializes capacity checks with other
// registrants; moving the count before that write would change SQLite locking.
func (a *App) registerHWID(userID int64, hwid string, meta deviceMeta) (bool, error) {
	hwid, meta = normalizeDeviceRegistration(hwid, meta)
	if meta.NormalizedHWID == "" {
		return true, nil
	}

	tx, err := a.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	allowed, err := registerDeviceInTransaction(tx, userID, hwid, meta)
	if err != nil || !allowed {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}
	return true, nil
}

func registerDeviceInTransaction(tx *sql.Tx, userID int64, hwid string, meta deviceMeta) (bool, error) {
	updated, err := updateRegisteredDevice(tx, userID, meta)
	if err != nil {
		return false, err
	}
	if updated {
		return true, nil
	}
	capacity, err := loadDeviceCapacity(tx, userID)
	if err != nil {
		return false, err
	}
	if !capacity.available() {
		return false, nil
	}
	if err := insertRegisteredDevice(tx, userID, hwid, meta); err != nil {
		return false, err
	}
	return true, nil
}
