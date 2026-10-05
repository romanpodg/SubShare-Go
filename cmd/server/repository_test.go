package main

import (
	"database/sql"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestRepositoryListUsersProjection(t *testing.T) {
	app := newIntegrationApp(t)
	users, err := app.listUsers()
	if err != nil || users != nil {
		t.Fatalf("empty list = %#v, err = %v", users, err)
	}
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	stamp := time.Date(2000, 1, 2, 3, 4, 0, 0, time.UTC)
	_, err = app.db.Exec(`UPDATE users SET time_zone = ' Asia/Omsk ', language = ' en ',
		activation_code = ' activation ', subscription_id = ' subscription ',
		subscription_name = ' Personal ', subscription_refresh_hours = -1,
		subscription_info_url = ' info ', subscription_extra_url = ' extra ',
		subscription_extra_status = ' notice ', activation_used_at = ?, starts_at = ?,
		expires_at = ?, blocked_reason = ' reason ', key_assignment_mode = 'selected' WHERE id = ?`,
		stamp, stamp, stamp.Add(time.Hour), userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`INSERT INTO user_devices(user_id, hwid, normalized_hwid, app_name, user_agent, last_seen_at)
		VALUES(?, ' OLD ', NULL, ' Stored ', 'Happ/3.0 (Android 14)', ?),
		      (?, 'new', 'new', NULL, 'Happ/3.0 (Android 14)', ?)`, userID, stamp, userID, stamp.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(`INSERT INTO users(name, email, token, time_zone, language, max_devices, subscription_refresh_hours)
		VALUES('Empty', '', 'empty-token', ' ', ' ', -1, 0)`)
	if err != nil {
		t.Fatal(err)
	}
	keyID := insertAssignedDeliveryKey(t, app, userID, nil, "Assigned", externalTestVLESS, "vless", "full", 1)
	users, err = app.listUsers()
	if err != nil || len(users) != 2 {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	empty, user := users[0], users[1]
	if empty.ID <= userID || user.ID != userID {
		t.Fatalf("users not ordered by descending id: %#v", users)
	}
	if empty.TimeZone != "Europe/Moscow" || empty.Language != "ru" || empty.SubscriptionRefreshHours != 12 || empty.MaxDevices != 0 {
		t.Fatalf("defaults = %#v", empty)
	}
	if empty.ConnectedDevices == nil || empty.ConnectedHWIDs == nil || len(empty.ConnectedDevices) != 0 || len(empty.ConnectedHWIDs) != 0 {
		t.Fatalf("empty device arrays = %#v", empty)
	}
	if user.TimeZone != "Asia/Omsk" || user.Language != "en" || user.ActivationCode != "activation" || user.SubscriptionID != "subscription" ||
		user.SubscriptionName != "Personal" || user.SubscriptionRefreshHours != 12 || user.SubscriptionInfoURL != "info" ||
		user.SubscriptionExtraURL != "extra" || user.SubscriptionExtraStatus != "notice" || user.BlockedReason != "reason" || user.KeyAssignmentMode != "selected" {
		t.Fatalf("trimmed user fields = %#v", user)
	}
	if user.ActivationUsedAt != "02/01/2000 09:04" || user.StartsAtInput != "02/01/2000 09:04" || user.ExpiresAtInput != "02/01/2000 10:04" || user.EffectiveStatus != "expired" {
		t.Fatalf("localized dates/status = %#v", user)
	}
	if user.ConnectedDeviceCount != 2 || len(user.ConnectedHWIDs) != 2 || len(user.ConnectedDevices) != 2 {
		t.Fatalf("device aggregates = %#v", user)
	}
	if user.AssignedKeyIDs != strconv.FormatInt(keyID, 10) || empty.AssignedKeyIDs != "" {
		t.Fatalf("assigned keys = %q, empty = %q", user.AssignedKeyIDs, empty.AssignedKeyIDs)
	}
	newer, older := user.ConnectedDevices[0], user.ConnectedDevices[1]
	if newer.HWID != "new" || older.HWID != "OLD" || older.NormalizedHWID != "old" || older.AppName != "Stored" || newer.AppName != "Happ" || newer.Platform != "Android" || newer.OSVersion != "14" {
		t.Fatalf("device ordering and metadata = %#v", user.ConnectedDevices)
	}
	if older.LastSeenAt != stamp.Local().Format("2006-01-02 15:04:05") {
		t.Fatalf("last seen = %q", older.LastSeenAt)
	}
	if _, err := app.db.Exec(`UPDATE users SET time_zone = 'invalid/zone' WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	users, err = app.listUsers()
	if err != nil || users[1].ActivationUsedAt != "02/01/2000 03:04" {
		t.Fatalf("invalid zone UTC fallback: users = %#v, err = %v", users, err)
	}
}

func TestRepositorySubscriptionSettings(t *testing.T) {
	app := newIntegrationApp(t)
	_, err := app.db.Exec(`UPDATE subscription_settings SET title = NULL, refresh_hours = -1,
		subscription_format = 'invalid', time_zone = ' ', language = ' ' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := app.getSubscriptionSettings()
	defaults := model.SubscriptionSettings{Title: "AllKeys", RefreshHours: 12, SubscriptionFormat: model.SubscriptionFormatLinks, TimeZone: "Europe/Moscow", Language: "ru"}
	if err != nil || !reflect.DeepEqual(settings, defaults) {
		t.Fatalf("settings = %#v, err = %v, want %#v", settings, err, defaults)
	}
	nullSettings, err := scanSubscriptionSettings(app.db.QueryRow(`SELECT NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL`))
	if err != nil || !reflect.DeepEqual(nullSettings, defaults) {
		t.Fatalf("null settings = %#v, err = %v, want %#v", nullSettings, err, defaults)
	}
	_, err = app.db.Exec(`UPDATE subscription_settings SET title = ' Title ', refresh_hours = 24,
		info_url = ' info ', extra_url = ' extra ', extra_status = ' notice ', subscription_format = ' XRAY-JSON ',
		show_subscription_expiration = -1, time_zone = ' UTC ', language = ' en ', provider_id = ' provider ',
		happ_no_limit_mode = 1, happ_no_limit_mode_xhttp_only = 2, happ_mandatory_hwid = -1,
		happ_notify_expiration = 1, happ_hide_server_settings = 1, happ_subscription_body = ' body ' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	want := model.SubscriptionSettings{
		Title: "Title", RefreshHours: 24, InfoURL: "info", ExtraURL: "extra", ExtraStatus: "notice",
		SubscriptionFormat: model.SubscriptionFormatXrayJSON, ShowSubscriptionExpiration: true,
		TimeZone: "UTC", Language: "en", ProviderID: "provider", HappNoLimitMode: true,
		HappNoLimitModeXHTTPOnly: true, HappMandatoryHWID: true, HappNotifyExpiration: true,
		HappHideServerSettings: true, HappSubscriptionBody: " body ",
	}
	settings, err = app.getSubscriptionSettings()
	if err != nil || !reflect.DeepEqual(settings, want) {
		t.Fatalf("settings = %#v, err = %v, want %#v", settings, err, want)
	}
	if _, err := app.db.Exec(`DELETE FROM subscription_settings`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.getSubscriptionSettings(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing settings error = %v", err)
	}
}

func TestRepositorySubscriptionAccess(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	tests := []struct {
		name, status, reason, subscription string
		starts, expires                    any
		code                               int
		wantReason                         string
	}{
		{"active", "active", "", "subscription-token", past, future, 200, ""},
		{"null dates", "active", "", "subscription-token", nil, nil, 200, ""},
		{"unknown status", "unknown", "", "subscription-token", nil, nil, 200, ""},
		{"blocked precedence", " BLOCKED ", " custom ", "subscription-token", future, past, 403, "custom"},
		{"blocked default", "blocked", " ", "subscription-token", nil, nil, 403, "subscription blocked"},
		{"paused precedence", "paused", "", "subscription-token", future, past, 403, "subscription paused"},
		{"future precedence", "active", "", "subscription-token", future, past, 403, "subscription is not active yet"},
		{"expired", "active", "", "subscription-token", past, past, 410, "subscription expired"},
		{"missing", "active", "", "missing", nil, nil, 404, "subscription not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := app.db.Exec(`UPDATE users SET status = ?, blocked_reason = ?, starts_at = ?, expires_at = ? WHERE id = ?`, test.status, test.reason, test.starts, test.expires, userID); err != nil {
				t.Fatal(err)
			}
			allowed, id, code, reason, err := app.subscriptionAccessAllowed(test.subscription)
			wantID := userID
			if test.code == http.StatusNotFound {
				wantID = 0
			}
			if err != nil || allowed != (test.code == http.StatusOK) || id != wantID || code != test.code || reason != test.wantReason {
				t.Fatalf("access = %v, %d, %d, %q, %v", allowed, id, code, reason, err)
			}
		})
	}
}

func TestRepositoryRegisterHWID(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	for _, limit := range []int{0, -1, 2} {
		t.Run("limit="+strconv.Itoa(limit), func(t *testing.T) {
			if _, err := app.db.Exec(`DELETE FROM user_devices; UPDATE users SET max_devices = ? WHERE id = ?`, limit, userID); err != nil {
				t.Fatal(err)
			}
			for _, hwid := range []string{" ONE ", "two"} {
				allowed, err := app.registerHWID(userID, hwid, deviceMeta{DeviceName: "Phone", AppVersion: "1"})
				if err != nil || !allowed {
					t.Fatalf("register %q = %v, %v", hwid, allowed, err)
				}
			}
			allowed, err := app.registerHWID(userID, "one", deviceMeta{AppVersion: "2"})
			if err != nil || !allowed {
				t.Fatalf("existing device at limit = %v, %v", allowed, err)
			}
			var count int
			var name, version, raw string
			if err := app.db.QueryRow(`SELECT hwid, device_name, app_version FROM user_devices WHERE user_id = ? AND normalized_hwid = 'one'`, userID).Scan(&raw, &name, &version); err != nil {
				t.Fatal(err)
			}
			if raw != "ONE" || name != "Phone" || version != "2" {
				t.Fatalf("updated metadata: raw=%q, name=%q, version=%q", raw, name, version)
			}
			allowed, err = app.registerHWID(userID, "three", deviceMeta{})
			if err != nil || allowed != (limit <= 0) {
				t.Fatalf("new device = %v, %v", allowed, err)
			}
			if err := app.db.QueryRow(`SELECT COUNT(*) FROM user_devices WHERE user_id = ?`, userID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			wantCount := 2
			if limit <= 0 {
				wantCount = 3
			}
			if count != wantCount {
				t.Fatalf("device count = %d, want %d", count, wantCount)
			}
		})
	}
	if allowed, err := app.registerHWID(userID, " ", deviceMeta{}); err != nil || !allowed {
		t.Fatalf("empty HWID = %v, %v", allowed, err)
	}
}

func TestRepositoryRegisterHWIDLegacyCountAndRollback(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	_, err := app.db.Exec(`UPDATE users SET max_devices = 2 WHERE id = ?;
		INSERT INTO user_devices(user_id, hwid, normalized_hwid) VALUES(?, ' OLD ', NULL), (?, 'old', '')`, userID, userID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if allowed, err := app.registerHWID(userID, "new", deviceMeta{}); err != nil || !allowed {
		t.Fatalf("distinct legacy HWIDs should count once: %v, %v", allowed, err)
	}
	if allowed, err := app.registerHWID(userID, "third", deviceMeta{}); err != nil || allowed {
		t.Fatalf("legacy limit not enforced: %v, %v", allowed, err)
	}
	_, err = app.db.Exec(`UPDATE users SET max_devices = 0 WHERE id = ?;
		CREATE TRIGGER reject_device BEFORE INSERT ON user_devices BEGIN SELECT RAISE(ABORT, 'reject device'); END`, userID)
	if err != nil {
		t.Fatal(err)
	}
	if allowed, err := app.registerHWID(userID, "rejected", deviceMeta{}); err == nil || allowed {
		t.Fatalf("insert failure = %v, %v", allowed, err)
	}
	if _, err := app.db.Exec(`DROP TRIGGER reject_device`); err != nil {
		t.Fatal(err)
	}
	if allowed, err := app.registerHWID(userID, "accepted", deviceMeta{}); err != nil || !allowed {
		t.Fatalf("transaction not released after failure: %v, %v", allowed, err)
	}
	if allowed, err := app.registerHWID(userID+1, "missing", deviceMeta{}); !errors.Is(err, sql.ErrNoRows) || allowed {
		t.Fatalf("missing user = %v, %v", allowed, err)
	}
}

func TestRepositoryRedeemActivationCode(t *testing.T) {
	for _, test := range []struct {
		name   string
		stored any
	}{
		{"null", nil},
		{"blank", " "},
		{"existing", " existing "},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			userID := seedSubscriptionUser(t, app, model.UserStatusActive)
			if _, err := app.db.Exec(`UPDATE users SET subscription_id = ? WHERE id = ?`, test.stored, userID); err != nil {
				t.Fatal(err)
			}
			id, code, reason, err := app.redeemActivationCode("activation-token")
			if err != nil || code != http.StatusOK || reason != "" || id == "" {
				t.Fatalf("redeem = %q, %d, %q, %v", id, code, reason, err)
			}
			if test.stored == " existing " && id != "existing" {
				t.Fatalf("existing subscription replaced: %q", id)
			}
			var persisted string
			var used sql.NullTime
			if err := app.db.QueryRow(`SELECT subscription_id, activation_used_at FROM users WHERE id = ?`, userID).Scan(&persisted, &used); err != nil {
				t.Fatal(err)
			}
			if persisted != id || !used.Valid {
				t.Fatalf("activation not persisted: %q, %+v", persisted, used)
			}
			id, code, reason, err = app.redeemActivationCode("activation-token")
			if err != nil || id != "" || code != http.StatusForbidden || reason != "Ключ уже активирован" {
				t.Fatalf("repeat redeem = %q, %d, %q, %v", id, code, reason, err)
			}
			id, code, reason, err = app.redeemActivationCode("missing")
			if err != nil || id != "" || code != http.StatusNotFound || reason != "Подписка не найдена" {
				t.Fatalf("missing redeem = %q, %d, %q, %v", id, code, reason, err)
			}
		})
	}
}

func TestRepositoryRedeemActivationConditionalUpdate(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	// Model another redemption winning between the SELECT and conditional UPDATE.
	_, err := app.db.Exec(`CREATE TRIGGER activation_claimed BEFORE UPDATE OF activation_used_at ON users
		BEGIN SELECT RAISE(IGNORE); END`)
	if err != nil {
		t.Fatal(err)
	}
	id, code, reason, err := app.redeemActivationCode("activation-token")
	if err != nil || id != "" || code != http.StatusForbidden || reason != "Ключ уже активирован" {
		t.Fatalf("lost activation claim = %q, %d, %q, %v", id, code, reason, err)
	}
	if _, err := app.db.Exec(`DROP TRIGGER activation_claimed;
		CREATE TRIGGER activation_failed BEFORE UPDATE OF activation_used_at ON users
		BEGIN SELECT RAISE(ABORT, 'activation failure'); END`); err != nil {
		t.Fatal(err)
	}
	id, code, reason, err = app.redeemActivationCode("activation-token")
	if err == nil || id != "" || code != 0 || reason != "" {
		t.Fatalf("activation update error = %q, %d, %q, %v", id, code, reason, err)
	}
}

func TestRepositorySubscriptionAccessTimeBoundaries(t *testing.T) {
	instant := time.Date(2000, 1, 2, 3, 4, 0, 0, time.UTC)
	row := subscriptionAccessRow{
		startsAt:  sql.NullTime{Time: instant.In(time.FixedZone("UTC+6", 6*60*60)), Valid: true},
		expiresAt: sql.NullTime{Time: instant, Valid: true},
	}
	for _, test := range []struct {
		name   string
		now    time.Time
		code   int
		reason string
	}{
		{"just before", instant.Add(-time.Nanosecond), http.StatusForbidden, "subscription is not active yet"},
		{"equal start and expiry", instant, http.StatusOK, ""},
		{"just after", instant.Add(time.Nanosecond), http.StatusGone, "subscription expired"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, reason := row.accessDecision(test.now)
			if code != test.code || reason != test.reason {
				t.Fatalf("decision = %d, %q, want %d, %q", code, reason, test.code, test.reason)
			}
		})
	}
}

func TestRepositoryDatabaseErrors(t *testing.T) {
	app := newIntegrationApp(t)
	if err := app.db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.listUsers(); err == nil {
		t.Fatal("listUsers hid database error")
	}
	if _, err := app.getSubscriptionSettings(); err == nil {
		t.Fatal("getSubscriptionSettings hid database error")
	}
	if allowed, _, code, reason, err := app.subscriptionAccessAllowed("id"); err == nil || allowed || code != 0 || reason != "" {
		t.Fatalf("access DB error = %v, %d, %q, %v", allowed, code, reason, err)
	}
	if id, code, reason, err := app.redeemActivationCode("code"); err == nil || id != "" || code != 0 || reason != "" {
		t.Fatalf("activation DB error = %q, %d, %q, %v", id, code, reason, err)
	}
	if allowed, err := app.registerHWID(1, "hwid", deviceMeta{}); err == nil || allowed {
		t.Fatalf("register DB error = %v, %v", allowed, err)
	}
}
