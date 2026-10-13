import type { Page } from "@playwright/test";
import { expect, test, type Locator } from "@playwright/test";
import { openKeysPage } from "./key-list-fixture";
import { expectNoHorizontalOverflow } from "./smoke-assertions";

test("key-list controls, alignment, and labels remain readable across viewports", async ({ page }) => {
  const { categoryNavigator, categoryNavigatorGrid, longKeyLabel, longSourceName, longClientDisplayName } = await checkKeyHealthAndInitialLayout({ page });
  await checkCategoryGridStyling({ categoryNavigator, categoryNavigatorGrid });
  const { sourceRailLine, sourceRailContainer, expectAlignedCategoryGeometry, keyModuleCard, uncategorizedBlock, sourcePanel, sourceBlock, reservePanel, sourceRailLabel, expectRailLabelCentered, sourceRailBadge, reserveBlock, reserveRailLabel, reserveRailBadge } = await prepareCategoryGeometryChecks({ categoryNavigator, page });
  await checkCategoryRailViewports({ sourceRailLine, sourceRailContainer, page, expectAlignedCategoryGeometry, keyModuleCard, uncategorizedBlock, sourcePanel, sourceBlock, reservePanel, sourceRailLabel });
  const { firstKeySelection } = await checkStickyCategoryHandoff({ sourceBlock, page, sourceRailLabel, expectRailLabelCentered, sourceRailContainer, sourceRailBadge, sourcePanel, reserveBlock, reserveRailLabel, reserveRailBadge, reservePanel });
  const { longKeyCard, longClientDisplay } = await checkSelectionAndLongLabels({ firstKeySelection, page, longKeyLabel, longSourceName, longClientDisplayName });
  await checkKeyCardLabels({ page, categoryNavigator, longKeyCard, longClientDisplay });
  await checkBulkSelection({ categoryNavigator, page, firstKeySelection });
});

async function checkKeyHealthAndInitialLayout({ page }: { page: Page; }) {
  const { longKeyLabel, longSourceName, longClientDisplayName } = await openKeysPage(page);

  const inactiveHealthyCard = page.locator(".ui-key-card").filter({ hasText: longClientDisplayName }).first();
  await expect(inactiveHealthyCard.getByText("Неактивен", { exact: true })).toBeVisible();
  await expect(inactiveHealthyCard.getByText("ДОСТУПЕН · 38 ms", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Проверить все" }).click();
  await expect(page.getByText("Проверка конфигураций запущена", { exact: true })).toBeVisible();
  await expect(page.getByText(/undefined/)).toHaveCount(0);

  const headerControls = page.locator(".ui-key-module-header").locator("button, a[role='button'], label.ui-icon-button");
  await expect(headerControls).toHaveCount(6);
  const boxes = await headerControls.evaluateAll((controls) =>
    controls.map((control) => {
      const rect = control.getBoundingClientRect();
      return { top: Math.round(rect.top), height: Math.round(rect.height) };
    })
  );
  expect(new Set(boxes.map((box) => box.height))).toEqual(new Set([44]));
  expect(Math.max(...boxes.map((box) => box.top)) - Math.min(...boxes.map((box) => box.top))).toBeLessThanOrEqual(2);

  await expect(page.locator(".ui-key-card")).toHaveCount(2);
  const informationalSlots = page.locator(".ui-key-order-column-info > .ui-key-order-slot");
  const configurationSlots = page.locator(".ui-key-order-column-real > .ui-key-order-slot");
  await expect(informationalSlots).toHaveCount(2);
  await expect(configurationSlots).toHaveCount(2);
  await expect(informationalSlots.nth(0).locator(".ui-key-card")).toHaveCount(1);
  await expect(configurationSlots.nth(0).locator(".ui-key-order-gap")).toHaveCount(1);
  await expect(informationalSlots.nth(1).locator(".ui-key-order-gap")).toHaveCount(1);
  await expect(configurationSlots.nth(1).locator(".ui-key-card")).toHaveCount(1);
  const alignedSlotTops = await Promise.all([
    informationalSlots.nth(0).evaluate((element) => Math.round(element.getBoundingClientRect().top)),
    configurationSlots.nth(0).evaluate((element) => Math.round(element.getBoundingClientRect().top)),
    informationalSlots.nth(1).evaluate((element) => Math.round(element.getBoundingClientRect().top)),
    configurationSlots.nth(1).evaluate((element) => Math.round(element.getBoundingClientRect().top)),
  ]);
  expect(alignedSlotTops[0]).toBe(alignedSlotTops[1]);
  expect(alignedSlotTops[2]).toBe(alignedSlotTops[3]);
  expect(alignedSlotTops[2]).toBeGreaterThan(alignedSlotTops[0]);

  const keyCardFontSize = await page.locator(".ui-key-card").first().evaluate((element) =>
    Number.parseFloat(window.getComputedStyle(element).fontSize)
  );
  expect(keyCardFontSize).toBeGreaterThanOrEqual(14);

  const categoryNavigator = page.locator(".ui-key-category-nav");
  await expect(categoryNavigator).toBeVisible();
  await expect(categoryNavigator.locator(".ui-key-category-nav-main")).toHaveCount(7);
  const categoryNavigatorMetrics = await categoryNavigator.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(categoryNavigatorMetrics.scrollWidth).toBeLessThanOrEqual(categoryNavigatorMetrics.clientWidth);

  const categoryNavigatorGrid = categoryNavigator.locator(".ui-key-category-nav-grid");
  return { categoryNavigator, categoryNavigatorGrid, longKeyLabel, longSourceName, longClientDisplayName };
}

async function checkCategoryGridStyling({ categoryNavigator, categoryNavigatorGrid }: { categoryNavigator: Locator; categoryNavigatorGrid: Locator; }) {
  for (const [containerWidth, expectedColumns] of [
    [400, 1],
    [600, 2],
    [800, 3],
    [960, 4],
  ] as const) {
    await categoryNavigator.evaluate((element, width) => {
      element.style.width = `${width}px`;
    }, containerWidth);
    await expect.poll(() =>
      categoryNavigatorGrid.evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)
    ).toBe(expectedColumns);
  }

  const categoryGridMetrics = await categoryNavigatorGrid.evaluate((element) => {
    const computed = getComputedStyle(element);
    const gridBounds = element.getBoundingClientRect();
    const itemMetrics = Array.from(element.children).map((item) => {
      const bounds = item.getBoundingClientRect();
      const itemStyle = getComputedStyle(item);
      return {
        top: Math.round(bounds.top),
        right: bounds.right,
        width: bounds.width,
        borderWidths: [
          itemStyle.borderTopWidth,
          itemStyle.borderRightWidth,
          itemStyle.borderBottomWidth,
          itemStyle.borderLeftWidth,
        ],
        backgroundColor: itemStyle.backgroundColor,
      };
    });
    const rowCounts = Array.from(
      itemMetrics.reduce((rows, item) => rows.set(item.top, (rows.get(item.top) ?? 0) + 1), new Map<number, number>())
        .values()
    );
    return {
      backgroundColor: computed.backgroundColor,
      gap: computed.gap,
      padding: computed.padding,
      rowCounts,
      itemWidths: itemMetrics.map((item) => item.width),
      itemBorderWidths: itemMetrics.map((item) => item.borderWidths),
      itemBackgrounds: itemMetrics.map((item) => item.backgroundColor),
      unusedLastRowWidth:
        gridBounds.right - Number.parseFloat(computed.paddingRight) - itemMetrics[itemMetrics.length - 1]!.right,
    };
  });
  expect(categoryGridMetrics.backgroundColor).toBe("rgba(0, 0, 0, 0)");
  expect(categoryGridMetrics.gap).toBe("8px");
  expect(categoryGridMetrics.padding).toBe("8px");
  expect(categoryGridMetrics.rowCounts).toEqual([4, 3]);
  expect(Math.max(...categoryGridMetrics.itemWidths) - Math.min(...categoryGridMetrics.itemWidths)).toBeLessThanOrEqual(1);
  expect(categoryGridMetrics.itemBorderWidths.flat()).toEqual(
    Array(categoryGridMetrics.itemBorderWidths.length * 4).fill("1px")
  );
  expect(categoryGridMetrics.itemBackgrounds).not.toContain("rgba(0, 0, 0, 0)");
  expect(categoryGridMetrics.unusedLastRowWidth).toBeGreaterThan(200);
}

async function prepareCategoryGeometryChecks({ categoryNavigator, page }: { categoryNavigator: Locator; page: Page; }) {
  await categoryNavigator.evaluate((element) => {
    element.style.width = "";
  });

  const keyModuleCard = page.locator(".ui-key-module-card");
  const uncategorizedBlock = page.locator('[data-category-block="uncategorized"]');
  const sourceBlock = page.locator('[data-category-block="EXAMPLE VPN"]');
  const reserveBlock = page.locator('[data-category-block="Резерв"]');
  const sourcePanel = sourceBlock.locator(".ui-key-category-panel");
  const reservePanel = reserveBlock.locator(".ui-key-category-panel");
  const sourceRailContainer = sourceBlock.locator(".ui-key-category-rail");
  const sourceRailLine = sourceBlock.locator(".ui-key-category-rail-line");
  const sourceRailLabel = sourceBlock.locator(".ui-key-category-rail-label");
  const reserveRailLabel = reserveBlock.locator(".ui-key-category-rail-label");
  const sourceRailBadge = sourceBlock.locator(".ui-key-category-rail-badge");
  const reserveRailBadge = reserveBlock.locator(".ui-key-category-rail-badge");

  const expectRailLabelCentered = async (rail: Locator, badge: Locator, panel: Locator) => {
    const bounds = await Promise.all([rail.boundingBox(), badge.boundingBox(), panel.boundingBox()]);
    expect(bounds.every(Boolean)).toBe(true);
    const [railBounds, badgeBounds, panelBounds] = bounds as [
      NonNullable<(typeof bounds)[number]>,
      NonNullable<(typeof bounds)[number]>,
      NonNullable<(typeof bounds)[number]>,
    ];
    const railCenter = railBounds.x + railBounds.width / 2;
    const badgeCenter = badgeBounds.x + badgeBounds.width / 2;
    expect(Math.abs(railCenter - badgeCenter)).toBeLessThanOrEqual(1);
    expect(badgeBounds.x).toBeGreaterThanOrEqual(railBounds.x - 1);
    expect(badgeBounds.x + badgeBounds.width).toBeLessThanOrEqual(railBounds.x + railBounds.width + 1);
    expect(badgeBounds.x).toBeGreaterThan(panelBounds.x + panelBounds.width);
  };

  const expectAlignedCategoryGeometry = async (expectedMarginRight: number) => {
    await expect(keyModuleCard).toHaveCSS("margin-right", `${expectedMarginRight}px`);
    const bounds = await Promise.all([
      uncategorizedBlock.boundingBox(),
      sourcePanel.boundingBox(),
      keyModuleCard.boundingBox(),
      sourceRailContainer.boundingBox(),
    ]);
    expect(bounds.every(Boolean)).toBe(true);
    const [uncategorizedBounds, panelBounds, moduleBounds, railBounds] = bounds as [
      NonNullable<(typeof bounds)[number]>,
      NonNullable<(typeof bounds)[number]>,
      NonNullable<(typeof bounds)[number]>,
      NonNullable<(typeof bounds)[number]>,
    ];
    expect(Math.abs(panelBounds.x - uncategorizedBounds.x)).toBeLessThanOrEqual(1);
    expect(Math.abs(panelBounds.width - uncategorizedBounds.width)).toBeLessThanOrEqual(1);
    expect(Math.abs(railBounds.x - panelBounds.x - panelBounds.width - 32)).toBeLessThanOrEqual(1);
    expect(Math.abs(railBounds.y - panelBounds.y)).toBeLessThanOrEqual(1);
    expect(Math.abs(railBounds.height - panelBounds.height)).toBeLessThanOrEqual(1);
    expect(railBounds.x).toBeGreaterThan(moduleBounds.x + moduleBounds.width);
    expect(Math.round(railBounds.width)).toBe(52);
    await expectRailLabelCentered(sourceRailContainer, sourceRailBadge, sourcePanel);
  };

  await expectAlignedCategoryGeometry(84);
  return { sourceRailLine, sourceRailContainer, expectAlignedCategoryGeometry, keyModuleCard, uncategorizedBlock, sourcePanel, sourceBlock, reservePanel, sourceRailLabel, expectRailLabelCentered, sourceRailBadge, reserveBlock, reserveRailLabel, reserveRailBadge };
}

async function checkCategoryRailViewports({ sourceRailLine, sourceRailContainer, page, expectAlignedCategoryGeometry, keyModuleCard, uncategorizedBlock, sourcePanel, sourceBlock, reservePanel, sourceRailLabel }: { sourceRailLine: Locator; sourceRailContainer: Locator; page: Page; expectAlignedCategoryGeometry: (expectedMarginRight: number) => Promise<void>; keyModuleCard: Locator; uncategorizedBlock: Locator; sourcePanel: Locator; sourceBlock: Locator; reservePanel: Locator; sourceRailLabel: Locator; }) {
  const railTerminalMetrics = await sourceRailLine.evaluate((element) => {
    const line = getComputedStyle(element);
    const before = getComputedStyle(element, "::before");
    const after = getComputedStyle(element, "::after");
    return {
      lineWidth: line.width,
      lineColor: line.backgroundColor,
      beforeWidth: before.width,
      beforeHeight: before.height,
      beforeColor: before.backgroundColor,
      afterWidth: after.width,
      afterHeight: after.height,
      afterColor: after.backgroundColor,
    };
  });
  expect(railTerminalMetrics.lineWidth).toBe("1px");
  expect(railTerminalMetrics.lineColor).not.toBe("rgba(0, 0, 0, 0)");
  expect(railTerminalMetrics.beforeWidth).toBe("28px");
  expect(railTerminalMetrics.beforeHeight).toBe("1px");
  expect(railTerminalMetrics.afterWidth).toBe("28px");
  expect(railTerminalMetrics.afterHeight).toBe("1px");
  expect(railTerminalMetrics.beforeColor).toBe(railTerminalMetrics.lineColor);
  expect(railTerminalMetrics.afterColor).toBe(railTerminalMetrics.lineColor);
  await expect(sourceRailContainer).toHaveAttribute("aria-hidden", "true");
  await expectNoHorizontalOverflow(page);

  await page.setViewportSize({ width: 1920, height: 900 });
  await expectAlignedCategoryGeometry(0);
  await expectNoHorizontalOverflow(page);

  await page.setViewportSize({ width: 1024, height: 900 });
  await expectAlignedCategoryGeometry(84);
  await expect(sourceRailContainer).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.setViewportSize({ width: 768, height: 900 });
  await expect(keyModuleCard).toHaveCSS("margin-right", "0px");
  await expect(sourceRailContainer).toBeHidden();
  const compactCategoryBounds = await Promise.all([
    uncategorizedBlock.boundingBox(),
    sourcePanel.boundingBox(),
  ]);
  expect(compactCategoryBounds.every(Boolean)).toBe(true);
  expect(Math.abs(compactCategoryBounds[0]!.x - compactCategoryBounds[1]!.x)).toBeLessThanOrEqual(1);
  expect(Math.abs(compactCategoryBounds[0]!.width - compactCategoryBounds[1]!.width)).toBeLessThanOrEqual(1);
  await expectNoHorizontalOverflow(page);
  await page.setViewportSize({ width: 1440, height: 900 });

  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.getByRole("button", { name: "Перейти к категории «EXAMPLE VPN»" }).click();
  await expect(sourceBlock).toBeFocused();
  await Promise.all([sourcePanel, reservePanel].map((panel) =>
    panel.evaluate((element) => {
      element.style.minHeight = "900px";
    })
  ));
  await expect(sourceRailLabel).toHaveCSS("position", "sticky");
  await expect(sourceRailLabel).toHaveCSS("top", "80px");
  await expectAlignedCategoryGeometry(84);
}

async function checkStickyCategoryHandoff({ sourceBlock, page, sourceRailLabel, expectRailLabelCentered, sourceRailContainer, sourceRailBadge, sourcePanel, reserveBlock, reserveRailLabel, reserveRailBadge, reservePanel }: { sourceBlock: Locator; page: Page; sourceRailLabel: Locator; expectRailLabelCentered: (rail: Locator, badge: Locator, panel: Locator) => Promise<void>; sourceRailContainer: Locator; sourceRailBadge: Locator; sourcePanel: Locator; reserveBlock: Locator; reserveRailLabel: Locator; reserveRailBadge: Locator; reservePanel: Locator; }) {
  const sourceDocumentTop = await sourceBlock.evaluate((element) =>
    element.getBoundingClientRect().top + window.scrollY
  );
  await page.evaluate((scrollTop) => window.scrollTo(0, scrollTop), sourceDocumentTop + 240);
  await expect.poll(() =>
    sourceRailLabel.evaluate((element) => Math.round(element.getBoundingClientRect().top))
  ).toBe(80);
  await expectRailLabelCentered(sourceRailContainer, sourceRailBadge, sourcePanel);
  const firstCategoryBounds = await Promise.all([
    sourcePanel.evaluate((element) => element.getBoundingClientRect().bottom),
    sourceRailBadge.evaluate((element) => element.getBoundingClientRect().bottom),
  ]);
  expect(firstCategoryBounds[1]).toBeLessThanOrEqual(firstCategoryBounds[0] + 1);

  const reserveDocumentTop = await reserveBlock.evaluate((element) =>
    element.getBoundingClientRect().top + window.scrollY
  );
  await page.evaluate((scrollTop) => window.scrollTo(0, scrollTop), reserveDocumentTop + 120);
  await expect.poll(() =>
    reserveRailLabel.evaluate((element) => Math.round(element.getBoundingClientRect().top))
  ).toBe(80);
  await expectRailLabelCentered(sourceRailContainer, sourceRailBadge, sourcePanel);
  await expectRailLabelCentered(
    reserveBlock.locator(".ui-key-category-rail"),
    reserveRailBadge,
    reservePanel
  );
  const categoryHandoffBounds = await Promise.all([
    sourceRailBadge.evaluate((element) => element.getBoundingClientRect().bottom),
    reservePanel.evaluate((element) => element.getBoundingClientRect().top),
    reserveRailLabel.evaluate((element) => element.getBoundingClientRect().top),
  ]);
  expect(categoryHandoffBounds[0]).toBeLessThanOrEqual(categoryHandoffBounds[1] + 1);
  expect(Math.round(categoryHandoffBounds[2])).toBe(80);

  await Promise.all([sourcePanel, reservePanel].map((panel) =>
    panel.evaluate((element) => {
      element.style.minHeight = "";
    })
  ));
  await page.evaluate(() => {
    window.scrollTo(0, 0);
  });

  const hideSourceCategory = page.getByRole("button", { name: "Скрыть категорию «EXAMPLE VPN»" }).first();
  await hideSourceCategory.click();
  await expect(sourceBlock.getByText("Категория скрыта")).toBeVisible();
  await page.getByRole("button", { name: "Показать категорию «EXAMPLE VPN»" }).first().click();
  await expect(sourceBlock.getByText("Категория скрыта")).toHaveCount(0);

  const categoryIconControls = sourceBlock.locator(".ui-key-category-controls button");
  await expect(categoryIconControls).toHaveCount(4);
  const categoryIconWidths = await categoryIconControls.evaluateAll((controls) =>
    controls.map((control) => Math.round(control.getBoundingClientRect().width))
  );
  expect(new Set(categoryIconWidths)).toEqual(new Set([44]));

  await page.evaluate(() => window.scrollTo(0, 0));
  const firstKeySelection = page.locator(".ui-key-card").first().getByRole("checkbox");
  return { firstKeySelection };
}

async function checkSelectionAndLongLabels({ firstKeySelection, page, longKeyLabel, longSourceName, longClientDisplayName }: { firstKeySelection: Locator; page: Page; longKeyLabel: string; longSourceName: string; longClientDisplayName: string; }) {
  await firstKeySelection.locator("..").click();
  await expect(firstKeySelection).toBeChecked();
  await expect(page.getByText("Выбрано: 1")).toBeVisible();
  await expect(page.getByRole("button", { name: "Внешние источники", exact: true })).toHaveCount(0);
  const bulkEditButton = page.getByRole("button", { name: "Изменить выбранные ключи (1)" });
  const bulkDeleteButton = page.getByRole("button", { name: "Удалить выбранные ключи (1)" });
  await expect(bulkEditButton).toBeVisible();
  await expect(bulkDeleteButton).toBeVisible();
  const bulkButtonSizes = await Promise.all([
    bulkEditButton.evaluate((element) => Math.round(element.getBoundingClientRect().width)),
    bulkDeleteButton.evaluate((element) => Math.round(element.getBoundingClientRect().width)),
  ]);
  expect(bulkButtonSizes).toEqual([44, 44]);

  const longKeyCard = page.locator(".ui-key-card").filter({
    has: page.locator(".ui-key-card-title", { hasText: "Очень длинное название ключа" }),
  });
  const longKeyTitle = longKeyCard.locator(".ui-key-card-title");
  const longKeySource = longKeyCard.locator(".ui-key-card-source");
  const longClientDisplay = page.locator(".ui-key-card-client-name", { hasText: "Обход блокировок" });
  await expect(longKeyTitle).toHaveAttribute("title", longKeyLabel);
  await expect(longKeySource).toHaveAttribute("title", `Источник: ${longSourceName}`);
  await expect(longClientDisplay).toHaveAttribute("title", longClientDisplayName);
  return { longKeyCard, longClientDisplay };
}

async function checkKeyCardLabels({ page, categoryNavigator, longKeyCard, longClientDisplay }: { page: Page; categoryNavigator: Locator; longKeyCard: Locator; longClientDisplay: Locator; }) {
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const toolbarOverflow = await page.locator(".ui-key-module-header").evaluate((element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }));
    expect(toolbarOverflow.scrollWidth).toBeLessThanOrEqual(toolbarOverflow.clientWidth);
    const navigatorOverflow = await categoryNavigator.evaluate((element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }));
    expect(navigatorOverflow.scrollWidth).toBeLessThanOrEqual(navigatorOverflow.clientWidth);
    const compactTextMetrics = await longKeyCard.evaluate((card) => {
      const title = card.querySelector(".ui-key-card-title") as HTMLElement;
      const source = card.querySelector(".ui-key-card-source") as HTMLElement;
      const titleStyle = getComputedStyle(title);
      const sourceStyle = getComputedStyle(source);
      return {
        cardClientWidth: card.clientWidth,
        cardScrollWidth: card.scrollWidth,
        titleClientWidth: title.clientWidth,
        titleScrollWidth: title.scrollWidth,
        titleHeight: title.getBoundingClientRect().height,
        titleLineHeight: Number.parseFloat(titleStyle.lineHeight),
        titleWhiteSpace: titleStyle.whiteSpace,
        titleOverflow: titleStyle.overflow,
        titleTextOverflow: titleStyle.textOverflow,
        sourceClientWidth: source.clientWidth,
        sourceScrollWidth: source.scrollWidth,
        sourceHeight: source.getBoundingClientRect().height,
        sourceLineHeight: Number.parseFloat(sourceStyle.lineHeight),
        sourceWhiteSpace: sourceStyle.whiteSpace,
        sourceOverflow: sourceStyle.overflow,
        sourceTextOverflow: sourceStyle.textOverflow,
      };
    });
    const clientDisplayMetrics = await longClientDisplay.evaluate((element) => {
      const computed = getComputedStyle(element);
      return {
        clientWidth: element.clientWidth,
        scrollWidth: element.scrollWidth,
        height: element.getBoundingClientRect().height,
        lineHeight: Number.parseFloat(computed.lineHeight),
        whiteSpace: computed.whiteSpace,
        overflow: computed.overflow,
        textOverflow: computed.textOverflow,
      };
    });
    expect(compactTextMetrics.cardScrollWidth).toBeLessThanOrEqual(compactTextMetrics.cardClientWidth);
    expect(compactTextMetrics.titleClientWidth).toBeGreaterThan(0);
    expect(compactTextMetrics.titleScrollWidth).toBeGreaterThan(compactTextMetrics.titleClientWidth);
    expect(compactTextMetrics.titleHeight).toBeLessThanOrEqual(compactTextMetrics.titleLineHeight + 1);
    expect(compactTextMetrics.titleWhiteSpace).toBe("nowrap");
    expect(compactTextMetrics.titleOverflow).toBe("hidden");
    expect(compactTextMetrics.titleTextOverflow).toBe("ellipsis");
    expect(compactTextMetrics.sourceClientWidth).toBeGreaterThan(0);
    expect(compactTextMetrics.sourceScrollWidth).toBeGreaterThan(compactTextMetrics.sourceClientWidth);
    expect(compactTextMetrics.sourceHeight).toBeLessThanOrEqual(compactTextMetrics.sourceLineHeight + 1);
    expect(compactTextMetrics.sourceWhiteSpace).toBe("nowrap");
    expect(compactTextMetrics.sourceOverflow).toBe("hidden");
    expect(compactTextMetrics.sourceTextOverflow).toBe("ellipsis");
    expect(clientDisplayMetrics.clientWidth).toBeGreaterThan(0);
    expect(clientDisplayMetrics.scrollWidth).toBeGreaterThan(clientDisplayMetrics.clientWidth);
    expect(clientDisplayMetrics.height).toBeLessThanOrEqual(clientDisplayMetrics.lineHeight + 1);
    expect(clientDisplayMetrics.whiteSpace).toBe("nowrap");
    expect(clientDisplayMetrics.overflow).toBe("hidden");
    expect(clientDisplayMetrics.textOverflow).toBe("ellipsis");
    await expect(longKeyCard.getByRole("status")).toBeVisible();
    await expectNoHorizontalOverflow(page);
  }
}

async function checkBulkSelection({ categoryNavigator, page, firstKeySelection }: { categoryNavigator: Locator; page: Page; firstKeySelection: Locator; }) {
  const mobileCategoryColumns = await categoryNavigator
    .locator(".ui-key-category-nav-grid")
    .evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length);
  expect(mobileCategoryColumns).toBe(1);
  await page.setViewportSize({ width: 1440, height: 900 });

  await firstKeySelection.locator("..").click();
  await expect(firstKeySelection).not.toBeChecked();
  await expect(page.getByRole("button", { name: "Внешние источники", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /Изменить выбранные ключи/ })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /Удалить выбранные ключи/ })).toHaveCount(0);
}
