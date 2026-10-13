package main

import (
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKeyAssignmentModesControlFutureKeys(t *testing.T) {
	app := newIntegrationApp(t)
	allUser := seedSubscriptionUser(t, app, model.UserStatusActive)
	selectedResult, err := app.db.Exec(`
		INSERT INTO users(name, token, activation_code, subscription_id, status, key_assignment_mode)
		VALUES('Selected', 'selected-token', 'selected-code', 'selected-sub', 'active', 'selected')
	`)
	requireFixturef(t, err == nil, "seed selected user: %v", err)

	selectedUser, err := selectedResult.LastInsertId()
	requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

	body := `{"label":"new","url":"vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls","status":"active","kind":"real"}`
	recorder := httptest.NewRecorder()
	app.keys().legacy.CreateKey(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(body)))
	requireFixturef(t, recorder.Code == http.StatusOK, "create key status=%d body=%s", recorder.Code, recorder.Body.String())

	var allCount, selectedCount int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ?`, allUser).Scan(&allCount)
		requireFixture(t, err == nil, err)
	}

	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ?`, selectedUser).Scan(&selectedCount)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, allCount == 1, "future assignment mismatch: all=%d selected=%d", allCount, selectedCount)
	requireFixturef(t, selectedCount == 0, "future assignment mismatch: all=%d selected=%d", allCount, selectedCount)

}

func TestUpdateKeyAssignmentAllSelectedAndValidation(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	activeID, activeKey, err := app.profileKeyring.GetActiveEncryptionKey()
	requireFixturef(t, err == nil, "fixture GetActiveEncryptionKey: %v", err)

	_, bikKey, err := app.profileKeyring.GetActiveBlindIndexKey()
	requireFixturef(t, err == nil, "fixture GetActiveBlindIndexKey: %v", err)

	first, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status) VALUES('one', ?, 'active')`, profilestorage.ComputeBlindIndex(bikKey, "vless://assignment-one"))
	requireFixture(t, err == nil, err)

	firstID, err := first.LastInsertId()
	requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

	env1, err := profilestorage.Encrypt([]byte("vless://assignment-one"), activeID, activeKey, firstID)
	requireFixturef(t, err == nil, "fixture Encrypt: %v", err)

	{
		_, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, firstID, env1)
		requireFixturef(t, err == nil, "fixture Exec: %v", err)
	}

	second, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status) VALUES('two', ?, 'active')`, profilestorage.ComputeBlindIndex(bikKey, "vless://assignment-two"))
	requireFixture(t, err == nil, err)

	secondID, err := second.LastInsertId()
	requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

	env2, err := profilestorage.Encrypt([]byte("vless://assignment-two"), activeID, activeKey, secondID)
	requireFixturef(t, err == nil, "fixture Encrypt: %v", err)

	{
		_, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, secondID, env2)
		requireFixturef(t, err == nil, "fixture Exec: %v", err)
	}

	{
		err := app.updateUserKeyAssignment(userID, model.KeyAssignmentModeAll, nil)
		requireFixturef(t, err == nil, "assign all: %v", err)
	}

	var count int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ?`, userID).Scan(&count)
		requireFixturef(t, err == nil, "all assignment count=%d err=%v", count, err)
		requireFixturef(t, count == 2, "all assignment count=%d err=%v", count, err)
	}

	{
		err := app.updateUserKeyAssignment(userID, model.KeyAssignmentModeSelected, []int64{secondID, secondID})
		requireFixturef(t, err == nil, "assign selected: %v", err)
	}

	var selectedID int64
	{
		err := app.db.QueryRow(`SELECT key_id FROM user_keys WHERE user_id = ?`, userID).Scan(&selectedID)
		requireFixturef(t, err == nil, "selected assignment=%d err=%v", selectedID, err)
		requireFixturef(t, selectedID == secondID, "selected assignment=%d err=%v", selectedID, err)
	}

	{
		err := app.updateUserKeyAssignment(userID, "invalid", []int64{firstID})
		requireFixture(t, err != nil, "invalid mode was accepted")
	}

	{
		err := app.updateUserKeyAssignment(userID, model.KeyAssignmentModeSelected, []int64{999999})
		requireFixturef(t, errors.Is(err, errAssignmentKeyNotFound), "missing key error=%v", err)
	}

}
