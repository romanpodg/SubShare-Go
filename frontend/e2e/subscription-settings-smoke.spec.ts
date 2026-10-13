import type { Locator, Page } from "@playwright/test";
import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow } from "./smoke-assertions";

test("subscription settings section icons stay square and responsive", async ({ page }) => {
  const { runtimeErrors, readRoutingDeliveryMode } = await mountSubscriptionRoutingFixture(page);
  const { fullHDSettingsOverflow, settingsColumns, settingsDialog } = await checkSubscriptionControlsAndSettingsDialog({ page });
  const { routingDeliveryOptions, routingWorkspace, routingDialog, routingScrollArea, routingFooter } = await checkSettingsAndRoutingDesktopLayouts({ fullHDSettingsOverflow, page, settingsColumns, settingsDialog });
  await checkConstrainedRoutingWorkspace({ page, routingDeliveryOptions, routingWorkspace, routingDialog, routingScrollArea, routingFooter });
  await checkRoutingFooterAndApply({ routingDialog, page, routingScrollArea, routingFooter, readRoutingDeliveryMode, runtimeErrors });
});

async function checkSubscriptionControlsAndSettingsDialog({ page }: { page: Page; }) {
  await page.goto("/admin/settings/subscription");
  await expect(page.getByRole("heading", { name: "Настройки подписки" })).toBeVisible();
  const deliveryTabs = page.getByRole("tab");
  await expect(deliveryTabs).toHaveCount(2);
  await expect(deliveryTabs.first()).toHaveAttribute("aria-selected", "true");
  await expect(page.getByLabel("Имя заголовка 1")).toHaveValue("X-Edge");
  await expect(page.getByLabel("Значение заголовка 1")).toHaveValue("enabled");
  await expect(page.getByRole("button", { name: "Удалить заголовок 1" })).toBeVisible();
  await expect(page.getByText("Не передаётся", { exact: true }).first()).toBeVisible();
  const sectionIcons = page.locator(".ui-settings-section-icon");
  await expect(sectionIcons).toHaveCount(2);

  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const iconMetrics = await sectionIcons.evaluateAll((elements) =>
      elements.map((element) => {
        const frame = element.getBoundingClientRect();
        const icon = element.querySelector("svg")?.getBoundingClientRect();
        return {
          width: Math.round(frame.width),
          height: Math.round(frame.height),
          centerDeltaX: icon ? Math.abs((frame.left + frame.width / 2) - (icon.left + icon.width / 2)) : -1,
          centerDeltaY: icon ? Math.abs((frame.top + frame.height / 2) - (icon.top + icon.height / 2)) : -1,
        };
      })
    );
    expect(iconMetrics.map(({ width: iconWidth, height }) => [iconWidth, height])).toEqual([
      [44, 44],
      [44, 44],
    ]);
    expect(iconMetrics.every(({ centerDeltaX, centerDeltaY }) => centerDeltaX <= 1 && centerDeltaY <= 1)).toBe(true);
    await expectNoHorizontalOverflow(page);
  }

  await page.setViewportSize({ width: 1440, height: 900 });
  await deliveryTabs.filter({ hasText: "Ремарки" }).click();
  await expect(page.locator("textarea")).toHaveCount(0);
  await expect(page.locator('[data-testid^="remark-card-"]')).toHaveCount(5);
  await expect(page.getByLabel("Ремарка 1: Подписка истекла")).toHaveValue("Срок действия истёк");
  const remarkGrid = page.getByTestId("remark-grid");
  expect(await remarkGrid.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(2);
  await page.setViewportSize({ width: 768, height: 900 });
  expect(await remarkGrid.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(1);
  await expectNoHorizontalOverflow(page);

  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.getByRole("button", { name: "Изменить" }).click();
  const settingsDialog = page.getByRole("dialog", { name: "Настройки сервиса" });
  await expect(settingsDialog).toBeVisible();
  const settingsColumns = settingsDialog.locator(".ui-joined-grid").first();
  expect(await settingsColumns.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(2);
  const localization = settingsDialog.getByTestId("settings-localization");
  await expect(settingsDialog.getByTestId("settings-secondary-column")).toContainText("Локализация сервиса");
  await expect(localization.getByText("Часовой пояс", { exact: true })).toBeVisible();
  await expect(localization.getByText("Язык интерфейса", { exact: true })).toBeVisible();
  const fullHDSettingsOverflow = await settingsDialog.locator(".ui-modal-content").evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
  }));
  return { fullHDSettingsOverflow, settingsColumns, settingsDialog };
}

async function checkSettingsAndRoutingDesktopLayouts({ fullHDSettingsOverflow, page, settingsColumns, settingsDialog }: { fullHDSettingsOverflow: { clientHeight: number; scrollHeight: number; }; page: Page; settingsColumns: Locator; settingsDialog: Locator; }) {
  expect(fullHDSettingsOverflow.scrollHeight).toBeLessThanOrEqual(fullHDSettingsOverflow.clientHeight + 1);

  await page.setViewportSize({ width: 900, height: 900 });
  expect(await settingsColumns.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(1);
  await expectNoHorizontalOverflow(page);

  await page.setViewportSize({ width: 1366, height: 600 });
  const shortSettingsMetrics = await settingsDialog.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return { top: rect.top, bottom: rect.bottom, viewportHeight: window.innerHeight };
  });
  expect(shortSettingsMetrics.top).toBeGreaterThanOrEqual(0);
  expect(shortSettingsMetrics.bottom).toBeLessThanOrEqual(shortSettingsMetrics.viewportHeight);
  const settingsContent = settingsDialog.locator(".ui-modal-content");
  await settingsContent.evaluate((element) => { element.scrollTop = element.scrollHeight; });
  await expect(settingsDialog.getByRole("button", { name: "Сохранить" })).toBeVisible();
  await settingsDialog.getByRole("button", { name: "Закрыть" }).click();

  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.getByRole("button", { name: "Открыть редактор" }).click();
  const routingDialog = page.getByRole("dialog", { name: "Роутинг" });
  await expect(routingDialog.getByRole("radio", { name: /Не передавать/ })).toBeChecked();
  await expect(routingDialog.getByLabel("Добавить профиль, Happ-ссылка")).toContainText("happ://routing/add/server-add-payload");
  await expect(routingDialog.getByLabel("Отключить роутинг, Happ-ссылка")).toContainText("happ://routing/off");
  const routingDeliveryOptions = routingDialog.getByTestId("routing-delivery-options");
  const routingWorkspace = routingDialog.getByTestId("routing-workspace");
  const routingScrollArea = routingDialog.getByTestId("routing-scroll-area");
  const routingFooter = routingDialog.getByTestId("routing-action-footer");
  expect(await routingDeliveryOptions.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(3);
  expect(await routingWorkspace.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(2);
  const largeViewportGeometry = await routingDialog.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return {
      left: rect.left,
      right: rect.right,
      top: rect.top,
      bottom: rect.bottom,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      scrollWidth: element.scrollWidth,
      clientWidth: element.clientWidth,
    };
  });
  expect(largeViewportGeometry.left).toBeGreaterThanOrEqual(0);
  expect(largeViewportGeometry.right).toBeLessThanOrEqual(largeViewportGeometry.viewportWidth);
  expect(largeViewportGeometry.top).toBeGreaterThanOrEqual(0);
  expect(largeViewportGeometry.bottom).toBeLessThanOrEqual(largeViewportGeometry.viewportHeight);
  expect(largeViewportGeometry.scrollWidth).toBeLessThanOrEqual(largeViewportGeometry.clientWidth);
  const largeViewportOverflow = await routingScrollArea.evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
  }));
  expect(largeViewportOverflow.scrollHeight).toBeLessThanOrEqual(largeViewportOverflow.clientHeight + 1);
  const alignedColumnTops = await Promise.all([
    routingDialog.getByTestId("routing-import-card").evaluate((element) => element.getBoundingClientRect().top),
    routingDialog.getByTestId("routing-editor").evaluate((element) => element.getBoundingClientRect().top),
  ]);
  expect(Math.abs(alignedColumnTops[0] - alignedColumnTops[1])).toBeLessThanOrEqual(1);
  return { routingDeliveryOptions, routingWorkspace, routingDialog, routingScrollArea, routingFooter };
}

async function checkConstrainedRoutingWorkspace({ page, routingDeliveryOptions, routingWorkspace, routingDialog, routingScrollArea, routingFooter }: { page: Page; routingDeliveryOptions: Locator; routingWorkspace: Locator; routingDialog: Locator; routingScrollArea: Locator; routingFooter: Locator; }) {
  await page.setViewportSize({ width: 1440, height: 768 });
  expect(await routingDeliveryOptions.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(3);
  expect(await routingWorkspace.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(2);
  const constrainedGeometry = await routingDialog.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return {
      left: rect.left,
      right: rect.right,
      top: rect.top,
      bottom: rect.bottom,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      scrollWidth: element.scrollWidth,
      clientWidth: element.clientWidth,
    };
  });
  expect(constrainedGeometry.left).toBeGreaterThanOrEqual(0);
  expect(constrainedGeometry.right).toBeLessThanOrEqual(constrainedGeometry.viewportWidth);
  expect(constrainedGeometry.top).toBeGreaterThanOrEqual(0);
  expect(constrainedGeometry.bottom).toBeLessThanOrEqual(constrainedGeometry.viewportHeight);
  expect(constrainedGeometry.scrollWidth).toBeLessThanOrEqual(constrainedGeometry.clientWidth);
  const constrainedOverflow = await routingScrollArea.evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
    scrollTop: element.scrollTop,
    overflowY: getComputedStyle(element).overflowY,
  }));
  expect(constrainedOverflow.scrollHeight).toBeGreaterThan(constrainedOverflow.clientHeight);
  expect(constrainedOverflow.scrollTop).toBe(0);
  expect(constrainedOverflow.overflowY).toBe("auto");
  await routingScrollArea.hover();
  await page.mouse.wheel(0, 600);
  await expect.poll(() => routingScrollArea.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  await routingDialog.getByTestId("routing-manual-card").scrollIntoViewIfNeeded();
  const manualCardReachability = await Promise.all([
    routingDialog.getByTestId("routing-manual-card").evaluate((element) => element.getBoundingClientRect().bottom),
    routingScrollArea.evaluate((element) => element.getBoundingClientRect().bottom),
  ]);
  expect(manualCardReachability[0]).toBeLessThanOrEqual(manualCardReachability[1] + 1);
  await expect(routingDialog.getByLabel("Отключить роутинг, Happ-ссылка")).toBeVisible();
  await expect(routingDialog.getByRole("button", { name: "Закрыть" })).toBeVisible();
  await expect(routingFooter).toBeVisible();
  await expect(routingDialog.getByRole("button", { name: "Применить" })).toBeVisible();
  const fixedChromeGeometry = await Promise.all([
    routingDialog.getByRole("button", { name: "Закрыть" }).evaluate((element) => {
      const rect = element.getBoundingClientRect();
      return { top: rect.top, bottom: rect.bottom };
    }),
    routingFooter.evaluate((element) => {
      const rect = element.getBoundingClientRect();
      return { top: rect.top, bottom: rect.bottom };
    }),
  ]);
  expect(fixedChromeGeometry[0].top).toBeGreaterThanOrEqual(constrainedGeometry.top);
  expect(fixedChromeGeometry[1].bottom).toBeLessThanOrEqual(constrainedGeometry.bottom);

  await page.setViewportSize({ width: 900, height: 700 });
  expect(await routingDeliveryOptions.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(1);
  expect(await routingWorkspace.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(1);
}

async function checkRoutingFooterAndApply({ routingDialog, page, routingScrollArea, routingFooter, readRoutingDeliveryMode, runtimeErrors }: { routingDialog: Locator; page: Page; routingScrollArea: Locator; routingFooter: Locator; readRoutingDeliveryMode: () => string; runtimeErrors: string[]; }) {
  await expect(routingDialog.getByRole("button", { name: "Сбросить" })).toBeVisible();
  await expect(routingDialog.getByRole("button", { name: "Применить" })).toBeVisible();
  expect(await routingDialog.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);

  await page.setViewportSize({ width: 1440, height: 900 });
  await routingScrollArea.evaluate((element) => { element.scrollTop = 0; });
  await routingDialog.getByRole("button", { name: "Исключения и правила" }).click();
  await expect(routingDialog.getByLabel("DirectSites, элемент 5")).toHaveValue("five.test");
  await routingScrollArea.evaluate((element) => { element.scrollTop = element.scrollHeight; });
  await expect(routingFooter).toBeVisible();
  await expect(routingDialog.getByRole("button", { name: "Применить" })).toBeVisible();
  await expect(routingDialog.getByRole("button", { name: "Закрыть" })).toBeVisible();
  await routingDialog.getByRole("radio", { name: /Добавлять и активировать/ }).click();
  await routingDialog.getByRole("button", { name: "Применить" }).click();
  await expect(routingDialog).toBeHidden();
  await expect(page.getByText("Передаётся с подпиской", { exact: true })).toBeVisible();
  expect(readRoutingDeliveryMode()).toBe("onadd");
  expect(runtimeErrors).toEqual([]);
}

async function mountSubscriptionRoutingFixture(page: Page) {
  const runtimeErrors: string[] = [];
  let routingDeliveryMode = "disabled";
  const routingConfig = JSON.stringify({
    Name: "SubShare Routing",
    GlobalProxy: false,
    RouteOrder: "block-proxy-direct",
    DomainStrategy: "IPIfNonMatch",
    FakeDNS: false,
    UseChunkFiles: true,
    DnsHosts: {},
    DirectSites: ["one.test", "two.test", "three.test", "four.test", "five.test"],
    DirectIp: [],
    ProxySites: [],
    ProxyIp: [],
    BlockSites: [],
    BlockIp: [],
  });
  page.on("pageerror", (error) => runtimeErrors.push(error.message));
  await mountSubscriptionAuthentication(page);
  await mountSubscriptionPageResponses(page);
  await page.route("**/api/v1/routing-settings", async (route) => {
    if (route.request().method() === "PUT") {
      const body = route.request().postDataJSON() as { delivery_mode: string; };
      routingDeliveryMode = body.delivery_mode;
    }
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        config_json: routingConfig,
        delivery_mode: routingDeliveryMode,
        add_url: "happ://routing/add/server-add-payload",
        onadd_url: "happ://routing/onadd/server-onadd-payload",
        off_url: "happ://routing/off",
      }),
    });
  });

  return { runtimeErrors, readRoutingDeliveryMode: () => routingDeliveryMode };

}
async function mountSubscriptionAuthentication(page: Page) {
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
async function mountSubscriptionPageResponses(page: Page) {
  await page.route("**/api/v1/subscription-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        title: "AllKeys",
        refresh_hours: 12,
        info_url: "",
        extra_url: "",
        extra_status: "down",
        subscription_format: "xray-json",
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
  await page.route("**/api/v1/subscription-delivery-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        response_headers: [{ key: "X-Edge", value: "enabled" }],
        remarks: { expired: ["Срок действия истёк"], paused: [], blocked: [], limited: [], empty: [] },
      }),
    })
  );

}
