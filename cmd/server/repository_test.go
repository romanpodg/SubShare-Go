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

func TestRepositoryListUsersEmptyThenPopulated(t *testing.T) {
	app := newIntegrationApp(t)
	users, err := app.listUsers()
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "empty user list", users, []model.User(nil))

	seedSubscriptionUser(t, app, model.UserStatusActive)
	users, err = app.listUsers()
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "users added after empty read", len(users), 1)
}

func TestRepositoryListUsersProjection(t *testing.T) {
	fixture := newRepositoryUserFixture(t)
	empty, user := fixture.listUsers(t)

	requireRepositoryEqual(t, "descending user order", empty.ID > fixture.userID, true)
	requireRepositoryEqual(t, "populated user ID", user.ID, fixture.userID)
	requireRepositoryEqual(t, "default time zone", empty.TimeZone, "Europe/Moscow")
	requireRepositoryEqual(t, "default language", empty.Language, "ru")
	requireRepositoryEqual(t, "default refresh hours", empty.SubscriptionRefreshHours, 12)
	requireRepositoryEqual(t, "negative device limit", empty.MaxDevices, 0)
	requireRepositoryEqual(t, "time zone trimming", user.TimeZone, "Asia/Omsk")
	requireRepositoryEqual(t, "language trimming", user.Language, "en")
	requireRepositoryEqual(t, "activation code trimming", user.ActivationCode, "activation")
	requireRepositoryEqual(t, "subscription ID trimming", user.SubscriptionID, "subscription")
	requireRepositoryEqual(t, "subscription name trimming", user.SubscriptionName, "Personal")
	requireRepositoryEqual(t, "negative refresh hours", user.SubscriptionRefreshHours, 12)
	requireRepositoryEqual(t, "info URL trimming", user.SubscriptionInfoURL, "info")
	requireRepositoryEqual(t, "extra URL trimming", user.SubscriptionExtraURL, "extra")
	requireRepositoryEqual(t, "extra status trimming", user.SubscriptionExtraStatus, "notice")
	requireRepositoryEqual(t, "blocked reason trimming", user.BlockedReason, "reason")
	requireRepositoryEqual(t, "key assignment mode", user.KeyAssignmentMode, "selected")
	requireRepositoryEqual(t, "activation display date", user.ActivationUsedAt, "02/01/2000 09:04")
	requireRepositoryEqual(t, "start display date", user.StartsAtInput, "02/01/2000 09:04")
	requireRepositoryEqual(t, "expiry display date", user.ExpiresAtInput, "02/01/2000 10:04")
	requireRepositoryEqual(t, "expired status", user.EffectiveStatus, "expired")
	requireRepositoryEqual(t, "assigned key ID", user.AssignedKeyIDs, strconv.FormatInt(fixture.keyID, 10))
	requireRepositoryEqual(t, "unassigned keys", empty.AssignedKeyIDs, "")
}

func TestRepositoryListUsersDevices(t *testing.T) {
	fixture := newRepositoryUserFixture(t)
	empty, user := fixture.listUsers(t)

	requireRepositoryEqual(t, "empty device array", empty.ConnectedDevices, []model.ConnectedDevice{})
	requireRepositoryEqual(t, "empty HWID array", empty.ConnectedHWIDs, []string{})
	requireRepositoryEqual(t, "device aggregate count", user.ConnectedDeviceCount, 2)
	requireRepositoryEqual(t, "HWID aggregate count", len(user.ConnectedHWIDs), 2)
	requireRepositoryEqual(t, "projected device count", len(user.ConnectedDevices), 2)
	newer, older := user.ConnectedDevices[0], user.ConnectedDevices[1]
	requireRepositoryEqual(t, "newest device first", newer.HWID, "new")
	requireRepositoryEqual(t, "older HWID trimming", older.HWID, "OLD")
	requireRepositoryEqual(t, "legacy HWID normalization", older.NormalizedHWID, "old")
	requireRepositoryEqual(t, "stored app name", older.AppName, "Stored")
	requireRepositoryEqual(t, "inferred app name", newer.AppName, "Happ")
	requireRepositoryEqual(t, "inferred platform", newer.Platform, "Android")
	requireRepositoryEqual(t, "inferred OS version", newer.OSVersion, "14")
	requireRepositoryEqual(t, "local last-seen date", older.LastSeenAt, fixture.stamp.Local().Format("2006-01-02 15:04:05"))
}

func TestRepositoryListUsersInvalidTimeZone(t *testing.T) {
	fixture := newRepositoryUserFixture(t)
	_, user := fixture.listUsers(t)
	requireRepositoryEqual(t, "valid time zone before update", user.ActivationUsedAt, "02/01/2000 09:04")
	execRepositoryFixtureSQL(t, fixture.app, `UPDATE users SET time_zone = 'invalid/zone' WHERE id = ?`, fixture.userID)
	_, user = fixture.listUsers(t)
	requireRepositoryEqual(t, "invalid time zone uses UTC", user.ActivationUsedAt, "02/01/2000 03:04")
}

type repositoryUserFixture struct {
	app           *App
	userID, keyID int64
	stamp         time.Time
}

func newRepositoryUserFixture(t *testing.T) repositoryUserFixture {
	t.Helper()
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	stamp := time.Date(2000, 1, 2, 3, 4, 0, 0, time.UTC)
	execRepositoryFixtureSQL(t, app, `UPDATE users SET time_zone = ' Asia/Omsk ', language = ' en ',
		activation_code = ' activation ', subscription_id = ' subscription ',
		subscription_name = ' Personal ', subscription_refresh_hours = -1,
		subscription_info_url = ' info ', subscription_extra_url = ' extra ',
		subscription_extra_status = ' notice ', activation_used_at = ?, starts_at = ?,
		expires_at = ?, blocked_reason = ' reason ', key_assignment_mode = 'selected' WHERE id = ?`,
		stamp, stamp, stamp.Add(time.Hour), userID)
	execRepositoryFixtureSQL(t, app, `INSERT INTO user_devices(user_id, hwid, normalized_hwid, app_name, user_agent, last_seen_at)
		VALUES(?, ' OLD ', NULL, ' Stored ', 'Happ/3.0 (Android 14)', ?),
		      (?, 'new', 'new', NULL, 'Happ/3.0 (Android 14)', ?)`, userID, stamp, userID, stamp.Add(time.Hour))
	execRepositoryFixtureSQL(t, app, `INSERT INTO users(name, email, token, time_zone, language, max_devices, subscription_refresh_hours)
		VALUES('Empty', '', 'empty-token', ' ', ' ', -1, 0)`)
	keyID := insertAssignedDeliveryKey(t, app, userID, nil, "Assigned", externalTestVLESS, "vless", "full", 1)
	return repositoryUserFixture{app: app, userID: userID, keyID: keyID, stamp: stamp}
}

func (fixture repositoryUserFixture) listUsers(t *testing.T) (model.User, model.User) {
	t.Helper()
	users, err := fixture.app.listUsers()
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "fixture user count", len(users), 2)
	return users[0], users[1]
}

func TestRepositorySubscriptionSettings(t *testing.T) {
	app := newIntegrationApp(t)
	execRepositoryFixtureSQL(t, app, `UPDATE subscription_settings SET title = NULL, refresh_hours = -1,
		subscription_format = 'invalid', time_zone = ' ', language = ' ' WHERE id = 1`)
	settings, err := app.getSubscriptionSettings()
	requireRepositorySuccess(t, err)
	defaults := model.SubscriptionSettings{Title: "AllKeys", RefreshHours: 12, SubscriptionFormat: model.SubscriptionFormatLinks, TimeZone: "Europe/Moscow", Language: "ru"}
	requireRepositoryEqual(t, "default subscription settings", settings, defaults)
	nullSettings, err := scanSubscriptionSettings(app.db.QueryRow(`SELECT NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL`))
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "null subscription settings", nullSettings, defaults)
	execRepositoryFixtureSQL(t, app, `UPDATE subscription_settings SET title = ' Title ', refresh_hours = 24,
		info_url = ' info ', extra_url = ' extra ', extra_status = ' notice ', subscription_format = ' XRAY-JSON ',
		show_subscription_expiration = -1, time_zone = ' UTC ', language = ' en ', provider_id = ' provider ',
		happ_no_limit_mode = 1, happ_no_limit_mode_xhttp_only = 2, happ_mandatory_hwid = -1,
		happ_notify_expiration = 1, happ_hide_server_settings = 1, happ_subscription_body = ' body ' WHERE id = 1`)
	want := model.SubscriptionSettings{
		Title: "Title", RefreshHours: 24, InfoURL: "info", ExtraURL: "extra", ExtraStatus: "notice",
		SubscriptionFormat: model.SubscriptionFormatXrayJSON, ShowSubscriptionExpiration: true,
		TimeZone: "UTC", Language: "en", ProviderID: "provider", HappNoLimitMode: true,
		HappNoLimitModeXHTTPOnly: true, HappMandatoryHWID: true, HappNotifyExpiration: true,
		HappHideServerSettings: true, HappSubscriptionBody: " body ",
	}
	settings, err = app.getSubscriptionSettings()
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "normalized subscription settings", settings, want)
	execRepositoryFixtureSQL(t, app, `DELETE FROM subscription_settings`)
	_, err = app.getSubscriptionSettings()
	requireRepositoryEqual(t, "missing settings error", errors.Is(err, sql.ErrNoRows), true)
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
			execRepositoryFixtureSQL(t, app, `UPDATE users SET status = ?, blocked_reason = ?, starts_at = ?, expires_at = ? WHERE id = ?`, test.status, test.reason, test.starts, test.expires, userID)
			allowed, id, code, reason, err := app.subscriptionAccessAllowed(test.subscription)
			requireRepositorySuccess(t, err)
			wantID := userID
			if test.code == http.StatusNotFound {
				wantID = 0
			}
			requireRepositoryEqual(t, "access allowed", allowed, test.code == http.StatusOK)
			requireRepositoryEqual(t, "access user ID", id, wantID)
			requireRepositoryEqual(t, "access status code", code, test.code)
			requireRepositoryEqual(t, "access reason", reason, test.wantReason)
		})
	}
}

func TestRepositoryRegisterHWID(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	for _, limit := range []int{0, -1, 2} {
		t.Run("limit="+strconv.Itoa(limit), func(t *testing.T) {
			execRepositoryFixtureSQL(t, app, `DELETE FROM user_devices; UPDATE users SET max_devices = ? WHERE id = ?`, limit, userID)
			for _, hwid := range []string{" ONE ", "two"} {
				allowed, err := app.registerHWID(userID, hwid, deviceMeta{DeviceName: "Phone", AppVersion: "1"})
				requireRepositorySuccess(t, err)
				requireRepositoryEqual(t, "initial device registration", allowed, true)
			}
			allowed, err := app.registerHWID(userID, "one", deviceMeta{AppVersion: "2"})
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "existing device at limit", allowed, true)
			var count int
			var name, version, raw string
			err = app.db.QueryRow(`SELECT hwid, device_name, app_version FROM user_devices WHERE user_id = ? AND normalized_hwid = 'one'`, userID).Scan(&raw, &name, &version)
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "stored HWID", raw, "ONE")
			requireRepositoryEqual(t, "preserved device name", name, "Phone")
			requireRepositoryEqual(t, "updated device version", version, "2")
			allowed, err = app.registerHWID(userID, "three", deviceMeta{})
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "new device respects limit", allowed, limit <= 0)
			err = app.db.QueryRow(`SELECT COUNT(*) FROM user_devices WHERE user_id = ?`, userID).Scan(&count)
			requireRepositorySuccess(t, err)
			wantCount := 2
			if limit <= 0 {
				wantCount = 3
			}
			requireRepositoryEqual(t, "persisted device count", count, wantCount)
		})
	}
	allowed, err := app.registerHWID(userID, " ", deviceMeta{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "empty HWID allowed", allowed, true)
}

func TestRepositoryRegisterHWIDLegacyCountAndRollback(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	execRepositoryFixtureSQL(t, app, `UPDATE users SET max_devices = 2 WHERE id = ?;
		INSERT INTO user_devices(user_id, hwid, normalized_hwid) VALUES(?, ' OLD ', NULL), (?, 'old', '')`, userID, userID, userID)
	allowed, err := app.registerHWID(userID, "new", deviceMeta{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "distinct legacy HWIDs count once", allowed, true)
	allowed, err = app.registerHWID(userID, "third", deviceMeta{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "legacy device limit", allowed, false)
	execRepositoryFixtureSQL(t, app, `UPDATE users SET max_devices = 0 WHERE id = ?;
		CREATE TRIGGER reject_device BEFORE INSERT ON user_devices BEGIN SELECT RAISE(ABORT, 'reject device'); END`, userID)
	allowed, err = app.registerHWID(userID, "rejected", deviceMeta{})
	requireRepositoryEqual(t, "insert error returned", err != nil, true)
	requireRepositoryEqual(t, "failed insert rejected", allowed, false)
	execRepositoryFixtureSQL(t, app, `DROP TRIGGER reject_device`)
	allowed, err = app.registerHWID(userID, "accepted", deviceMeta{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "transaction released after failure", allowed, true)
	allowed, err = app.registerHWID(userID+1, "missing", deviceMeta{})
	requireRepositoryEqual(t, "missing user error", errors.Is(err, sql.ErrNoRows), true)
	requireRepositoryEqual(t, "missing user rejected", allowed, false)
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
			execRepositoryFixtureSQL(t, app, `UPDATE users SET subscription_id = ? WHERE id = ?`, test.stored, userID)
			id, err := app.redeemActivationCode("activation-token")
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "redeemed subscription populated", id != "", true)
			if test.stored == " existing " {
				requireRepositoryEqual(t, "existing subscription retained", id, "existing")
			}
			var persisted string
			var used sql.NullTime
			err = app.db.QueryRow(`SELECT subscription_id, activation_used_at FROM users WHERE id = ?`, userID).Scan(&persisted, &used)
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "subscription persisted", persisted, id)
			requireRepositoryEqual(t, "activation timestamp persisted", used.Valid, true)
			id, err = app.redeemActivationCode("activation-token")
			requireRepositoryEqual(t, "repeat redeem subscription", id, "")
			requireRepositoryEqual(t, "repeat redeem outcome", errors.Is(err, errActivationAlreadyUsed), true)
			id, err = app.redeemActivationCode("missing")
			requireRepositoryEqual(t, "missing redeem subscription", id, "")
			requireRepositoryEqual(t, "missing redeem outcome", errors.Is(err, errActivationNotFound), true)
		})
	}
}

func TestRepositoryRedeemActivationConditionalUpdate(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	// Model another redemption winning between the SELECT and conditional UPDATE.
	execRepositoryFixtureSQL(t, app, `CREATE TRIGGER activation_claimed BEFORE UPDATE OF activation_used_at ON users
		BEGIN SELECT RAISE(IGNORE); END`)
	id, err := app.redeemActivationCode("activation-token")
	requireRepositoryEqual(t, "lost claim subscription", id, "")
	requireRepositoryEqual(t, "lost claim outcome", errors.Is(err, errActivationAlreadyUsed), true)
	execRepositoryFixtureSQL(t, app, `DROP TRIGGER activation_claimed;
		CREATE TRIGGER activation_failed BEFORE UPDATE OF activation_used_at ON users
		BEGIN SELECT RAISE(ABORT, 'activation failure'); END`)
	id, err = app.redeemActivationCode("activation-token")
	requireRepositoryEqual(t, "activation update error returned", err != nil, true)
	requireRepositoryEqual(t, "failed update subscription", id, "")
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
			requireRepositoryEqual(t, "time-boundary status", code, test.code)
			requireRepositoryEqual(t, "time-boundary reason", reason, test.reason)
		})
	}
}

func TestRepositoryDatabaseErrors(t *testing.T) {
	app := newIntegrationApp(t)
	requireRepositorySuccess(t, app.db.Close())
	_, err := app.listUsers()
	requireRepositoryEqual(t, "list database error returned", err != nil, true)
	_, err = app.getSubscriptionSettings()
	requireRepositoryEqual(t, "settings database error returned", err != nil, true)
	allowed, _, code, reason, err := app.subscriptionAccessAllowed("id")
	requireRepositoryEqual(t, "access database error returned", err != nil, true)
	requireRepositoryEqual(t, "access denied on database error", allowed, false)
	requireRepositoryEqual(t, "access database error status", code, 0)
	requireRepositoryEqual(t, "access database error reason", reason, "")
	id, err := app.redeemActivationCode("code")
	requireRepositoryEqual(t, "activation database error returned", err != nil, true)
	requireRepositoryEqual(t, "activation database error subscription", id, "")
	allowed, err = app.registerHWID(1, "hwid", deviceMeta{})
	requireRepositoryEqual(t, "registration database error returned", err != nil, true)
	requireRepositoryEqual(t, "registration denied on database error", allowed, false)
}

func execRepositoryFixtureSQL(t *testing.T, app *App, query string, args ...any) {
	t.Helper()
	_, err := app.db.Exec(query, args...)
	requireRepositorySuccess(t, err)
}

func requireRepositorySuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func requireRepositoryEqual(t *testing.T, field string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		// Field-specific failures avoid dumping generated subscription tokens.
		t.Fatalf("%s mismatch", field)
	}
}
