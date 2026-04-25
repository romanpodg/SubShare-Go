import type { User, VLESSKey, KeyCheckResult, SubscriptionSettings } from "./types";

let csrfToken: string | null = null;

export function setCsrfToken(token: string) {
  csrfToken = token;
}

export function getCsrfToken(): string | null {
  return csrfToken;
}

class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};

  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (csrfToken && method !== "GET") {
    headers["X-CSRF-Token"] = csrfToken;
  }

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, data.error || res.statusText);
  }

  return res.json();
}

// Auth
export const auth = {
  login: (username: string, password: string) =>
    request<{ csrf_token: string }>("POST", "/api/auth/login", { username, password }),
  logout: () => request<{ message: string }>("POST", "/api/auth/logout"),
  me: () => request<{ authenticated: boolean; csrf_token: string }>("GET", "/api/auth/me"),
};

// Users
export const users = {
  list: () => request<{ users: User[] }>("GET", "/api/admin/users"),
  create: (data: {
    name: string;
    email: string;
    activation_code: string;
    status: string;
    issue_days: number;
    blocked_reason?: string;
  }) => request<{ message: string }>("POST", "/api/admin/users", data),
  delete: (id: number) => request<{ message: string }>("DELETE", `/api/admin/users/${id}`),
  updateKeys: (id: number, key_ids: number[]) =>
    request<{ message: string }>("PUT", `/api/admin/users/${id}/keys`, { key_ids }),
  updateSubscription: (
    id: number,
    data: {
      status: string;
      starts_at: string;
      expires_at: string;
      blocked_reason?: string;
      subscription_name?: string;
      subscription_refresh_hours?: number;
      subscription_info_url?: string;
      subscription_extra_url?: string;
      subscription_extra_status?: string;
    }
  ) => request<{ message: string }>("PUT", `/api/admin/users/${id}/subscription`, data),
  getSubscriptionURLs: (id: number) =>
    request<{ plain_url: string; encrypted_url: string }>("GET", `/api/admin/users/${id}/subscription-urls`),
  updateSettings: (
    id: number,
    data: {
      time_zone: string;
      language: string;
    }
  ) => request<{ message: string }>("PUT", `/api/admin/users/${id}/settings`, data),
  updateHwid: (id: number, max_devices: number) =>
    request<{ message: string }>("PUT", `/api/admin/users/${id}/hwid`, { max_devices }),
  deleteHwid: (id: number, hwid: string) =>
    request<{ message: string }>(
      "DELETE",
      `/api/admin/users/${id}/hwid/${encodeURIComponent(hwid)}`
    ),
};

// Keys
export const keys = {
  list: () => request<{ keys: VLESSKey[] }>("GET", "/api/admin/keys"),
  create: (data: { label: string; url?: string; status: string; kind: string; template_text?: string }) =>
    request<{ message: string }>("POST", "/api/admin/keys", data),
  update: (
    id: number,
    data: {
      label: string;
      status: string;
      raw_url?: string;
      uuid: string;
      host: string;
      port: string;
      query: string;
      fragment: string;
      kind: string;
      template_text?: string;
    }
  ) => request<{ message: string }>("PUT", `/api/admin/keys/${id}`, data),
  delete: (id: number) => request<{ message: string }>("DELETE", `/api/admin/keys/${id}`),
  check: (id: number) => request<KeyCheckResult>("POST", `/api/admin/keys/${id}/check`),
  checkAll: () =>
    request<{ checked: number; keys: KeyCheckResult[] }>("POST", "/api/admin/keys/check-all"),
  reorder: (ids: number[]) =>
    request<{ message: string }>("PUT", "/api/admin/keys/order", { ids }),
};

// Subscription
export const subscription = {
  activate: (activation_code: string) =>
    request<{ subscription_url: string; message: string }>(
      "POST",
      "/api/subscription/activate",
      { activation_code }
    ),
};

export const subscriptionSettings = {
  get: () => request<SubscriptionSettings>("GET", "/api/admin/subscription-settings"),
  update: (data: SubscriptionSettings) =>
    request<{ message: string }>("PUT", "/api/admin/subscription-settings", data),
};

export const routingSettings = {
  get: () => request<{ config_json: string }>("GET", "/api/admin/routing-settings"),
  update: (config_json: string) =>
    request<{ message: string }>("PUT", "/api/admin/routing-settings", { config_json }),
};
