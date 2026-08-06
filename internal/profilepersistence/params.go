package profilepersistence

type CreateProfileParams struct {
	Label        string
	Status       string
	Kind         string
	Category     string
	CategoryID   *int64
	TemplateText string
	Protocol     string
	BuiltURI     string
}

type UpdateProfileParams struct {
	ID               int64
	ExpectedRevision int64
	Label            string
	Status           string
	Kind             string
	Category         string
	CategoryID       *int64
	TemplateText     string
	Protocol         string
	NewURI           string
}

type CloneProfileParams struct {
	ID               int64
	ExpectedRevision int64
	NewLabel         string
}

type CreateLegacyKeyParams struct {
	Label        string
	Status       string
	Kind         string
	Category     string
	TemplateText string
	KeyURL       string
}

type UpdateLegacyKeyParams struct {
	ID           int64
	Label        string
	Status       string
	Kind         string
	Category     string
	TemplateText string
	BuiltURL     string
}
