package keymanagement

import "github.com/romanpodg/SubShare-Go/internal/model"

type RevealParams struct {
	ID              int64
	Target          string
	ProfileRevision int64
}

type RevealResult struct {
	KeyID                    int64
	ConfirmedProfileRevision int64
	Target                   string
	RawURI                   string
	Secrets                  model.StructuredSecretsMap
}

type CreateLocalParams struct {
	Label        string
	Status       string
	Kind         string
	Category     string
	CategoryID   *int64
	TemplateText string
	CreationMode string
	RawURI       string
	Protocol     string
	Structured   *model.StructuredProfilePatch
}

type UpdateLocalParams struct {
	ID              int64
	ProfileRevision int64
	Label           string
	Status          string
	Kind            string
	Category        string
	CategoryID      *int64
	TemplateText    string
	PatchMode       string
	RawURI          string
	StructuredPatch *model.StructuredProfilePatch
}

type CloneParams struct {
	ID                      int64
	ExpectedProfileRevision int64
	NewLabel                string
}

type CreateLegacyParams struct {
	Label        string
	URL          string
	Category     string
	TemplateText string
	Kind         string
	Status       string
}

type UpdateLegacyParams struct {
	Label        string
	Category     string
	TemplateText string
	Kind         string
	Status       string
	RawURL       string
	UUID         string
	Host         string
	Port         string
	Query        string
	Fragment     string
}
