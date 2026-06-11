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
  starts_at: string;
  expires_at: string;
  blocked_reason: string;
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
  category: string;
  kind: "real" | "informational";
  template_text: string;
  url_short: string;
  status: "active" | "non-active";
  status_label: string;
  check_status: "up" | "down" | "unknown";
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

export interface ExternalSourceCategory {
  name: string;
  sources_count: number;
}

export interface KeyCategory {
  name: string;
  color: string;
  keys_count: number;
}

export interface ExternalSourcePreviewKey {
  label: string;
  scheme: string;
  url_short: string;
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
  keys: ExternalSourcePreviewKey[];
}

export interface Admin {
  id: number;
  username: string;
  role: "super_admin" | "support_admin";
  created_at: string;
}
