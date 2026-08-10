package profilepersistence

type CreateProfileParams struct {
	Label             string
	ClientDisplayName string
	Status            string
	Kind              string
	Category          string
	CategoryID        *int64
	TemplateText      string
	Protocol          string
	BuiltURI          string
}

type UpdateProfileParams struct {
	ID                int64
	ExpectedRevision  int64
	Label             string
	ClientDisplayName *string
	Status            string
	Kind              string
	Category          string
	CategoryID        *int64
	TemplateText      string
	Protocol          string
	NewURI            string
}

type UpdateClientDisplayNameParams struct {
	ID                int64
	ExpectedRevision  int64
	ClientDisplayName *string
}

type CloneProfileParams struct {
	ID               int64
	ExpectedRevision int64
	NewLabel         string
}
