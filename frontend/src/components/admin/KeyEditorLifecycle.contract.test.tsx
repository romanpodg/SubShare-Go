import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { KeyEditorModal } from "./KeyEditorModal";

const api = vi.hoisted(() => ({ get: vi.fn(), reveal: vi.fn(), updateProfile: vi.fn(), clone: vi.fn() }));
vi.mock("@/lib/api", () => ({
  keys: { ...api, editorSchema: vi.fn().mockResolvedValue({ data: { protocols: [], exclusion_reason_codes: {} } }), listCategories: vi.fn().mockResolvedValue({ categories: [] }) },
  ApiError: class extends Error {
    constructor(public status: number, public code: string, message: string, public fieldErrors = {}) { super(message); }
  },
}));

function detail(id = 1, revision = 7) {
  return {
    id, label: `Profile ${id}`, client_display_name: `Client ${id}`, client_display_name_overridden: true,
    category: "", kind: "real", status: "active", ownership: "local", protocol: "vless", profile_revision: revision,
    capabilities: {}, unknown_query_parameters: [],
    safe_structured: { server: `profile${id}.example`, port: "443", port_kind: "single", display_name: `Client ${id}` },
  };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => { resolve = yes; });
  return { promise, resolve };
}
function editor() {
  const onClose = vi.fn();
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  const view = (open: boolean, keyId: number) => <ToastProvider><KeyEditorModal open={open} keyId={keyId} onClose={onClose} onRefresh={onRefresh} /></ToastProvider>;
  const rendered = render(view(true, 1));
  return { ...rendered, onClose, onRefresh, switch: (open: boolean, keyId: number) => rendered.rerender(view(open, keyId)) };
}
function reveal() { fireEvent.click(screen.getByRole("button", { name: /Раскрыть учетные данные/i })); }
function save() { fireEvent.click(screen.getByRole("button", { name: "Сохранить" })); }

describe("editor request failure and dialog contracts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.get.mockImplementation((id: number) => Promise.resolve({ data: detail(id) }));
    api.updateProfile.mockResolvedValue({ data: detail() });
    api.clone.mockResolvedValue({ data: detail(3) });
    api.reveal.mockResolvedValue({ data: { raw_uri: "vless://11111111-1111-4111-8111-111111111111@profile1.example:443#First" } });
  });

  it("keeps credentials hidden and permits retry after reveal failure", async () => {
    api.reveal.mockRejectedValueOnce(new Error("reveal failed"));
    editor();
    await screen.findByDisplayValue("Profile 1");
    reveal();
    expect(await screen.findByText("reveal failed")).toBeInTheDocument();
    expect(screen.getByLabelText("UUID / ID")).toHaveValue("");
    expect(screen.getByLabelText("UUID / ID")).toBeDisabled();
    reveal();
    await screen.findByDisplayValue("11111111-1111-4111-8111-111111111111");
    expect(api.reveal).toHaveBeenCalledTimes(2);
  });

  it("refreshes the revision after reveal conflict before another request", async () => {
    api.reveal.mockRejectedValueOnce(new ApiError(409, "profile_revision_conflict", "revision changed"));
    editor();
    await screen.findByDisplayValue("Profile 1");
    reveal();
    await screen.findByRole("dialog", { name: /Конфликт версий/i });
    api.get.mockResolvedValue({ data: { ...detail(1, 9), label: "Latest profile" } });
    fireEvent.click(screen.getByRole("button", { name: "Загрузить свежую версию" }));
    await screen.findByDisplayValue("Latest profile");
    reveal();
    await waitFor(() => expect(api.reveal).toHaveBeenCalledTimes(2));
    expect(api.reveal.mock.calls[1]).toEqual([1, { profile_revision: 9, target: "raw" }]);
  });

  it.each([
    new Error("save network failed"),
    new ApiError(422, "validation_failed", "server rejected input", { server: ["invalid host"] }),
  ])("retains the draft when update fails: $message", async (error) => {
    api.updateProfile.mockRejectedValue(error);
    const session = editor();
    await screen.findByDisplayValue("Profile 1");
    fireEvent.change(screen.getByLabelText("Название"), { target: { value: "Changed label" } });
    save();
    expect(await screen.findByText(error.message)).toBeInTheDocument();
    expect(screen.getByLabelText("Название")).toHaveValue("Changed label");
    expect(session.onRefresh).not.toHaveBeenCalled();
    expect(session.onClose).not.toHaveBeenCalled();
  });

  it("opens update conflict without refreshing or closing", async () => {
    api.updateProfile.mockRejectedValue(new ApiError(409, "profile_revision_conflict", "revision changed"));
    const session = editor();
    await screen.findByDisplayValue("Profile 1");
    save();
    await screen.findByRole("dialog", { name: /Конфликт версий/i });
    expect(session.onRefresh).not.toHaveBeenCalled();
    expect(session.onClose).not.toHaveBeenCalled();
  });

  it("retains the modal if refresh fails after a successful mutation", async () => {
    const session = editor();
    session.onRefresh.mockRejectedValue(new Error("refresh failed"));
    await screen.findByDisplayValue("Profile 1");
    save();
    expect(await screen.findByText("refresh failed")).toBeInTheDocument();
    expect(api.updateProfile).toHaveBeenCalledTimes(1);
    expect(session.onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Сохранить" })).toBeEnabled();
  });

  it("retains the source modal on clone failure", async () => {
    api.get.mockResolvedValue({ data: { ...detail(), ownership: "external_source" } });
    api.clone.mockRejectedValue(new Error("clone failed"));
    const session = editor();
    await screen.findByDisplayValue("Profile 1");
    fireEvent.click(screen.getByRole("button", { name: "Клонировать как локальный" }));
    expect(await screen.findByText("clone failed")).toBeInTheDocument();
    expect(api.clone).toHaveBeenCalledWith(1, { expected_profile_revision: 7 });
    expect(session.onClose).not.toHaveBeenCalled();
  });

  it("supports cancelling then confirming dirty close", async () => {
    const session = editor();
    await screen.findByDisplayValue("Profile 1");
    fireEvent.change(screen.getByLabelText("Название"), { target: { value: "Changed label" } });
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));
    const confirm = await screen.findByRole("dialog", { name: "Закрыть без сохранения?" });
    fireEvent.click(within(confirm).getByRole("button", { name: "Отмена" }));
    expect(session.onClose).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Название")).toHaveValue("Changed label");
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));
    fireEvent.click(within(await screen.findByRole("dialog", { name: "Закрыть без сохранения?" })).getByText("Закрыть"));
    expect(session.onClose).toHaveBeenCalledTimes(1);
  });

  it("withholds repeated submit while its mutation is pending", async () => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.updateProfile.mockReturnValue(pending.promise);
    editor();
    await screen.findByDisplayValue("Profile 1");
    save(); save();
    expect(api.updateProfile).toHaveBeenCalledTimes(1);
    await act(async () => pending.resolve({ data: detail() }));
  });

  it("withholds repeated legacy reveal while its request is pending", async () => {
    const pending = deferred<{ data: { raw_uri: string } }>();
    api.reveal.mockReturnValue(pending.promise);
    editor();
    await screen.findByDisplayValue("Profile 1");
    reveal(); reveal();
    expect(api.reveal).toHaveBeenCalledTimes(1);
    await act(async () => pending.resolve({ data: { raw_uri: "vless://11111111-1111-4111-8111-111111111111@profile1.example:443#First" } }));
  });

  it("refreshes a committed update without closing a different profile session", async () => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.updateProfile.mockReturnValue(pending.promise);
    const session = editor();
    await screen.findByDisplayValue("Profile 1");
    save();
    session.switch(false, 1);
    session.switch(true, 2);
    await screen.findByDisplayValue("Profile 2");
    await act(async () => pending.resolve({ data: detail() }));
    expect(session.onRefresh).toHaveBeenCalledTimes(1);
    expect(session.onClose).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Название")).toHaveValue("Profile 2");
    expect(screen.queryByText("Профиль успешно обновлён")).not.toBeInTheDocument();
  });

  it("refreshes a committed clone without closing a different profile session", async () => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.clone.mockReturnValue(pending.promise);
    api.get.mockImplementation((id: number) => Promise.resolve({ data: { ...detail(id), ownership: "external_source" } }));
    const session = editor();
    await screen.findByDisplayValue("Profile 1");
    fireEvent.click(screen.getByRole("button", { name: "Клонировать как локальный" }));
    session.switch(false, 1);
    session.switch(true, 2);
    await screen.findByDisplayValue("Profile 2");
    await act(async () => pending.resolve({ data: detail(3) }));
    expect(session.onRefresh).toHaveBeenCalledTimes(1);
    expect(session.onClose).not.toHaveBeenCalled();
    expect(screen.queryByText("Локальный клон успешно создан")).not.toBeInTheDocument();
  });
});
