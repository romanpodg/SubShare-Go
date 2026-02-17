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
  time_zone: string;
  language: string;
}
