import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeysSection } from "./KeysSection";

const mocks = vi.hoisted(() => ({
  checkAll: vi.fn(),
  listCategories: vi.fn(),
  toast: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  keys: {
    checkAll: mocks.checkAll,
    listCategories: mocks.listCategories,
  },
}));

vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast: mocks.toast }),
}));

describe("KeysSection bulk health check", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listCategories.mockResolvedValue({ categories: [] });
    mocks.checkAll.mockResolvedValue({ job_id: 42, status: "queued" });
  });

  it("announces an asynchronous start without rendering a final count", async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined);
    render(<KeysSection keys={[]} subscriptionFormat="links" onRefresh={onRefresh} />);

    fireEvent.click(screen.getByRole("button", { name: "Проверить все" }));

    await waitFor(() => expect(mocks.checkAll).toHaveBeenCalledOnce());
    expect(mocks.toast).toHaveBeenCalledWith("Проверка конфигураций запущена", "success");
    expect(mocks.toast).not.toHaveBeenCalledWith(expect.stringContaining("undefined"), expect.anything());
    expect(onRefresh).not.toHaveBeenCalled();
  });
});
