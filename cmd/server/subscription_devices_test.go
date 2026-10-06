package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestSubscriptionDeviceRequestPrecedenceAndByteLimits(t *testing.T) {
	for _, test := range []struct {
		name, query, hwidHeader, deviceHeader, identity, reason string
	}{
		{"query", " Query ", "header", "device", "query", ""},
		{"HWID header", " ", " Header ", "device", "header", ""},
		{"device header", " ", " ", " Device ", "device", ""},
		{"128 bytes", strings.Repeat("x", 128), "", "", strings.Repeat("x", 128), ""},
		{"129 bytes does not fall back", strings.Repeat("x", 129), "valid", "valid", "", "invalid HWID"},
		{"unicode byte limit accepted", strings.Repeat("界", 42), "", "", strings.Repeat("界", 42), ""},
		{"unicode byte limit denied", strings.Repeat("界", 43), "", "", "", "invalid HWID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedSubscriptionUser(t, f.app, "active")
			request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid="+url.QueryEscape(test.query), nil)
			request.Header.Set("X-HWID", test.hwidHeader)
			request.Header.Set("X-Device-ID", test.deviceHeader)
			allowed, reason, err := f.app.subscriptionDeviceAllowed(request, id, model.SubscriptionSettings{})
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "device request outcome", allowed, test.reason == "")
			requireRepositoryEqual(t, "device request reason", reason, test.reason)
			if test.identity == "" {
				requireRepositoryEqual(t, "denied identity is not persisted", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices"), 0)
				return
			}
			var normalized string
			requireRepositorySuccess(t, f.app.db.QueryRow("SELECT normalized_hwid FROM user_devices WHERE user_id = ?", id).Scan(&normalized))
			requireRepositoryEqual(t, "selected identity persisted", normalized, test.identity)
		})
	}
}

func TestSubscriptionDeviceRequirementNeedsProviderAndFlag(t *testing.T) {
	for _, test := range []struct {
		provider  string
		mandatory bool
		allowed   bool
	}{
		{"", false, true}, {"", true, true}, {"provider", false, true}, {"provider", true, false},
	} {
		f := newUserMutationFixture(t)
		id := seedSubscriptionUser(t, f.app, "active")
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		allowed, reason, err := f.app.subscriptionDeviceAllowed(request, id, model.SubscriptionSettings{ProviderID: test.provider, HappMandatoryHWID: test.mandatory})
		requireRepositorySuccess(t, err)
		requireRepositoryEqual(t, "provider and flag requirement retained", allowed, test.allowed)
		wantReason := ""
		if !test.allowed {
			wantReason = "HWID is required for this subscription"
		}
		requireRepositoryEqual(t, "missing device reason", reason, wantReason)
		requireRepositoryEqual(t, "missing identity never consumes a slot", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices"), 0)
	}
}

func TestSubscriptionDeviceRegisteredRoutesRetainFailureResponse(t *testing.T) {
	for _, suffix := range []string{"", "/subbody", "/subbody/plain"} {
		t.Run(suffix, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			seedSubscriptionUser(t, f.app, "active")
			faults.deviceRowsAffectedErr = errors.New("private device diagnostic")
			response := f.request(http.MethodGet, "/sub/subscription-token"+suffix+"?hwid=device", "")
			requireRepositoryEqual(t, "device persistence failure is server error", response.Code, http.StatusInternalServerError)
			requireRepositoryEqual(t, "device failure body", response.Body.String(), "failed to load subscription\n")
			requireRepositoryEqual(t, "persistence error does not become capacity denial", response.Header().Get("Subscription-Status"), "")
			requireRepositoryEqual(t, "failed persistence adds no device", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices"), 0)
		})
	}
}

func TestSubscriptionDeviceRegisteredRoutesRetainLimitRemarks(t *testing.T) {
	for _, test := range []struct{ name, message, remarks, want string }{
		{"default", " ", `{}`, model.DefaultDeviceLimitMessage},
		{"custom", " custom limit ", `{}`, "custom limit"},
		{"remark override", "custom limit", `{"limited":["first line","second line"]}`, "first line\nsecond line"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedSubscriptionUser(t, f.app, "active")
			allowed, err := f.app.registerHWID(id, "existing", deviceMeta{})
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "first slot occupied", allowed, true)
			f.app.deviceLimitMessage = test.message
			execRepositoryFixtureSQL(t, f.app, "UPDATE subscription_delivery_settings SET remarks_json = ? WHERE id = 1", test.remarks)
			for _, suffix := range []string{"", "/subbody", "/subbody/plain"} {
				response := f.request(http.MethodGet, "/sub/subscription-token"+suffix+"?hwid=new", "")
				requireRepositoryEqual(t, "device limit HTTP status", response.Code, http.StatusForbidden)
				requireRepositoryEqual(t, "device limit status header", response.Header().Get("Subscription-Status"), "limited")
				requireRepositoryEqual(t, "device limit response body", response.Body.String(), test.want+"\n")
			}
			requireRepositoryEqual(t, "denied device leaves occupied slot intact", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), 1)
		})
	}
}
