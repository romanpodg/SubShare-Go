import { expect, type Page } from "@playwright/test";

interface KeyListBrowserLabels { longKeyLabel: string; longSourceName: string; longClientDisplayName: string }

function keyListBrowserLabels(): KeyListBrowserLabels {
const longKeyLabel = "Очень длинное название ключа 👇 без гарантии, с дополнительным описанием маршрута и региона подключения";
const longSourceName = "EXAMPLE VPN | @examplebot — резервный внешний источник с очень длинным служебным названием для проверки синхронизации импортированных конфигураций и резервных маршрутов";
const longClientDisplayName = "🔴 Обход блокировок без гарантии — резервный маршрут через Амстердам с дополнительным описанием подключения";
return { longKeyLabel, longSourceName, longClientDisplayName };
}

async function mountKeyListCommon(page: Page) {
await page.route("**/api/auth/me", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ username: "owner", role: "owner", csrf_token: "test" }),
    })
  );
await page.route("**/api/v1/panel-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ app_name: "SubShare", logo_url: "", theme: "dark" }),
    })
  );
await page.route("**/api/v1/build-info", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ version: "test", commit: "abc", build_time: "now", schema_version: 5 }),
    })
  );
}

async function mountKeyListMetadata(page: Page) {
await page.route("**/api/v1/subscription-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        title: "SubShare",
        refresh_hours: 24,
        info_url: "",
        extra_url: "",
        extra_status: "down",
        subscription_format: "links",
        time_zone: "UTC",
        language: "ru",
        provider_id: "subshare",
        happ_no_limit_mode: false,
        happ_no_limit_mode_xhttp_only: false,
        happ_mandatory_hwid: false,
        happ_notify_expiration: false,
        happ_hide_server_settings: false,
        happ_subscription_body: "",
      }),
    })
  );
await page.route("**/api/v1/key-categories", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          { id: 1, name: "EXAMPLE VPN", color: "#22b8cf", keys_count: 1 },
          { id: 2, name: "Резерв", color: "#f59e0b", keys_count: 0 },
          { id: 3, name: "Авто", color: "#8b5cf6", keys_count: 0 },
          { id: 4, name: "Игровой", color: "#22c55e", keys_count: 0 },
          { id: 5, name: "Хистерия", color: "#eab308", keys_count: 0 },
          {
            id: 6,
            name: "Очень длинное название дополнительной категории",
            color: "#06b6d4",
            keys_count: 0,
          },
        ],
      }),
    })
  );
}

async function mountKeyListSummaries(page: Page, labels: KeyListBrowserLabels) {
const { longKeyLabel, longSourceName, longClientDisplayName } = labels;
const handleKeysRoute = (route: import("@playwright/test").Route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          {
            id: 11,
            label: longClientDisplayName,
            client_display_name: longClientDisplayName,
            category: "EXAMPLE VPN",
            kind: "real",
            template_text: "",
            status: "non-active",
            check_status: "up",
            check_error: "",
            last_latency_ms: 38,
            last_checked_at: "2026-07-29T10:00:00Z",
            external_source_id: 7,
            external_source_name: longSourceName,
            protocol: "vless",
            created_at: "2026-07-29T10:00:00Z",
          },
          {
            id: 12,
            label: longKeyLabel,
            client_display_name: "",
            category: "",
            kind: "informational",
            template_text: "Service window: 03:00 UTC",
            status: "active",
            check_status: "unknown",
            check_error: "",
            last_latency_ms: 0,
            last_checked_at: "",
            external_source_id: 7,
            external_source_name: longSourceName,
            created_at: "2026-07-29T10:00:00Z",
          },
        ],
        meta: { page: 1, page_size: 50, total: 2, total_pages: 1 },
      }),
    });
await page.route("**/api/v1/keys?*", handleKeysRoute);
await page.route("**/api/v1/keys", handleKeysRoute);
}

async function mountKeyListDetails(page: Page, labels: KeyListBrowserLabels) {
const { longSourceName, longClientDisplayName } = labels;
await page.route("**/api/v1/keys/11", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          id: 11,
          label: longClientDisplayName,
          client_display_name: longClientDisplayName,
          client_display_name_overridden: true,
          category_id: 1,
          category: "EXAMPLE VPN",
          kind: "real",
          status: "non-active",
          check_status: "up",
          check_error: { code: "", message: "" },
          last_latency_ms: 38,
          last_checked_at: "2026-07-29T10:00:00Z",
          template_text: "",
          ownership: "external_source",
          external_source_id: 7,
          external_source_name: longSourceName,
          protocol: "vless",
          profile_schema_version: 1,
          profile_compatibility: "supported",
          profile_warnings: [],
          profile_revision: 1,
          created_at: "2026-07-29T10:00:00Z",
          updated_at: "2026-07-29T10:00:00Z",
          unknown_query_parameters: [],
          capabilities: {},
        },
      }),
    })
  );
}

async function mountKeyListOperations(page: Page) {
await page.route("**/api/v1/keys/check-all", (route) =>
    route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ job_id: 42, status: "queued" }),
    })
  );
await page.route("**/api/v1/key-editor-schema", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ data: { protocols: [], exclusion_reason_codes: {} } }),
    })
  );
}

export async function openKeysPage(page: Page) {
await page.setViewportSize({ width: 1440, height: 900 });
const labels = keyListBrowserLabels();
await mountKeyListCommon(page);
await mountKeyListMetadata(page);
await mountKeyListSummaries(page, labels);
await mountKeyListOperations(page);
await mountKeyListDetails(page, labels);
await page.goto("/admin/keys");
await expect(page.locator(".ui-key-card")).toHaveCount(2);
return labels;
}
