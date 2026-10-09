package main

import (
	"database/sql"
	"errors"
)

// responsePolicyStore uses the App's existing database and autocommit boundary.
type responsePolicyStore struct{ db *sql.DB }

type responseDeletionFailure uint8

const (
	responseDeleteOK responseDeletionFailure = iota
	responseDeleteNotFound
	responseDeleteLookupFailed
	responseDeleteSystemProtected
	responseDeleteWriteFailed
)

func responseDeletionGuard(system bool, err error) responseDeletionFailure {
	if errors.Is(err, sql.ErrNoRows) {
		return responseDeleteNotFound
	}
	if err != nil {
		return responseDeleteLookupFailed
	}
	if system {
		return responseDeleteSystemProtected
	}
	return responseDeleteOK
}

func (a *App) responsePolicyStore() responsePolicyStore { return responsePolicyStore{a.db} }

func (a *App) listSubscriptionTemplates() ([]subscriptionTemplate, error) {
	return a.responsePolicyStore().listTemplates()
}
func (a *App) listResponseRules() ([]responseRule, error) { return a.responsePolicyStore().listRules() }
func (a *App) loadTemplate(id *int64) (*subscriptionTemplate, error) {
	return a.responsePolicyStore().loadTemplate(id)
}
