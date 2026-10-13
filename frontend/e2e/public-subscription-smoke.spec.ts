import type { Page } from "@playwright/test";
import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow, expectNoWcagViolations } from "./smoke-assertions";

test("public subscription page stays dark, accessible, and responsive", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "light" });
  await page.route("**/api/v1/panel-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        panelTitle: "SubShare",
        logoDataUrl: "",
        faviconDataUrl: "",
        pageTitleAdmin: "Панель управления — SubShare",
        pageTitleAdminLogin: "Вход — SubShare",
        pageTitleSubscription: "VPN-подписка — SubShare",
      }),
    })
  );
  await page.route("**/api/v1/subscription-page-config", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ config_json: "", default_config_json: "" }),
    })
  );
  await page.goto("/subscription");

  await expect(page.getByRole("heading", { name: "SubShare", exact: true })).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.getByLabel(/ключ активации/i)).toBeVisible();

  const viewportCases = [
    { width: 1440, height: 900 },
    { width: 768, height: 900 },
    { width: 390, height: 844 },
  ];

  for (const viewport of viewportCases) {
    await checkStaticGlobe(page, viewport);
    await checkPlatformSelection(page, viewport);
    await checkSubscriptionActions(page, viewport);

  }
});

async function checkStaticGlobe(page: Page, viewport: { width: number; height: number; }) {
  await page.setViewportSize(viewport);
  await page.evaluate(() => document.fonts.ready);

  const globe = page.getByRole("img", { name: "Цифровой глобус сети SubShare" });
  await expect(globe).toBeVisible();
  await expect(globe).toHaveAttribute("data-motion", "static");
  await expect.poll(() =>
    globe.locator("canvas").evaluate((element) => {
      const canvas = element as HTMLCanvasElement;
      const context = canvas.getContext("2d");
      if (!context) return 0;
      if (canvas.width === 0) return 0;
      if (canvas.height === 0) return 0;
      const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data;
      let paintedPixels = 0;
      for (let index = 3; index < pixels.length; index += 4) {
        if (pixels[index] > 0) paintedPixels += 1;
      }
      return paintedPixels;
    })
  ).toBeGreaterThan(500);
  const globeBounds = await globe.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    const parentBounds = element.parentElement?.getBoundingClientRect();
    return {
      width: bounds.width,
      height: bounds.height,
      parent: parentBounds ? { left: parentBounds.left, right: parentBounds.right, top: parentBounds.top, bottom: parentBounds.bottom } : null,
      left: bounds.left, right: bounds.right, top: bounds.top, bottom: bounds.bottom,
    };
  });
  expect(globeBounds.width).toBeGreaterThan(250);
  expect(Math.abs(globeBounds.width - globeBounds.height)).toBeLessThanOrEqual(1);
  expect(globeBounds.parent).not.toBeNull();
  expect(globeBounds.left).toBeGreaterThanOrEqual(globeBounds.parent!.left);
  expect(globeBounds.right).toBeLessThanOrEqual(globeBounds.parent!.right);
  expect(globeBounds.top).toBeGreaterThanOrEqual(globeBounds.parent!.top);
  expect(globeBounds.bottom).toBeLessThanOrEqual(globeBounds.parent!.bottom);

}
async function checkPlatformSelection(page: Page, viewport: { width: number; height: number; }) {
  const statusText = page.getByText("Подписка готова к подключению", { exact: true });
  const statusDot = statusText.locator("xpath=preceding-sibling::*[1]");
  const statusCenters = await Promise.all([
    statusText.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return bounds.top + bounds.height / 2;
    }),
    statusDot.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return bounds.top + bounds.height / 2;
    }),
  ]);
  expect(Math.abs(statusCenters[0] - statusCenters[1])).toBeLessThanOrEqual(1);

  const platformGroup = page.getByRole("group", { name: "Выбор платформы" });
  await expect(platformGroup).toBeVisible();
  expect(
    await platformGroup.evaluate((element) => getComputedStyle(element).backgroundColor)
  ).toBe("rgba(0, 0, 0, 0)");

  const allPlatformsButton = page.getByRole("button", { name: "Все", exact: true });
  await allPlatformsButton.click();
  await expect(allPlatformsButton).toHaveAttribute("aria-pressed", "true");
  const platformButtons = platformGroup.getByRole("button");
  await expect(platformButtons).toHaveCount(5);
  const platformBackgrounds = await platformButtons.evaluateAll((buttons) =>
    buttons.map((button) => getComputedStyle(button).backgroundColor)
  );
  expect(platformBackgrounds.every((background) => background !== "rgba(0, 0, 0, 0)")).toBe(true);

  if (viewport.width === 390) {
    const platformRows = await platformButtons.evaluateAll((buttons) =>
      Array.from(new Set(buttons.map((button) => Math.round(button.getBoundingClientRect().top))))
    );
    expect(platformRows.length).toBeGreaterThan(1);
  }

}
async function checkSubscriptionActions(page: Page, viewport: { width: number; height: number; }) {
  const windowsLink = page.locator('[data-button-id="windows"]');
  const windowsIcon = windowsLink.locator('[style*="--button-icon-mask"]');
  await expect(windowsIcon).toBeVisible();
  await expect.poll(() => windowsLink.evaluate((element) => {
    const icon = element.querySelector<HTMLElement>('[style*="--button-icon-mask"]');
    return icon !== null && getComputedStyle(icon).backgroundColor === getComputedStyle(element).color;
  })).toBe(true);
  const iconColors = await windowsLink.evaluate((element) => {
    const icon = element.querySelector<HTMLElement>('[style*="--button-icon-mask"]');
    return [
      getComputedStyle(element).color,
      icon ? getComputedStyle(icon).backgroundColor : "",
    ];
  });
  expect(iconColors[1]).toBe(iconColors[0]);
  await expect(page.locator('[data-button-id="google-play"] img')).toBeVisible();

  const steps = page.locator("article");
  await expect(steps).toHaveCount(3);
  const firstStepBounds = await steps.nth(0).boundingBox();
  const thirdStepBounds = await steps.nth(2).boundingBox();
  expect(firstStepBounds).not.toBeNull();
  expect(thirdStepBounds).not.toBeNull();
  if (viewport.width === 1440) {
    expect(thirdStepBounds!.width).toBeGreaterThan(firstStepBounds!.width * 1.8);
  } else {
    expect(Math.abs(thirdStepBounds!.width - firstStepBounds!.width)).toBeLessThanOrEqual(1);
  }

  await expectNoHorizontalOverflow(page);
  await expectNoWcagViolations(page);
}
