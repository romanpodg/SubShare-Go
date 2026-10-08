import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "@/components/ui/Toast";
import { KeyEditorModal } from "./KeyEditorModal";

const api = vi.hoisted(() => ({ get: vi.fn(), reveal: vi.fn(), updateProfile: vi.fn(), clone: vi.fn() }));
vi.mock("@/lib/api", () => ({
  keys: { ...api, editorSchema: vi.fn().mockResolvedValue({ data: { protocols: [], exclusion_reason_codes: {} } }), listCategories: vi.fn().mockResolvedValue({ categories: [] }) },
  ApiError: class extends Error { status = 409; },
}));

function detail(id: number) {
  return {
    id, label: `Profile ${id}`, client_display_name: `Client ${id}`, client_display_name_overridden: true,
    category: "", kind: "real", status: "active", ownership: "local", protocol: "vless", profile_revision: id,
    capabilities: {}, unknown_query_parameters: [],
    safe_structured: { server: `profile${id}.example`, port: "443", port_kind: "single", display_name: `Client ${id}` },
  };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function session() {
  const onClose = vi.fn();
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  const view = (open: boolean, keyId: number) => <ToastProvider><KeyEditorModal open={open} keyId={keyId} onClose={onClose} onRefresh={onRefresh} /></ToastProvider>;
  const result = render(view(true, 1));
  return { ...result, switch: (open: boolean, keyId: number) => result.rerender(view(open, keyId)) };
}

const transitions = [
  { name: "close/reopen a different profile", close: true, nextId: 2 },
  { name: "close/reopen the same profile", close: true, nextId: 1 },
  { name: "switch profiles while open", close: false, nextId: 2 },
];

describe("editor session request lifetime", () => {
  beforeEach(() => { vi.clearAllMocks(); api.get.mockImplementation((id: number) => Promise.resolve({ data: detail(id) })); });

  it.each(transitions)("does not apply an old detail after $name", async ({ close, nextId }) => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.get.mockImplementationOnce(() => pending.promise);
    const editor = session();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith(1));
    if (close) editor.switch(false, 1);
    editor.switch(true, nextId);
    await screen.findByDisplayValue(`Profile ${nextId}`);
    await act(async () => pending.resolve({ data: { ...detail(1), label: "Stale profile" } }));
    expect(screen.getByLabelText("Название")).toHaveValue(`Profile ${nextId}`);
    expect(screen.getByLabelText("Сервер (Host)")).toHaveValue(`profile${nextId}.example`);
  });

  it.each(transitions)("does not apply a late reveal after $name", async ({ close, nextId }) => {
    const pending = deferred<{ data: { raw_uri: string } }>();
    api.reveal.mockReturnValue(pending.promise);
    const editor = session();
    await screen.findByDisplayValue("Profile 1");
    fireEvent.click(screen.getByRole("button", { name: /Раскрыть учетные данные/i }));
    await waitFor(() => expect(api.reveal).toHaveBeenCalledTimes(1));
    if (close) editor.switch(false, 1);
    editor.switch(true, nextId);
    await screen.findByDisplayValue(`Profile ${nextId}`);
    await act(async () => pending.resolve({ data: { raw_uri: "vless://11111111-1111-4111-8111-111111111111@profile1.example:443#First" } }));
    expect(screen.getByLabelText("Сервер (Host)")).toHaveValue(`profile${nextId}.example`);
    expect(screen.getByLabelText("UUID / ID")).toHaveValue("");
    expect(screen.getByLabelText("UUID / ID")).toBeDisabled();
    expect(screen.queryByText("Сырая ссылка раскрыта")).not.toBeInTheDocument();
  });

  it.each(transitions)("withholds a late load failure after $name", async ({ close, nextId }) => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.get.mockImplementationOnce(() => pending.promise);
    const editor = session();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith(1));
    if (close) editor.switch(false, 1);
    editor.switch(true, nextId);
    await screen.findByDisplayValue(`Profile ${nextId}`);
    await act(async () => pending.reject(new Error("old-session load failed")));
    expect(screen.queryByText("old-session load failed")).not.toBeInTheDocument();
  });
});
