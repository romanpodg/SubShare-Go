import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { KeyEditorModal } from "./KeyEditorModal";

const api = vi.hoisted(() => ({ get: vi.fn(), reveal: vi.fn(), updateProfile: vi.fn(), createProfile: vi.fn() }));
vi.mock("@/lib/api", () => ({
  keys: { ...api, editorSchema: vi.fn().mockResolvedValue({ data: { protocols: [], exclusion_reason_codes: {} } }), listCategories: vi.fn().mockResolvedValue({ categories: [] }) },
  ApiError: class extends Error { constructor(public status: number, public code: string, message: string) { super(message); } },
}));

const legacyRaw = "vless://11111111-1111-4111-8111-111111111111@original.example:443?unknown=keep#First";
const nativeProtocols = ["shadowsocks", "hysteria2", "tuic"] as const;
const passwordLabels = { shadowsocks: "Пароль / PSK", hysteria2: "Пароль авторизации (Auth)", tuic: "Пароль" };
function detail(protocol = "vless", revision = 7) {
  return {
    id: 1, label: "Profile", client_display_name: "Client", client_display_name_overridden: true,
    category: "", kind: "real", status: "active", ownership: "local", protocol, profile_revision: revision,
    capabilities: {}, unknown_query_parameters: [],
    safe_structured: {
      server: "original.example", port: "443", port_kind: "single", display_name: "Client",
      shadowsocks: { method: "aes-256-gcm", plugin_name: "plugin", password_present: true, plugin_options_present: true },
      hysteria2: { sni: "", insecure: false, authentication_present: true },
      tuic: { generation: 5, sni: "", alpn: ["h3"], skip_cert_verify: false, congestion_controller: "bbr", udp_relay_mode: "native", heartbeat: "10s" },
    },
  };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => { resolve = yes; });
  return { promise, resolve };
}
function editor(props: Partial<React.ComponentProps<typeof KeyEditorModal>> = {}) {
  render(<ToastProvider><KeyEditorModal open keyId={1} onClose={vi.fn()} onRefresh={vi.fn().mockResolvedValue(undefined)} {...props} /></ToastProvider>);
}
async function save() {
  fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
  await waitFor(() => expect(api.updateProfile).toHaveBeenCalled());
  return JSON.parse(JSON.stringify(api.updateProfile.mock.calls.at(-1)?.[1]));
}
const secrets = { password: "fixture-password", authentication: "fixture-auth", uuid: "11111111-1111-4111-8111-111111111111", plugin_options: "fixture-options" };

describe("pending reveal and credential provenance contracts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.get.mockResolvedValue({ data: detail() });
    api.updateProfile.mockResolvedValue({ data: detail() });
    api.createProfile.mockResolvedValue({ data: detail() });
    api.reveal.mockResolvedValue({ data: { raw_uri: legacyRaw, secrets } });
  });

  it.each([false, true])("preserves connection edits during reveal, raw switch=%s", async (switchRaw) => {
    const pending = deferred<{ data: { raw_uri: string } }>();
    api.reveal.mockReturnValue(pending.promise);
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.click(screen.getByRole("button", { name: /Раскрыть учетные данные/i }));
    fireEvent.change(screen.getByLabelText("Сервер (Host)"), { target: { value: "entered.example" } });
    fireEvent.change(screen.getByLabelText("Порт"), { target: { value: "8443" } });
    if (switchRaw) fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    await act(async () => pending.resolve({ data: { raw_uri: legacyRaw } }));
    fireEvent.click(screen.getByRole("button", { name: "Структурированный" }));
    expect(screen.getByLabelText("Сервер (Host)")).toHaveValue("entered.example");
    expect(screen.getByLabelText("Порт")).toHaveValue("8443");
    const payload = await save();
    expect(payload.raw_uri).toContain("entered.example:8443");
    expect(payload.raw_uri).toContain("unknown=keep");
  });

  it.each(nativeProtocols)("keeps a typed %s password while reveal is pending", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    const pending = deferred<{ data: { secrets: typeof secrets } }>();
    api.reveal.mockReturnValue(pending.promise);
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.click(screen.getByRole("button", { name: "Раскрыть секреты" }));
    fireEvent.change(screen.getByLabelText(passwordLabels[protocol]), { target: { value: "entered-password" } });
    await act(async () => pending.resolve({ data: { secrets } }));
    expect(screen.getByLabelText(passwordLabels[protocol])).toHaveValue("entered-password");
  });

  it.each(nativeProtocols)("does not patch revealed %s credentials on a metadata-only save", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.click(screen.getByRole("button", { name: "Раскрыть секреты" }));
    await screen.findByDisplayValue(protocol === "hysteria2" ? "fixture-auth" : "fixture-password");
    fireEvent.change(screen.getByLabelText("Название"), { target: { value: "Renamed" } });
    expect((await save()).structured_patch).toEqual({});
  });

  it("sends an explicit clear for unknown Shadowsocks plugin options", async () => {
    api.get.mockResolvedValue({ data: detail("shadowsocks") });
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.change(screen.getByLabelText("Параметры плагина"), { target: { value: "temporary" } });
    fireEvent.change(screen.getByLabelText("Параметры плагина"), { target: { value: "" } });
    expect((await save()).structured_patch).toEqual({ shadowsocks: { plugin_options: { operation: "clear" } } });
  });

  it("retains the explicit empty TUIC UUID set contract", async () => {
    api.get.mockResolvedValue({ data: detail("tuic") });
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.change(screen.getByLabelText("UUID"), { target: { value: "22222222-2222-4222-8222-222222222222" } });
    fireEvent.change(screen.getByLabelText("UUID"), { target: { value: "" } });
    expect((await save()).structured_patch).toEqual({ tuic: { uuid: { operation: "set", value: "" } } });
  });

  it("retains the explicit Hysteria obfuscation-password clear contract", async () => {
    const loaded = detail("hysteria2");
    api.get.mockResolvedValue({ data: { ...loaded, safe_structured: { ...loaded.safe_structured, hysteria2: { ...loaded.safe_structured.hysteria2, obfuscation_type: "salamander" } } } });
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.change(screen.getByLabelText("Пароль обфускации"), { target: { value: "temporary" } });
    fireEvent.change(screen.getByLabelText("Пароль обфускации"), { target: { value: "" } });
    expect((await save()).structured_patch).toEqual({ hysteria2: { obfuscation_password: { operation: "clear" } } });
  });

  it.each(["vless", ...nativeProtocols])("does not restore stale %s secrets after revision refresh", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.click(screen.getByRole("button", { name: protocol === "vless" ? /Раскрыть учетные данные/i : "Раскрыть секреты" }));
    await screen.findByDisplayValue(protocol === "vless" ? "11111111-1111-4111-8111-111111111111" : protocol === "hysteria2" ? "fixture-auth" : "fixture-password");
    api.updateProfile.mockRejectedValueOnce(new ApiError(409, "profile_revision_conflict", "revision changed"));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await screen.findByRole("dialog", { name: /Конфликт версий/i });
    const latest = detail(protocol, 9);
    latest.label = "Latest profile";
    latest.safe_structured.server = "latest.example";
    api.get.mockResolvedValue({ data: latest });
    fireEvent.click(screen.getByRole("button", { name: "Загрузить свежую версию" }));
    await screen.findByDisplayValue("Latest profile");
    const payload = await save();
    expect(payload.profile_revision).toBe(9);
    expect(payload.patch_mode).toBe("structured");
    expect(payload.structured_patch).toEqual({});
    expect(payload.raw_uri).toBeUndefined();
  });

  it.each(["vless", ...nativeProtocols])("discards a pending %s reveal after loading a newer revision", async (protocol) => {
    api.get.mockResolvedValue({ data: detail(protocol) });
    const pending = deferred<{ data: { raw_uri: string; secrets: typeof secrets } }>();
    api.reveal.mockReturnValue(pending.promise);
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.click(screen.getByRole("button", { name: protocol === "vless" ? /Раскрыть учетные данные/i : "Раскрыть секреты" }));
    api.updateProfile.mockRejectedValueOnce(new ApiError(409, "profile_revision_conflict", "revision changed"));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await screen.findByRole("dialog", { name: /Конфликт версий/i });
    const latest = detail(protocol, 9);
    latest.label = "Latest profile";
    api.get.mockResolvedValue({ data: latest });
    fireEvent.click(screen.getByRole("button", { name: "Загрузить свежую версию" }));
    await screen.findByDisplayValue("Latest profile");
    await act(async () => pending.resolve({ data: { raw_uri: legacyRaw, secrets } }));
    const label = protocol === "vless" ? "UUID / ID" : passwordLabels[protocol as typeof nativeProtocols[number]];
    expect(screen.getByLabelText(label)).toHaveValue("");
    expect(screen.queryByText(protocol === "vless" ? "Сырая ссылка раскрыта" : "Секретные поля раскрыты")).not.toBeInTheDocument();
    const payload = await save();
    expect(payload.profile_revision).toBe(9);
    expect(payload.structured_patch).toEqual({});
    expect(payload.raw_uri).toBeUndefined();
  });

  it.each([
    { name: "malformed", raw: "{ invalid", message: /синтаксиса/i },
    { name: "duplicate", raw: '{"outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"one.example","address":"two.example","port":443,"password":"fixture-password"}]}}]}', message: /повторяющиеся ключи/i },
  ])("withholds update for $name raw JSON while keeping entered bytes", async ({ raw, message }) => {
    api.get.mockResolvedValue({ data: detail("xray-json") });
    const initial = '{"outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"original.example","port":443,"password":"fixture-password"}]}}]}';
    api.reveal.mockResolvedValue({ data: { raw_uri: initial } });
    editor();
    await screen.findByDisplayValue("Profile");
    fireEvent.click(screen.getByRole("button", { name: "Раскрыть сырую ссылку" }));
    await waitFor(() => expect(screen.getByLabelText("Raw-конфигурация")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Raw-конфигурация"), { target: { value: raw } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(await screen.findAllByText(message)).not.toHaveLength(0);
    expect(api.updateProfile).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Raw-конфигурация")).toHaveValue(raw);
  });

  it("does not create a replacement profile when the requested edit failed to load", async () => {
    api.get.mockRejectedValue(new Error("profile load failed"));
    editor({ initialKind: "informational", initialLabel: "Existing profile" });
    await screen.findByText("profile load failed");
    fireEvent.change(screen.getByLabelText("Текст информационного ключа"), { target: { value: "local template" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(api.createProfile).not.toHaveBeenCalled();
    expect(api.updateProfile).not.toHaveBeenCalled();
    expect(await screen.findByText(/Профиль не загружен/i)).toBeInTheDocument();
  });
});
