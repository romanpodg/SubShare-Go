import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "@/components/ui/Toast";
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

const validLink =
  "vless://00000000-0000-0000-0000-000000000000@example.com:443?type=ws&security=tls#Edge";

function renderEditor(kind: "real" | "informational" = "real") {
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
          rawConfig: kind === "real" ? validLink : "",
        }}
      />
    </ToastProvider>
  );
  return { ...result, onClose, onSubmit };
}

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
