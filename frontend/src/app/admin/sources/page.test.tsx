import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SourcesPage from "./page";

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  categories: vi.fn(),
  keyCategories: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  apiV1: {
    sources: {
      list: mocks.list,
      categories: mocks.categories,
      keyCategories: mocks.keyCategories,
    },
  },
}));
vi.mock("@/hooks/useAuth", () => ({
  useAuth: () => ({ role: "owner" }),
}));
vi.mock("@/components/admin/SourceCreateDrawer", () => ({
  SourceCreateDrawer: ({ open }: { open: boolean }) =>
    open ? <div role="dialog">Новый внешний источник</div> : null,
}));
vi.mock("@/components/admin/SourceDetailDrawer", () => ({
  SourceDetailDrawer: () => null,
}));
vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast: vi.fn() }),
}));

describe("SourcesPage", () => {
  beforeEach(() => {
    mocks.list.mockResolvedValue({
      data: [
        {
          id: 1,
          name: "Remote edge",
          category: "Main",
          key_category: "Edge",
          source_url_masked: "https://provider.example/•••",
          enabled: true,
          pass_hwid: false,
          has_hwid_value: false,
          import_status: "ok",
          last_error: "",
          imported_keys: 3,
          last_import_count: 3,
          last_synced_at: "2026-07-28 10:00:00",
          created_at: "2026-07-28 10:00:00",
          updated_at: "2026-07-28 10:00:00",
        },
      ],
      meta: { page: 1, page_size: 20, total: 1, total_pages: 1 },
    });
    mocks.categories.mockResolvedValue({ data: [] });
    mocks.keyCategories.mockResolvedValue({ data: [] });
  });

  it("renders safe summaries and opens the dedicated create drawer", async () => {
    render(<SourcesPage />);

    expect(await screen.findByText("Remote edge")).toBeInTheDocument();
    expect(screen.getByText("https://provider.example/•••")).toBeInTheDocument();
    expect(screen.queryByText(/secret-token/)).not.toBeInTheDocument();
    expect(screen.queryByText(/access=private/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Добавить источник/i }));
    expect(screen.getByRole("dialog")).toHaveTextContent("Новый внешний источник");
  });
});
