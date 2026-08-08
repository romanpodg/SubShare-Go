package keymanagement

import (
	"context"
	"errors"
	"testing"
)

func TestService_LegacyValidation(t *testing.T) {
	fakeRepo := newFakeRepo()
	svc := newFakeService(fakeRepo)
	ctx := context.Background()

	// 1. Create with invalid kind
	_, _, err := svc.CreateLegacy(ctx, CreateLegacyParams{
		Label:  "Test",
		Kind:   "invalid_kind",
		Status: "active",
		URL:    "vless://00000000-0000-0000-0000-000000000000@example.com:443#t",
	})
	if !errors.Is(err, ErrInvalidKeyKind) {
		t.Fatalf("expected ErrInvalidKeyKind, got %v", err)
	}

	// 2. Create real key without URL
	_, _, err = svc.CreateLegacy(ctx, CreateLegacyParams{
		Label:  "Test",
		Kind:   "real",
		Status: "active",
		URL:    "",
	})
	if !errors.Is(err, ErrURLRequired) {
		t.Fatalf("expected ErrURLRequired, got %v", err)
	}

	// 3. Create real key without Label
	_, _, err = svc.CreateLegacy(ctx, CreateLegacyParams{
		Label:  "",
		Kind:   "real",
		Status: "active",
		URL:    "vless://00000000-0000-0000-0000-000000000000@example.com:443#t",
	})
	if !errors.Is(err, ErrLabelRequired) {
		t.Fatalf("expected ErrLabelRequired, got %v", err)
	}

	// 4. Update non-existent key
	_, err = svc.UpdateLegacy(ctx, 999, UpdateLegacyParams{
		Label:  "Update NonExistent",
		Kind:   "real",
		Status: "active",
		RawURL: "vless://00000000-0000-0000-0000-000000000000@example.com:443#t",
	})
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}

	// 5. Delete non-existent key
	err = svc.DeleteLegacy(ctx, 999)
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestInvalidProfileURIErrorCarriesTypedPublicDetail(t *testing.T) {
	err := invalidProfileURIError(errors.New("invalid endpoint"))
	if !errors.Is(err, ErrInvalidProfileURI) {
		t.Fatalf("error %v does not wrap ErrInvalidProfileURI", err)
	}
	if message := InvalidProfileURIMessage(err); message != "invalid endpoint" {
		t.Fatalf("public message = %q, want %q", message, "invalid endpoint")
	}
}
