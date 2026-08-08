package keymanagement

import (
	"context"
	"errors"
	"testing"
)

func TestService_CategoryOperations(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newFakeService(repo)

	// List
	cats, err := svc.ListCategories(ctx)
	if err != nil || len(cats) != 0 {
		t.Fatalf("ListCategories failed: err=%v, count=%d", err, len(cats))
	}

	// Create
	cat, err := svc.CreateCategory(ctx, CreateCategoryParams{Name: "  TestCat  ", Color: "#ff0000"})
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	if cat.Name != "TestCat" || cat.Color != "#FF0000" {
		t.Fatalf("unexpected created category: %#v", cat)
	}

	// Create empty name error
	_, err = svc.CreateCategory(ctx, CreateCategoryParams{Name: "   ", Color: ""})
	if !errors.Is(err, ErrCategoryNameRequired) {
		t.Fatalf("expected ErrCategoryNameRequired, got %v", err)
	}

	// Update
	updCat, err := svc.UpdateCategory(ctx, UpdateCategoryParams{OldName: "TestCat", NewName: "RenamedCat", Color: "#00ff00"})
	if err != nil {
		t.Fatalf("UpdateCategory failed: %v", err)
	}
	if updCat.Name != "RenamedCat" || updCat.Color != "#00FF00" {
		t.Fatalf("unexpected updated category: %#v", updCat)
	}

	// Update empty name error
	_, err = svc.UpdateCategory(ctx, UpdateCategoryParams{OldName: "", NewName: "X"})
	if !errors.Is(err, ErrCategoryOldAndNewRequired) {
		t.Fatalf("expected ErrCategoryOldAndNewRequired, got %v", err)
	}

	// Rename
	renCat, err := svc.RenameCategory(ctx, RenameCategoryParams{OldName: "RenamedCat", NewName: "RenamedCat2"})
	if err != nil {
		t.Fatalf("RenameCategory failed: %v", err)
	}
	if renCat.Name != "RenamedCat2" {
		t.Fatalf("unexpected renamed category: %#v", renCat)
	}

	// Delete
	err = svc.DeleteCategory(ctx, DeleteCategoryParams{Name: "RenamedCat2", Mode: "keep_keys"})
	if err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}

	// Delete invalid mode error
	err = svc.DeleteCategory(ctx, DeleteCategoryParams{Name: "RenamedCat2", Mode: "invalid"})
	if !errors.Is(err, ErrInvalidDeleteMode) {
		t.Fatalf("expected ErrInvalidDeleteMode, got %v", err)
	}

	// Reorder categories
	err = svc.ReorderCategories(ctx, ReorderCategoriesParams{Names: []string{"CatA", "CatB"}})
	if err != nil {
		t.Fatalf("ReorderCategories failed: %v", err)
	}

	// Reorder categories empty error
	err = svc.ReorderCategories(ctx, ReorderCategoriesParams{Names: []string{}})
	if !errors.Is(err, ErrCategoryNamesRequired) {
		t.Fatalf("expected ErrCategoryNamesRequired, got %v", err)
	}

	// Reorder categories duplicate error
	err = svc.ReorderCategories(ctx, ReorderCategoriesParams{Names: []string{"CatA", "CatA"}})
	if !errors.Is(err, ErrDuplicateCategoryNames) {
		t.Fatalf("expected ErrDuplicateCategoryNames, got %v", err)
	}

	// Reorder keys
	err = svc.ReorderKeys(ctx, []int64{2, 1})
	if err != nil {
		t.Fatalf("ReorderKeys failed: %v", err)
	}
}

func TestNormalizeKeyCategoryColor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "#d8b33d"},
		{"   ", "#d8b33d"},
		{"#ff0000", "#FF0000"},
		{"#00FF00", "#00FF00"},
		{"ff0000", "#d8b33d"},
		{"#FFF", "#d8b33d"},
		{"#GGGGGG", "#d8b33d"},
		{"#1234567890abcdef1234", "#d8b33d"},
		{"  #0000FF  ", "#0000FF"},
	}

	for _, tt := range tests {
		got := NormalizeKeyCategoryColor(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeKeyCategoryColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
