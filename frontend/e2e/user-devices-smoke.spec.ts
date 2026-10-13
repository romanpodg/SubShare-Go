import type { Locator, Page } from "@playwright/test";
import { expect, test } from "@playwright/test";
import { expectUniformAccentBorder } from "./smoke-assertions";

test("user HWID list handles an empty device response", async ({ page }) => {
  const { runtimeErrors } = await mountDeviceList({ page });
  const { telegramInputChrome, addUserDialog } = await checkEmptyDeviceList({ page });
  await checkEmptyDeviceList2({ telegramInputChrome, addUserDialog, page, runtimeErrors });
});

async function mountDeviceList({ page }: { page: Page; }) {
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
  await page.route("**/api/v1/keys?*", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ data: [], meta: { page: 1, page_size: 50, total: 0, total_pages: 0 } }),
    })
  );
  await page.route("**/api/v1/keys", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ data: [], meta: { page: 1, page_size: 50, total: 0, total_pages: 0 } }),
    })
  );
  return { runtimeErrors };
}

async function checkEmptyDeviceList({ page }: { page: Page; }) {
  await page.route("**/api/v1/users**", (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/v1/users/1") {
      return route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            id: 1,
            name: "Alice",
            email: "alice",
            status: "active",
            effective_status: "active",
            starts_at: "",
            expires_at: "",
            max_devices: 2,
            connected_device_count: 0,
            connected_devices: null,
            connected_hwids: null,
            assigned_key_ids: "",
          },
        }),
      });
    }
    return route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [{
          id: 1,
          name: "Alice",
          email: "alice",
          status: "active",
          effective_status: "active",
          starts_at: "",
          expires_at: "",
          max_devices: 2,
          connected_device_count: 0,
          created_at: "2026-07-30T00:00:00Z",
        }],
        meta: { page: 1, page_size: 25, total: 1, total_pages: 1 },
      }),
    });
  });

  await page.goto("/admin/users");
  await page.getByRole("button", { name: "Добавить", exact: true }).click();
  const addUserDialog = page.getByRole("dialog", { name: "Добавить пользователя" });
  await expectUniformAccentBorder(addUserDialog.getByLabel("Имя", { exact: true }));
  const telegramInput = addUserDialog.getByLabel("Имя пользователя Telegram", { exact: true });
  await expectUniformAccentBorder(telegramInput, addUserDialog.locator(".ui-input-group"));
  const telegramInputChrome = await telegramInput.evaluate((element) => {
    const computed = getComputedStyle(element);
    return {
      borderWidth: computed.borderWidth,
      backgroundColor: computed.backgroundColor,
      boxShadow: computed.boxShadow,
    };
  });
  expect(telegramInputChrome.borderWidth).toBe("0px");
  expect(telegramInputChrome.backgroundColor).toBe("rgba(0, 0, 0, 0)");
  return { telegramInputChrome, addUserDialog };
}

async function checkEmptyDeviceList2({ telegramInputChrome, addUserDialog, page, runtimeErrors }: { telegramInputChrome: { borderWidth: string; backgroundColor: string; boxShadow: string; }; addUserDialog: Locator; page: Page; runtimeErrors: string[]; }) {
  expect(telegramInputChrome.boxShadow).toBe("none");
  await addUserDialog.getByRole("button", { name: "Закрыть" }).click();

  await page.getByRole("button", { name: "Открыть", exact: true }).click();
  await page.getByRole("tab", { name: "Устройства" }).click();

  await expect(page.getByText("Устройства ещё не подключались")).toBeVisible();
  await expect(page.getByRole("button", { name: "Управление HWID" })).toBeVisible();
  expect(runtimeErrors).toEqual([]);
}
