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
});
