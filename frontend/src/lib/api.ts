import type {
  User,
  VLESSKey,
  KeyCategory,
  KeyCheckResult,
  SubscriptionSettings,
  ExternalSourceCategory,
  ExternalSourcePreview,
  Admin,
  AuditEvent,
  DashboardData,
  KeySummary,
  PageMeta,
  ResponseRule,
  SubscriptionTemplate,
  SubscriptionTemplateFormat,
  UserSummary,
  APIToken,
  SourceSyncRun,
  SubscriptionDeliverySettings,
  SubscriptionDeliverySettingsUpdate,
  BackgroundJob,
  SourceDetail,
  SourceSummary,
  SourceWriteInput,
  ExternalImportResultCounts,
  ExternalSourcePreviewKey,
} from "./types";

let csrfToken: string | null = null;
let unauthorizedHandler: (() => void) | null = null;

export function setCsrfToken(token: string | null) {
  csrfToken = token;
}

export function getCsrfToken(): string | null {
  return csrfToken;
}

export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler;
}

export class ApiError extends Error {
  status: number;
  code: string;
  fieldErrors: Record<string, string[]>;
  requestId: string;
  field_errors: Record<string, string[]>;
  request_id: string;

  constructor(
    status: number,
    code: string,
    message: string,
    fieldErrors: Record<string, string[]> = {},
    requestId = ""
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.fieldErrors = fieldErrors;
    this.requestId = requestId;
    this.field_errors = fieldErrors;
    this.request_id = requestId;
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

  const responseText = await res.text();
  if (!res.ok) {
    let data: {
      error?: string;
      code?: string;
      message?: string;
      field_errors?: Record<string, string[]>;
      request_id?: string;
    } = {};
    if (responseText) {
      try {
        data = JSON.parse(responseText) as typeof data;
      } catch {
        data = {};
      }
    }
    if (res.status === 401) {
      csrfToken = null;
      unauthorizedHandler?.();
    }
    throw new ApiError(
      res.status,
      data.code || `http_${res.status}`,
      data.message || data.error || res.statusText || "Request failed",
      data.field_errors || {},
      data.request_id || res.headers.get("X-Request-ID") || ""
    );
  }

  if (res.status === 204 || responseText === "") {
    return undefined as T;
  }
  try {
    return JSON.parse(responseText) as T;
  } catch {
    throw new ApiError(
      res.status,
      "invalid_response",
      "Server returned an invalid JSON response",
      {},
      res.headers.get("X-Request-ID") || ""
    );
  }
}

// Auth
export const auth = {
  login: (username: string, password: string) =>
    request<{ csrf_token: string }>("POST", "/api/auth/login", { username, password }),
  logout: () => request<{ message: string }>("POST", "/api/auth/logout"),
  me: () => request<{ authenticated: boolean; csrf_token: string; role?: "owner" | "operator" | "viewer" }>("GET", "/api/auth/me"),
};

// Admins
export const admins = {
  list: () => request<{ admins: Admin[] }>("GET", "/api/v1/admins"),
  create: (data: { username: string; password: string; role: string }) =>
    request<{ message: string }>("POST", "/api/v1/admins", data),
  update: (id: number, data: { password?: string; role?: string }) =>
    request<{ message: string }>("PUT", `/api/v1/admins/${id}`, data),
  delete: (id: number) =>
    request<{ message: string }>("DELETE", `/api/v1/admins/${id}`),
};

// Users
export const users = {
  list: () => request<{ users: User[] }>("GET", "/api/v1/users/full"),
  create: (data: {
    name: string;
    email: string;
    activation_code: string;
    status: string;
    issue_days: number;
    blocked_reason?: string;
  }) => request<{ message: string }>("POST", "/api/v1/users", data),
  delete: (id: number) => request<{ message: string }>("DELETE", `/api/v1/users/${id}`),
  updateKeys: (id: number, key_ids: number[]) =>
    request<{ message: string }>("PUT", `/api/v1/users/${id}/key-assignment`, { mode: "selected", key_ids }),
  updateKeyAssignment: (id: number, mode: "all" | "selected", key_ids: number[] = []) =>
    request<{ message: string }>("PUT", `/api/v1/users/${id}/key-assignment`, { mode, key_ids }),
  updateSubscription: (
    id: number,
    data: {
      status?: string | null;
      starts_at?: string | null;
      expires_at?: string | null;
      blocked_reason?: string | null;
      subscription_name?: string | null;
      subscription_refresh_hours?: number | null;
      subscription_info_url?: string | null;
      subscription_extra_url?: string | null;
      subscription_extra_status?: string | null;
    }
  ) => request<{ message: string }>("PATCH", `/api/v1/users/${id}/subscription`, data),
  getSubscriptionURLs: (id: number) =>
    request<{ plain_url: string; encrypted_url: string }>("GET", `/api/v1/users/${id}/subscription-urls`),
  updateSettings: (
    id: number,
    data: {
      time_zone: string;
      language: string;
    }
  ) => request<{ message: string }>("PUT", `/api/v1/users/${id}/settings`, data),
  updateHwid: (id: number, max_devices: number) =>
    request<{ message: string }>("PUT", `/api/v1/users/${id}/hwid`, { max_devices }),
  deleteHwid: (id: number, hwid: string) =>
    request<{ message: string }>(
      "DELETE",
      `/api/v1/users/${id}/hwid/${encodeURIComponent(hwid)}`
    ),
};

// Keys
export const keys = {
  list: () => request<{ keys: VLESSKey[] }>("GET", "/api/v1/keys/full"),
  listCategories: () =>
    request<{ data: KeyCategory[] }>("GET", "/api/v1/key-categories")
      .then(({ data }) => ({ categories: data })),
  createCategory: (name: string, color?: string) =>
    request<{ category: KeyCategory; message: string }>("POST", "/api/v1/key-categories", { name, color }),
  updateCategory: (old_name: string, new_name: string, color: string) =>
    request<{ category: KeyCategory; message: string }>("PUT", "/api/v1/key-categories", { old_name, new_name, color }),
  reorderCategories: (names: string[]) =>
    request<{ message: string }>("PUT", "/api/v1/key-categories/order", { names }),
  renameCategory: (old_name: string, new_name: string) =>
    request<{ message: string }>("PUT", "/api/v1/key-categories/rename", { old_name, new_name }),
  deleteCategory: (name: string, mode: "delete_with_keys" | "keep_keys") =>
    request<{ message: string }>("POST", "/api/v1/key-categories/delete", { name, mode }),
  create: (data: { label: string; url?: string; status: string; kind: string; category?: string; template_text?: string }) =>
    request<{ message: string }>("POST", "/api/v1/keys", data),
  bulkUpdateStatus: (ids: number[], status: string, category?: string) =>
    request<{ message: string; updated: number }>("POST", "/api/v1/keys/bulk/status", { ids, status, category }),
  bulkDelete: (ids: number[]) =>
    request<{ message: string; deleted: number }>("POST", "/api/v1/keys/bulk/delete", { ids }),
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
      category?: string;
      template_text?: string;
    }
  ) => request<{ message: string }>("PUT", `/api/v1/keys/${id}`, data),
  delete: (id: number) => request<{ message: string }>("DELETE", `/api/v1/keys/${id}`),
  check: (id: number) => request<KeyCheckResult>("POST", `/api/v1/keys/${id}/check`),
  checkAll: () =>
    request<{ checked: number; keys: KeyCheckResult[] }>("POST", "/api/v1/keys/check-all"),
  reorder: (ids: number[]) =>
    request<{ message: string }>("PUT", "/api/v1/keys/order", { ids }),
};

// Subscription
export const subscription = {
  activate: (activation_code: string) =>
    request<{ subscription_url: string; message: string }>(
      "POST",
      "/api/v1/subscriptions/activate",
      { activation_code }
    ),
};

export const subscriptionSettings = {
  get: () => request<SubscriptionSettings>("GET", "/api/v1/subscription-settings"),
  update: (data: SubscriptionSettings) =>
    request<{ message: string }>("PUT", "/api/v1/subscription-settings", data),
};

export const routingSettings = {
  get: () => request<{ config_json: string }>("GET", "/api/v1/routing-settings"),
  update: (config_json: string) =>
    request<{ message: string }>("PUT", "/api/v1/routing-settings", { config_json }),
};

export const subscriptionPageConfig = {
  getPublic: () =>
    request<{ config_json: string; default_config_json: string }>(
      "GET",
      "/api/v1/subscription-page-config"
    ),
  getAdmin: () =>
    request<{ config_json: string; default_config_json: string }>(
      "GET",
      "/api/v1/subscription-page-config"
    ),
  update: (config_json: string) =>
    request<{ config_json: string }>(
      "PUT",
      "/api/v1/subscription-page-config",
      { config_json }
    ),
};

export const apiV1 = {
  dashboard: () => request<DashboardData>("GET", "/api/v1/dashboard"),
  buildInfo: () =>
    request<{ version: string; commit: string; build_time: string; schema_version: number }>(
      "GET",
      "/api/v1/build-info"
    ),
  users: (params: {
    query?: string;
    status?: string;
    page?: number;
    page_size?: number;
    sort?: string;
  } = {}) =>
    request<{ data: UserSummary[]; meta: PageMeta }>(
      "GET",
      `/api/v1/users?${new URLSearchParams(
        Object.entries(params)
          .filter(([, value]) => value !== undefined && value !== "")
          .map(([key, value]) => [key, String(value)])
      )}`
    ),
  user: (id: number) =>
    request<{ data: User }>("GET", `/api/v1/users/${id}`),
  keys: (params: {
    query?: string;
    status?: string;
    page?: number;
    page_size?: number;
  } = {}) =>
    request<{ data: KeySummary[]; meta: PageMeta }>(
      "GET",
      `/api/v1/keys?${new URLSearchParams(
        Object.entries(params)
          .filter(([, value]) => value !== undefined && value !== "")
          .map(([key, value]) => [key, String(value)])
      )}`
    ),
  sources: {
    list: (params: {
      query?: string;
      status?: string;
      page?: number;
      page_size?: number;
      sort?: string;
    } = {}) =>
      request<{ data: SourceSummary[]; meta: PageMeta }>(
        "GET",
        `/api/v1/sources?${new URLSearchParams(
          Object.entries(params)
            .filter(([, value]) => value !== undefined && value !== "")
            .map(([key, value]) => [key, String(value)])
        )}`
      ),
    get: (id: number) =>
      request<{ data: SourceDetail }>("GET", `/api/v1/sources/${id}`),
    preview: (data: {
      source_url: string;
      pass_hwid: boolean;
      hwid_version: string;
      hwid_model_name: string;
      hwid_value: string;
      raw_body?: string;
      raw_content_type?: string;
      raw_final_url?: string;
    }) => request<ExternalSourcePreview>("POST", "/api/v1/sources/preview", data),
    create: (data: SourceWriteInput) =>
      request<{
        data: SourceDetail;
        imported_count: number;
        skipped_count: number;
        warnings: string[];
        detected_format: "links" | "xray-json";
        result_counts: ExternalImportResultCounts;
        items: ExternalSourcePreviewKey[];
      }>("POST", "/api/v1/sources", data),
    update: (id: number, data: SourceWriteInput) =>
      request<{ message: string }>("PUT", `/api/v1/sources/${id}`, data),
    delete: (id: number) =>
      request<{ message: string; deleted_keys: number }>(
        "DELETE",
        `/api/v1/sources/${id}`
      ),
    sync: (id: number) =>
      request<{ job_id: number; status: "queued" }>(
        "POST",
        `/api/v1/sources/${id}/sync`
      ),
    categories: () =>
      request<{ data: ExternalSourceCategory[] }>(
        "GET",
        "/api/v1/source-categories"
      ),
    keyCategories: () =>
      request<{ data: KeyCategory[] }>("GET", "/api/v1/key-categories"),
    syncRuns: (id: number) =>
      request<{ data: SourceSyncRun[]; source_id: string }>(
        "GET",
        `/api/v1/sources/${id}/sync-runs`
      ),
  },
  auditEvents: (page = 1, pageSize = 25) =>
    request<{ data: AuditEvent[]; meta: PageMeta }>(
      "GET",
      `/api/v1/audit-events?page=${page}&page_size=${pageSize}`
    ),
  templates: {
    list: () => request<{ data: SubscriptionTemplate[] }>("GET", "/api/v1/templates"),
    preview: (data: {
      name: string;
      format: SubscriptionTemplateFormat;
      content: string;
      enabled: boolean;
    }) => request<{ content: string; content_type: string }>("POST", "/api/v1/templates/preview", data),
    create: (data: {
      name: string;
      format: SubscriptionTemplateFormat;
      content: string;
      enabled: boolean;
    }) => request<{ id: number; slug: string }>("POST", "/api/v1/templates", data),
    update: (
      id: number,
      data: {
        name: string;
        format: SubscriptionTemplateFormat;
        content: string;
        enabled: boolean;
      }
    ) => request<{ message: string }>("PUT", `/api/v1/templates/${id}`, data),
    delete: (id: number) =>
      request<{ message: string }>("DELETE", `/api/v1/templates/${id}`),
  },
  responseRules: {
    list: () => request<{ data: ResponseRule[] }>("GET", "/api/v1/response-rules"),
    create: (data: Omit<ResponseRule, "id" | "is_system" | "created_at" | "updated_at">) =>
      request<{ id: number }>("POST", "/api/v1/response-rules", data),
    update: (
      id: number,
      data: Omit<ResponseRule, "id" | "is_system" | "created_at" | "updated_at">
    ) => request<{ message: string }>("PUT", `/api/v1/response-rules/${id}`, data),
    delete: (id: number) =>
      request<{ message: string }>("DELETE", `/api/v1/response-rules/${id}`),
  },
  apiTokens: {
    list: () => request<{ data: APIToken[] }>("GET", "/api/v1/api-tokens"),
    create: (data: { name: string; scopes: string[]; expires_at: string }) =>
      request<{
        id: number;
        name: string;
        prefix: string;
        scopes: string[];
        expires_at: string;
        token: string;
      }>("POST", "/api/v1/api-tokens", data),
    revoke: (id: number) =>
      request<{ message: string }>("DELETE", `/api/v1/api-tokens/${id}`),
  },
  sourceSyncRuns: (id: number) =>
    request<{ data: SourceSyncRun[]; source_id: string }>(
      "GET",
      `/api/v1/sources/${id}/sync-runs`
    ),
  deliverySettings: {
    get: () =>
      request<SubscriptionDeliverySettings>(
        "GET",
        "/api/v1/subscription-delivery-settings"
      ),
    update: (data: SubscriptionDeliverySettingsUpdate) =>
      request<{ message: string }>(
        "PUT",
        "/api/v1/subscription-delivery-settings",
        data
      ),
  },
  jobs: {
    list: (page = 1, pageSize = 10) =>
      request<{ data: BackgroundJob[]; meta: PageMeta }>(
        "GET",
        `/api/v1/jobs?page=${page}&page_size=${pageSize}`
      ),
    get: (id: number) =>
      request<{ data: BackgroundJob }>("GET", `/api/v1/jobs/${id}`),
    retry: (id: number) =>
      request<{ job_id: number; retry_of: number; status: "queued" }>(
        "POST",
        `/api/v1/jobs/${id}/retry`
      ),
  },
};
