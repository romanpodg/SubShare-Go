export interface User {
  id: number;
  name: string;
  email: string;
  time_zone: string;
  language: string;
  activation_code: string;
  subscription_id: string;
  subscription_name: string;
  subscription_refresh_hours: number;
  subscription_info_url: string;
  subscription_extra_url: string;
  subscription_extra_status: string;
  activation_used_at: string;
  status: "active" | "paused" | "blocked";
  effective_status: EffectiveUserStatus;
  starts_at: string;
  expires_at: string;
  blocked_reason: string;
  key_assignment_mode: "all" | "selected";
  assigned_key_ids: string;
  max_devices: number;
  connected_device_count: number;
  connected_hwids: string[];
  connected_devices: ConnectedDevice[];
  created_at: string;
}

export interface ConnectedDevice {
  hwid: string;
  normalized_hwid: string;
  device_name: string;
  device_model: string;
  device_brand: string;
  platform: string;
  os_version: string;
  app_name: string;
  app_version: string;
  client_app: string;
  client_version: string;
  user_agent: string;
  created_at: string;
  last_seen_at: string;
}

export interface VLESSKey {
  id: number;
  label: string;
  url: string;
  category_id?: number;
  category: string;
  kind: "real" | "informational";
  template_text: string;
  url_short: string;
  status: "active" | "non-active";
  status_label: string;
  check_status: "up" | "down" | "unknown" | "unsupported_check";
  check_status_label: string;
  check_error: string;
  last_latency_ms: number;
  last_checked_at: string;
  edit_uuid: string;
  edit_host: string;
  edit_port: string;
  edit_query: string;
  edit_fragment: string;
  external_source_id: number;
  external_source_name: string;
  client_display_name: string;
  protocol?: ExternalProfileProtocol;
  profile_schema_version?: number;
  profile_compatibility?: ExternalProfileCompatibility;
  profile_warnings?: string[];
  created_at: string;
}

export interface KeyCheckResult {
  id: number;
  check_status: string;
  check_status_label: string;
  check_error: string;
  last_checked_at: string;
  last_latency_ms: number;
}

export interface SubscriptionSettings {
  title: string;
  refresh_hours: number;
  info_url: string;
  extra_url: string;
  extra_status: string;
  subscription_format: "links" | "xray-json";
  show_subscription_expiration: boolean;
  time_zone: string;
  language: string;
  provider_id: string;
  happ_no_limit_mode: boolean;
  happ_no_limit_mode_xhttp_only: boolean;
  happ_mandatory_hwid: boolean;
  happ_notify_expiration: boolean;
  happ_hide_server_settings: boolean;
  happ_subscription_body: string;
}

export interface ExternalSubscriptionSource {
  id: number;
  name: string;
  category: string;
  key_category: string;
  key_insert_mode: "top" | "bottom";
  source_url: string;
  enabled: boolean;
  apply_remote_metadata: boolean;
  pass_hwid: boolean;
  hwid_version: string;
  hwid_model_name: string;
  hwid_value: string;
  last_import_count: number;
  import_status: "idle" | "syncing" | "ok" | "error";
  last_error: string;
  last_synced_at: string;
  meta_title: string;
  meta_refresh_hours: number;
  meta_support_url: string;
  meta_web_page_url: string;
  meta_announce: string;
  created_at: string;
  updated_at: string;
}

export interface SourceSummary {
  id: number;
  name: string;
  category: string;
  key_category: string;
  source_url_masked: string;
  enabled: boolean;
  pass_hwid: boolean;
  has_hwid_value: boolean;
  last_import_count: number;
  imported_keys: number;
  import_status: "idle" | "syncing" | "ok" | "error";
  last_error: string;
  last_synced_at: string;
  created_at: string;
  updated_at: string;
}

export interface SourceDetail extends SourceSummary {
  source_url: string;
  key_insert_mode: "top" | "bottom";
  hwid_version: string;
  hwid_model_name: string;
  meta_title: string;
  meta_refresh_hours: number;
  meta_support_url: string;
  meta_web_page_url: string;
  meta_announce: string;
}

export interface SourceWriteInput {
  name: string;
  category: string;
  key_category: string;
  key_insert_mode: "top" | "bottom";
  source_url: string;
  enabled: boolean;
  apply_remote_metadata: boolean;
  pass_hwid: boolean;
  hwid_version: string;
  hwid_model_name: string;
  hwid_value?: string;
  clear_hwid_value?: boolean;
  raw_body?: string;
  raw_content_type?: string;
  raw_final_url?: string;
  selected_item_refs?: string[];
}

export interface ExternalSourceCategory {
  name: string;
  sources_count: number;
}

export interface KeyCategory {
  id?: number;
  name: string;
  color: string;
  keys_count: number;
}

export interface ExternalSourcePreviewKey {
  item_ref?: string;
  line_index?: number;
  label: string;
  display_name?: string;
  protocol?: ExternalDetectedProtocol;
  scheme: string;
  host?: string;
  port?: string;
  compatibility?: ExternalProfileCompatibility | "unsupported";
  status?: ExternalImportItemStatus;
  warnings?: string[];
  error_code?: string;
  /** Deprecated safe endpoint summary; never a complete URI. */
  url_short: string;
}

export type ExternalProfileProtocol =
  | "legacy"
  | "vless"
  | "vmess"
  | "trojan"
  | "xray-json"
  | "shadowsocks"
  | "hysteria2"
  | "tuic";

export type ExternalDetectedProtocol = ExternalProfileProtocol | "hysteria" | "unknown" | string;

export type ExternalProfileCompatibility = "legacy" | "full" | "read_only";

export type ExternalImportItemStatus =
  | "accepted"
  | "added"
  | "rejected"
  | "duplicate"
  | "updated"
  | "unchanged"
  | "compatibility_only"
  | "ambiguous"
  | "unsupported";

export interface ExternalImportResultCounts {
  accepted: number;
  added?: number;
  rejected: number;
  duplicate: number;
  updated: number;
  unchanged: number;
  compatibility_only: number;
  ambiguous: number;
  unsupported: number;
  removed?: number;
}

export type RoutingDeliveryMode = "disabled" | "add" | "onadd";

export interface RoutingSettings {
  config_json: string;
  delivery_mode: RoutingDeliveryMode;
  add_url: string;
  onadd_url: string;
  off_url: string;
  message?: string;
}

export interface RoutingSettingsUpdate {
  config_json: string;
  delivery_mode: RoutingDeliveryMode;
}

export interface ExternalSourcePreview {
  source_url: string;
  suggested_name: string;
  detected_format: "links" | "xray-json";
  key_count: number;
  metadata: {
    title: string;
    refresh_hours: number;
    support_url: string;
    profile_web_page: string;
    announce: string;
    content_type: string;
    content_disp: string;
    http_status: string;
    final_url: string;
  };
  warnings: string[];
  result_counts?: ExternalImportResultCounts;
  keys: ExternalSourcePreviewKey[];
}

export interface Admin {
  id: number;
  username: string;
  role: "owner" | "operator" | "viewer";
  created_at: string;
}

export interface PageMeta {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export type EffectiveUserStatus = "active" | "expired" | "paused" | "blocked" | "limited";

export interface UserSummary {
  id: number;
  name: string;
  email: string;
  status: "active" | "paused" | "blocked";
  effective_status: EffectiveUserStatus;
  starts_at: string;
  expires_at: string;
  max_devices: number;
  connected_device_count: number;
  created_at: string;
}

export interface KeySummary {
  id: number;
  label: string;
  client_display_name: string;
  category_id?: number;
  category: string;
  kind: "real" | "informational";
  template_text?: string;
  status: "active" | "non-active";
  check_status: "up" | "down" | "unknown" | "unsupported_check";
  check_error: string;
  last_latency_ms: number;
  last_checked_at: string;
  external_source_id: number;
  external_source_name: string;
  protocol?: ExternalProfileProtocol;
  profile_schema_version?: number;
  profile_compatibility?: ExternalProfileCompatibility;
  profile_warnings?: string[];
  created_at: string;
}

export interface AuditEvent {
  id: number;
  actor: string;
  action: string;
  target_type: string;
  target_id: string;
  metadata: Record<string, unknown>;
  request_id: string;
  created_at: string;
}

export interface DashboardData {
  users: Record<"total" | EffectiveUserStatus, number>;
  keys: {
    total: number;
    up: number;
    down: number;
    unknown: number;
  };
  sources: {
    total: number;
    errors: number;
  };
  devices: number;
  backup: {
    enabled: boolean;
    status: "disabled" | "pending" | "healthy" | "error";
    file: string;
    last_modified: string;
    size_bytes: number;
  };
  recent_audit_events: AuditEvent[];
  degraded_sections: Array<"sources" | "devices" | "audit">;
}

export type SubscriptionTemplateFormat = "base64" | "plain" | "xray-json" | "mihomo" | "sing-box";

export interface SubscriptionTemplate {
  id: number;
  slug: string;
  name: string;
  format: SubscriptionTemplateFormat;
  content: string;
  enabled: boolean;
  is_system: boolean;
  created_at: string;
  updated_at: string;
}

export interface ResponseRuleCondition {
  headerName: string;
  operator:
    | "EQUALS"
    | "NOT_EQUALS"
    | "CONTAINS"
    | "NOT_CONTAINS"
    | "STARTS_WITH"
    | "NOT_STARTS_WITH"
    | "ENDS_WITH"
    | "NOT_ENDS_WITH"
    | "REGEX"
    | "NOT_REGEX";
  value: string;
  caseSensitive: boolean;
}

export interface ResponseRule {
  id: number;
  name: string;
  description: string;
  enabled: boolean;
  priority: number;
  operator: "AND" | "OR";
  conditions: ResponseRuleCondition[];
  response_type:
    | "browser"
    | "base64"
    | "plain"
    | "xray-json"
    | "mihomo"
    | "sing-box"
    | "block"
    | "not-found";
  template_id: number | null;
  headers: Array<{ key: string; value: string }>;
  is_system: boolean;
  created_at: string;
  updated_at: string;
}

export interface APIToken {
  id: number;
  name: string;
  prefix: string;
  scopes: string[];
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
  revoked_at: string | null;
}

export interface SourceSyncRun {
  id: number;
  source_id: number;
  status: "running" | "succeeded" | "failed";
  imported_count: number;
  skipped_count: number;
  result_counts?: ExternalImportResultCounts;
  error_message: string;
  started_at: string;
  finished_at: string | null;
}

export interface SubscriptionDeliverySettingsUpdate {
  response_headers: Array<{ key: string; value: string }>;
  remarks: Record<"expired" | "paused" | "blocked" | "limited" | "empty", string[]>;
}

export interface SubscriptionDeliverySettings extends SubscriptionDeliverySettingsUpdate {
  readonly capabilities: ProtocolCapability[];
  readonly generation_exclusion_reason_codes: string[];
}

export type CapabilitySupport =
  | "supported"
  | "unsupported"
  | "conditionally_supported"
  | "compatibility_only";

export interface OutputCapability {
  status: CapabilitySupport;
  reason_code?: string;
  target_version?: string;
  minimum_version?: string;
  syntax_validation?: "structurally_generated" | "official_binary" | "address_probe_only";
  runtime_interoperability?: "not_tested";
}

export interface ProtocolCapability {
  protocol: "vless" | "vmess" | "trojan" | "shadowsocks" | "hysteria2" | "tuic";
  generation: string;
  outputs: Record<string, OutputCapability>;
}

export interface SubscriptionGenerationFailure {
  error_code: "all_profiles_excluded";
  output_format: "mihomo" | "sing-box" | "xray-json";
  eligible_count: number;
  excluded_count: number;
  exclusion_counts: Record<string, number>;
}

export interface BackgroundJob {
  id: number;
  kind: string;
  status: "queued" | "running" | "succeeded" | "succeeded_with_warnings" | "failed";
  target_type: string;
  target_id: string;
  error_message: string;
  result_counts?: {
    total_selected: number;
    checked: number;
    healthy: number;
    unhealthy: number;
    check_failed: number;
    skipped_disabled: number;
    skipped_unsupported: number;
    persisted_ok: number;
    persist_failed: number;
  };
  run_after: string | null;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
}

// Stage 8 Types
export type KeyOwnership = "local" | "external_source";

export type TriStatePatch<T> =
  | { operation: "set"; value: T }
  | { operation: "clear" };

export interface SafeShadowsocksDetail {
  method: string;
  password_present: boolean;
  plugin_name?: string;
  plugin_options_present: boolean;
  user_info_style: string;
}

export interface SafeHysteria2Detail {
  authentication_present: boolean;
  sni: string;
  insecure: boolean;
  certificate_sha256: string;
  obfuscation_type: string;
  obfuscation_password_present: boolean;
}

export interface TUICFieldObservationDTO {
  field: string;
  field_class: string;
  provenance: string;
}

export interface SafeTUICDetail {
  generation: 4 | 5;
  uuid_present: boolean;
  password_present: boolean;
  token_present: boolean;
  sni: string;
  alpn: string[];
  skip_cert_verify: boolean;
  disable_sni: boolean;
  congestion_controller: string;
  udp_relay_mode: string;
  udp_over_stream: boolean;
  zero_rtt: boolean;
  heartbeat: string;
  field_observations: TUICFieldObservationDTO[];
}

export interface SafeXrayJSONDetail {
  has_raw_json: boolean;
  network: string;
  security: string;
}

export interface SafeStructuredProfile {
  server: string;
  port: string;
  port_kind: "single" | "expression";
  display_name: string;
  shadowsocks?: SafeShadowsocksDetail;
  hysteria2?: SafeHysteria2Detail;
  tuic?: SafeTUICDetail;
  xray_json?: SafeXrayJSONDetail;
}

export interface UnknownQueryParamDTO {
  key: string;
  has_value: boolean;
}

export interface SanitizedCheckError {
  code: string;
  message: string;
}

export interface KeyProfileDetailResponse {
  id: number;
  label: string;
  client_display_name: string;
  client_display_name_overridden: boolean;
  category_id: number | null;
  category: string;
  kind: "real" | "informational";
  status: "active" | "non-active";
  check_status: "up" | "down" | "unknown" | "unsupported_check";
  check_error: SanitizedCheckError;
  last_latency_ms: number;
  last_checked_at: string;
  template_text: string;
  ownership: KeyOwnership;
  external_source_id: number | null;
  external_source_name: string;
  protocol: ExternalProfileProtocol;
  profile_schema_version: number;
  profile_compatibility: ExternalProfileCompatibility;
  profile_warnings: string[];
  profile_revision: number;
  created_at: string;
  updated_at: string;
  safe_structured?: SafeStructuredProfile;
  unknown_query_parameters: UnknownQueryParamDTO[];
  capabilities: Record<string, OutputCapability>;
}

export interface KeyRawSecretResponse {
  key_id: number;
  confirmed_profile_revision: number;
  target: "raw";
  raw_uri: string;
}

export interface StructuredSecretsMap {
  password?: string;
  authentication?: string;
  uuid?: string;
  token?: string;
  obfuscation_password?: string;
  plugin_options?: string;
  raw_json?: string;
}

export interface KeyStructuredSecretsResponse {
  key_id: number;
  confirmed_profile_revision: number;
  target: "structured-secrets";
  secrets: StructuredSecretsMap;
}

export interface ShadowsocksStructuredPatch {
  method?: TriStatePatch<string>;
  password?: TriStatePatch<string>;
  plugin_name?: TriStatePatch<string>;
  plugin_options?: TriStatePatch<string>;
}

export interface Hysteria2StructuredPatch {
  authentication?: TriStatePatch<string>;
  sni?: TriStatePatch<string>;
  insecure?: TriStatePatch<boolean>;
  certificate_sha256?: TriStatePatch<string>;
  obfuscation_type?: TriStatePatch<string>;
  obfuscation_password?: TriStatePatch<string>;
}

export interface TUICStructuredPatch {
  uuid?: TriStatePatch<string>;
  password?: TriStatePatch<string>;
  sni?: TriStatePatch<string>;
  alpn?: TriStatePatch<string[]>;
  skip_cert_verify?: TriStatePatch<boolean>;
  congestion_controller?: TriStatePatch<string>;
  udp_relay_mode?: TriStatePatch<string>;
  udp_over_stream?: TriStatePatch<boolean>;
  zero_rtt?: TriStatePatch<boolean>;
  heartbeat?: TriStatePatch<string>;
}

export interface StructuredProfilePatch {
  server?: TriStatePatch<string>;
  port?: TriStatePatch<string>;
  display_name?: TriStatePatch<string>;
  shadowsocks?: ShadowsocksStructuredPatch;
  hysteria2?: Hysteria2StructuredPatch;
  tuic?: TUICStructuredPatch;
}

export interface UpdateKeyProfileInput {
  label: string;
  client_display_name?: string;
  category_id?: number | null;
  category?: string;
  status: "active" | "non-active";
  kind: "real" | "informational";
  template_text?: string;
  profile_revision: number;
  patch_mode: "raw" | "structured";
  raw_uri?: string;
  structured_patch?: StructuredProfilePatch;
}

export interface CreateKeyProfileInput {
  label: string;
  client_display_name?: string;
  category_id?: number | null;
  category?: string;
  status: "active" | "non-active";
  kind: "real" | "informational";
  template_text?: string;
  creation_mode: "raw" | "structured";
  raw_uri?: string;
  protocol?: ExternalProfileProtocol;
  structured?: StructuredProfilePatch;
}

export interface DeclarativeWarningRule {
  field: string;
  operator: "equals" | "not_equals" | "is_true" | "is_false" | "in";
  value?: unknown;
  warning_code: string;
  warning_message: string;
}

export interface ProtocolEditorFieldSchema {
  field_type: "text" | "password" | "select" | "boolean" | "string_list" | "port_expression";
  required: boolean;
  can_clear: boolean;
  options?: Array<{ value: string; label: string }>;
  default_value?: unknown;
  provenance?: string;
  warning_rules?: DeclarativeWarningRule[];
}

export interface ProtocolSchemaDTO {
  protocol: ExternalProfileProtocol;
  label: string;
  supported_creations: ("raw" | "structured")[];
  fields: Record<string, ProtocolEditorFieldSchema>;
}

export interface KeyEditorSchemaResponse {
  protocols: ProtocolSchemaDTO[];
  exclusion_reason_codes: Record<string, string>;
}
