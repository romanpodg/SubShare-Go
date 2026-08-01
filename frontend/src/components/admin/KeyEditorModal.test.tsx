import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "@/components/ui/Toast";
import { copyToClipboard } from "@/lib/clipboard";
import {
  CONFIG_LIMIT,
  configurationByteLength,
} from "@/lib/xray-json-document";
import {
  KeyEditorModal,
  findUnknownKeyTemplateTokens,
  formatXrayJSON,
  isValidKeyPort,
  renderKeyTemplatePreview,
} from "./KeyEditorModal";

vi.mock("@/lib/api", () => ({
  keys: {
    listCategories: vi.fn().mockResolvedValue({ categories: [] }),
  },
}));

vi.mock("@/lib/clipboard", () => ({
  copyToClipboard: vi.fn(),
}));

const validLink =
  "vless://00000000-0000-0000-0000-000000000000@example.com:443?type=ws&security=tls#Edge";

const validXrayObject = {
  log: { loglevel: "warning" },
  outbounds: [
    {
      tag: "Edge",
      protocol: "vless",
      settings: {
        vnext: [
          {
            address: "example.com",
            port: 443,
            users: [
              {
                id: "00000000-0000-0000-0000-000000000000",
                encryption: "none",
              },
            ],
          },
        ],
      },
      streamSettings: {
        network: "ws",
        security: "tls",
        wsSettings: { path: "/edge", headers: { Host: "example.com" } },
        tlsSettings: { serverName: "example.com" },
      },
    },
  ],
};

const minifiedXray = JSON.stringify(validXrayObject);

function renderEditor(
  kind: "real" | "informational" = "real",
  rawConfig = kind === "real" ? validLink : ""
) {
  const onClose = vi.fn();
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const result = render(
    <ToastProvider>
      <KeyEditorModal
        open
        title="Редактор"
        submitLabel="Сохранить"
        onClose={onClose}
        onSubmit={onSubmit}
        initialValue={{
          label: "Тест",
          status: "active",
          category: "",
          kind,
          templateText: "",
          rawConfig,
        }}
      />
    </ToastProvider>
  );
  return { ...result, onClose, onSubmit };
}

beforeEach(() => {
  vi.mocked(copyToClipboard).mockResolvedValue(true);
});

describe("key editor helpers", () => {
  it("renders the same informational substitutions as the server", () => {
    expect(
      renderKeyTemplatePreview(
        "Привет, {user_name}! @{telegram}; ключей: {real_keys_count}",
        "Fallback"
      )
    ).toBe("Привет, Иван! @ivanov; ключей: 3");
    expect(renderKeyTemplatePreview("", "Название")).toBe("Название");
  });

  it("finds only unsupported template tokens", () => {
    expect(
      findUnknownKeyTemplateTokens("{user_name} {unknown_token} {unknown_token}")
    ).toEqual(["{unknown_token}"]);
  });

  it("validates ports and formats JSON without changing its data", () => {
    expect(isValidKeyPort("1")).toBe(true);
    expect(isValidKeyPort("65535")).toBe(true);
    expect(isValidKeyPort("0")).toBe(false);
    expect(isValidKeyPort("65536")).toBe(false);
    expect(isValidKeyPort("443x")).toBe(false);
    expect(formatXrayJSON('{"outbounds":[]}')).toBe(
      '{\n  "outbounds": []\n}'
    );
    expect(() => formatXrayJSON("{broken")).toThrow();
  });
});

describe("KeyEditorModal", () => {
  it("displays minified Xray JSON formatted without creating a false dirty state", () => {
    const { container, onClose } = renderEditor("real", minifiedXray);
    const raw = container.querySelector<HTMLTextAreaElement>("#key-editor-raw");

    expect(raw).toHaveValue(formatXrayJSON(minifiedXray));
    expect(screen.getByText("БЕЗ ИЗМЕНЕНИЙ")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(
      screen.queryByRole("dialog", { name: "Закрыть без сохранения?" })
    ).not.toBeInTheDocument();
  });

  it("formats a complete pasted object and preserves invalid or partial paste", () => {
    const { container } = renderEditor();
    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    const raw = container.querySelector<HTMLTextAreaElement>("#key-editor-raw")!;

    fireEvent.paste(raw, {
      clipboardData: { getData: () => minifiedXray },
    });
    expect(raw).toHaveValue(formatXrayJSON(minifiedXray));

    raw.setSelectionRange(0, raw.value.length);
    fireEvent.paste(raw, {
      clipboardData: { getData: () => '{"outbounds":' },
    });
    expect(raw).toHaveValue('{"outbounds":');
    expect(screen.getByRole("alert")).toHaveTextContent("ошибку синтаксиса");
  });

  it("copies pretty Xray JSON and submits the canonical pretty representation", async () => {
    const { container, onSubmit } = renderEditor();
    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    const raw = container.querySelector<HTMLTextAreaElement>("#key-editor-raw")!;
    fireEvent.change(raw, { target: { value: minifiedXray } });

    fireEvent.click(screen.getByRole("button", { name: "Копировать" }));
    await waitFor(() => {
      expect(copyToClipboard).toHaveBeenCalledWith(formatXrayJSON(minifiedXray));
    });

    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ rawConfig: formatXrayJSON(minifiedXray) })
      );
    });
  });

  it("keeps canonical Xray JSON stable after save, reopen, and formatting again", async () => {
    const first = renderEditor("real", minifiedXray);
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(first.onSubmit).toHaveBeenCalledTimes(1));
    const savedRaw = first.onSubmit.mock.calls[0]?.[0].rawConfig;
    expect(savedRaw).toBe(formatXrayJSON(minifiedXray));
    first.unmount();

    const reopened = renderEditor("real", savedRaw);
    const raw = reopened.container.querySelector<HTMLTextAreaElement>("#key-editor-raw")!;
    expect(raw).toHaveValue(savedRaw);
    expect(screen.getByText("БЕЗ ИЗМЕНЕНИЙ")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Форматировать" }));
    expect(raw).toHaveValue(savedRaw);
    expect(screen.getByText("БЕЗ ИЗМЕНЕНИЙ")).toBeInTheDocument();
  });

  it("keeps duplicate-key JSON raw-only while allowing raw copy and save", async () => {
    const duplicateXray = minifiedXray.replace(
      '"log":{"loglevel":"warning"}',
      '"duplicate":1,"duplicate":2,"log":{"loglevel":"warning"}'
    );
    const { container, onSubmit } = renderEditor("real", duplicateXray);
    const raw = container.querySelector<HTMLTextAreaElement>("#key-editor-raw")!;

    expect(raw).toHaveValue(duplicateXray);
    expect(screen.getAllByText(/повторяющиеся ключи/i).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Форматировать" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Копировать" }));
    await waitFor(() => {
      expect(copyToClipboard).toHaveBeenCalledWith(duplicateXray);
    });

    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ rawConfig: duplicateXray })
      );
    });
  });

  it("preserves a compact near-limit value when pretty formatting would exceed the limit", async () => {
    const emptyPadding = JSON.stringify({ ...validXrayObject, padding: "" });
    const paddingLength = CONFIG_LIMIT - configurationByteLength(emptyPadding) - 1;
    const nearLimitXray = JSON.stringify({
      ...validXrayObject,
      padding: "x".repeat(paddingLength),
    });
    expect(configurationByteLength(nearLimitXray)).toBeLessThanOrEqual(CONFIG_LIMIT);
    expect(configurationByteLength(formatXrayJSON(nearLimitXray))).toBeGreaterThan(CONFIG_LIMIT);

    const { container, onSubmit } = renderEditor("real", nearLimitXray);
    const raw = container.querySelector<HTMLTextAreaElement>("#key-editor-raw")!;
    expect(raw).toHaveValue(nearLimitXray);
    expect(screen.getByText(/Форматированная версия превышает лимит/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ rawConfig: nearLimitXray })
      );
    });
  });

  it("uses the wide shell and preserves independent format drafts", () => {
    const { container } = renderEditor();
    const dialog = screen.getByRole("dialog", { name: "Редактор" });
    expect(dialog).toHaveClass("max-w-6xl");
    expect(dialog).toHaveClass("overflow-hidden");

    const raw = container.querySelector<HTMLTextAreaElement>("#key-editor-raw");
    expect(raw).toHaveValue(validLink);

    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    expect(raw).toHaveValue("");
    fireEvent.change(raw!, { target: { value: '{"outbounds":[]}' } });

    fireEvent.click(screen.getByRole("button", { name: "Ключ-ссылка" }));
    expect(raw).toHaveValue(validLink);
    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    expect(raw).toHaveValue('{"outbounds":[]}');
  });

  it("inserts a template token and renders a live preview", () => {
    renderEditor("informational");
    fireEvent.click(screen.getByRole("button", { name: "{user_name}" }));
    expect(screen.getByText("Иван")).toBeInTheDocument();
  });

  it("asks before closing a dirty editor", () => {
    const { onClose } = renderEditor();
    fireEvent.change(screen.getByLabelText("Название"), {
      target: { value: "Изменено" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));

    expect(
      screen.getByRole("dialog", { name: "Закрыть без сохранения?" })
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});
