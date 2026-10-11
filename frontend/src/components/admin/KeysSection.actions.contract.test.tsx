import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { KeySummary } from "@/lib/types";
import { KeysSection } from "@/components/admin/KeysSection";

const mocks = vi.hoisted(() => ({ listCategories: vi.fn(), bulkDelete: vi.fn(), delete: vi.fn(), reorder: vi.fn(), reorderCategories: vi.fn(), update: vi.fn(), check: vi.fn(), checkAll: vi.fn(), toast: vi.fn() }));
vi.mock("@/lib/api", () => ({ keys: mocks }));
vi.mock("@/components/ui/Toast", () => ({ useToast: () => ({ toast: mocks.toast }) }));
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
beforeEach(() => {
  vi.resetAllMocks();
  mocks.listCategories.mockResolvedValue({ categories: [] });
  mocks.bulkDelete.mockResolvedValue({ deleted: 2 });
  mocks.reorder.mockResolvedValue(undefined);
  mocks.reorderCategories.mockResolvedValue(undefined);
  mocks.update.mockResolvedValue(undefined);
  mocks.delete.mockResolvedValue(undefined);
  mocks.check.mockResolvedValue(undefined);
  mocks.checkAll.mockResolvedValue({ job_id: 42, status: "queued" });
});
function fixtureKey(id: number, label = `key-${id}`): KeySummary {
  return { id, label, kind: "real", protocol: "vless", client_display_name: "", category: "", template_text: "", status: "active", check_status: "unknown", check_error: "", last_latency_ms: 0, last_checked_at: "", external_source_id: 0, external_source_name: "", created_at: "" };
}
function selectKey(label: string) { fireEvent.click(screen.getByRole("checkbox", { name: `Выбрать конфигурацию ${label}` })); }
function openBulkDelete(count: number) { fireEvent.click(screen.getByRole("button", { name: `Удалить выбранные ключи (${count})` })); return screen.getByRole("dialog"); }
function dragCard(source: Pick<KeySummary, "label">, target: Pick<KeySummary, "label">) {
  const sourceCard = cardFor(source);
  const targetCard = cardFor(target);
  fireEvent.dragStart(sourceCard, { dataTransfer: { setData: vi.fn(), effectAllowed: "" } });
  fireEvent.drop(targetCard);
}
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
function cardFor(key: Pick<KeySummary, "label">) { return screen.getByRole("checkbox", { name: `Выбрать конфигурацию ${key.label}` }).closest(".ui-key-card") as HTMLElement; }
function startCardDrag(key: Pick<KeySummary, "label">) { fireEvent.dragStart(cardFor(key), { dataTransfer: { setData: vi.fn(), effectAllowed: "" } }); }
function categoryFixtures() { return [{ name: "A", color: "#d8b33d", keys_count: 1 }, { name: "B", color: "#123456", keys_count: 0 }]; }
async function chooseKeyCategory(name: string) {
  fireEvent.click(screen.getByLabelText("Категория ключа"));
  fireEvent.click(await screen.findByRole("option", { name }));
}
async function openCategoryFixture(key: KeySummary, refresh = vi.fn()) {
  mocks.listCategories.mockResolvedValue({ categories: categoryFixtures() });
  render(<KeysSection keys={[key]} subscriptionFormat="links" onRefresh={refresh} />);
  await screen.findByRole("button", { name: "Редактировать категорию «B»" });
  fireEvent.click(screen.getByRole("button", { name: "Редактировать категорию «A»" }));
  return refresh;
}

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
    dragCard({ label: "visible-one" }, { label: "visible-three" });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить порядок" }));
    // Existing drops insert before the target after removing the source item.
    await waitFor(() => expect(mocks.reorder).toHaveBeenCalledWith([2, 1, 3]));
  });
  it("retains unsaved order across refetch and adopts current props after saving", async () => {
    const refresh = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(<KeysSection keys={[fixtureKey(1), fixtureKey(2), fixtureKey(3)]} subscriptionFormat="links" onRefresh={refresh} />);
    dragCard({ label: "key-3" }, { label: "key-1" });
    rerender(<KeysSection keys={[fixtureKey(1), fixtureKey(2), fixtureKey(3), fixtureKey(4)]} subscriptionFormat="links" onRefresh={refresh} />);
    expect(screen.queryByRole("checkbox", { name: "Выбрать конфигурацию key-4" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Сохранить порядок" }));
    await waitFor(() => expect(mocks.reorder).toHaveBeenCalledWith([3, 1, 2]));
    await waitFor(() => expect(screen.getByRole("checkbox", { name: "Выбрать конфигурацию key-4" })).toBeInTheDocument());
  });
  it("keeps pending order after a failed save and sends it again on retry", async () => {
    mocks.reorder.mockRejectedValueOnce(new Error("fixture order failure"));
    render(<KeysSection keys={[fixtureKey(1), fixtureKey(2), fixtureKey(3)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    dragCard({ label: "key-3" }, { label: "key-1" });
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
  it("cancels a single delete without changing the list or refreshing", () => {
    const refresh = vi.fn();
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={refresh} />);
    fireEvent.click(within(cardFor({ label: "key-1" })).getByRole("button", { name: "Удалить" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Отмена" }));
    expect(mocks.delete).not.toHaveBeenCalled();
    expect(refresh).not.toHaveBeenCalled();
    expect(cardFor({ label: "key-1" })).toBeInTheDocument();
  });
  it("retains a key after single-delete failure and permits retry", async () => {
    mocks.delete.mockRejectedValueOnce(new Error("fixture single-delete failure"));
    const refresh = vi.fn();
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={refresh} />);
    fireEvent.click(within(cardFor({ label: "key-1" })).getByRole("button", { name: "Удалить" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Не удалось удалить конфигурацию", "error"));
    expect(refresh).not.toHaveBeenCalled();
    fireEvent.click(within(cardFor({ label: "key-1" })).getByRole("button", { name: "Удалить" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledTimes(2));
    expect(mocks.delete.mock.calls).toEqual([[1], [1]]);
  });
  it("reports a single health failure without refreshing", async () => {
    mocks.check.mockRejectedValueOnce(new Error("fixture single-health failure"));
    const refresh = vi.fn();
    render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={refresh} />);
    fireEvent.click(within(cardFor({ label: "key-1" })).getByRole("button", { name: "Проверить" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Не удалось проверить конфигурацию", "error"));
    expect(mocks.check).toHaveBeenCalledWith(1);
    expect(refresh).not.toHaveBeenCalled();
  });
  it.each(["drag end", "unmount"])("removes its document drag listeners on %s", async (finish) => {
    const added = vi.spyOn(document, "addEventListener");
    const removed = vi.spyOn(document, "removeEventListener");
    const { unmount } = render(<KeysSection keys={[fixtureKey(1)]} subscriptionFormat="links" onRefresh={vi.fn()} />);
    startCardDrag({ label: "key-1" });
    const registrations = added.mock.calls.filter(([name, , capture]) => ["dragover", "drop", "dragend"].includes(name) && capture === true);
    expect(registrations).toHaveLength(3);
    if (finish === "unmount") unmount();
    else fireEvent.dragEnd(cardFor({ label: "key-1" }));
    for (const [name, listener, capture] of registrations) expect(removed).toHaveBeenCalledWith(name, listener, capture);
  });
  it("submits the complete category order including an empty category while filtered", async () => {
    mocks.listCategories.mockResolvedValue({ categories: categoryFixtures() });
    const refresh = vi.fn();
    render(<KeysSection keys={[{ ...fixtureKey(1), category: "A" }]} subscriptionFormat="links" onRefresh={refresh} />);
    await screen.findByRole("button", { name: "Переместить категорию «A» вниз" });
    fireEvent.change(screen.getByPlaceholderText("Поиск по названию, категории, URL, ID..."), { target: { value: "key-1" } });
    fireEvent.click(screen.getByRole("button", { name: "Переместить категорию «A» вниз" }));
    await waitFor(() => expect(mocks.reorderCategories).toHaveBeenCalledWith(["B", "A"]));
    await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
  });
  it("reports a category-order failure without refreshing", async () => {
    mocks.listCategories.mockResolvedValue({ categories: categoryFixtures() });
    mocks.reorderCategories.mockRejectedValueOnce(new Error("fixture category-order failure"));
    const refresh = vi.fn();
    render(<KeysSection keys={[{ ...fixtureKey(1), category: "A" }]} subscriptionFormat="links" onRefresh={refresh} />);
    fireEvent.click(await screen.findByRole("button", { name: "Переместить категорию «A» вниз" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("fixture category-order failure", "error"));
    expect(refresh).not.toHaveBeenCalled();
  });
  it.each(["B (0)", "Без категории"])("retains the legacy metadata-only move payload for %s", async (option) => {
    const refresh = await openCategoryFixture({ ...fixtureKey(1), category: "A" });
    await chooseKeyCategory(option);
    fireEvent.click(screen.getByRole("button", { name: "Перенести" }));
    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith(1, {
      label: "key-1", status: "active", kind: "real", category: option === "Без категории" ? "" : "B",
      uuid: "", host: "", port: "", query: "", fragment: "", template_text: undefined,
    }));
    await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
  });
  it("keeps informational content when moving a key between categories", async () => {
    await openCategoryFixture({ ...fixtureKey(1), kind: "informational", category: "A", template_text: "fixture information" });
    await chooseKeyCategory("B (0)");
    fireEvent.click(screen.getByRole("button", { name: "Перенести" }));
    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith(1, expect.objectContaining({ kind: "informational", category: "B", template_text: "fixture information" })));
  });
  it("reports a failed key-category move and retains the current key", async () => {
    mocks.update.mockRejectedValueOnce(new Error("fixture move failure"));
    const refresh = await openCategoryFixture({ ...fixtureKey(1), category: "A" });
    await chooseKeyCategory("B (0)");
    fireEvent.click(screen.getByRole("button", { name: "Перенести" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Не удалось перенести ключ", "error"));
    expect(refresh).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Перенести" })).not.toBeDisabled();
  });
});
