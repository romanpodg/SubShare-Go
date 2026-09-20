// Package sources fetches, parses and synchronises external subscription
// feeds. Fetching goes through an injectable HTTP client behind an SSRF guard,
// parsing is pure, and Sync applies the reconciliation policy through the
// Profile store's SourceSync transaction.
package sources

import (
	"fmt"
	"io"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

const (
	MaxBodyBytes   = 10 << 20 // 10 MB
	MaxImportItems = 10000
)

type Metadata struct {
	Title           string
	RefreshHours    int
	SupportURL      string
	WebPageURL      string
	Announce        string
	ContentType     string
	ContentDisp     string
	SourceFinalURL  string
	HTTPStatusCode  int
	HTTPStatusLabel string
}

type ParsedKey struct {
	Label                 string
	URL                   string
	Scheme                string
	Protocol              string
	Host                  string
	Port                  string
	Ref                   string
	ItemRef               string
	LineIndex             int
	Compatibility         string
	ProfileSchemaVersion  int
	Fingerprint           string
	FingerprintCandidates []string
	WarningCodes          []string
	InitialStatus         string
}

func (key ParsedKey) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "external_profile{protocol="+key.Protocol+", item_ref="+key.ItemRef+", raw=[redacted]}")
}

func (key ParsedKey) String() string {
	return "external_profile{protocol=" + key.Protocol + ", item_ref=" + key.ItemRef + ", raw=[redacted]}"
}

func (key ParsedKey) GoString() string { return key.String() }

func (key ParsedKey) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("external_profile_internal_only")
}

type ParseResult struct {
	DetectedFormat string
	Metadata       Metadata
	Keys           []ParsedKey
	Items          []ImportItem
	Counts         Counts
	Warnings       []string
}

func (result ParseResult) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, fmt.Sprintf("external_subscription{format=%s, items=%d, raw=[redacted]}", result.DetectedFormat, len(result.Items)))
}

func (result ParseResult) String() string {
	return fmt.Sprintf("external_subscription{format=%s, items=%d, raw=[redacted]}", result.DetectedFormat, len(result.Items))
}

func (result ParseResult) GoString() string { return result.String() }

type ImportItem struct {
	ItemRef       string   `json:"item_ref"`
	LineIndex     int      `json:"line_index"`
	Protocol      string   `json:"protocol"`
	Scheme        string   `json:"scheme"`
	DisplayName   string   `json:"display_name"`
	Label         string   `json:"label"`
	Host          string   `json:"host"`
	Port          string   `json:"port"`
	Compatibility string   `json:"compatibility"`
	Status        string   `json:"status"`
	Warnings      []string `json:"warnings"`
	ErrorCode     string   `json:"error_code,omitempty"`
	// URLShort is a deprecated compatibility field. It contains only a safe
	// endpoint summary and never any userinfo, query string, or fragment.
	URLShort string `json:"url_short"`
}

type Counts struct {
	Accepted          int `json:"accepted"`
	Added             int `json:"added"`
	Rejected          int `json:"rejected"`
	Duplicate         int `json:"duplicate"`
	Updated           int `json:"updated"`
	Unchanged         int `json:"unchanged"`
	CompatibilityOnly int `json:"compatibility_only"`
	Ambiguous         int `json:"ambiguous"`
	Unsupported       int `json:"unsupported"`
	Removed           int `json:"removed"`
}

type SyncResult struct {
	Imported int
	Skipped  int
	Counts   Counts
	Items    []ImportItem
}

const ExternalProfileSchemaVersion = 1

const (
	StatusAccepted          = model.ExternalImportStatusAccepted
	StatusAdded             = model.ExternalImportStatusAdded
	StatusRejected          = model.ExternalImportStatusRejected
	StatusDuplicate         = model.ExternalImportStatusDuplicate
	StatusUpdated           = model.ExternalImportStatusUpdated
	StatusUnchanged         = model.ExternalImportStatusUnchanged
	StatusCompatibilityOnly = model.ExternalImportStatusCompatibilityOnly
	StatusAmbiguous         = model.ExternalImportStatusAmbiguous
	StatusUnsupported       = model.ExternalImportStatusUnsupported
)

func NonNilWarnings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func ContainsWarningCode(codes []string, wanted ...string) bool {
	for _, code := range codes {
		for _, candidate := range wanted {
			if code == candidate {
				return true
			}
		}
	}
	return false
}
