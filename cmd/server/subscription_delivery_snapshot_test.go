package main

import (
	"database/sql"
	"testing"
)

type persistedDeliveryState struct {
	ID          int64
	SourceID    sql.NullInt64
	Raw         string
	Label       string
	Status      string
	Fingerprint string
	CreatedAt   string
}

func loadPersistedDeliveryState(t *testing.T, app *App, firstID, secondID int64) []persistedDeliveryState {
	t.Helper()
	rows, queryErr := app.db.Query(`
			SELECT k.id, k.external_source_id, s.encrypted_url, k.label, k.status, COALESCE(k.profile_fingerprint, ''), CAST(k.created_at AS TEXT)
			FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id IN (?, ?) ORDER BY k.id
		`, firstID, secondID)
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	defer rows.Close()
	var result []persistedDeliveryState
	for rows.Next() {
		var item persistedDeliveryState
		if scanErr := rows.Scan(&item.ID, &item.SourceID, &item.Raw, &item.Label, &item.Status, &item.Fingerprint, &item.CreatedAt); scanErr != nil {
			t.Fatal(scanErr)
		}
		result = append(result, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		t.Fatal(rowsErr)
	}
	return result
}
