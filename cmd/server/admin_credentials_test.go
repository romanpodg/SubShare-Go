package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
	"golang.org/x/crypto/bcrypt"
)

type bootstrapHasherSpy struct {
	hashCalls int
}

func (hasher *bootstrapHasherSpy) Hash(string) (string, error) {
	hasher.hashCalls++
	return "synthetic-hash", nil
}

func (*bootstrapHasherSpy) RehashVerified(string) (string, error) {
	return "", errors.New("unexpected verified rehash")
}

func (*bootstrapHasherSpy) Verify(string, string) (adminpassword.Verification, error) {
	return adminpassword.Verification{}, errors.New("unexpected verification")
}

func TestBootstrapPasswordIsNotRequiredOrHashedWhenAdministratorExists(t *testing.T) {
	app := newIntegrationApp(t)
	if _, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'owner')`,
		"existing-owner",
		"synthetic-existing-hash",
	); err != nil {
		t.Fatalf("insert existing owner: %v", err)
	}
	spy := &bootstrapHasherSpy{}

	created, err := ensureBootstrapOwner(context.Background(), app.db, "unused-owner", "", spy)
	if err != nil || created {
		t.Fatalf("ensureBootstrapOwner = %v, %v", created, err)
	}
	if spy.hashCalls != 0 {
		t.Fatalf("bootstrap password was hashed %d times", spy.hashCalls)
	}
}

func TestBootstrapCreationRequiresSharedPasswordPolicyWithoutEchoingSecret(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     string
	}{
		{"missing", "", "ADMIN_PASSWORD is required when creating the initial owner"},
		{"too short", "private-short", "ADMIN_PASSWORD: password must contain at least 15 Unicode characters"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			created, err := ensureBootstrapOwner(
				context.Background(), app.db, "bootstrap-owner", test.password, adminpassword.NewDefault(),
			)
			if err == nil || created || err.Error() != test.want {
				t.Fatalf("ensureBootstrapOwner = %v, %v, want %q", created, err, test.want)
			}
			if test.password != "" && strings.Contains(err.Error(), test.password) {
				t.Fatalf("bootstrap error contains password: %v", err)
			}
			var count int
			if queryErr := app.db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&count); queryErr != nil || count != 0 {
				t.Fatalf("administrator count = %d, %v", count, queryErr)
			}
		})
	}
}

func TestBootstrapCreationStoresArgon2idAndPreservesSpaces(t *testing.T) {
	app := newIntegrationApp(t)
	value := "  bootstrap пароль  "
	created, err := ensureBootstrapOwner(
		context.Background(), app.db, "bootstrap-owner", value, adminpassword.NewDefault(),
	)
	if err != nil || !created {
		t.Fatalf("ensureBootstrapOwner = %v, %v", created, err)
	}
	var encoded string
	if err := app.db.QueryRow(`SELECT password_hash FROM admins WHERE username = ?`, "bootstrap-owner").Scan(&encoded); err != nil {
		t.Fatalf("load password hash: %v", err)
	}
	if adminpassword.Identify(encoded) != adminpassword.AlgorithmArgon2id {
		t.Fatalf("bootstrap hash algorithm = %s", adminpassword.Identify(encoded))
	}
	verified, err := adminpassword.NewDefault().Verify(value, encoded)
	if err != nil || !verified.Valid {
		t.Fatalf("exact bootstrap password did not verify: %#v, %v", verified, err)
	}
	trimmed, err := adminpassword.NewDefault().Verify(strings.TrimSpace(value), encoded)
	if err != nil || trimmed.Valid {
		t.Fatalf("trimmed bootstrap password unexpectedly verified: %#v, %v", trimmed, err)
	}
}

func TestConcurrentBootstrapCreatesOnlyOneOwner(t *testing.T) {
	app := newIntegrationApp(t)
	app.db.SetMaxOpenConns(2)
	type result struct {
		created bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for index := range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			created, err := ensureBootstrapOwner(
				context.Background(), app.db, "bootstrap-owner-"+strconv.Itoa(index),
				"bootstrap race password", adminpassword.NewDefault(),
			)
			results <- result{created: created, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	createdCount := 0
	for outcome := range results {
		if outcome.err != nil {
			t.Fatalf("concurrent bootstrap: %v", outcome.err)
		}
		if outcome.created {
			createdCount++
		}
	}
	var administratorCount int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&administratorCount); err != nil {
		t.Fatalf("count administrators: %v", err)
	}
	if createdCount != 1 || administratorCount != 1 {
		t.Fatalf("created results = %d, administrator rows = %d", createdCount, administratorCount)
	}
}

func TestBcryptLoginMigratesToArgon2id(t *testing.T) {
	app := newIntegrationApp(t)
	legacyPassword := "legacy-short"
	legacyHash, err := bcrypt.GenerateFromPassword([]byte(legacyPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate bcrypt: %v", err)
	}
	result, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'owner')`,
		"legacy-owner",
		string(legacyHash),
	)
	if err != nil {
		t.Fatalf("insert legacy owner: %v", err)
	}
	administratorID, _ := result.LastInsertId()

	gotID, authenticated, err := app.authenticateAdministrator(context.Background(), "legacy-owner", legacyPassword)
	if err != nil || !authenticated || gotID != administratorID {
		t.Fatalf("authenticateAdministrator = %d, %v, %v", gotID, authenticated, err)
	}
	var replacement string
	if err := app.db.QueryRow(`SELECT password_hash FROM admins WHERE id = ?`, administratorID).Scan(&replacement); err != nil {
		t.Fatalf("load replacement: %v", err)
	}
	if adminpassword.Identify(replacement) != adminpassword.AlgorithmArgon2id {
		t.Fatalf("replacement algorithm = %s", adminpassword.Identify(replacement))
	}
	verified, err := app.passwordHasher().Verify(legacyPassword, replacement)
	if err != nil || !verified.Valid || verified.NeedsRehash {
		t.Fatalf("replacement Verify = %#v, %v", verified, err)
	}
}

func TestFailedAuthenticationDoesNotMigrateBcrypt(t *testing.T) {
	app := newIntegrationApp(t)
	legacyHash, err := bcrypt.GenerateFromPassword([]byte("legacy-correct"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate bcrypt: %v", err)
	}
	if _, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'owner')`,
		"legacy-owner",
		string(legacyHash),
	); err != nil {
		t.Fatalf("insert legacy owner: %v", err)
	}

	_, authenticated, err := app.authenticateAdministrator(context.Background(), "legacy-owner", "legacy-wrong")
	if err != nil || authenticated {
		t.Fatalf("failed authentication = %v, %v", authenticated, err)
	}
	var stored string
	if err := app.db.QueryRow(`SELECT password_hash FROM admins WHERE username = ?`, "legacy-owner").Scan(&stored); err != nil {
		t.Fatalf("load stored hash: %v", err)
	}
	if stored != string(legacyHash) {
		t.Fatalf("bcrypt hash changed after failed authentication")
	}
}

func TestMigrationWriteFailureDoesNotFailLoginOrLogSecrets(t *testing.T) {
	app := newIntegrationApp(t)
	legacyPassword := "legacy write failure"
	legacyHash, err := bcrypt.GenerateFromPassword([]byte(legacyPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate bcrypt: %v", err)
	}
	if _, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'owner')`,
		"write-failure-owner",
		string(legacyHash),
	); err != nil {
		t.Fatalf("insert legacy owner: %v", err)
	}
	if _, err := app.db.Exec(`
		CREATE TRIGGER reject_password_migration
		BEFORE UPDATE OF password_hash ON admins
		BEGIN
			SELECT RAISE(FAIL, 'synthetic migration write failure');
		END
	`); err != nil {
		t.Fatalf("create migration failure trigger: %v", err)
	}

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	_, authenticated, err := app.authenticateAdministrator(context.Background(), "write-failure-owner", legacyPassword)
	if err != nil || !authenticated {
		t.Fatalf("authentication after migration write failure = %v, %v", authenticated, err)
	}
	logged := logs.String()
	if !strings.Contains(logged, "administrator password hash migration write failed") {
		t.Fatalf("migration warning was not logged: %q", logged)
	}
	if strings.Contains(logged, legacyPassword) || strings.Contains(logged, string(legacyHash)) {
		t.Fatalf("migration warning contains a password or hash: %q", logged)
	}
}

func TestConcurrentBcryptAuthenticationMigratesSafely(t *testing.T) {
	app := newIntegrationApp(t)
	legacyPassword := "legacy-concurrent"
	legacyHash, err := bcrypt.GenerateFromPassword([]byte(legacyPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate bcrypt: %v", err)
	}
	if _, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'owner')`,
		"concurrent-owner",
		string(legacyHash),
	); err != nil {
		t.Fatalf("insert legacy owner: %v", err)
	}

	const attempts = 2
	start := make(chan struct{})
	results := make(chan error, attempts)
	var workers sync.WaitGroup
	for range attempts {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, authenticated, authErr := app.authenticateAdministrator(context.Background(), "concurrent-owner", legacyPassword)
			if authErr != nil {
				results <- authErr
				return
			}
			if !authenticated {
				results <- errors.New("valid concurrent login was rejected")
			}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for result := range results {
		if result != nil {
			t.Fatal(result)
		}
	}

	var stored string
	if err := app.db.QueryRow(`SELECT password_hash FROM admins WHERE username = ?`, "concurrent-owner").Scan(&stored); err != nil {
		t.Fatalf("load migrated hash: %v", err)
	}
	if adminpassword.Identify(stored) != adminpassword.AlgorithmArgon2id {
		t.Fatalf("concurrent replacement algorithm = %s", adminpassword.Identify(stored))
	}
	verified, err := app.passwordHasher().Verify(legacyPassword, stored)
	if err != nil || !verified.Valid {
		t.Fatalf("concurrent replacement Verify = %#v, %v", verified, err)
	}
}

func TestLoginDoesNotDistinguishUnknownUserFromWrongPassword(t *testing.T) {
	app := newIntegrationApp(t)
	encoded, err := app.passwordHasher().Hash("synthetic password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'owner')`,
		"known-owner",
		encoded,
	); err != nil {
		t.Fatalf("insert owner: %v", err)
	}

	login := func(username, password string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"username": username, "password": password})
		recorder := httptest.NewRecorder()
		app.apiLogin(recorder, httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body)))
		return recorder
	}
	wrong := login("known-owner", "synthetic wrong password")
	unknown := login("unknown-owner", "synthetic wrong password")
	if wrong.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized || wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("credential failure responses differ: wrong=%d %q unknown=%d %q", wrong.Code, wrong.Body.String(), unknown.Code, unknown.Body.String())
	}
}

func TestAdministratorCreateAndChangeRoutesSharePasswordPolicy(t *testing.T) {
	app := newIntegrationApp(t)
	shortPassword := strings.Repeat("я", adminpassword.MinCodePoints-1)
	wantMessage := adminpassword.ErrTooShort.Error()

	callCreate := func(v1 bool, username string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]string{
			"username": username,
			"password": shortPassword,
			"role":     "operator",
		})
		request := httptest.NewRequest(http.MethodPost, "/api/admin/admins", bytes.NewReader(body))
		recorder := httptest.NewRecorder()
		handler := http.Handler(http.HandlerFunc(app.apiCreateAdmin))
		if v1 {
			request = httptest.NewRequest(http.MethodPost, "/api/v1/admins", bytes.NewReader(body))
			handler = httpapi.V1Envelope(handler)
		}
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	legacyCreate := callCreate(false, "legacy-create")
	v1Create := callCreate(true, "v1-create")
	assertPasswordPolicyResponse(t, legacyCreate, false, wantMessage)
	assertPasswordPolicyResponse(t, v1Create, true, wantMessage)

	exactPassword := "             a "
	validBody, _ := json.Marshal(map[string]string{
		"username": "exact-password-admin",
		"password": exactPassword,
		"role":     "operator",
	})
	validRequest := httptest.NewRequest(http.MethodPost, "/api/admin/admins", bytes.NewReader(validBody))
	validRecorder := httptest.NewRecorder()
	app.apiCreateAdmin(validRecorder, validRequest)
	if validRecorder.Code != http.StatusOK {
		t.Fatalf("exact password create status = %d; body=%q", validRecorder.Code, validRecorder.Body.String())
	}
	var exactHash string
	if err := app.db.QueryRow(
		`SELECT password_hash FROM admins WHERE username = ?`, "exact-password-admin",
	).Scan(&exactHash); err != nil {
		t.Fatalf("load exact API password hash: %v", err)
	}
	exactVerification, err := app.passwordHasher().Verify(exactPassword, exactHash)
	if err != nil || !exactVerification.Valid {
		t.Fatalf("exact API password Verify = %#v, %v", exactVerification, err)
	}
	trimmedVerification, err := app.passwordHasher().Verify(strings.TrimSpace(exactPassword), exactHash)
	if err != nil || trimmedVerification.Valid {
		t.Fatalf("trimmed API password unexpectedly verified = %#v, %v", trimmedVerification, err)
	}

	encoded, err := app.passwordHasher().Hash("existing password")
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	result, err := app.db.Exec(
		`INSERT INTO admins(username, password_hash, role) VALUES(?, ?, 'operator')`,
		"changed-admin",
		encoded,
	)
	if err != nil {
		t.Fatalf("insert changed admin: %v", err)
	}
	administratorID, _ := result.LastInsertId()

	callUpdate := func(v1 bool) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"password": shortPassword})
		path := "/api/admin/admins/1"
		if v1 {
			path = "/api/v1/admins/1"
		}
		request := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
		request.SetPathValue("id", strconv.FormatInt(administratorID, 10))
		recorder := httptest.NewRecorder()
		handler := http.Handler(http.HandlerFunc(app.apiUpdateAdmin))
		if v1 {
			handler = httpapi.V1Envelope(handler)
		}
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	legacyUpdate := callUpdate(false)
	v1Update := callUpdate(true)
	assertPasswordPolicyResponse(t, legacyUpdate, false, wantMessage)
	assertPasswordPolicyResponse(t, v1Update, true, wantMessage)

	var unchanged string
	if err := app.db.QueryRow(`SELECT password_hash FROM admins WHERE id = ?`, administratorID).Scan(&unchanged); err != nil {
		t.Fatalf("load unchanged hash: %v", err)
	}
	if unchanged != encoded {
		t.Fatalf("invalid API password was silently changed")
	}
}

func assertPasswordPolicyResponse(t *testing.T, recorder *httptest.ResponseRecorder, v1 bool, wantMessage string) {
	t.Helper()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%q", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	message := payload["error"]
	if v1 {
		message = payload["message"]
	}
	if message != wantMessage {
		t.Fatalf("message = %#v, want %q", message, wantMessage)
	}
	if v1 && payload["code"] != "compatibility_error" {
		t.Fatalf("v1 code = %#v", payload["code"])
	}
}
