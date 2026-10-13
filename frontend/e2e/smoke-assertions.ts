import AxeBuilder from "@axe-core/playwright";
import { expect, type Locator, type Page } from "@playwright/test";

export async function expectTechnicalScrollbar(locator: Locator) {
  await locator.page().mouse.move(1, 1);
  const styles = await locator.evaluate((element) => {
    const standard = getComputedStyle(element);
    const scrollbar = getComputedStyle(element, "::-webkit-scrollbar");
    const thumb = getComputedStyle(element, "::-webkit-scrollbar-thumb");
    const root = getComputedStyle(document.documentElement);
    return {
      standardColor: standard.scrollbarColor,
      standardWidth: standard.scrollbarWidth,
      webkitWidth: scrollbar.width,
      webkitHeight: scrollbar.height,
      thumbBackground: thumb.backgroundColor,
      thumbRadius: thumb.borderRadius,
      trackToken: root.getPropertyValue("--scrollbar-track").trim(),
      thumbToken: root.getPropertyValue("--scrollbar-thumb").trim(),
      hoverToken: root.getPropertyValue("--scrollbar-thumb-hover").trim(),
      activeToken: root.getPropertyValue("--scrollbar-thumb-active").trim(),
    };
  });

  expect(styles.standardWidth).toBe("thin");
  expect(styles.webkitWidth).toBe("10px");
  expect(styles.webkitHeight).toBe("10px");
  expect(styles.thumbRadius).toBe("0px");
  expect(styles.trackToken).toBe("#08090a");
  expect(styles.thumbToken).toBe("#282e29");
  expect(styles.hoverToken).toBe("#66862d");
  expect(styles.activeToken).toBe("#b7ff2a");
  expect(styles.standardColor).toContain("rgb(40, 46, 41)");
  // The pseudo-element can remain hovered after the menu-opening click.
  expect(["rgb(40, 46, 41)", "rgb(102, 134, 45)"]).toContain(styles.thumbBackground);
}

export async function expectUniformAccentBorder(control: Locator, frame = control) {
  const readBorderColors = () => frame.evaluate((element) => {
    const computed = getComputedStyle(element);
    return [
      computed.borderTopColor,
      computed.borderRightColor,
      computed.borderBottomColor,
      computed.borderLeftColor,
    ];
  });

  await control.click();
  await expect.poll(() => frame.evaluate((element) => {
    const computed = getComputedStyle(element);
    return [
      computed.borderTopColor,
      computed.borderRightColor,
      computed.borderBottomColor,
      computed.borderLeftColor,
    ];
  })).toEqual(Array(4).fill("rgb(183, 255, 42)"));
  await control.hover();
  await expect.poll(readBorderColors).toEqual(Array(4).fill("rgb(183, 255, 42)"));

  await control.page().keyboard.press("Tab");
  await control.page().keyboard.press("Shift+Tab");
  await expect(control).toBeFocused();
  await expect.poll(readBorderColors).toEqual(Array(4).fill("rgb(183, 255, 42)"));
  await expect.poll(() =>
    frame.evaluate((element) => getComputedStyle(element).outlineStyle)
  ).toBe("solid");
}

export async function expectSingleDividerStructure(locator: Locator) {
  const metrics = await locator.evaluate((element) => {
    const computed = getComputedStyle(element);
    const children = Array.from(element.children).filter(
      (child): child is HTMLElement => child instanceof HTMLElement
    );
    return {
      columnGap: computed.columnGap,
      rowGap: computed.rowGap,
      childBorders: children.slice(0, 3).map((child) => {
        const childStyle = getComputedStyle(child);
        return [
          childStyle.borderTopWidth,
          childStyle.borderRightWidth,
          childStyle.borderBottomWidth,
          childStyle.borderLeftWidth,
        ];
      }),
    };
  });

  expect(metrics.columnGap).toBe("1px");
  expect(metrics.rowGap).toBe("1px");
  expect(metrics.childBorders.length).toBeGreaterThan(1);
  expect(metrics.childBorders.flat()).toEqual(
    Array(metrics.childBorders.length * 4).fill("0px")
  );
}

interface FrameBounds {
  top: number;
  right: number;
  bottom: number;
  left: number;
}

function expectFrameContained(inner: FrameBounds, outer: FrameBounds) {
  expect(inner.top).toBeGreaterThanOrEqual(outer.top);
  expect(inner.right).toBeLessThanOrEqual(outer.right);
  expect(inner.bottom).toBeLessThanOrEqual(outer.bottom);
  expect(inner.left).toBeGreaterThanOrEqual(outer.left);
}

export async function expectEditorFooterContained(dialog: Locator) {
  const frames = await dialog.locator(".ui-key-editor-footer").evaluate((footer) => {
    const footerRect = footer.getBoundingClientRect();
    const modalRect = footer.closest<HTMLElement>('[role="dialog"]')!.getBoundingClientRect();
    return {
      footer: { top: footerRect.top, right: footerRect.right, bottom: footerRect.bottom, left: footerRect.left },
      modal: { top: modalRect.top, right: modalRect.right, bottom: modalRect.bottom, left: modalRect.left },
      buttons: Array.from(footer.querySelectorAll("button"), (button) => {
        const rect = button.getBoundingClientRect();
        return { top: rect.top, right: rect.right, bottom: rect.bottom, left: rect.left };
      }),
    };
  });
  expectFrameContained(frames.footer, frames.modal);
  expect(frames.buttons.length).toBe(2);
  for (const button of frames.buttons) {
    expectFrameContained(button, frames.footer);
    expectFrameContained(button, frames.modal);
  }
}

export async function expectNoHorizontalOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => (
    document.documentElement.scrollWidth - document.documentElement.clientWidth
  ))).toBeLessThanOrEqual(0);
}

export async function expectOverviewStatsFillLastRow(page: Page) {
  const stats = page.getByTestId("overview-stats");
  const metrics = await stats.evaluate((element) => {
    const grid = element.getBoundingClientRect();
    const cards = Array.from(element.children).map((child) => child.getBoundingClientRect());
    const lastRowTop = Math.max(...cards.map((card) => card.top));
    const lastRow = cards.filter((card) => Math.abs(card.top - lastRowTop) <= 1);
    return {
      gridRight: grid.right,
      lastRowRight: Math.max(...lastRow.map((card) => card.right)),
    };
  });

  expect(Math.abs(metrics.gridRight - metrics.lastRowRight)).toBeLessThanOrEqual(2);
}

export async function expectOverviewJoinedAccentAssignments(page: Page) {
  const joinedAccents = page.locator([
    "[data-testid='overview-stats'] > .technical-frame--joined-accent",
    "[data-testid='overview-recent-actions'].technical-frame--joined-accent",
    "[data-testid='overview-operational-status'].technical-frame--joined-accent",
    "[data-testid='overview-background-jobs'].technical-frame--joined-accent",
  ].join(", "));

  await expect(joinedAccents).toHaveCount(8);
  expect(await joinedAccents.evaluateAll((elements) => elements.every((element) =>
    element.parentElement?.matches(".ui-joined-grid, .ui-joined-list")
  ))).toBe(true);
}

export async function expectNoWcagViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(results.violations, results.violations.map((violation) => `${violation.id}: ${violation.help}`).join("\n")).toEqual([]);
}
