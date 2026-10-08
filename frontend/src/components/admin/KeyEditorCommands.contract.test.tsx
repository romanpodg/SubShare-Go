import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "@/components/ui/Toast";
import { KeyEditorModal } from "./KeyEditorModal";

const api = vi.hoisted(() => ({ get: vi.fn(), reveal: vi.fn(), createProfile: vi.fn(), updateProfile: vi.fn() }));
vi.mock("@/lib/api", () => ({
  keys: {
    ...api,
    listCategories: vi.fn().mockResolvedValue({ categories: [] }),
    editorSchema: vi.fn().mockResolvedValue({ data: { protocols: [], exclusion_reason_codes: {} } }),
  },
  ApiError: class extends Error { status = 409; },
}));

const protocols = ["shadowsocks", "hysteria2", "tuic"] as const;
const passwords = { shadowsocks: "Пароль / PSK", hysteria2: "Пароль авторизации (Auth)", tuic: "Пароль" };
function detail(protocol: typeof protocols[number]) {
  return {
    id: 11, label: "Panel", client_display_name: "Client", client_display_name_overridden: true,
    category: "Native", kind: "real", status: "active", ownership: "local", protocol, profile_revision: 17,
    capabilities: {}, unknown_query_parameters: [],
    safe_structured: {
      server: "native.example", port: "8443", port_kind: "single", display_name: "Client",
      shadowsocks: { method: "aes-256-gcm", plugin_name: "plugin", password_present: true },
      hysteria2: { sni: "sni.example", insecure: true, authentication_present: true },
      tuic: { sni: "sni.example", alpn: ["h3"], skip_cert_verify: true, congestion_controller: "bbr", udp_relay_mode: "native", heartbeat: "10s" },
    },
  };
}
function editor(props: Partial<React.ComponentProps<typeof KeyEditorModal>>) {
  const onClose = vi.fn();
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  render(<ToastProvider><KeyEditorModal open onClose={onClose} onRefresh={onRefresh} {...props} /></ToastProvider>);
  return { onClose, onRefresh };
}
function wire(value: unknown) { return JSON.parse(JSON.stringify(value)); }
async function updatePayload() {
  fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
  await waitFor(() => expect(api.updateProfile).toHaveBeenCalledTimes(1));
  expect(api.updateProfile.mock.calls[0][0]).toBe(11);
  return wire(api.updateProfile.mock.calls[0][1]);
}
function metadata() {
  return { label: "Panel", category: "Native", status: "active", kind: "real", template_text: "", profile_revision: 17, patch_mode: "structured" };
}

describe("editor native command wire contracts before extraction", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.createProfile.mockResolvedValue({ data: { id: 12 } });
    api.updateProfile.mockResolvedValue({ data: { id: 11 } });
  });

  it.each(protocols)("omits untouched %s fields and hidden credentials", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    editor({ keyId: 11 });
    await screen.findByDisplayValue("native.example");
    expect(await updatePayload()).toEqual({ ...metadata(), structured_patch: {} });
    expect(api.reveal).not.toHaveBeenCalled();
  });

  it.each(protocols)("sets an explicit empty %s credential", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    editor({ keyId: 11 });
    await screen.findByDisplayValue("native.example");
    const password = screen.getByLabelText(passwords[protocol]);
    fireEvent.change(password, { target: { value: "temporary-fixture" } });
    fireEvent.change(password, { target: { value: "" } });
    const field = protocol === "hysteria2" ? "authentication" : "password";
    expect(await updatePayload()).toEqual({ ...metadata(), structured_patch: { [protocol]: { [field]: { operation: "set", value: "" } } } });
  });

  it.each(["hysteria2", "tuic"] as const)("clears optional %s SNI without replacing secrets", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    editor({ keyId: 11 });
    await screen.findByDisplayValue("native.example");
    fireEvent.change(screen.getByLabelText(protocol === "tuic" ? "SNI" : "SNI (Server Name)"), { target: { value: "" } });
    expect(await updatePayload()).toEqual({ ...metadata(), structured_patch: { [protocol]: { sni: { operation: "clear" } } } });
  });

  it("clears the Shadowsocks plugin using its explicit clear operation", async () => {
    api.get.mockResolvedValue({ data: detail("shadowsocks") });
    editor({ keyId: 11 });
    await screen.findByDisplayValue("native.example");
    fireEvent.change(screen.getByDisplayValue("plugin"), { target: { value: "" } });
    expect(await updatePayload()).toEqual({ ...metadata(), structured_patch: { shadowsocks: { plugin_name: { operation: "clear" } } } });
  });

  it.each(protocols)("withholds %s raw update before reveal", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    const session = editor({ keyId: 11 });
    await screen.findByDisplayValue("native.example");
    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(await screen.findByText("Сначала раскройте raw-конфигурацию")).toBeInTheDocument();
    expect(api.updateProfile).not.toHaveBeenCalled();
    expect(session.onClose).not.toHaveBeenCalled();
  });

  it.each([
    { protocol: "shadowsocks" as const, expected: { method: { operation: "set", value: "2022-blake3-aes-128-gcm" }, password: { operation: "set", value: "fixture-password" } } },
    { protocol: "hysteria2" as const, expected: { authentication: { operation: "set", value: "fixture-password" }, insecure: { operation: "set", value: false } } },
    { protocol: "tuic" as const, expected: {
      uuid: { operation: "set", value: "11111111-1111-4111-8111-111111111111" }, password: { operation: "set", value: "fixture-password" },
      alpn: { operation: "set", value: ["h3"] }, skip_cert_verify: { operation: "set", value: false },
      congestion_controller: { operation: "set", value: "bbr" }, udp_relay_mode: { operation: "set", value: "native" },
      udp_over_stream: { operation: "set", value: false }, zero_rtt: { operation: "set", value: false }, heartbeat: { operation: "set", value: "10s" },
    } },
  ])("creates $protocol with its exact defaults and metadata trimming", async ({ protocol, expected }) => {
    editor({ initialProtocol: protocol, initialCategory: "Native" });
    fireEvent.change(await screen.findByLabelText("Название"), { target: { value: "  Panel  " } });
    fireEvent.change(screen.getByLabelText("Название в клиенте"), { target: { value: "  Client  " } });
    fireEvent.change(screen.getByLabelText("Сервер (Host)"), { target: { value: "create.example" } });
    fireEvent.change(screen.getByLabelText(passwords[protocol]), { target: { value: "fixture-password" } });
    if (protocol === "tuic") fireEvent.change(screen.getByLabelText("UUID"), { target: { value: "11111111-1111-4111-8111-111111111111" } });
    fireEvent.click(screen.getByRole("button", { name: "Добавить профиль" }));
    await waitFor(() => expect(api.createProfile).toHaveBeenCalledTimes(1));
    expect(wire(api.createProfile.mock.calls[0][0])).toEqual({
      label: "Panel", client_display_name: "Client", category: "Native", status: "active", kind: "real", template_text: "", creation_mode: "structured", protocol,
      structured: { server: { operation: "set", value: "create.example" }, port: { operation: "set", value: "" }, display_name: { operation: "set", value: "  Client  " }, [protocol]: expected },
    });
  });

  it("keeps the source update payload restricted to represented local metadata", async () => {
    api.get.mockResolvedValue({ data: { ...detail("shadowsocks"), ownership: "external_source" } });
    editor({ keyId: 11 });
    fireEvent.change(await screen.findByLabelText("Название в клиенте"), { target: { value: "  Changed client  " } });
    expect(await updatePayload()).toEqual({ label: "Panel", category: "Native", status: "active", kind: "real", profile_revision: 17, patch_mode: "structured", client_display_name: "Changed client" });
    expect(api.reveal).not.toHaveBeenCalled();
  });
});
