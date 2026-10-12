package main

import (
	"errors"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

var errUserSettingsNotFound = errors.New("user settings target not found")

type userSettingsValidationError struct{ cause error }

func (err *userSettingsValidationError) Error() string { return err.cause.Error() }
func (err *userSettingsValidationError) Unwrap() error { return err.cause }

// One original background-context UPDATE; confirm absence only with a known
// zero count. A reporting error does not imply that the write was rolled back.
func (a *App) updateUserSettings(id int64, input model.UpdateUserSettingsRequest) (userSettingsInput, error) {
	next, err := prepareUserSettings(input)
	if err != nil {
		return userSettingsInput{}, &userSettingsValidationError{cause: err}
	}
	result, err := a.db.Exec(`UPDATE users SET time_zone = ?, language = ? WHERE id = ?`, next.timeZone, next.language, id)
	if err != nil {
		return userSettingsInput{}, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return userSettingsInput{}, err
	}
	if updated == 0 {
		return userSettingsInput{}, errUserSettingsNotFound
	}
	return next, nil
}
