import { expect, test } from "@playwright/test";
import { openKeysPage } from "./key-list-fixture";

test("sidebar keeps its item alignment and supports scrolling when collapsed or short", async ({ page }) => {
  await openKeysPage(page);
  const sidebar = page.locator("aside[data-collapsed]");
  const sidebarNav = sidebar.locator(".ui-sidebar-nav");
  const sidebarLinks = sidebar.locator(".ui-sidebar-nav-item");
  await expect(sidebar).toHaveAttribute("data-collapsed", "false");
  const expandedSidebarTops = await sidebarLinks.evaluateAll((links) =>
    links.map((link) => Math.round(link.getBoundingClientRect().top))
  );
  const sidebarScrollbar = await sidebarNav.evaluate((element) => {
    const computed = getComputedStyle(element);
    const webkitScrollbar = getComputedStyle(element, "::-webkit-scrollbar");
    return {
      standardWidth: computed.scrollbarWidth,
      webkitDisplay: webkitScrollbar.display,
      webkitWidth: webkitScrollbar.width,
    };
  });
  expect(sidebarScrollbar.standardWidth).toBe("none");
  expect(sidebarScrollbar.webkitDisplay).toBe("none");
  expect(sidebarScrollbar.webkitWidth).toBe("0px");

  await sidebar.getByRole("button", { name: "Свернуть меню" }).click();
  await expect(sidebar).toHaveAttribute("data-collapsed", "true");
  await expect.poll(() => sidebar.evaluate((element) => Math.round(element.getBoundingClientRect().width))).toBe(72);
  const collapsedSidebarTops = await sidebarLinks.evaluateAll((links) =>
    links.map((link) => Math.round(link.getBoundingClientRect().top))
  );
  expect(collapsedSidebarTops).toEqual(expandedSidebarTops);

  await sidebar.getByRole("button", { name: "Развернуть меню" }).click();
  await expect(sidebar).toHaveAttribute("data-collapsed", "false");
  await expect.poll(() => sidebar.evaluate((element) => Math.round(element.getBoundingClientRect().width))).toBe(248);
  await page.setViewportSize({ width: 1440, height: 560 });
  const sidebarScrollMetrics = await sidebarNav.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
    return {
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      scrollTop: element.scrollTop,
    };
  });
  expect(sidebarScrollMetrics.scrollHeight).toBeGreaterThan(sidebarScrollMetrics.clientHeight);
  expect(sidebarScrollMetrics.scrollTop).toBeGreaterThan(0);
  await sidebarNav.evaluate((element) => {
    element.scrollTop = 0;
  });
  await page.setViewportSize({ width: 1440, height: 900 });
});
