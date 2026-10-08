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

describe("editor session request lifetime", () => {
  beforeEach(() => { vi.clearAllMocks(); api.get.mockImplementation((id: number) => Promise.resolve({ data: detail(id) })); });

  it("does not apply an old detail after closing and opening a different profile", async () => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.get.mockImplementation((id: number) => id === 1 ? pending.promise : Promise.resolve({ data: detail(id) }));
    const editor = session();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith(1));
    editor.switch(false, 1);
    editor.switch(true, 2);
    await screen.findByDisplayValue("Profile 2");
    await act(async () => pending.resolve({ data: detail(1) }));
    expect(screen.getByLabelText("Название")).toHaveValue("Profile 2");
    expect(screen.getByLabelText("Сервер (Host)")).toHaveValue("profile2.example");
  });

  it("does not apply a late reveal to a reopened profile", async () => {
    const pending = deferred<{ data: { raw_uri: string } }>();
    api.reveal.mockReturnValue(pending.promise);
    const editor = session();
    await screen.findByDisplayValue("Profile 1");
    fireEvent.click(screen.getByRole("button", { name: /Раскрыть учетные данные/i }));
    await waitFor(() => expect(api.reveal).toHaveBeenCalledTimes(1));
    editor.switch(false, 1);
    editor.switch(true, 2);
    await screen.findByDisplayValue("Profile 2");
    await act(async () => pending.resolve({ data: { raw_uri: "vless://11111111-1111-4111-8111-111111111111@profile1.example:443#First" } }));
    expect(screen.getByLabelText("Сервер (Host)")).toHaveValue("profile2.example");
    expect(screen.getByLabelText("UUID / ID")).toHaveValue("");
    expect(screen.getByLabelText("UUID / ID")).toBeDisabled();
    expect(screen.queryByText("Сырая ссылка раскрыта")).not.toBeInTheDocument();
  });

  it("withholds a late load failure from the new session", async () => {
    const pending = deferred<{ data: ReturnType<typeof detail> }>();
    api.get.mockImplementation((id: number) => id === 1 ? pending.promise : Promise.resolve({ data: detail(id) }));
    const editor = session();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith(1));
    editor.switch(false, 1);
    editor.switch(true, 2);
    await screen.findByDisplayValue("Profile 2");
    await act(async () => pending.reject(new Error("old-session load failed")));
    expect(screen.queryByText("old-session load failed")).not.toBeInTheDocument();
  });
});
