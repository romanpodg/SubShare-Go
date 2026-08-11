import { expect, test, type Locator, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

async function expectTechnicalScrollbar(locator: Locator) {
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

async function expectUniformAccentBorder(control: Locator, frame = control) {
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

async function expectSingleDividerStructure(locator: Locator) {
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

async function expectEditorFooterContained(dialog: Locator) {
  const containment = await dialog.locator(".ui-key-editor-footer").evaluate((footer) => {
    const footerRect = footer.getBoundingClientRect();
    const modalRect = footer.closest<HTMLElement>('[role="dialog"]')!.getBoundingClientRect();
    const buttons = Array.from(footer.querySelectorAll("button")).map((button) => {
      const rect = button.getBoundingClientRect();
      return {
        insideFooter:
          rect.top >= footerRect.top &&
          rect.right <= footerRect.right &&
          rect.bottom <= footerRect.bottom &&
          rect.left >= footerRect.left,
        insideModal:
          rect.top >= modalRect.top &&
          rect.right <= modalRect.right &&
          rect.bottom <= modalRect.bottom &&
          rect.left >= modalRect.left,
      };
    });
    return {
      footerInsideModal:
        footerRect.top >= modalRect.top &&
        footerRect.right <= modalRect.right &&
        footerRect.bottom <= modalRect.bottom &&
        footerRect.left >= modalRect.left,
      buttons,
    };
  });

  expect(containment.footerInsideModal).toBe(true);
  expect(containment.buttons.length).toBe(2);
  expect(containment.buttons.every(({ insideFooter, insideModal }) => insideFooter && insideModal)).toBe(true);
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => (
    document.documentElement.scrollWidth - document.documentElement.clientWidth
  ))).toBeLessThanOrEqual(0);
}

async function expectOverviewStatsFillLastRow(page: Page) {
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

async function expectOverviewJoinedAccentAssignments(page: Page) {
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

async function expectNoWcagViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(results.violations, results.violations.map((violation) => `${violation.id}: ${violation.help}`).join("\n")).toEqual([]);
}

async function openKeysPage(page: Page) {
  await page.setViewportSize({ width: 1440, height: 900 });
  const longKeyLabel = "Очень длинное название ключа 👇 без гарантии, с дополнительным описанием маршрута и региона подключения";
  const longSourceName = "EXAMPLE VPN | @examplebot — резервный внешний источник с очень длинным служебным названием для проверки синхронизации импортированных конфигураций и резервных маршрутов";
  const longClientDisplayName = "🔴 Обход блокировок без гарантии — резервный маршрут через Амстердам с дополнительным описанием подключения";

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
  await page.route("**/api/v1/subscription-settings", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        title: "SubShare",
        refresh_hours: 24,
        info_url: "",
        extra_url: "",
        extra_status: "down",
        subscription_format: "links",
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
  await page.route("**/api/v1/key-categories", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          { id: 1, name: "EXAMPLE VPN", color: "#22b8cf", keys_count: 1 },
          { id: 2, name: "Резерв", color: "#f59e0b", keys_count: 0 },
          { id: 3, name: "Авто", color: "#8b5cf6", keys_count: 0 },
          { id: 4, name: "Игровой", color: "#22c55e", keys_count: 0 },
          { id: 5, name: "Хистерия", color: "#eab308", keys_count: 0 },
          {
            id: 6,
            name: "Очень длинное название дополнительной категории",
            color: "#06b6d4",
            keys_count: 0,
          },
        ],
      }),
    })
  );
  const handleKeysRoute = (route: import("@playwright/test").Route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          {
            id: 11,
            label: longClientDisplayName,
            client_display_name: longClientDisplayName,
            category: "EXAMPLE VPN",
            kind: "real",
            template_text: "",
            status: "non-active",
            check_status: "up",
            check_error: "",
            last_latency_ms: 38,
            last_checked_at: "2026-07-29T10:00:00Z",
            external_source_id: 7,
            external_source_name: longSourceName,
            protocol: "vless",
            created_at: "2026-07-29T10:00:00Z",
          },
          {
            id: 12,
            label: longKeyLabel,
            client_display_name: "",
            category: "",
            kind: "informational",
            template_text: "Service window: 03:00 UTC",
            status: "active",
            check_status: "unknown",
            check_error: "",
            last_latency_ms: 0,
            last_checked_at: "",
            external_source_id: 7,
            external_source_name: longSourceName,
            created_at: "2026-07-29T10:00:00Z",
          },
        ],
        meta: { page: 1, page_size: 50, total: 2, total_pages: 1 },
      }),
    });
  await page.route("**/api/v1/keys?*", handleKeysRoute);
  await page.route("**/api/v1/keys", handleKeysRoute);
  await page.route("**/api/v1/keys/check-all", (route) =>
    route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ job_id: 42, status: "queued" }),
    })
  );
  await page.route("**/api/v1/key-editor-schema", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ data: { protocols: [], exclusion_reason_codes: {} } }),
    })
  );
  await page.route("**/api/v1/keys/11", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          id: 11,
          label: longClientDisplayName,
          client_display_name: longClientDisplayName,
          client_display_name_overridden: true,
          category_id: 1,
          category: "EXAMPLE VPN",
          kind: "real",
          status: "non-active",
          check_status: "up",
          check_error: { code: "", message: "" },
          last_latency_ms: 38,
          last_checked_at: "2026-07-29T10:00:00Z",
          template_text: "",
          ownership: "external_source",
          external_source_id: 7,
          external_source_name: longSourceName,
          protocol: "vless",
          profile_schema_version: 1,
          profile_compatibility: "supported",
          profile_warnings: [],
          profile_revision: 1,
          created_at: "2026-07-29T10:00:00Z",
          updated_at: "2026-07-29T10:00:00Z",
          unknown_query_parameters: [],
          capabilities: {},
        },
      }),
    })
  );

  await page.goto("/admin/keys");
  await expect(page.locator(".ui-key-card")).toHaveCount(2);
  return { longKeyLabel, longSourceName, longClientDisplayName };
}

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

test("authenticated overview renders dashboard data without runtime errors", async ({ page }) => {
  const runtimeErrors: string[] = [];
  page.on("pageerror", (error) => runtimeErrors.push(error.message));
  await page.route("**/api/auth/me", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ username: "owner", role: "owner", csrf_token: "test" }),
    })
  );
  await page.route("**/api/v1/build-info", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ version: "test", commit: "abc", build_time: "now", schema_version: 5 }),
    })
  );
  await page.route("**/api/v1/dashboard", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        users: { total: 2, active: 1, expired: 1, paused: 0, blocked: 0, limited: 0 },
        keys: { total: 3, up: 2, down: 1, unknown: 0 },
        sources: { total: 1, errors: 0 },
        devices: 1,
        backup: { enabled: true, status: "healthy", file: "backup.db", last_modified: "2026-07-28T00:00:00Z", size_bytes: 100 },
        recent_audit_events: [],
        degraded_sections: [],
      }),
    })
  );
  await page.route("**/api/v1/jobs**", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [{
          id: 17,
          kind: "keys_health_check",
          status: "succeeded_with_warnings",
          target_type: "key",
          target_id: "all",
          error_message: "",
          result_counts: { total_selected: 100, checked: 100, healthy: 80, unhealthy: 20, persist_failed: 21 },
          run_after: null,
          started_at: "2026-08-10T23:00:00Z",
          finished_at: "2026-08-10T23:01:00Z",
          created_at: "2026-08-10T23:00:00Z",
        }],
        meta: { page: 1, page_size: 6, total: 1, total_pages: 1 },
      }),
    })
  );
  await page.goto("/admin/overview");
  await expect(page.getByText("2/3")).toBeVisible();
  await expect(page.getByText("готова")).toBeVisible();
  await page.setViewportSize({ width: 1920, height: 1080 });
  const warningStatus = page.locator(".ui-background-job-status");
  await expect(warningStatus).toHaveText("Завершено с предупреждениями");
  const warningStatusMetrics = await warningStatus.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
    height: element.getBoundingClientRect().height,
    lineHeight: Number.parseFloat(getComputedStyle(element).lineHeight),
    whiteSpace: getComputedStyle(element).whiteSpace,
  }));
  expect(warningStatusMetrics.whiteSpace).toBe("nowrap");
  expect(warningStatusMetrics.scrollWidth).toBeLessThanOrEqual(warningStatusMetrics.clientWidth);
  expect(warningStatusMetrics.height).toBeLessThanOrEqual(warningStatusMetrics.lineHeight + 1);
  await expectSingleDividerStructure(page.locator(".ui-joined-grid").first());
  for (const viewport of [
    { width: 1920, height: 1080 },
    { width: 1440, height: 900 },
    { width: 1024, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    await expectOverviewStatsFillLastRow(page);
    await expectOverviewJoinedAccentAssignments(page);
    await expectNoHorizontalOverflow(page);
  }
  expect(runtimeErrors).toEqual([]);
});

test("user HWID list handles an empty device response", async ({ page }) => {
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
  expect(telegramInputChrome.boxShadow).toBe("none");
  await addUserDialog.getByRole("button", { name: "Закрыть" }).click();

  await page.getByRole("button", { name: "Открыть", exact: true }).click();
  await page.getByRole("tab", { name: "Устройства" }).click();

  await expect(page.getByText("Устройства ещё не подключались")).toBeVisible();
  await expect(page.getByRole("button", { name: "Управление HWID" })).toBeVisible();
  expect(runtimeErrors).toEqual([]);
});

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

test("key-list controls, alignment, and labels remain readable across viewports", async ({ page }) => {
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
});

test("key editor remains usable and responsive", async ({ page }) => {
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
});

test("key drag autoscrolls the page near the viewport edge", async ({ page }) => {
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
});

test("subscription settings section icons stay square and responsive", async ({ page }) => {
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
  await page.route("**/api/v1/routing-settings", async (route) => {
    if (route.request().method() === "PUT") {
      const body = route.request().postDataJSON() as { delivery_mode: string };
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
  expect(routingDeliveryMode).toBe("onadd");
  expect(runtimeErrors).toEqual([]);
});

test("owner can preview and create a source in the responsive drawer", async ({ page }) => {
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
  await page.route("**/api/v1/source-categories", (route) =>
    route.fulfill({ contentType: "application/json", body: JSON.stringify({ data: [] }) })
  );
  await page.route("**/api/v1/key-categories", (route) =>
    route.fulfill({ contentType: "application/json", body: JSON.stringify({ data: [] }) })
  );
  await page.route("**/api/v1/sources?**", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        data: [],
        meta: { page: 1, page_size: 20, total: 0, total_pages: 0 },
      }),
    })
  );
  await page.route("**/api/v1/sources/preview", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        source_url: "https://provider.example/sub",
        suggested_name: "Provider",
        detected_format: "links",
        key_count: 1,
        metadata: {},
        warnings: [],
        keys: [{ label: "Edge", scheme: "vless", url_short: "vless://•••" }],
      }),
    })
  );
  await page.route("**/api/v1/sources", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        data: { id: 1, name: "Provider" },
        imported_count: 1,
        skipped_count: 0,
        warnings: [],
        detected_format: "links",
      }),
    });
  });

  await page.goto("/admin/sources");
  const toolbarControls = page.locator(".ui-toolbar").locator("input, select, button");
  await expect(toolbarControls).toHaveCount(5);
  const toolbarHeights = await toolbarControls.evaluateAll((controls) =>
    controls.map((control) => Math.round(control.getBoundingClientRect().height))
  );
  expect(new Set(toolbarHeights)).toEqual(new Set([44]));
  await expect(page.locator(".ui-data-table")).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.getByRole("button", { name: "Добавить источник" }).click();
  const drawer = page.getByRole("dialog");
  await expect(drawer).toBeVisible();
  const box = await drawer.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(page.viewportSize()?.width ?? 1920);

  await page.getByLabel("URL подписки").fill("https://provider.example/sub");
  await page.getByRole("button", { name: "Проверить источник" }).click();
  await expect(page.getByLabel("Название")).toHaveValue("Provider");
  await page.getByRole("button", { name: "Проверить параметры" }).click();
  await expect(page.getByText("Edge")).toBeVisible();
  await page.getByRole("button", { name: "Добавить источник" }).last().click();
  await expect(drawer).toBeHidden();
  expect(runtimeErrors).toEqual([]);
});

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
    await page.setViewportSize(viewport);
    await page.evaluate(() => document.fonts.ready);

    const globe = page.getByRole("img", { name: "Цифровой глобус сети SubShare" });
    await expect(globe).toBeVisible();
    await expect(globe).toHaveAttribute("data-motion", "static");
    await expect.poll(() =>
      globe.locator("canvas").evaluate((element) => {
        const canvas = element as HTMLCanvasElement;
        const context = canvas.getContext("2d");
        if (!context || canvas.width === 0 || canvas.height === 0) return 0;
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
        contained:
          parentBounds !== undefined &&
          bounds.left >= parentBounds.left &&
          bounds.right <= parentBounds.right &&
          bounds.top >= parentBounds.top &&
          bounds.bottom <= parentBounds.bottom,
      };
    });
    expect(globeBounds.width).toBeGreaterThan(250);
    expect(Math.abs(globeBounds.width - globeBounds.height)).toBeLessThanOrEqual(1);
    expect(globeBounds.contained).toBe(true);

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
});
