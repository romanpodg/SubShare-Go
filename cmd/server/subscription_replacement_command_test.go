package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestSubscriptionReplacementCommandKeepsAuditInHTTP(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	next, err := f.app.replaceUserSubscription(id, model.UpdateSubscriptionRequest{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "returned normalized status", next.status, "active")
	requireRepositoryEqual(t, "legacy replacement persisted", readMutationSubscription(t, f.app, id), mutationSubscriptionState{status: "active", refresh: 12})
	_, err = f.app.replaceUserSubscription(999999, model.UpdateSubscriptionRequest{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "missing target remains absent", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE id = 999999"), 0)
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}

func TestSubscriptionReplacementCommandValidationRetainsState(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	before := readMutationSubscription(t, f.app, id)
	next, err := f.app.replaceUserSubscription(id, model.UpdateSubscriptionRequest{SubscriptionName: strings.Repeat("界", 41)})
	var invalid *subscriptionReplacementValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("validation error identity lost: %v", err)
	}
	if !errors.Is(err, invalid.cause) {
		t.Fatal("validation cause lost")
	}
	requireRepositoryEqual(t, "no provisional state returned", reflect.DeepEqual(next, subscriptionReplacement{}), true)
	requireRepositoryEqual(t, "validation retains persisted state", readMutationSubscription(t, f.app, id), before)
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}

func TestSubscriptionReplacementCommandSQLFailureRetainsState(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	before := readMutationSubscription(t, f.app, id)
	execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_replacement BEFORE UPDATE ON users
		BEGIN SELECT RAISE(ABORT, 'private replacement fixture diagnostic'); END`)
	next, err := f.app.replaceUserSubscription(id, model.UpdateSubscriptionRequest{Status: "paused"})
	if err == nil {
		t.Fatal("failed SQL reported success")
	}
	var invalid *subscriptionReplacementValidationError
	if errors.As(err, &invalid) {
		t.Fatal("persistence failure classified as input failure")
	}
	requireRepositoryEqual(t, "no provisional state returned", reflect.DeepEqual(next, subscriptionReplacement{}), true)
	requireRepositoryEqual(t, "SQL failure retains persisted state", readMutationSubscription(t, f.app, id), before)
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}
