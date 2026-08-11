import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "@/components/ui/Toast";
import { copyToClipboard } from "@/lib/clipboard";
import { KeyEditorModal } from "./KeyEditorModal";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  reveal: vi.fn(),
  clone: vi.fn(),
  createProfile: vi.fn(),
  updateProfile: vi.fn(),
}));

vi.mock("@/lib/clipboard", () => ({ copyToClipboard: vi.fn().mockResolvedValue(true) }));
vi.mock("@/lib/api", () => ({
  keys: {
    listCategories: vi.fn().mockResolvedValue({ categories: [] }),
    editorSchema: vi.fn().mockResolvedValue({ data: { protocols: [], exclusion_reason_codes: {} } }),
    get: mocks.get,
    reveal: mocks.reveal,
    clone: mocks.clone,
    createProfile: mocks.createProfile,
    updateProfile: mocks.updateProfile,
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, msg: string) { super(msg); this.status = status; }
  },
}));

const vlessDetail = {
  id: 1,
  label: "VLESS profile",
  client_display_name: "Subscriber VLESS",
  client_display_name_overridden: true,
  category: "",
  kind: "real" as const,
  status: "active" as const,
  ownership: "local" as const,
  protocol: "vless" as const,
  profile_revision: 7,
  safe_structured: {
    server: "vless.example",
    port: "443",
    port_kind: "single" as const,
    display_name: "VLESS profile",
  },
  capabilities: {},
  unknown_query_parameters: [],
};

function renderEditor(props: Partial<React.ComponentProps<typeof KeyEditorModal>> = {}) {
  const onClose = vi.fn();
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  const result = render(
    <ToastProvider>
      <KeyEditorModal open onClose={onClose} onRefresh={onRefresh} {...props} />
    </ToastProvider>
  );
  return { ...result, onClose, onRefresh };
}

describe("KeyEditorModal regression recovery", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.get.mockResolvedValue({ data: vlessDetail });
    mocks.reveal.mockResolvedValue({
      data: {
        key_id: 1,
        confirmed_profile_revision: 7,
        target: "raw",
        raw_uri: "vless://11111111-1111-4111-8111-111111111111@vless.example:443?type=ws&security=reality&path=%2Fedge&host=cdn.example&sni=sni.example&flow=xtls-rprx-vision&pbk=public-key&sid=abcd&unknown=keep#VLESS%20profile",
      },
    });
    mocks.clone.mockResolvedValue({ data: { id: 2 } });
    mocks.createProfile.mockResolvedValue({ data: { id: 2 } });
    mocks.updateProfile.mockResolvedValue({ data: { id: 1 } });
  });

  it("renders all six create protocols from the shared capability registry", async () => {
    renderEditor();
    const select = await screen.findByLabelText("Протокол");
    fireEvent.click(select);
    expect(screen.getAllByRole("option").map((option) => option.textContent)).toEqual([
      "VLESS", "VMess", "Trojan", "Shadowsocks", "Hysteria 2", "TUIC v5",
    ]);
    expect(screen.queryByRole("option", { name: /Hysteria v1/i })).not.toBeInTheDocument();
  });

  it("does not expose the developer capability matrix in the normal editor", async () => {
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        capabilities: {
          "connectivity-probe": {
            status: "conditionally_supported",
            reason_code: "dns_only_udp_quic_probe_unavailable",
            syntax_validation: "address_probe_only",
            runtime_interoperability: "not_tested",
          },
        },
      },
    });
    renderEditor({ keyId: 1 });

    await screen.findByDisplayValue("VLESS profile");
    expect(screen.queryByText(/Generator Capabilities/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Совместимость выгрузки/i)).not.toBeInTheDocument();
    expect(screen.queryByText("connectivity-probe")).not.toBeInTheDocument();
    expect(screen.queryByText("dns_only_udp_quic_probe_unavailable")).not.toBeInTheDocument();
  });

  it("falls back the client display name to the panel name on create", async () => {
    renderEditor({ initialProtocol: "vless" });
    fireEvent.change(await screen.findByLabelText("Название"), { target: { value: "Panel node" } });
    fireEvent.change(screen.getByLabelText("Сервер (Host)"), { target: { value: "create.example" } });
    fireEvent.change(screen.getByLabelText("UUID / ID"), { target: { value: "61111111-1111-4111-8111-111111111111" } });
    fireEvent.click(screen.getByRole("button", { name: "Добавить профиль" }));
    await waitFor(() => expect(mocks.createProfile).toHaveBeenCalledWith(expect.objectContaining({
      label: "Panel node",
      client_display_name: "Panel node",
      creation_mode: "raw",
      raw_uri: expect.stringContaining("Panel%20node"),
    })));
  });

  it("opens local VLESS in structured mode and preserves fields across raw toggles", async () => {
    renderEditor({ keyId: 1 });
    expect(await screen.findByTestId("legacy-structured-editor")).toBeInTheDocument();
    expect(screen.getByDisplayValue("vless.example")).not.toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: /Раскрыть учетные данные/i }));
    expect(await screen.findByDisplayValue("xtls-rprx-vision")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Flow"), { target: { value: "vision-updated" } });

    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    expect((screen.getByLabelText("Raw-конфигурация") as HTMLTextAreaElement).value).toContain("flow=vision-updated");
    fireEvent.click(screen.getByRole("button", { name: "Копировать raw" }));
    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledWith(expect.stringContaining("flow=xtls-rprx-vision")));
    fireEvent.click(screen.getByRole("button", { name: "Структурированный" }));
    expect(screen.getByDisplayValue("vision-updated")).toBeInTheDocument();
  });

  it("updates client display metadata independently from the panel name", async () => {
    renderEditor({ keyId: 1 });
    expect(await screen.findByLabelText("Название")).toHaveValue("VLESS profile");
    const clientName = screen.getByLabelText("Название в клиенте");
    expect(clientName).toHaveValue("Subscriber VLESS");
    fireEvent.change(clientName, { target: { value: "Public edge" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      label: "VLESS profile",
      client_display_name: "Public edge",
      structured_patch: expect.not.objectContaining({ display_name: expect.anything() }),
    })));
  });

  it("keeps an explicit client name when only the panel name changes", async () => {
    renderEditor({ keyId: 1 });
    const panelName = await screen.findByLabelText("Название");
    fireEvent.change(panelName, { target: { value: "Internal operation name" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      label: "Internal operation name",
      client_display_name: undefined,
    })));
  });

  it("formats revealed XRAY-JSON for display, copies exact raw, and preserves invalid typing", async () => {
    const minified = '{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111"}]}]}}]}';
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        protocol: "xray-json",
        safe_structured: { server: "xray.example", port: "443", port_kind: "single", display_name: "Xray", xray_json: { has_raw_json: true, network: "tcp", security: "none" } },
      },
    });
    mocks.reveal.mockResolvedValue({ data: { key_id: 1, confirmed_profile_revision: 7, target: "raw", raw_uri: minified } });
    renderEditor({ keyId: 1 });
    fireEvent.click(await screen.findByRole("button", { name: "Раскрыть сырую ссылку" }));
    const textarea = screen.getByLabelText("Raw-конфигурация");
    await waitFor(() => expect((textarea as HTMLTextAreaElement).value).toContain("\n  \"outbounds\""));

    fireEvent.click(screen.getByRole("button", { name: "Копировать raw" }));
    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledWith(minified));

    fireEvent.change(textarea, { target: { value: '{"outbounds":' } });
    expect(textarea).toHaveValue('{"outbounds":');
    expect(screen.getByRole("alert")).toHaveTextContent(/ошибку синтаксиса/i);
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).not.toHaveBeenCalled());
  });

  it("keeps duplicate-key XRAY-JSON byte-exact and warns instead of formatting it", async () => {
    const duplicate = '{"outbounds":[{"protocol":"vless","tag":"first","tag":"second","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111"}]}]}}]}';
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        protocol: "xray-json",
        safe_structured: { server: "xray.example", port: "443", port_kind: "single", display_name: "Xray", xray_json: { has_raw_json: true, network: "tcp", security: "none" } },
      },
    });
    mocks.reveal.mockResolvedValue({ data: { key_id: 1, confirmed_profile_revision: 7, target: "raw", raw_uri: duplicate } });
    renderEditor({ keyId: 1 });
    fireEvent.click(await screen.findByRole("button", { name: "Раскрыть сырую ссылку" }));
    await waitFor(() => expect(screen.getByLabelText("Raw-конфигурация")).toHaveValue(duplicate));
    expect(screen.getByText(/повторяющиеся ключи/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Копировать raw" }));
    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledWith(duplicate));
  });

  it("blocks saving when independently edited structured and raw drafts diverge", async () => {
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        protocol: "shadowsocks",
        safe_structured: {
          server: "ss.example",
          port: "8388",
          port_kind: "single",
          display_name: "SS profile",
          shadowsocks: { method: "aes-256-gcm", password_present: true },
        },
      },
    });
    mocks.reveal.mockResolvedValue({
      data: { key_id: 1, confirmed_profile_revision: 7, target: "raw", raw_uri: "ss://YWVzLTI1Ni1nY206c2VjcmV0QHNzLmV4YW1wbGU6ODM4OA==#SS" },
    });
    renderEditor({ keyId: 1 });
    fireEvent.click(await screen.findByRole("button", { name: "XRAY-JSON" }));
    fireEvent.click(screen.getByRole("button", { name: "Раскрыть сырую ссылку" }));
    await screen.findByDisplayValue(/ss:\/\//);
    fireEvent.click(screen.getByRole("button", { name: "Структурированный" }));
    fireEvent.change(screen.getByLabelText("Сервер (Host)"), { target: { value: "changed.example" } });
    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    fireEvent.change(screen.getByLabelText("Raw-конфигурация"), { target: { value: "ss://independent-draft" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(await screen.findByText(/Structured-версия уже изменена/i)).toBeInTheDocument();
    expect(mocks.updateProfile).not.toHaveBeenCalled();
  });

  it("uses full width for informational keys without a profile-editor pane", async () => {
    renderEditor({ initialKind: "informational" });
    await screen.findByLabelText("Название");
    expect(screen.getByTestId("key-editor-workspace")).not.toHaveClass("lg:grid-cols-2");
    expect(screen.queryByText("РЕДАКТОР ПРОФИЛЯ")).not.toBeInTheDocument();
  });

  it("inserts informational variables at the selection and renders a safe live preview", async () => {
    renderEditor({ initialKind: "informational" });
    const editor = await screen.findByLabelText("Текст информационного ключа") as HTMLTextAreaElement;
    fireEvent.change(editor, { target: { value: "Hello world {expires_date} {unknown} <script>alert(1)</script>" } });
    editor.focus();
    editor.setSelectionRange(6, 11);
    fireEvent.click(screen.getByTitle("Вставить {user_name}"));

    await waitFor(() => expect(editor).toHaveValue("Hello {user_name} {expires_date} {unknown} <script>alert(1)</script>"));
    await waitFor(() => expect(editor.selectionStart).toBe("Hello {user_name}".length));
    const preview = screen.getByTestId("informational-preview");
    expect(preview).toHaveTextContent("Hello Иван 25/08/2026 {unknown} <script>alert(1)</script>");
    expect(preview.querySelector("[data-unknown-placeholder]")).toHaveTextContent("{unknown}");
    expect(preview.querySelector("script")).toBeNull();
  });

  it("preserves an existing informational template exactly while editing", async () => {
    const existingTemplate = "Line 1\n{user_name} + {telegram} + {unknown}";
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        label: "Existing info",
        kind: "informational",
        protocol: "informational",
        template_text: existingTemplate,
        safe_structured: undefined,
      },
    });
    renderEditor({ keyId: 1, initialKind: "informational" });
    expect(await screen.findByLabelText("Текст информационного ключа")).toHaveValue(existingTemplate);
    expect(screen.getByTestId("informational-preview")).toHaveTextContent("Line 1 Иван + ivan_example + {unknown}");
  });

  it("keeps source configuration locked while allowing local status and client-name metadata", async () => {
    mocks.get.mockResolvedValue({ data: { ...vlessDetail, ownership: "external_source", external_source_name: "Provider" } });
    const { container } = renderEditor({ keyId: 1 });
    expect(await screen.findByDisplayValue("vless.example")).toBeDisabled();
    expect(screen.getByLabelText("Название")).toBeDisabled();
    expect(screen.getByLabelText("Статус")).not.toBeDisabled();
    expect(screen.getByLabelText("Тип")).toBeDisabled();
    expect(screen.getByLabelText("Категория")).toBeDisabled();
    expect(screen.getByLabelText("Название в клиенте")).not.toBeDisabled();
    expect(screen.queryByText(/Локальный статус доставки/)).not.toBeInTheDocument();
    const form = container.querySelector("form");
    const sourceBanner = container.querySelector(".ui-key-editor-source-banner");
    const scrollRegion = container.querySelector(".ui-key-editor-scroll-region");
    const footer = container.querySelector(".ui-key-editor-footer");
    expect(sourceBanner?.parentElement).toBe(form);
    expect(scrollRegion?.parentElement).toBe(form);
    expect(footer?.parentElement).toBe(form);
    expect(sourceBanner).toHaveClass("shrink-0");
    expect(scrollRegion).toHaveClass("min-h-0", "flex-[0_1_auto]", "overflow-y-auto");
    expect(footer).toHaveClass("shrink-0");
    expect(footer).not.toHaveClass("-mb-5");
    expect(screen.getByRole("dialog")).toHaveClass("max-h-[calc(100dvh-1.5rem)]");
    expect(screen.getByRole("dialog")).not.toHaveClass("h-[calc(100dvh-1.5rem)]");
    expect(screen.getByRole("button", { name: "Сохранить" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Название в клиенте"), { target: { value: "Local source override" } });
    expect(screen.getByRole("button", { name: "Сохранить" })).not.toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).toHaveBeenCalledWith(1, {
      label: "VLESS profile",
      client_display_name: "Local source override",
      category: "",
      status: "active",
      kind: "real",
      template_text: undefined,
      profile_revision: 7,
      patch_mode: "structured",
    }));
    expect(mocks.reveal).not.toHaveBeenCalled();
  });

  it("updates a source-owned status in both directions without sending a client-name change", async () => {
    mocks.get.mockResolvedValue({
      data: { ...vlessDetail, ownership: "external_source", external_source_name: "Provider" },
    });
    const first = renderEditor({ keyId: 1 });
    const statusControl = await screen.findByLabelText("Статус");
    fireEvent.click(statusControl);
    fireEvent.click(screen.getByRole("option", { name: "Неактивен" }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      status: "non-active",
      client_display_name: undefined,
      profile_revision: 7,
    })));

    first.unmount();
    vi.clearAllMocks();
    mocks.get.mockResolvedValue({
      data: { ...vlessDetail, status: "non-active", ownership: "external_source", external_source_name: "Provider" },
    });
    mocks.updateProfile.mockResolvedValue({ data: { id: 1 } });
    renderEditor({ keyId: 1 });
    const inactiveStatusControl = await screen.findByLabelText("Статус");
    fireEvent.click(inactiveStatusControl);
    fireEvent.click(screen.getByRole("option", { name: "Активен" }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      status: "active",
      client_display_name: undefined,
      profile_revision: 7,
    })));
  });

  it("resets a source override to the derived source name and keeps cloning available", async () => {
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        label: "Source B",
        client_display_name: "CUSTOM",
        client_display_name_overridden: true,
        ownership: "external_source",
        external_source_name: "Provider",
        safe_structured: { ...vlessDetail.safe_structured, display_name: "Source B" },
      },
    });
    renderEditor({ keyId: 1 });
    const clientName = await screen.findByLabelText("Название в клиенте");
    expect(clientName).toHaveValue("CUSTOM");
    fireEvent.click(screen.getByRole("button", { name: "Использовать название источника" }));
    expect(clientName).toHaveValue("");
    expect(clientName).toHaveAttribute("placeholder", "Source B");
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(mocks.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      client_display_name: "",
      profile_revision: 7,
    })));

    fireEvent.click(screen.getByRole("button", { name: "Клонировать как локальный" }));
    await waitFor(() => expect(mocks.clone).toHaveBeenCalledWith(1, { expected_profile_revision: 7 }));
  });

  it("shows the source label fallback instead of an XRAY outbound tag", async () => {
    mocks.get.mockResolvedValue({
      data: {
        ...vlessDetail,
        label: "🌟 Human source name",
        client_display_name: "🌟 Human source name",
        client_display_name_overridden: false,
        ownership: "external_source",
        external_source_name: "Provider",
        safe_structured: { ...vlessDetail.safe_structured, display_name: "proxy" },
      },
    });
    renderEditor({ keyId: 1 });
    const clientName = await screen.findByLabelText("Название в клиенте");
    expect(clientName).toHaveValue("");
    expect(clientName).toHaveAttribute("placeholder", "🌟 Human source name");
    expect(screen.getByText(/По умолчанию: 🌟 Human source name/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Использовать название источника" })).toBeDisabled();
  });
});
