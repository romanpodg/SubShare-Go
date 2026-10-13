import type { Locator, Page } from "@playwright/test";
import { expect, test } from "@playwright/test";
import { openKeysPage } from "./key-list-fixture";
import { expectEditorFooterContained, expectNoHorizontalOverflow, expectSingleDividerStructure, expectTechnicalScrollbar } from "./smoke-assertions";

test("key editor remains usable and responsive", async ({ page }) => {
  const { rawEditorHeight, configurationDialog, longClientDisplayName, desktopEditorMetrics } = await openAndCheckDesktopEditors({ page });
  const { categoryDialog } = await checkEditorViewportLayouts({ rawEditorHeight, configurationDialog, page });
  const { editDialog } = await checkCategoryDiscardAndSourceOwnership({ categoryDialog, configurationDialog, page, longClientDisplayName });
  await checkSourceOwnedEditorLayout({ editDialog, desktopEditorMetrics, page });
});

async function openAndCheckDesktopEditors({ page }: { page: Page; }) {
  const { longClientDisplayName } = await openKeysPage(page);
  await page.setViewportSize({ width: 1920, height: 1080 });

  const informationalColumnTitle = page
    .locator(".ui-key-column-title")
    .filter({ hasText: "Информационные сообщения" })
    .first();
  const informationalAddButton = informationalColumnTitle.getByRole("button", { name: "Добавить" });
  const informationalButtonGap = await informationalColumnTitle.evaluate((title, button) => {
    const titleRect = title.getBoundingClientRect();
    const buttonRect = (button as HTMLElement).getBoundingClientRect();
    return Math.round(titleRect.bottom - buttonRect.bottom);
  }, await informationalAddButton.elementHandle());
  expect(informationalButtonGap).toBeGreaterThanOrEqual(2);

  await informationalAddButton.click();
  const informationalDialog = page.getByRole("dialog", { name: "Добавить информационный ключ" });
  await expect(informationalDialog.getByLabel("Тип")).toHaveText("Информационный ключ");
  await informationalDialog.getByRole("button", { name: "Закрыть" }).click();

  const configurationColumnTitle = page
    .locator(".ui-key-column-title")
    .filter({ hasText: "Рабочие конфигурации" })
    .first();
  await configurationColumnTitle.getByRole("button", { name: "Добавить" }).click();
  const configurationDialog = page.getByRole("dialog", { name: "Добавить конфигурацию" });
  await expect(configurationDialog.getByLabel("Тип")).toHaveText("Конфигурация");
  await expectSingleDividerStructure(configurationDialog.locator(".ui-key-editor-workspace"));
  await expectTechnicalScrollbar(page.locator("body"));
  await expectTechnicalScrollbar(configurationDialog.locator(".ui-key-editor-scroll-region"));
  await expectTechnicalScrollbar(configurationDialog.locator("#key-editor-raw"));

  await configurationDialog.getByLabel("Статус").click();
  const statusMenu = page.locator(".ui-select-menu");
  await expect(statusMenu).toBeVisible();
  await expectTechnicalScrollbar(statusMenu);
  await statusMenu.locator('[role="option"][aria-selected="true"]').click();

  const desktopEditorMetrics = await configurationDialog.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return {
      width: Math.round(rect.width),
      scrollWidth: element.scrollWidth,
      clientWidth: element.clientWidth,
    };
  });
  expect(desktopEditorMetrics.width).toBeGreaterThan(900);
  expect(desktopEditorMetrics.scrollWidth).toBeLessThanOrEqual(desktopEditorMetrics.clientWidth);
  const fullHDScrollMetrics = await configurationDialog.locator(".ui-key-editor-scroll-region").evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
    overflowY: getComputedStyle(element).overflowY,
  }));
  expect(fullHDScrollMetrics.scrollHeight).toBeLessThanOrEqual(fullHDScrollMetrics.clientHeight + 1);
  expect(Math.abs(fullHDScrollMetrics.clientHeight - fullHDScrollMetrics.scrollHeight)).toBeLessThanOrEqual(1);
  expect(fullHDScrollMetrics.overflowY).toBe("auto");
  const rawEditorHeight = await configurationDialog.locator("#key-editor-raw").evaluate((element) =>
    Math.round(element.getBoundingClientRect().height)
  );
  return { rawEditorHeight, configurationDialog, longClientDisplayName, desktopEditorMetrics };
}

async function checkEditorViewportLayouts({ rawEditorHeight, configurationDialog, page }: { rawEditorHeight: number; configurationDialog: Locator; page: Page; }) {
  expect(rawEditorHeight).toBeGreaterThanOrEqual(240);
  expect(rawEditorHeight).toBeLessThanOrEqual(325);
  await expectEditorFooterContained(configurationDialog);
  const compactDesktopLayout = await configurationDialog.evaluate((dialog) => {
    const dialogRect = dialog.getBoundingClientRect();
    const scrollRect = dialog.querySelector(".ui-key-editor-scroll-region")!.getBoundingClientRect();
    const footerRect = dialog.querySelector(".ui-key-editor-footer")!.getBoundingClientRect();
    return {
      dialogHeight: Math.round(dialogRect.height),
      editorToFooterGap: Math.round(footerRect.top - scrollRect.bottom),
    };
  });
  expect(compactDesktopLayout.dialogHeight).toBeLessThan(850);
  expect(compactDesktopLayout.editorToFooterGap).toBeLessThanOrEqual(16);
  const metadataSelectWidths = await Promise.all([
    configurationDialog.getByLabel("Статус").evaluate((element) => Math.round(element.getBoundingClientRect().width)),
    configurationDialog.getByLabel("Тип").evaluate((element) => Math.round(element.getBoundingClientRect().width)),
  ]);
  expect(metadataSelectWidths[0]).toBeGreaterThan(200);
  expect(Math.abs(metadataSelectWidths[0] - metadataSelectWidths[1])).toBeLessThanOrEqual(2);
  const desktopWorkspaceColumns = await configurationDialog
    .locator(".ui-key-editor-workspace")
    .evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length);
  expect(desktopWorkspaceColumns).toBe(2);

  await page.setViewportSize({ width: 768, height: 900 });
  const tabletWorkspaceColumns = await configurationDialog
    .locator(".ui-key-editor-workspace")
    .evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length);
  expect(tabletWorkspaceColumns).toBe(1);
  const tabletDialogWidth = await configurationDialog.evaluate((element) =>
    Math.round(element.getBoundingClientRect().width)
  );
  expect(tabletDialogWidth).toBeLessThanOrEqual(768);
  await page.setViewportSize({ width: 1366, height: 768 });
  await expectEditorFooterContained(configurationDialog);
  const mediumViewportDialog = await configurationDialog.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return { top: rect.top, bottom: rect.bottom, viewportHeight: window.innerHeight };
  });
  expect(mediumViewportDialog.top).toBeGreaterThanOrEqual(0);
  expect(mediumViewportDialog.bottom).toBeLessThanOrEqual(mediumViewportDialog.viewportHeight);
  await page.setViewportSize({ width: 1366, height: 520 });
  const shortScrollRegion = configurationDialog.locator(".ui-key-editor-scroll-region");
  const shortEditorMetrics = await shortScrollRegion.evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
    overflowY: getComputedStyle(element).overflowY,
  }));
  expect(shortEditorMetrics.scrollHeight).toBeGreaterThan(shortEditorMetrics.clientHeight);
  expect(shortEditorMetrics.overflowY).toBe("auto");
  await expectEditorFooterContained(configurationDialog);
  await shortScrollRegion.evaluate((element) => { element.scrollTop = element.scrollHeight; });
  await expect(configurationDialog.getByRole("button", { name: "Добавить профиль" })).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 900 });

  await configurationDialog.getByRole("button", { name: "Добавить категорию" }).click();
  const categoryDialog = page.getByRole("dialog", { name: "Добавить категорию" });
  return { categoryDialog };
}

async function checkCategoryDiscardAndSourceOwnership({ categoryDialog, configurationDialog, page, longClientDisplayName }: { categoryDialog: Locator; configurationDialog: Locator; page: Page; longClientDisplayName: string; }) {
  const categoryOverflow = await categoryDialog.evaluate((element) => ({
    horizontal: element.scrollWidth > element.clientWidth,
    vertical: element.scrollHeight > element.clientHeight,
  }));
  expect(categoryOverflow).toEqual({ horizontal: false, vertical: false });
  await categoryDialog.getByRole("button", { name: "Закрыть" }).click();

  await configurationDialog.getByLabel("Название", { exact: true }).fill("Несохранённый черновик");
  await configurationDialog.getByRole("button", { name: "Закрыть" }).click();
  const discardDialog = page.getByRole("dialog", { name: "Закрыть без сохранения?" });
  const discardFrameMetrics = await discardDialog.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
    overflowX: getComputedStyle(element).overflowX,
    overflowY: getComputedStyle(element).overflowY,
  }));
  expect(discardFrameMetrics.scrollWidth).toBeLessThanOrEqual(discardFrameMetrics.clientWidth);
  expect(discardFrameMetrics.scrollHeight).toBeLessThanOrEqual(discardFrameMetrics.clientHeight);
  expect(discardFrameMetrics.overflowX).toBe("hidden");
  expect(discardFrameMetrics.overflowY).toBe("hidden");

  const discardContentMetrics = await discardDialog.locator(".ui-modal-content").evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
  }));
  expect(discardContentMetrics.scrollWidth).toBeLessThanOrEqual(discardContentMetrics.clientWidth);
  expect(discardContentMetrics.scrollHeight).toBeLessThanOrEqual(discardContentMetrics.clientHeight);
  await discardDialog.getByRole("button", { name: "Закрыть" }).last().click();

  await page.setViewportSize({ width: 1920, height: 1080 });
  const realKeyCard = page.locator(".ui-key-card").filter({ hasText: "Обход блокировок" });
  await realKeyCard.getByRole("button", { name: "Изменить" }).click();
  const editDialog = page.getByRole("dialog", {
    name: new RegExp("Изменить конфигурацию — .*Обход блокировок"),
  });
  await expect(editDialog).toBeVisible();
  await expect(editDialog).toHaveClass(/max-w-6xl/);
  await expect(editDialog.getByLabel("Название", { exact: true })).toHaveValue(longClientDisplayName);
  await expect(editDialog.getByRole("button", { name: "XRAY-JSON" })).toHaveClass(/bg-accent/);
  await expect(editDialog.locator(".ui-key-editor-source-banner")).toBeVisible();
  await expect(editDialog.locator(".ui-key-editor-workspace")).toHaveCount(1);
  await expect(editDialog.locator(".ui-key-editor-scroll-region")).toHaveCount(1);
  await expect(editDialog.getByRole("button", { name: "Сохранить" })).toBeVisible();
  return { editDialog };
}

async function checkSourceOwnedEditorLayout({ editDialog, desktopEditorMetrics, page }: { editDialog: Locator; desktopEditorMetrics: { width: number; scrollWidth: number; clientWidth: number; }; page: Page; }) {
  const footerMetrics = await editDialog.locator(".ui-key-editor-footer").evaluate((footer) => {
    const styles = getComputedStyle(footer);
    const buttons = Array.from(footer.querySelectorAll("button")).map((button) => button.getBoundingClientRect());
    return {
      alignItems: styles.alignItems,
      justifyContent: styles.justifyContent,
      paddingTop: styles.paddingTop,
      paddingBottom: styles.paddingBottom,
      buttonHeights: buttons.map((button) => Math.round(button.height)),
      buttonWidths: buttons.map((button) => Math.round(button.width)),
      buttonGap: buttons.length === 2 ? Math.round(buttons[1].left - buttons[0].right) : 0,
    };
  });
  expect(footerMetrics.alignItems).toBe("center");
  expect(footerMetrics.justifyContent).toBe("flex-end");
  expect(footerMetrics.paddingTop).toBe("16px");
  expect(footerMetrics.paddingBottom).toBe("16px");
  expect(footerMetrics.buttonHeights).toEqual([44, 44]);
  expect(footerMetrics.buttonWidths).toEqual([144, 144]);
  expect(footerMetrics.buttonGap).toBe(12);
  await expectEditorFooterContained(editDialog);
  const sourceOwnedFullHDScrollMetrics = await editDialog.locator(".ui-key-editor-scroll-region").evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
  }));
  expect(sourceOwnedFullHDScrollMetrics.scrollHeight).toBeLessThanOrEqual(sourceOwnedFullHDScrollMetrics.clientHeight + 1);
  expect(Math.abs(sourceOwnedFullHDScrollMetrics.clientHeight - sourceOwnedFullHDScrollMetrics.scrollHeight)).toBeLessThanOrEqual(1);

  const editDialogMetrics = await editDialog.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return {
      width: Math.round(rect.width),
      scrollWidth: element.scrollWidth,
      clientWidth: element.clientWidth,
      workspaceColumns: getComputedStyle(
        element.querySelector(".ui-key-editor-workspace") as Element
      ).gridTemplateColumns.split(" ").length,
    };
  });
  expect(editDialogMetrics.width).toBe(desktopEditorMetrics.width);
  expect(editDialogMetrics.scrollWidth).toBeLessThanOrEqual(editDialogMetrics.clientWidth);
  expect(editDialogMetrics.workspaceColumns).toBe(2);
  await editDialog.getByRole("button", { name: "Закрыть" }).click();

  await expectNoHorizontalOverflow(page);
}
