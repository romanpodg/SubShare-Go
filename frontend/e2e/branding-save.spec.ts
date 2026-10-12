import { expect, test, type Page } from "@playwright/test";

const wire = { panelTitle: "Server panel", logoDataUrl: "", faviconDataUrl: "", pageTitleAdmin: "Server admin", pageTitleAdminLogin: "Server login", pageTitleSubscription: "Server subscription" };
async function openBranding(page: Page, put: (body: typeof wire) => Promise<{ status: number; body: unknown }>) {
  await page.route("**/api/auth/me", route => route.fulfill({ contentType: "application/json", body: JSON.stringify({ authenticated: true, role: "owner", csrf_token: "fixture-csrf" }) }));
  await page.route("**/api/v1/panel-settings", async route => {
    if (route.request().method() === "GET") { await route.fulfill({ contentType: "application/json", body: JSON.stringify(wire) }); return; }
    expect(route.request().headers()["x-csrf-token"]).toBe("fixture-csrf");
    const result = await put(route.request().postDataJSON());
    await route.fulfill({ status: result.status, contentType: "application/json", body: JSON.stringify(result.body) });
  });
  await page.goto("/admin/settings/branding");
  await page.getByRole("button", { name: "Настроить панель" }).click();
  await expect(page.getByLabel("Заголовок панели")).toHaveValue("Server panel");
}

test("branding denial stays visible and retry reopens from acknowledged data", async ({ page }) => {
  let attempts = 0;
  await openBranding(page, async body => {
    attempts += 1;
    return attempts === 1 ? { status: 403, body: { code: "save_failed", message: "fixture denied" } } : { status: 200, body };
  });
  const dialog = page.getByRole("dialog", { name: "Настройки панели" });
  await page.getByLabel("Заголовок панели").fill("Saved panel");
  await dialog.getByRole("button", { name: "Сохранить" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("fixture denied");
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Сохранить" }).click();
  await expect(dialog).toBeHidden();
  await page.getByRole("button", { name: "Настроить панель" }).click();
  await expect(page.getByLabel("Заголовок панели")).toHaveValue("Saved panel");
  expect(attempts).toBe(2);
});

test("pending branding save guards fields and closing until acknowledged", async ({ page }) => {
  let release!: () => void;
  const pending = new Promise<void>(resolve => { release = resolve; });
  let attempts = 0;
  await openBranding(page, async body => { attempts += 1; await pending; return { status: 200, body }; });
  const dialog = page.getByRole("dialog", { name: "Настройки панели" });
  await page.getByLabel("Заголовок панели").fill("Acknowledged panel");
  await dialog.getByRole("button", { name: "Сохранить" }).click();
  await expect(dialog.getByRole("button", { name: "Сохранить" })).toBeDisabled();
  await expect(page.getByLabel("Заголовок панели")).toBeDisabled();
  await dialog.getByRole("button", { name: "Закрыть" }).click();
  await expect(dialog).toBeVisible();
  expect(attempts).toBe(1);
  release();
  await expect(dialog).toBeHidden();
  await page.getByRole("button", { name: "Настроить панель" }).click();
  await expect(page.getByLabel("Заголовок панели")).toHaveValue("Acknowledged panel");
});
