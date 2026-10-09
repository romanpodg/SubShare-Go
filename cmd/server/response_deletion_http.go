package main

import (
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
)

type responseDeletionHTTPError struct {
	Status        int
	Code, Message string
}

type responseDeletionHTTPCommand struct {
	remove                  func(int64) responseDeletionFailure
	errors                  map[responseDeletionFailure]responseDeletionHTTPError
	action, target, message string
}

func (a *App) serveResponseDeletion(w http.ResponseWriter, r *http.Request, command responseDeletionHTTPCommand) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	if writeResponseDeletionFailure(w, r, command.remove(id), command.errors) {
		return
	}
	a.recordAuditEvent(r, command.action, command.target, strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, command.message)
}

var templateDeletionErrors = map[responseDeletionFailure]responseDeletionHTTPError{
	responseDeleteNotFound:        {http.StatusNotFound, "template_not_found", "template not found"},
	responseDeleteLookupFailed:    {http.StatusInternalServerError, "template_delete_failed", "failed to delete template"},
	responseDeleteSystemProtected: {http.StatusConflict, "system_template", "system templates cannot be deleted"},
	responseDeleteWriteFailed:     {http.StatusConflict, "template_in_use", "template is used by a response rule"},
}

var ruleDeletionErrors = map[responseDeletionFailure]responseDeletionHTTPError{
	responseDeleteNotFound:        {http.StatusNotFound, "rule_not_found", "response rule not found"},
	responseDeleteLookupFailed:    {http.StatusInternalServerError, "rule_delete_failed", "failed to delete response rule"},
	responseDeleteSystemProtected: {http.StatusConflict, "system_rule", "system response rules cannot be deleted"},
	responseDeleteWriteFailed:     {http.StatusInternalServerError, "rule_delete_failed", "failed to delete response rule"},
}

func writeResponseDeletionFailure(w http.ResponseWriter, r *http.Request, failure responseDeletionFailure, policy map[responseDeletionFailure]responseDeletionHTTPError) bool {
	if failure == responseDeleteOK {
		return false
	}
	output := policy[failure]
	httpapi.WriteV1Error(w, r, output.Status, output.Code, output.Message)
	return true
}
