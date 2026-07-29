import { expect, test } from "@playwright/test";

test("login page is usable at desktop and mobile widths", async ({ page }) => {
  await page.route("**/api/panel-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ app_name: "SubShare", logo_url: "", theme: "dark" }),
    })
  );
  await page.goto("/admin/login");
  await expect(page.getByRole("heading", { name: /SubShare|Вход/i })).toBeVisible();
  await expect(page.getByLabel(/логин|username/i)).toBeVisible();
  await expect(page.getByLabel(/пароль|password/i)).toBeVisible();
});

test("authenticated overview renders dashboard data without runtime errors", async ({ page }) => {
  const runtimeErrors: string[] = [];
  page.on("pageerror", (error) => runtimeErrors.push(error.message));
  await page.route("**/api/auth/me", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ username: "owner", role: "owner", csrf_token: "test" }),
    })
  );
  await page.route("**/api/v1/build-info", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ version: "test", commit: "abc", build_time: "now", schema_version: 5 }),
    })
  );
  await page.route("**/api/v1/dashboard", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        users: { total: 2, active: 1, expired: 1, paused: 0, blocked: 0, limited: 0 },
        keys: { total: 3, up: 2, down: 1, unknown: 0 },
        sources: { total: 1, errors: 0 },
        devices: 1,
        backup: { enabled: true, status: "healthy", file: "backup.db", last_modified: "2026-07-28T00:00:00Z", size_bytes: 100 },
        recent_audit_events: [],
        degraded_sections: [],
      }),
    })
  );
  await page.route("**/api/v1/jobs**", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ data: [], meta: { page: 1, page_size: 6, total: 0, total_pages: 0 } }),
    })
  );
  await page.goto("/admin/overview");
  await expect(page.getByText("2/3")).toBeVisible();
  await expect(page.getByText("готова")).toBeVisible();
  expect(runtimeErrors).toEqual([]);
});

test("owner can preview and create a source in the responsive drawer", async ({ page }) => {
  const runtimeErrors: string[] = [];
  page.on("pageerror", (error) => runtimeErrors.push(error.message));
  await page.route("**/api/auth/me", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ username: "owner", role: "owner", csrf_token: "test" }),
    })
  );
  await page.route("**/api/panel-settings", (route) =>
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
  await page.route("**/api/v1/source-categories", (route) =>
    route.fulfill({ contentType: "application/json", body: JSON.stringify({ data: [] }) })
  );
  await page.route("**/api/v1/key-categories", (route) =>
    route.fulfill({ contentType: "application/json", body: JSON.stringify({ data: [] }) })
  );
  await page.route("**/api/v1/sources?**", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [],
        meta: { page: 1, page_size: 20, total: 0, total_pages: 0 },
      }),
    })
  );
  await page.route("**/api/v1/sources/preview", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        source_url: "https://provider.example/sub",
        suggested_name: "Provider",
        detected_format: "links",
        key_count: 1,
        metadata: {},
        warnings: [],
        keys: [{ label: "Edge", scheme: "vless", url_short: "vless://•••" }],
      }),
    })
  );
  await page.route("**/api/v1/sources", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        data: { id: 1, name: "Provider" },
        imported_count: 1,
        skipped_count: 0,
        warnings: [],
        detected_format: "links",
      }),
    });
  });

  await page.goto("/admin/sources");
  await page.getByRole("button", { name: "Добавить источник" }).click();
  const drawer = page.getByRole("dialog");
  await expect(drawer).toBeVisible();
  const box = await drawer.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(page.viewportSize()?.width ?? 1920);

  await page.getByLabel("URL подписки").fill("https://provider.example/sub");
  await page.getByRole("button", { name: "Проверить источник" }).click();
  await expect(page.getByLabel("Название")).toHaveValue("Provider");
  await page.getByRole("button", { name: "Проверить параметры" }).click();
  await expect(page.getByText("Edge")).toBeVisible();
  await page.getByRole("button", { name: "Добавить источник" }).last().click();
  await expect(drawer).toBeHidden();
  expect(runtimeErrors).toEqual([]);
});
