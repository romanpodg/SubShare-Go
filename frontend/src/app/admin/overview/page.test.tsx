import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import OverviewPage from "./page";

const mocks = vi.hoisted(() => ({
  dashboard: vi.fn(),
  jobsList: vi.fn(),
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    apiV1: {
      dashboard: mocks.dashboard,
      jobs: {
        list: mocks.jobsList,
        retry: vi.fn(),
      },
    },
  };
});
vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast: vi.fn() }),
}));

describe("OverviewPage", () => {
  beforeEach(() => {
    mocks.dashboard.mockResolvedValue({
      users: { total: 5, active: 2, expired: 1, paused: 1, blocked: 0, limited: 1 },
      keys: { total: 2, up: 1, down: 0, unknown: 1 },
      sources: { total: 3, errors: 0 },
      devices: 1,
      backup: {
        enabled: false,
        status: "disabled",
        file: "",
        last_modified: "",
        size_bytes: 0,
      },
      recent_audit_events: [],
      degraded_sections: [],
    });
    mocks.jobsList.mockRejectedValue(new Error("jobs temporarily unavailable"));
  });

  it("keeps dashboard data visible when jobs fail", async () => {
    render(<OverviewPage />);

    expect(await screen.findByText("5")).toBeInTheDocument();
    expect(screen.getByText("1/2")).toBeInTheDocument();
    expect(screen.getByText("jobs temporarily unavailable")).toBeInTheDocument();
    expect(screen.queryByText(/Не удалось загрузить обзор/i)).not.toBeInTheDocument();
  });

  it("renders partial health persistence as a completed warning", async () => {
    mocks.jobsList.mockResolvedValue({
      data: [{
        id: 17,
        kind: "keys_health_check",
        status: "succeeded_with_warnings",
        target_type: "key",
        target_id: "all",
        error_message: "",
        result_counts: {
          total_selected: 100,
          checked: 100,
          healthy: 80,
          unhealthy: 20,
          check_failed: 0,
          skipped_disabled: 0,
          skipped_unsupported: 0,
          persisted_ok: 79,
          persist_failed: 21,
        },
        run_after: null,
        started_at: "2026-08-10T23:00:00Z",
        finished_at: "2026-08-10T23:01:00Z",
        created_at: "2026-08-10T23:00:00Z",
      }],
      meta: { page: 1, page_size: 6, total: 1, total_pages: 1 },
    });

    render(<OverviewPage />);

    const status = await screen.findByText("Завершено с предупреждениями");
    expect(status).toHaveClass("ui-background-job-status", "lg:whitespace-nowrap");
    expect(status.parentElement).toHaveClass("lg:grid-cols-[180px_240px_minmax(0,1fr)_auto]");
    expect(screen.getByText("Не удалось сохранить результаты: 21")).toBeInTheDocument();
    expect(screen.queryByText("Ошибка")).not.toBeInTheDocument();
  });
});
