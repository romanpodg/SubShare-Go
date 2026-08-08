package httpapi

import (
	"errors"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

type KeyCategoryHandler struct {
	service *keymanagement.Service
	audit   AuditRecorder
}

func NewKeyCategoryHandler(service *keymanagement.Service, audit AuditRecorder) *KeyCategoryHandler {
	return &KeyCategoryHandler{
		service: service,
		audit:   audit,
	}
}

func (h *KeyCategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list key categories")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
	})
}

func (h *KeyCategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req model.CreateKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cat, err := h.service.CreateCategory(r.Context(), keymanagement.CreateCategoryParams{
		Name:  req.Name,
		Color: req.Color,
	})
	if err != nil {
		if errors.Is(err, keymanagement.ErrCategoryNameRequired) {
			writeError(w, http.StatusBadRequest, "category name is required")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create key category")
		return
	}
	cat.ID = 0

	writeJSON(w, http.StatusOK, map[string]any{
		"category": cat,
		"message":  "key category saved",
	})
}

func (h *KeyCategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cat, err := h.service.UpdateCategory(r.Context(), keymanagement.UpdateCategoryParams{
		OldName: req.OldName,
		NewName: req.NewName,
		Color:   req.Color,
	})
	if err != nil {
		if errors.Is(err, keymanagement.ErrCategoryOldAndNewRequired) {
			writeError(w, http.StatusBadRequest, "both old_name and new_name are required")
			return
		}
		if errors.Is(err, keymanagement.ErrCategoryNameEmpty) {
			writeError(w, http.StatusBadRequest, "category name cannot be empty")
			return
		}
		if errors.Is(err, keymanagement.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "key category not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}
	cat.ID = 0

	writeJSON(w, http.StatusOK, map[string]any{
		"category": cat,
		"message":  "key category updated",
	})
}

func (h *KeyCategoryHandler) RenameCategory(w http.ResponseWriter, r *http.Request) {
	var req model.RenameKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cat, err := h.service.RenameCategory(r.Context(), keymanagement.RenameCategoryParams{
		OldName: req.OldName,
		NewName: req.NewName,
	})
	if err != nil {
		if errors.Is(err, keymanagement.ErrCategoryOldAndNewRequired) {
			writeError(w, http.StatusBadRequest, "both old_name and new_name are required")
			return
		}
		if errors.Is(err, keymanagement.ErrCategoryNameEmpty) {
			writeError(w, http.StatusBadRequest, "category name cannot be empty")
			return
		}
		if errors.Is(err, keymanagement.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "key category not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}
	cat.ID = 0

	writeJSON(w, http.StatusOK, map[string]any{
		"category": cat,
		"message":  "key category updated",
	})
}

func (h *KeyCategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	var req model.DeleteKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.DeleteCategory(r.Context(), keymanagement.DeleteCategoryParams{
		Name: req.Name,
		Mode: req.Mode,
	})
	if err != nil {
		if errors.Is(err, keymanagement.ErrInvalidDeleteMode) {
			writeError(w, http.StatusBadRequest, "invalid delete mode")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete key category")
		return
	}

	writeMessage(w, "key category deleted")
}

func (h *KeyCategoryHandler) ReorderCategories(w http.ResponseWriter, r *http.Request) {
	var req model.ReorderKeyCategoriesRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.ReorderCategories(r.Context(), keymanagement.ReorderCategoriesParams{
		Names: req.Names,
	})
	if err != nil {
		if errors.Is(err, keymanagement.ErrCategoryNamesRequired) {
			writeError(w, http.StatusBadRequest, "category names are required")
			return
		}
		if errors.Is(err, keymanagement.ErrCategoryNameEmpty) {
			writeError(w, http.StatusBadRequest, "category name cannot be empty")
			return
		}
		if errors.Is(err, keymanagement.ErrDuplicateCategoryNames) {
			writeError(w, http.StatusBadRequest, "duplicate category names")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to reorder key categories")
		return
	}

	writeMessage(w, "key categories reordered")
}

func (h *KeyCategoryHandler) ReorderKeys(w http.ResponseWriter, r *http.Request) {
	var req model.ReorderKeysRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.ReorderKeys(r.Context(), req.IDs)
	if err != nil {
		if errors.Is(err, keymanagement.ErrInvalidKeyOrderCount) {
			writeError(w, http.StatusBadRequest, "ids list must include all keys")
			return
		}
		if errors.Is(err, keymanagement.ErrUnknownKeyInOrder) {
			writeError(w, http.StatusBadRequest, "ids list contains unknown key")
			return
		}
		if errors.Is(err, keymanagement.ErrDuplicateKeyInOrder) {
			writeError(w, http.StatusBadRequest, "ids list contains duplicates")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to reorder keys")
		return
	}

	if h.audit != nil {
		h.audit(r, "keys.reorder", "key", "multiple", map[string]any{"count": len(req.IDs)})
	}
	writeMessage(w, "keys reordered")
}
