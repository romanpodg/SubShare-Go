import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow, expectNoWcagViolations } from "./smoke-assertions";

test("login page is usable at desktop and mobile widths", async ({ page }) => {
  await page.route("**/api/v1/panel-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ app_name: "SubShare", logo_url: "", theme: "dark" }),
    })
  );

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/admin/login");
  await expect(page.getByRole("heading", { name: /SubShare|Вход/i })).toBeVisible();
  const usernameInput = page.getByLabel(/логин|username/i);
  const passwordInput = page.locator('input[type="password"]');
  await expect(usernameInput).toBeVisible();
  await expect(passwordInput).toHaveCount(1);
  await expect(passwordInput).toBeVisible();

  const signalCore = page.getByRole("img", { name: /сигнальное ядро SubShare/i });
  await expect(signalCore).toBeVisible();
  await expect(signalCore.getByText("SIGNAL CORE")).toBeVisible();
  const signalCoreBounds = await signalCore.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    const parentRect = element.parentElement?.getBoundingClientRect();
    return {
      width: rect.width,
      height: rect.height,
      fitsWidth: parentRect ? rect.left >= parentRect.left && rect.right <= parentRect.right : false,
      fitsHeight: parentRect ? rect.top >= parentRect.top && rect.bottom <= parentRect.bottom : false,
    };
  });
  expect(signalCoreBounds.width).toBeGreaterThan(500);
  expect(signalCoreBounds.height).toBeGreaterThan(200);
  expect(signalCoreBounds.fitsWidth).toBe(true);
  expect(signalCoreBounds.fitsHeight).toBe(true);

  await page.evaluate(() => document.fonts.ready);
  const displayFamilies = await Promise.all([
    page.getByTestId("login-display-ru").evaluate((element) => getComputedStyle(element).fontFamily),
    page.getByTestId("login-display-en").evaluate((element) => getComputedStyle(element).fontFamily),
  ]);
  expect(displayFamilies).toEqual([
    expect.stringContaining("Onest Variable"),
    expect.stringContaining("Onest Variable"),
  ]);

  await expect(usernameInput).toBeFocused();
  await usernameInput.press("Tab");
  await expect(passwordInput).toBeFocused();

  const animatedOrbit = signalCore.locator(".signal-core-orbit--outer");
  await expect(animatedOrbit).not.toHaveCSS("animation-name", "none");
  await expectNoHorizontalOverflow(page);
  await expectNoWcagViolations(page);

  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect(animatedOrbit).toHaveCSS("animation-name", "none");
  await expect(signalCore.locator(".signal-core-flow")).toHaveCSS("animation-name", "none");

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(signalCore).toBeHidden();
  await expect(page.getByRole("button", { name: /войти в систему/i })).toBeVisible();
  await expect(usernameInput).toBeVisible();
  await expect(passwordInput).toBeVisible();
  await expectNoHorizontalOverflow(page);
  await expectNoWcagViolations(page);
});
