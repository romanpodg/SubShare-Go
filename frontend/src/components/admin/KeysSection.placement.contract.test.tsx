import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ComponentProps } from "react";
import type { KeySummary } from "@/lib/types";
import { KeysSection } from "./KeysSection";

const mocks = vi.hoisted(() => ({ listCategories: vi.fn(), listSummaries: vi.fn(), reorder: vi.fn(), toast: vi.fn() }));
vi.mock("@/lib/api", () => ({ keys: mocks }));
vi.mock("@/components/ui/Toast", () => ({ useToast: () => ({ toast: mocks.toast }) }));
type EditorProps = ComponentProps<typeof import("./KeyEditorModal").KeyEditorModal>;
// Keep AddKeyModal's real completion/refresh adapter; replace only the already
// separately characterized editor persistence boundary.
vi.mock("./KeyEditorModal", () => ({ KeyEditorModal: (props: EditorProps) => props.open ? (
  <div role="dialog" aria-label="fixture key editor">
    <output data-testid="fixture-category">{props.initialCategory || ""}</output>
    <output data-testid="fixture-kind">{props.initialKind || "real"}</output>
    <button onClick={() => void props.onRefresh()}>fixture create completed</button>
  </div>
) : null }));

function placementKey(id: number, category: string): KeySummary {
  return { id, category, label: `key-${id}`, kind: "real", client_display_name: "", template_text: "", status: "active", check_status: "unknown", check_error: "", last_latency_ms: 0, last_checked_at: "", external_source_id: 0, external_source_name: "", created_at: "" };
}
const initialKeys = [placementKey(1, "A"), placementKey(2, "B")];
beforeEach(() => {
  vi.resetAllMocks(); vi.useFakeTimers();
  mocks.listCategories.mockResolvedValue({ categories: [] });
  mocks.listSummaries.mockResolvedValue({ data: [...initialKeys, placementKey(3, "A")] });
  mocks.reorder.mockResolvedValue(undefined);
});
afterEach(() => { cleanup(); vi.useRealTimers(); });

async function completePlacement() {
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "fixture create completed" })); });
  await act(async () => { await vi.advanceTimersByTimeAsync(100); });
}
function clickCategoryAdd(kind: "real" | "informational") {
  const block = document.querySelector('[data-category-block="A"]') as HTMLElement;
  const column = block.querySelector(kind === "real" ? ".ui-key-order-column-real" : ".ui-key-order-column-info") as HTMLElement;
  fireEvent.click(within(column).getByRole("button", { name: "Добавить" }));
}

describe("KeysSection creation placement", () => {
  it.each(["real", "informational"] as const)("passes the category and %s kind while using the complete order", async (kind) => {
    const refresh = vi.fn().mockResolvedValue(undefined);
    render(<KeysSection keys={initialKeys} subscriptionFormat="links" onRefresh={refresh} />);
    clickCategoryAdd(kind);
    expect(screen.getByTestId("fixture-category")).toHaveTextContent("A");
    expect(screen.getByTestId("fixture-kind")).toHaveTextContent(kind);
    await completePlacement();
    expect(mocks.listSummaries).toHaveBeenCalledWith({ page_size: 200 });
    expect(mocks.reorder).toHaveBeenCalledWith([1, 2, 3]);
    expect(refresh).toHaveBeenCalledTimes(2);
  });
  it("uses an uncategorized real key for the global add control", async () => {
    render(<KeysSection keys={initialKeys} subscriptionFormat="links" onRefresh={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Добавить конфигурацию" }));
    expect(screen.getByTestId("fixture-category")).toBeEmptyDOMElement();
    expect(screen.getByTestId("fixture-kind")).toHaveTextContent("real");
    await completePlacement();
    expect(mocks.reorder).toHaveBeenCalledWith([1, 2, 3]);
  });
  it("retains the successful creation refresh when optional placement fails", async () => {
    mocks.reorder.mockRejectedValueOnce(new Error("fixture placement failure"));
    const refresh = vi.fn().mockResolvedValue(undefined);
    render(<KeysSection keys={initialKeys} subscriptionFormat="links" onRefresh={refresh} />);
    clickCategoryAdd("real"); await completePlacement();
    expect(refresh).toHaveBeenCalledOnce();
    expect(mocks.toast).not.toHaveBeenCalled();
  });
  it("leaves creation successful when the optional fresh-list read fails", async () => {
    mocks.listSummaries.mockRejectedValueOnce(new Error("fixture fresh-list failure"));
    const refresh = vi.fn().mockResolvedValue(undefined);
    render(<KeysSection keys={initialKeys} subscriptionFormat="links" onRefresh={refresh} />);
    clickCategoryAdd("real"); await completePlacement();
    expect(refresh).toHaveBeenCalledOnce();
    expect(mocks.reorder).not.toHaveBeenCalled();
  });
});
