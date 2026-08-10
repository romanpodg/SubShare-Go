package keypersistence

import "time"

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

type CreateCategoryParams struct {
	Name  string
	Color string
}

type UpdateCategoryParams struct {
	OldName string
	NewName string
	Color   string
}

type DeleteCategoryParams struct {
	Name string
	Mode string
}

type BulkUpdateKeysParams struct {
	IDs           []int64
	Status        string
	ApplyCategory bool
	Category      string
	CategoryID    int64
}

type HealthCheckTarget struct {
	ID         int64
	URL        string
	Status     string
	Kind       string
	Unreadable bool
}

type SaveHealthCheckResultParams struct {
	ID      int64
	Status  string
	Error   string
	Latency int64
}

type HealthCheckResult struct {
	ID            int64
	Status        string
	Error         string
	LastCheckedAt *time.Time
	Latency       int64
}
