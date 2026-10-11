import { expect, test, type Page } from "@playwright/test";
import { openKeysPage } from "./key-list-fixture";

async function expectKeyDragAutoScroll(page: Page) {
  await openKeysPage(page);

  await page.evaluate(() => {
    document.body.style.minHeight = "3200px";
    window.scrollTo(0, 0);
  });
  const dragHandle = page.locator(".ui-key-card [title*='Перетащите']").first();
  const dragHandleBox = await dragHandle.boundingBox();
  expect(dragHandleBox).not.toBeNull();
  if (dragHandleBox) {
    await page.mouse.move(dragHandleBox.x + dragHandleBox.width / 2, dragHandleBox.y + dragHandleBox.height / 2);
    await page.mouse.down();
    await page.mouse.move(dragHandleBox.x + dragHandleBox.width / 2 + 8, dragHandleBox.y + dragHandleBox.height / 2 + 8, {
      steps: 4,
    });
    await page.mouse.move(dragHandleBox.x + dragHandleBox.width / 2, 895, { steps: 8 });
    await expect.poll(() => page.evaluate(() => window.scrollY), { timeout: 3_000 }).toBeGreaterThan(40);
    await page.mouse.up();
  }
  await page.evaluate(() => {
    document.body.style.minHeight = "";
    window.scrollTo(0, 0);
  });
}

test("key drag autoscrolls the page near the viewport edge", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await expectKeyDragAutoScroll(page);
});

test("key drag autoscrolls near the viewport edge with reduced motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expectKeyDragAutoScroll(page);
});
