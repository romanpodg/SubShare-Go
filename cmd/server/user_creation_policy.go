package main

import (
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

// createUserInput is the validated, normalized input consumed by the command.
type createUserInput struct {
	name, email, activationCode, status, blockedReason string
	issueDays                                          int
}

func validateCreateUserRequest(req model.CreateUserRequest) (createUserInput, string) {
	in := createUserInput{
		name: strings.TrimSpace(req.Name), email: strings.TrimSpace(req.Email),
		activationCode: strings.TrimSpace(req.ActivationCode), issueDays: req.IssueDays,
	}
	status, ok := model.NormalizeUserStatus(req.Status)
	if !ok {
		return in, "invalid subscription status"
	}
	in.status = status
	if in.issueDays <= 0 {
		in.issueDays = 30
	}
	in.blockedReason = strings.TrimSpace(req.BlockedReason)
	if in.status != model.UserStatusBlocked {
		in.blockedReason = ""
	}
	return in, validateCreateUserInput(in)
}

func validateCreateUserInput(in createUserInput) string {
	if in.issueDays > 3650 {
		return "issue days must be between 1 and 3650"
	}
	if message := validateCreateUserRequiredFields(in); message != "" {
		return message
	}
	return validateCreateUserFieldLengths(in)
}

func validateCreateUserRequiredFields(in createUserInput) string {
	if in.name == "" {
		return "name is required"
	}
	if in.activationCode == "" || strings.Contains(in.activationCode, "/") {
		return "activation code is required"
	}
	return ""
}

func validateCreateUserFieldLengths(in createUserInput) string {
	if len(in.name) > 255 {
		return "name is too long (max 255 characters)"
	}
	if len(in.email) > 255 {
		return "email is too long (max 255 characters)"
	}
	if len(in.activationCode) > 128 {
		return "activation code is too long (max 128 characters)"
	}
	return ""
}
