import type { Page } from "@playwright/test";
import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow } from "./smoke-assertions";

test("owner can preview and create a source in the responsive drawer", async ({ page }) => {
  const { runtimeErrors } = await mountSourceDrawer({ page });
  await checkSourcePreview({ page, runtimeErrors });
});

async function mountSourceDrawer({ page }: { page: Page; }) {
  const runtimeErrors: string[] = [];
  page.on("pageerror", (error) => runtimeErrors.push(error.message));
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
  return { runtimeErrors };
}

async function checkSourcePreview({ page, runtimeErrors }: { page: Page; runtimeErrors: string[]; }) {
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
  const toolbarControls = page.locator(".ui-toolbar").locator("input, select, button");
  await expect(toolbarControls).toHaveCount(5);
  const toolbarHeights = await toolbarControls.evaluateAll((controls) =>
    controls.map((control) => Math.round(control.getBoundingClientRect().height))
  );
  expect(new Set(toolbarHeights)).toEqual(new Set([44]));
  await expect(page.locator(".ui-data-table")).toBeVisible();
  await expectNoHorizontalOverflow(page);

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
}
