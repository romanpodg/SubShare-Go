package model

import (
	"encoding/json"
	"strings"
	"time"
)

// Status constants for users.
const (
	UserStatusActive  = "active"
	UserStatusPaused  = "paused"
	UserStatusBlocked = "blocked"
)

// Status constants for keys.
const (
	KeyStatusActive    = "active"
	KeyStatusNonActive = "non-active"
)

// Kind constants for keys.
const (
	KeyKindReal          = "real"
	KeyKindInformational = "informational"
)

// Subscription format constants.
const (
	SubscriptionFormatLinks    = "links"
	SubscriptionFormatXrayJSON = "xray-json"
)

// External profile protocol identifiers are shared by persistence and API
// DTOs. "xray-json" remains the legacy structured configuration container.
const (
	ProfileProtocolLegacy      = "legacy"
	ProfileProtocolVLESS       = "vless"
	ProfileProtocolVMess       = "vmess"
	ProfileProtocolTrojan      = "trojan"
	ProfileProtocolXrayJSON    = "xray-json"
	ProfileProtocolShadowsocks = "shadowsocks"
	ProfileProtocolHysteria2   = "hysteria2"
	ProfileProtocolTUIC        = "tuic"
)

const (
	ProfileCompatibilityLegacy   = "legacy"
	ProfileCompatibilityFull     = "full"
	ProfileCompatibilityReadOnly = "read_only"
)

const (
	ExternalImportStatusAccepted          = "accepted"
	ExternalImportStatusAdded             = "added"
	ExternalImportStatusRejected          = "rejected"
	ExternalImportStatusDuplicate         = "duplicate"
	ExternalImportStatusUpdated           = "updated"
	ExternalImportStatusUnchanged         = "unchanged"
	ExternalImportStatusCompatibilityOnly = "compatibility_only"
	ExternalImportStatusAmbiguous         = "ambiguous"
	ExternalImportStatusUnsupported       = "unsupported"
)

const (
	KeyAssignmentModeAll      = "all"
	KeyAssignmentModeSelected = "selected"
)

// DefaultDeviceLimitMessage is the fallback message when the HWID device limit is exceeded.
const DefaultDeviceLimitMessage = "You have reached the maximum number of allowed devices for your subscription"
const AdminSessionCookieName = "subshare_admin_session"

// Admin represents an administrator account.
type Admin struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// AdminSession represents an authenticated admin session.
type AdminSession struct {
	AdminID   int64
	Role      string
	ExpiresAt time.Time
	CSRFToken string
	AuthKind  string
	Scopes    []string
}

// User represents a subscription user.
type User struct {
	ID                       int64             `json:"id"`
	Name                     string            `json:"name"`
	Email                    string            `json:"email"`
	Token                    string            `json:"token"`
	TimeZone                 string            `json:"time_zone"`
	Language                 string            `json:"language"`
	ActivationCode           string            `json:"activation_code"`
	SubscriptionID           string            `json:"subscription_id"`
	SubscriptionName         string            `json:"subscription_name"`
	SubscriptionRefreshHours int               `json:"subscription_refresh_hours"`
	SubscriptionInfoURL      string            `json:"subscription_info_url"`
	SubscriptionExtraURL     string            `json:"subscription_extra_url"`
	SubscriptionExtraStatus  string            `json:"subscription_extra_status"`
	ActivationUsedAt         string            `json:"activation_used_at"`
	Status                   string            `json:"status"`
	EffectiveStatus          string            `json:"effective_status"`
	StartsAtInput            string            `json:"starts_at"`
	ExpiresAtInput           string            `json:"expires_at"`
	BlockedReason            string            `json:"blocked_reason"`
	AssignedKeyIDs           string            `json:"assigned_key_ids"`
	MaxDevices               int               `json:"max_devices"`
	ConnectedDeviceCount     int               `json:"connected_device_count"`
	ConnectedHWIDs           []string          `json:"connected_hwids"`
	ConnectedDevices         []ConnectedDevice `json:"connected_devices"`
	KeyAssignmentMode        string            `json:"key_assignment_mode"`
	CreatedAt                time.Time         `json:"created_at"`
}

// EffectiveUserStatus derives the operational status without changing the
// persisted compatibility status.
func EffectiveUserStatus(status string, expiresAt time.Time, hasExpiry bool, connectedDevices, maxDevices int, now time.Time) string {
	normalized := NormalizeStoredStatus(status)
	if normalized == UserStatusBlocked {
		return "blocked"
	}
	if normalized == UserStatusPaused {
		return "paused"
	}
	if hasExpiry && now.After(expiresAt) {
		return "expired"
	}
	if maxDevices > 0 && connectedDevices >= maxDevices {
		return "limited"
	}
	return "active"
}

// ConnectedDevice stores device metadata collected from subscription client requests.
type ConnectedDevice struct {
	HWID           string `json:"hwid"`
	NormalizedHWID string `json:"normalized_hwid"`
	DeviceName     string `json:"device_name"`
	DeviceModel    string `json:"device_model"`
	DeviceBrand    string `json:"device_brand"`
	Platform       string `json:"platform"`
	OSVersion      string `json:"os_version"`
	AppName        string `json:"app_name"`
	AppVersion     string `json:"app_version"`
	ClientApp      string `json:"client_app"`
	ClientVersion  string `json:"client_version"`
	UserAgent      string `json:"user_agent"`
	CreatedAt      string `json:"created_at"`
	LastSeenAt     string `json:"last_seen_at"`
}

// VLESSKey represents a VLESS server key.
type VLESSKey struct {
	ID                          int64     `json:"id"`
	Label                       string    `json:"label"`
	URL                         string    `json:"url"`
	CategoryID                  int64     `json:"category_id"`
	Category                    string    `json:"category"`
	Kind                        string    `json:"kind"`
	TemplateText                string    `json:"template_text"`
	URLShort                    string    `json:"url_short"`
	Status                      string    `json:"status"`
	StatusLabel                 string    `json:"status_label"`
	CheckStatus                 string    `json:"check_status"`
	CheckStatusLabel            string    `json:"check_status_label"`
	CheckError                  string    `json:"check_error"`
	LastLatencyMS               int64     `json:"last_latency_ms"`
	LastCheckedAtText           string    `json:"last_checked_at"`
	EditUUID                    string    `json:"edit_uuid"`
	EditHost                    string    `json:"edit_host"`
	EditPort                    string    `json:"edit_port"`
	EditQuery                   string    `json:"edit_query"`
	EditFragment                string    `json:"edit_fragment"`
	ExternalSourceID            int64     `json:"external_source_id"`
	ExternalSourceName          string    `json:"external_source_name"`
	ClientDisplayName           string    `json:"client_display_name"`
	ClientDisplayNameOverridden bool      `json:"client_display_name_overridden"`
	Protocol                    string    `json:"protocol"`
	ProfileSchemaVersion        int       `json:"profile_schema_version"`
	ProfileCompatibility        string    `json:"profile_compatibility"`
	ProfileWarnings             []string  `json:"profile_warnings"`
	ProfileRevision             int64     `json:"profile_revision"`
	UpdatedAt                   time.Time `json:"updated_at"`
	CreatedAt                   time.Time `json:"created_at"`
}

// LoginRequest is the payload for POST /api/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateUserRequest is the payload for POST /api/admin/users.
type CreateUserRequest struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	ActivationCode string `json:"activation_code"`
	Status         string `json:"status"`
	IssueDays      int    `json:"issue_days"`
	BlockedReason  string `json:"blocked_reason"`
}

// UpdateUserKeysRequest is the payload for PUT /api/admin/users/{id}/keys.
type UpdateUserKeysRequest struct {
	KeyIDs []int64 `json:"key_ids"`
}

// UpdateSubscriptionRequest is the payload for PUT /api/admin/users/{id}/subscription.
type UpdateSubscriptionRequest struct {
	Status                   string `json:"status"`
	StartsAt                 string `json:"starts_at"`
	ExpiresAt                string `json:"expires_at"`
	BlockedReason            string `json:"blocked_reason"`
	SubscriptionName         string `json:"subscription_name"`
	SubscriptionRefreshHours int    `json:"subscription_refresh_hours"`
	SubscriptionInfoURL      string `json:"subscription_info_url"`
	SubscriptionExtraURL     string `json:"subscription_extra_url"`
	SubscriptionExtraStatus  string `json:"subscription_extra_status"`
}

// OptionalString distinguishes an omitted JSON field from an explicit null.
type OptionalString struct {
	Value string
	Set   bool
	Null  bool
}

func (value *OptionalString) UnmarshalJSON(data []byte) error {
	value.Set = true
	if string(data) == "null" {
		value.Null = true
		value.Value = ""
		return nil
	}
	return json.Unmarshal(data, &value.Value)
}

// OptionalInt distinguishes an omitted JSON field from an explicit null.
type OptionalInt struct {
	Value int
	Set   bool
	Null  bool
}

func (value *OptionalInt) UnmarshalJSON(data []byte) error {
	value.Set = true
	if string(data) == "null" {
		value.Null = true
		value.Value = 0
		return nil
	}
	return json.Unmarshal(data, &value.Value)
}

type PatchSubscriptionRequest struct {
	Status                   OptionalString `json:"status"`
	StartsAt                 OptionalString `json:"starts_at"`
	ExpiresAt                OptionalString `json:"expires_at"`
	BlockedReason            OptionalString `json:"blocked_reason"`
	SubscriptionName         OptionalString `json:"subscription_name"`
	SubscriptionRefreshHours OptionalInt    `json:"subscription_refresh_hours"`
	SubscriptionInfoURL      OptionalString `json:"subscription_info_url"`
	SubscriptionExtraURL     OptionalString `json:"subscription_extra_url"`
	SubscriptionExtraStatus  OptionalString `json:"subscription_extra_status"`
}

type UpdateKeyAssignmentRequest struct {
	Mode   string  `json:"mode"`
	KeyIDs []int64 `json:"key_ids"`
}

// UpdateUserSettingsRequest is the payload for PUT /api/admin/users/{id}/settings.
type UpdateUserSettingsRequest struct {
	TimeZone string `json:"time_zone"`
	Language string `json:"language"`
}

// UpdateHWIDRequest is the payload for PUT /api/admin/users/{id}/hwid.
type UpdateHWIDRequest struct {
	MaxDevices int `json:"max_devices"`
}

// CreateKeyRequest is the payload for POST /api/admin/keys.
type CreateKeyRequest struct {
	Label        string `json:"label"`
	URL          string `json:"url"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	Kind         string `json:"kind"`
	TemplateText string `json:"template_text"`
}

// UpdateKeyRequest is the payload for PUT /api/admin/keys/{id}.
type UpdateKeyRequest struct {
	Label        string `json:"label"`
	Status       string `json:"status"`
	Category     string `json:"category"`
	RawURL       string `json:"raw_url"`
	UUID         string `json:"uuid"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	Query        string `json:"query"`
	Fragment     string `json:"fragment"`
	Kind         string `json:"kind"`
	TemplateText string `json:"template_text"`
}

// ReorderKeysRequest is the payload for PUT /api/admin/keys/reorder.
type ReorderKeysRequest struct {
	IDs []int64 `json:"ids"`
}

// BulkUpdateKeyStatusRequest is the payload for POST /api/admin/keys/bulk/status.
type BulkUpdateKeyStatusRequest struct {
	IDs      []int64 `json:"ids"`
	Status   string  `json:"status"`
	Category string  `json:"category"`
}

// BulkDeleteKeysRequest is the payload for POST /api/admin/keys/bulk/delete.
type BulkDeleteKeysRequest struct {
	IDs []int64 `json:"ids"`
}

type KeyCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	KeysCount int    `json:"keys_count"`
}

type CreateKeyCategoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type RenameKeyCategoryRequest struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

type UpdateKeyCategoryRequest struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
	Color   string `json:"color"`
}

type DeleteKeyCategoryRequest struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type ReorderKeyCategoriesRequest struct {
	Names []string `json:"names"`
}

// ActivateRequest is the payload for POST /api/subscription/activate.
type ActivateRequest struct {
	ActivationCode string `json:"activation_code"`
}

// SubscriptionSettings contains global subscription metadata shown to clients.
type SubscriptionSettings struct {
	Title                      string `json:"title"`
	RefreshHours               int    `json:"refresh_hours"`
	InfoURL                    string `json:"info_url"`
	ExtraURL                   string `json:"extra_url"`
	ExtraStatus                string `json:"extra_status"`
	SubscriptionFormat         string `json:"subscription_format"`
	ShowSubscriptionExpiration bool   `json:"show_subscription_expiration"`
	TimeZone                   string `json:"time_zone"`
	Language                   string `json:"language"`
	ProviderID                 string `json:"provider_id"`
	HappNoLimitMode            bool   `json:"happ_no_limit_mode"`
	HappNoLimitModeXHTTPOnly   bool   `json:"happ_no_limit_mode_xhttp_only"`
	HappMandatoryHWID          bool   `json:"happ_mandatory_hwid"`
	HappNotifyExpiration       bool   `json:"happ_notify_expiration"`
	HappHideServerSettings     bool   `json:"happ_hide_server_settings"`
	HappSubscriptionBody       string `json:"happ_subscription_body"`
}

// UpdateSubscriptionSettingsRequest is the payload for PUT /api/admin/subscription-settings.
type UpdateSubscriptionSettingsRequest struct {
	Title                      string `json:"title"`
	RefreshHours               int    `json:"refresh_hours"`
	InfoURL                    string `json:"info_url"`
	ExtraURL                   string `json:"extra_url"`
	ExtraStatus                string `json:"extra_status"`
	SubscriptionFormat         string `json:"subscription_format"`
	ShowSubscriptionExpiration *bool  `json:"show_subscription_expiration"`
	TimeZone                   string `json:"time_zone"`
	Language                   string `json:"language"`
	ProviderID                 string `json:"provider_id"`
	HappNoLimitMode            bool   `json:"happ_no_limit_mode"`
	HappNoLimitModeXHTTPOnly   bool   `json:"happ_no_limit_mode_xhttp_only"`
	HappMandatoryHWID          bool   `json:"happ_mandatory_hwid"`
	HappNotifyExpiration       bool   `json:"happ_notify_expiration"`
	HappHideServerSettings     bool   `json:"happ_hide_server_settings"`
	HappSubscriptionBody       string `json:"happ_subscription_body"`
}

type RoutingSettings struct {
	ConfigJSON string `json:"config_json"`
}

// ExternalSubscriptionSource stores settings for a third-party subscription source.
type ExternalSubscriptionSource struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Category            string `json:"category"`
	KeyCategory         string `json:"key_category"`
	KeyInsertMode       string `json:"key_insert_mode"`
	SourceURL           string `json:"source_url"`
	Enabled             bool   `json:"enabled"`
	ApplyRemoteMetadata bool   `json:"apply_remote_metadata"`
	PassHWID            bool   `json:"pass_hwid"`
	HWIDVersion         string `json:"hwid_version"`
	HWIDModelName       string `json:"hwid_model_name"`
	HWIDValue           string `json:"hwid_value"`
	LastImportCount     int    `json:"last_import_count"`
	ImportStatus        string `json:"import_status"`
	LastError           string `json:"last_error"`
	LastSyncedAt        string `json:"last_synced_at"`
	MetaTitle           string `json:"meta_title"`
	MetaRefreshHours    int    `json:"meta_refresh_hours"`
	MetaSupportURL      string `json:"meta_support_url"`
	MetaWebPageURL      string `json:"meta_web_page_url"`
	MetaAnnounce        string `json:"meta_announce"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type ExternalSourceCategory struct {
	Name         string `json:"name"`
	SourcesCount int    `json:"sources_count"`
}

type ExternalSourceCategoryCreateRequest struct {
	Name string `json:"name"`
}

type ExternalSourceCategoryRenameRequest struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

type ExternalSourcePreviewRequest struct {
	SourceURL      string `json:"source_url"`
	PassHWID       bool   `json:"pass_hwid"`
	HWIDVersion    string `json:"hwid_version"`
	HWIDModelName  string `json:"hwid_model_name"`
	HWIDValue      string `json:"hwid_value"`
	RawBody        string `json:"raw_body"`
	RawContentType string `json:"raw_content_type"`
	RawFinalURL    string `json:"raw_final_url"`
}

type ExternalSourceImportRequest struct {
	Name                string   `json:"name"`
	Category            string   `json:"category"`
	KeyCategory         string   `json:"key_category"`
	KeyInsertMode       string   `json:"key_insert_mode"`
	SourceURL           string   `json:"source_url"`
	Enabled             bool     `json:"enabled"`
	ApplyRemoteMetadata bool     `json:"apply_remote_metadata"`
	PassHWID            bool     `json:"pass_hwid"`
	HWIDVersion         string   `json:"hwid_version"`
	HWIDModelName       string   `json:"hwid_model_name"`
	HWIDValue           string   `json:"hwid_value"`
	RawBody             string   `json:"raw_body"`
	RawContentType      string   `json:"raw_content_type"`
	RawFinalURL         string   `json:"raw_final_url"`
	SelectedItemRefs    []string `json:"selected_item_refs"`
}

type ExternalSourceUpdateRequest struct {
	Name                string `json:"name"`
	Category            string `json:"category"`
	KeyCategory         string `json:"key_category"`
	KeyInsertMode       string `json:"key_insert_mode"`
	SourceURL           string `json:"source_url"`
	Enabled             bool   `json:"enabled"`
	ApplyRemoteMetadata bool   `json:"apply_remote_metadata"`
	PassHWID            bool   `json:"pass_hwid"`
	HWIDVersion         string `json:"hwid_version"`
	HWIDModelName       string `json:"hwid_model_name"`
	HWIDValue           string `json:"hwid_value"`
}

// PanelSettings stores admin panel UI customization (title, logo, favicon, page titles).
type PanelSettings struct {
	PanelTitle             string `json:"panelTitle"`
	LogoDataURL            string `json:"logoDataUrl"`
	FaviconDataURL         string `json:"faviconDataUrl"`
	PageTitleAdmin         string `json:"pageTitleAdmin"`
	PageTitleAdminLogin    string `json:"pageTitleAdminLogin"`
	PageTitleSubscription  string `json:"pageTitleSubscription"`
	SubscriptionPageConfig string `json:"-"`
}

type UpdateSubscriptionPageConfigRequest struct {
	ConfigJSON string `json:"config_json"`
}

type SubscriptionPageConfig struct {
	Locale       string                  `json:"locale"`
	TemplateVars map[string]string       `json:"templateVars"`
	Theme        map[string]string       `json:"theme"`
	Blocks       []SubscriptionPageBlock `json:"blocks"`
}

type SubscriptionPageBlock struct {
	Type          string                      `json:"type"`
	ID            string                      `json:"id,omitempty"`
	BrandTitle    string                      `json:"brandTitle,omitempty"`
	BrandSubtitle string                      `json:"brandSubtitle,omitempty"`
	Subhead       string                      `json:"subhead,omitempty"`
	StatusBadge   string                      `json:"statusBadge,omitempty"`
	Logo          SubscriptionPageLogo        `json:"logo,omitempty"`
	Title         string                      `json:"title,omitempty"`
	Steps         []SubscriptionPageStep      `json:"steps,omitempty"`
	Copyright     string                      `json:"copyright,omitempty"`
	LanguageBadge string                      `json:"languageBadge,omitempty"`
	FooterLink    *SubscriptionPageFooterLink `json:"footerLink,omitempty"`
}

type SubscriptionPageFooterLink struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type SubscriptionPageLogo struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
}

type SubscriptionPageStep struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Block       SubscriptionPageStepBlock `json:"block"`
}

type SubscriptionPageStepBlock struct {
	Type            string                   `json:"type"`
	Buttons         []SubscriptionPageButton `json:"buttons,omitempty"`
	AddButtonLabel  string                   `json:"addButtonLabel,omitempty"`
	ManualLinkLabel string                   `json:"manualLinkLabel,omitempty"`
	CopyLabel       string                   `json:"copyLabel,omitempty"`
	CopiedLabel     string                   `json:"copiedLabel,omitempty"`
}

type SubscriptionPageButton struct {
	ID       string               `json:"id"`
	Label    string               `json:"label"`
	Href     string               `json:"href"`
	Variant  string               `json:"variant"`
	External bool                 `json:"external"`
	Icon     SubscriptionPageIcon `json:"icon"`
}

type SubscriptionPageIcon struct {
	Type string `json:"type"`
	Src  string `json:"src,omitempty"`
	Alt  string `json:"alt,omitempty"`
}

type CreateAdminRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateAdminRequest struct {
	Password string `json:"password"`
	Role     string `json:"role"`
}

// NormalizeUserStatus validates and normalizes a user status string.
// It returns the normalized status and true if valid, or empty string and false otherwise.
func NormalizeUserStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = UserStatusActive
	}
	switch status {
	case UserStatusActive, UserStatusPaused, UserStatusBlocked:
		return status, true
	default:
		return "", false
	}
}

// NormalizeStoredStatus normalizes a status from the database, defaulting to active.
func NormalizeStoredStatus(raw string) string {
	status, ok := NormalizeUserStatus(raw)
	if !ok {
		return UserStatusActive
	}
	return status
}

// NormalizeKeyStatus validates and normalizes a key status string.
func NormalizeKeyStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = KeyStatusActive
	}
	if status == "blocked" {
		status = KeyStatusNonActive
	}
	switch status {
	case KeyStatusActive, KeyStatusNonActive:
		return status, true
	default:
		return "", false
	}
}

// KeyStatusLabel returns a human-readable label for a key status.
func KeyStatusLabel(status string) string {
	switch status {
	case KeyStatusActive:
		return "active"
	case KeyStatusNonActive:
		return "non-active"
	default:
		return "non-active"
	}
}

// NormalizeCheckStatus normalizes a health check status string.
func NormalizeCheckStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "up", "down", "unknown":
		return status
	default:
		return "unknown"
	}
}

// NormalizeSubscriptionFormat validates and normalizes subscription format.
func NormalizeSubscriptionFormat(raw string) (string, bool) {
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		format = SubscriptionFormatLinks
	}
	switch format {
	case SubscriptionFormatLinks, SubscriptionFormatXrayJSON:
		return format, true
	default:
		return "", false
	}
}

// NormalizeKeyKind validates and normalizes key kind.
func NormalizeKeyKind(raw string) (string, bool) {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if kind == "" {
		kind = KeyKindReal
	}
	switch kind {
	case KeyKindReal, KeyKindInformational:
		return kind, true
	default:
		return "", false
	}
}

func NormalizeKeyAssignmentMode(raw string) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case KeyAssignmentModeAll, KeyAssignmentModeSelected:
		return mode, true
	default:
		return "", false
	}
}

// CheckStatusLabel returns a human-readable label for a check status (in Russian).
func CheckStatusLabel(status string) string {
	switch status {
	case "up":
		return "Доступен"
	case "down":
		return "Недоступен"
	default:
		return "Не проверен"
	}
}

// --- Stage 8 DTOs for Protocol-Aware Profile Editing ---

type KeyOwnership string

const (
	OwnershipLocal          KeyOwnership = "local"
	OwnershipExternalSource KeyOwnership = "external_source"
)

type TriStateOp string

const (
	TriStateSet   TriStateOp = "set"
	TriStateClear TriStateOp = "clear"
)

type TriStatePatch[T any] struct {
	Set       bool       `json:"-"`
	Operation TriStateOp `json:"operation"`
	Value     T          `json:"value,omitempty"`
}

func (t *TriStatePatch[T]) UnmarshalJSON(b []byte) error {
	t.Set = true
	type rawPatch TriStatePatch[T]
	return json.Unmarshal(b, (*rawPatch)(t))
}

type SafeShadowsocksDetail struct {
	Method               string `json:"method"`
	PasswordPresent      bool   `json:"password_present"`
	PluginName           string `json:"plugin_name,omitempty"`
	PluginOptionsPresent bool   `json:"plugin_options_present"`
	UserInfoStyle        string `json:"user_info_style"`
}

type SafeHysteria2Detail struct {
	AuthenticationPresent      bool   `json:"authentication_present"`
	SNI                        string `json:"sni"`
	Insecure                   bool   `json:"insecure"`
	CertificateSHA256          string `json:"certificate_sha256"`
	ObfuscationType            string `json:"obfuscation_type"`
	ObfuscationPasswordPresent bool   `json:"obfuscation_password_present"`
}

type TUICFieldObservationDTO struct {
	Field      string `json:"field"`
	FieldClass string `json:"field_class"`
	Provenance string `json:"provenance"`
}

type SafeTUICDetail struct {
	Generation                  int                       `json:"generation"`
	UUIDPresent                 bool                      `json:"uuid_present"`
	PasswordPresent             bool                      `json:"password_present"`
	TokenPresent                bool                      `json:"token_present"`
	SNI                         string                    `json:"sni"`
	ALPN                        []string                  `json:"alpn"`
	SkipCertificateVerification bool                      `json:"skip_cert_verify"`
	DisableSNI                  bool                      `json:"disable_sni"`
	CongestionController        string                    `json:"congestion_controller"`
	UDPRelayMode                string                    `json:"udp_relay_mode"`
	UDPOverStream               bool                      `json:"udp_over_stream"`
	ZeroRTT                     bool                      `json:"zero_rtt"`
	Heartbeat                   string                    `json:"heartbeat"`
	FieldObservations           []TUICFieldObservationDTO `json:"field_observations"`
}

type SafeXrayJSONDetail struct {
	HasRawJSON bool   `json:"has_raw_json"`
	Network    string `json:"network"`
	Security   string `json:"security"`
}

type SafeStructuredProfile struct {
	Server      string                 `json:"server"`
	Port        string                 `json:"port"`
	PortKind    string                 `json:"port_kind"`
	DisplayName string                 `json:"display_name"`
	Shadowsocks *SafeShadowsocksDetail `json:"shadowsocks,omitempty"`
	Hysteria2   *SafeHysteria2Detail   `json:"hysteria2,omitempty"`
	TUIC        *SafeTUICDetail        `json:"tuic,omitempty"`
	XrayJSON    *SafeXrayJSONDetail    `json:"xray_json,omitempty"`
}

type UnknownQueryParamDTO struct {
	Key      string `json:"key"`
	HasValue bool   `json:"has_value"`
}

type SanitizedCheckError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type KeyProfileDetailResponse struct {
	ID                          int64                     `json:"id"`
	Label                       string                    `json:"label"`
	ClientDisplayName           string                    `json:"client_display_name"`
	ClientDisplayNameOverridden bool                      `json:"client_display_name_overridden"`
	CategoryID                  *int64                    `json:"category_id"`
	Category                    string                    `json:"category"`
	Kind                        string                    `json:"kind"`
	Status                      string                    `json:"status"`
	CheckStatus                 string                    `json:"check_status"`
	CheckError                  SanitizedCheckError       `json:"check_error"`
	LastLatencyMS               int64                     `json:"last_latency_ms"`
	LastCheckedAt               string                    `json:"last_checked_at"`
	TemplateText                string                    `json:"template_text"`
	Ownership                   KeyOwnership              `json:"ownership"`
	ExternalSourceID            *int64                    `json:"external_source_id"`
	ExternalSourceName          string                    `json:"external_source_name"`
	Protocol                    string                    `json:"protocol"`
	ProfileSchemaVersion        int                       `json:"profile_schema_version"`
	ProfileCompatibility        string                    `json:"profile_compatibility"`
	ProfileWarnings             []string                  `json:"profile_warnings"`
	ProfileRevision             int64                     `json:"profile_revision"`
	CreatedAt                   time.Time                 `json:"created_at"`
	UpdatedAt                   time.Time                 `json:"updated_at"`
	SafeStructured              *SafeStructuredProfile    `json:"safe_structured,omitempty"`
	UnknownQueryParameters      []UnknownQueryParamDTO    `json:"unknown_query_parameters"`
	Capabilities                map[string]map[string]any `json:"capabilities"`
}

type KeySecretRevealRequest struct {
	ProfileRevision int64  `json:"profile_revision"`
	Target          string `json:"target"` // "raw" | "structured-secrets"
}

type KeyRawSecretResponse struct {
	KeyID                    int64  `json:"key_id"`
	ConfirmedProfileRevision int64  `json:"confirmed_profile_revision"`
	Target                   string `json:"target"`
	RawURI                   string `json:"raw_uri"`
}

type StructuredSecretsMap struct {
	Password            string `json:"password,omitempty"`
	Authentication      string `json:"authentication,omitempty"`
	UUID                string `json:"uuid,omitempty"`
	Token               string `json:"token,omitempty"`
	ObfuscationPassword string `json:"obfuscation_password,omitempty"`
	PluginOptions       string `json:"plugin_options,omitempty"`
	RawJSON             string `json:"raw_json,omitempty"`
}

type KeyStructuredSecretsResponse struct {
	KeyID                    int64                `json:"key_id"`
	ConfirmedProfileRevision int64                `json:"confirmed_profile_revision"`
	Target                   string               `json:"target"`
	Secrets                  StructuredSecretsMap `json:"secrets"`
}

type ShadowsocksStructuredPatch struct {
	Method        *TriStatePatch[string] `json:"method,omitempty"`
	Password      *TriStatePatch[string] `json:"password,omitempty"`
	PluginName    *TriStatePatch[string] `json:"plugin_name,omitempty"`
	PluginOptions *TriStatePatch[string] `json:"plugin_options,omitempty"`
}

type Hysteria2StructuredPatch struct {
	Authentication      *TriStatePatch[string] `json:"authentication,omitempty"`
	SNI                 *TriStatePatch[string] `json:"sni,omitempty"`
	Insecure            *TriStatePatch[bool]   `json:"insecure,omitempty"`
	CertificateSHA256   *TriStatePatch[string] `json:"certificate_sha256,omitempty"`
	ObfuscationType     *TriStatePatch[string] `json:"obfuscation_type,omitempty"`
	ObfuscationPassword *TriStatePatch[string] `json:"obfuscation_password,omitempty"`
}

type TUICStructuredPatch struct {
	UUID                 *TriStatePatch[string]   `json:"uuid,omitempty"`
	Password             *TriStatePatch[string]   `json:"password,omitempty"`
	SNI                  *TriStatePatch[string]   `json:"sni,omitempty"`
	ALPN                 *TriStatePatch[[]string] `json:"alpn,omitempty"`
	SkipCertVerify       *TriStatePatch[bool]     `json:"skip_cert_verify,omitempty"`
	CongestionController *TriStatePatch[string]   `json:"congestion_controller,omitempty"`
	UDPRelayMode         *TriStatePatch[string]   `json:"udp_relay_mode,omitempty"`
	UDPOverStream        *TriStatePatch[bool]     `json:"udp_over_stream,omitempty"`
	ZeroRTT              *TriStatePatch[bool]     `json:"zero_rtt,omitempty"`
	Heartbeat            *TriStatePatch[string]   `json:"heartbeat,omitempty"`
}

type StructuredProfilePatch struct {
	Server      *TriStatePatch[string]      `json:"server,omitempty"`
	Port        *TriStatePatch[string]      `json:"port,omitempty"`
	DisplayName *TriStatePatch[string]      `json:"display_name,omitempty"`
	Shadowsocks *ShadowsocksStructuredPatch `json:"shadowsocks,omitempty"`
	Hysteria2   *Hysteria2StructuredPatch   `json:"hysteria2,omitempty"`
	TUIC        *TUICStructuredPatch        `json:"tuic,omitempty"`
}

type UpdateKeyProfileRequest struct {
	Label             string                  `json:"label"`
	ClientDisplayName *string                 `json:"client_display_name,omitempty"`
	CategoryID        *int64                  `json:"category_id"`
	Category          string                  `json:"category"`
	Status            string                  `json:"status"`
	Kind              string                  `json:"kind"`
	TemplateText      string                  `json:"template_text"`
	ProfileRevision   int64                   `json:"profile_revision"`
	PatchMode         string                  `json:"patch_mode"` // "raw" | "structured"
	RawURI            string                  `json:"raw_uri,omitempty"`
	StructuredPatch   *StructuredProfilePatch `json:"structured_patch,omitempty"`
}

type CreateKeyProfileRequest struct {
	Label             string                  `json:"label"`
	ClientDisplayName *string                 `json:"client_display_name,omitempty"`
	CategoryID        *int64                  `json:"category_id"`
	Category          string                  `json:"category"`
	Status            string                  `json:"status"`
	Kind              string                  `json:"kind"`
	TemplateText      string                  `json:"template_text"`
	CreationMode      string                  `json:"creation_mode"` // "raw" | "structured"
	RawURI            string                  `json:"raw_uri,omitempty"`
	Protocol          string                  `json:"protocol,omitempty"`
	Structured        *StructuredProfilePatch `json:"structured,omitempty"`
}

type KeyCloneRequest struct {
	ExpectedProfileRevision int64  `json:"expected_profile_revision"`
	NewLabel                string `json:"new_label,omitempty"`
}

type DeclarativeWarningRule struct {
	Field          string `json:"field"`
	Operator       string `json:"operator"` // "equals" | "not_equals" | "is_true" | "is_false" | "in"
	Value          any    `json:"value,omitempty"`
	WarningCode    string `json:"warning_code"`
	WarningMessage string `json:"warning_message"`
}

type ProtocolEditorFieldSchema struct {
	FieldType    string                   `json:"field_type"`
	Required     bool                     `json:"required"`
	CanClear     bool                     `json:"can_clear"`
	Options      []map[string]string      `json:"options,omitempty"`
	DefaultValue any                      `json:"default_value,omitempty"`
	Provenance   string                   `json:"provenance,omitempty"`
	WarningRules []DeclarativeWarningRule `json:"warning_rules,omitempty"`
}

type ProtocolSchemaDTO struct {
	Protocol           string                               `json:"protocol"`
	Label              string                               `json:"label"`
	SupportedCreations []string                             `json:"supported_creations"`
	Fields             map[string]ProtocolEditorFieldSchema `json:"fields"`
}

type KeyEditorSchemaResponse struct {
	Protocols            []ProtocolSchemaDTO `json:"protocols"`
	ExclusionReasonCodes map[string]string   `json:"exclusion_reason_codes"`
}
