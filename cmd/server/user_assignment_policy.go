package main

import (
	"errors"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type userAssignmentInput struct {
	mode   string
	keyIDs []int64
}
type userAssignmentRequestError struct{ code, message string }

func (err *userAssignmentRequestError) Error() string { return err.message }

// Route validation deliberately precedes command validation: the existing all
// mode rejects any supplied IDs; selected nonpositive IDs keep their500 mapping.
func prepareUserAssignmentRequest(input model.UpdateKeyAssignmentRequest) (string, error) {
	mode, valid := model.KeyAssignmentMode(input.Mode).Normalize()
	if !valid {
		return "", &userAssignmentRequestError{code: "assignment_mode_invalid", message: "mode must be all or selected"}
	}
	if mode == model.KeyAssignmentModeAll && len(input.KeyIDs) > 0 {
		return "", &userAssignmentRequestError{code: "assignment_keys_invalid", message: "key_ids must be empty in all mode"}
	}
	return mode, nil
}

func normalizeUserAssignment(mode string, keyIDs []int64) (userAssignmentInput, error) {
	mode, valid := model.KeyAssignmentMode(mode).Normalize()
	if !valid {
		return userAssignmentInput{}, errors.New("invalid assignment mode")
	}
	normalizedIDs, err := normalizeUserAssignmentIDs(keyIDs)
	if err != nil {
		return userAssignmentInput{}, err
	}
	return userAssignmentInput{mode: mode, keyIDs: normalizedIDs}, nil
}

func normalizeUserAssignmentIDs(keyIDs []int64) ([]int64, error) {
	normalized := make([]int64, 0, len(keyIDs))
	seen := make(map[int64]struct{}, len(keyIDs))
	for _, id := range keyIDs {
		if id <= 0 {
			return nil, errors.New("invalid key id")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}
