import type { Locator, Page } from "@playwright/test";
import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow, expectOverviewJoinedAccentAssignments, expectOverviewStatsFillLastRow, expectSingleDividerStructure } from "./smoke-assertions";

test("authenticated overview renders dashboard data without runtime errors", async ({ page }) => {
  const { warningStatus, runtimeErrors } = await mountOverview({ page });
  await checkOverviewCards({ warningStatus, page, runtimeErrors });
});

async function mountOverview({ page }: { page: Page; }) {
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
  return { warningStatus, runtimeErrors };
}

async function checkOverviewCards({ warningStatus, page, runtimeErrors }: { warningStatus: Locator; page: Page; runtimeErrors: string[]; }) {
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
}
