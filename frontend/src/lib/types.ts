export interface User {
  id: number;
  name: string;
  email: string;
  activation_code: string;
  subscription_id: string;
  activation_used_at: string;
  status: "active" | "paused" | "blocked";
  starts_at: string;
  expires_at: string;
  blocked_reason: string;
  assigned_key_ids: string;
  max_devices: number;
  connected_device_count: number;
  connected_hwids: string[];
  created_at: string;
}

export interface VLESSKey {
  id: number;
  label: string;
  url: string;
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
