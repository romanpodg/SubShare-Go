import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { KeySummary } from "@/lib/types";
import { KeysSection } from "@/components/admin/KeysSection";

const mocks = vi.hoisted(() => ({ listCategories: vi.fn(), bulkDelete: vi.fn(), delete: vi.fn(), reorder: vi.fn(), check: vi.fn(), checkAll: vi.fn(), toast: vi.fn() }));
vi.mock("@/lib/api", () => ({ keys: mocks }));
vi.mock("@/components/ui/Toast", () => ({ useToast: () => ({ toast: mocks.toast }) }));
afterEach(cleanup);
beforeEach(() => {
  vi.resetAllMocks();
  mocks.listCategories.mockResolvedValue({ categories: [] });
  mocks.bulkDelete.mockResolvedValue({ deleted: 2 });
  mocks.reorder.mockResolvedValue(undefined);
  mocks.delete.mockResolvedValue(undefined);
  mocks.check.mockResolvedValue(undefined);
  mocks.checkAll.mockResolvedValue({ job_id: 42, status: "queued" });
});
function fixtureKey(id: number, label = `key-${id}`): KeySummary {
  return { id, label, kind: "real", protocol: "vless", client_display_name: "", category: "", template_text: "", status: "active", check_status: "unknown", check_error: "", last_latency_ms: 0, last_checked_at: "", external_source_id: 0, external_source_name: "", created_at: "" };
}
function selectKey(label: string) { fireEvent.click(screen.getByRole("checkbox", { name: `Выбрать конфигурацию ${label}` })); }
function openBulkDelete(count: number) { fireEvent.click(screen.getByRole("button", { name: `Удалить выбранные ключи (${count})` })); return screen.getByRole("dialog"); }
function dragCard(sourceLabel: string, targetLabel: string) {
  const source = screen.getByRole("checkbox", { name: `Выбрать конфигурацию ${sourceLabel}` }).closest(".ui-key-card")!;
  const target = screen.getByRole("checkbox", { name: `Выбрать конфигурацию ${targetLabel}` }).closest(".ui-key-card")!;
  fireEvent.dragStart(source, { dataTransfer: { setData: vi.fn(), effectAllowed: "" } });
  fireEvent.drop(target);
}
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }

describe("KeysSection action and order contracts", () => {
  it("cancels deletion without a request and retains selection", async () => {
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    selectKey("key-1");
    const dialog = openBulkDelete(1);
    fireEvent.click(within(dialog).getByRole("button", { name: "Отмена" }));
    expect(mocks.bulkDelete).not.toHaveBeenCalled();
    expect(screen.getByRole("checkbox", { name: "Выбрать конфигурацию key-1" })).toBeChecked();
  });
  it("sends selected IDs even when filtering hides a selected item", async () => {
    render(<KeysSection keys={[fixtureKey(1), fixtureKey(2)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    selectKey("key-1"); selectKey("key-2");
    fireEvent.change(screen.getByPlaceholderText("Поиск по названию, категории, URL, ID..."), { target: { value: "key-1" } });
    expect(screen.queryByRole("checkbox", { name: "Выбрать конфигурацию key-2" })).not.toBeInTheDocument();
    fireEvent.click(within(openBulkDelete(2)).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.bulkDelete).toHaveBeenCalledWith([1, 2]));
  });
  it("retains selection after failed deletion and permits a later retry", async () => {
    mocks.bulkDelete.mockRejectedValueOnce(new Error("fixture delete failure"));
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    selectKey("key-1"); fireEvent.click(within(openBulkDelete(1)).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("fixture delete failure", "error"));
    expect(screen.getByRole("checkbox", { name: "Выбрать конфигурацию key-1" })).toBeChecked();
    fireEvent.click(within(openBulkDelete(1)).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.bulkDelete).toHaveBeenCalledTimes(2));
  });
  it("blocks a repeated confirm while deletion is pending", async () => {
    const gate = deferred<{ deleted: number }>(); mocks.bulkDelete.mockReturnValue(gate.promise);
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    selectKey("key-1"); const button = within(openBulkDelete(1)).getByRole("button", { name: "Удалить" });
    fireEvent.click(button); expect(button).toBeDisabled(); fireEvent.click(button);
    expect(mocks.bulkDelete).toHaveBeenCalledOnce();
    await act(async () => { gate.resolve({ deleted: 1 }); });
  });
  it("uses the reported deletion count and clears the captured selection", async () => {
    mocks.bulkDelete.mockResolvedValue({ deleted: 1 });
    const refresh = vi.fn().mockResolvedValue(undefined);
    render(<KeysSection keys={[fixtureKey(1), fixtureKey(2)]} subscriptionFormat="links" onRefresh={refresh} />);
    selectKey("key-1"); selectKey("key-2");
    fireEvent.click(within(openBulkDelete(2)).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Конфигурация удалена", "success"));
    expect(refresh).toHaveBeenCalledOnce();
    expect(screen.queryByRole("button", { name: "Удалить выбранные ключи (2)" })).not.toBeInTheDocument();
  });
  it("saves the full ID order while the visible list is filtered", async () => {
    render(<KeysSection keys={[fixtureKey(1, "visible-one"), fixtureKey(2, "hidden-two"), fixtureKey(3, "visible-three")]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText("Поиск по названию, категории, URL, ID..."), { target: { value: "visible" } });
    dragCard("visible-one", "visible-three");
    fireEvent.click(screen.getByRole("button", { name: "Сохранить порядок" }));
    // Existing drops insert before the target after removing the source item.
    await waitFor(() => expect(mocks.reorder).toHaveBeenCalledWith([2, 1, 3]));
  });
  it("retains unsaved order across refetch and adopts current props after saving", async () => {
    const refresh = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(<KeysSection keys={[fixtureKey(1), fixtureKey(2), fixtureKey(3)]} subscriptionFormat="links" onRefresh={refresh} />);
    dragCard("key-3", "key-1");
    rerender(<KeysSection keys={[fixtureKey(1), fixtureKey(2), fixtureKey(3), fixtureKey(4)]} subscriptionFormat="links" onRefresh={refresh} />);
    expect(screen.queryByRole("checkbox", { name: "Выбрать конфигурацию key-4" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Сохранить порядок" }));
    await waitFor(() => expect(mocks.reorder).toHaveBeenCalledWith([3, 1, 2]));
    await waitFor(() => expect(screen.getByRole("checkbox", { name: "Выбрать конфигурацию key-4" })).toBeInTheDocument());
  });
  it("keeps pending order after a failed save and sends it again on retry", async () => {
    mocks.reorder.mockRejectedValueOnce(new Error("fixture order failure"));
    render(<KeysSection keys={[fixtureKey(1), fixtureKey(2), fixtureKey(3)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    dragCard("key-3", "key-1");
    const save = screen.getByRole("button", { name: "Сохранить порядок" });
    fireEvent.click(save);
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Не удалось изменить порядок конфигураций", "error"));
    expect(save).not.toBeDisabled();
    fireEvent.click(save);
    await waitFor(() => expect(mocks.reorder).toHaveBeenCalledTimes(2));
    expect(mocks.reorder.mock.calls).toEqual([[[3, 1, 2]], [[3, 1, 2]]]);
  });
  it("reports health launch failure without refreshing or claiming completion", async () => {
    mocks.checkAll.mockRejectedValueOnce(new Error("fixture health failure"));
    const refresh = vi.fn();
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={refresh} />);
    fireEvent.click(screen.getByRole("button", { name: "Проверить все" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Не удалось проверить конфигурации", "error"));
    expect(refresh).not.toHaveBeenCalled();
  });
  it("prunes removed IDs when refreshed keys replace the visible data", async () => {
    const refresh = vi.fn();
    const { rerender } = render(<KeysSection keys={[fixtureKey(1), fixtureKey(2)]} subscriptionFormat="links" onRefresh={refresh} />);
    selectKey("key-1"); selectKey("key-2");
    rerender(<KeysSection keys={[fixtureKey(2)]} subscriptionFormat="links" onRefresh={refresh} />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Удалить выбранные ключи (1)" })).toBeInTheDocument());
    fireEvent.click(within(openBulkDelete(1)).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.bulkDelete).toHaveBeenCalledWith([2]));
  });
  it("selects and unselects only filtered IDs while retaining hidden selection", async () => {
    render(<KeysSection keys={[fixtureKey(1), fixtureKey(2)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    selectKey("key-2");
    fireEvent.change(screen.getByPlaceholderText("Поиск по названию, категории, URL, ID..."), { target: { value: "key-1" } });
    fireEvent.click(screen.getByRole("checkbox", { name: "Выбрать все конфигурации" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Снять выбор со всех конфигураций" }));
    fireEvent.click(within(openBulkDelete(1)).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.bulkDelete).toHaveBeenCalledWith([2]));
  });
});
