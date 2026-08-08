import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "@/components/ui/Toast";
import { KeyEditorModal } from "./KeyEditorModal";

vi.mock("@/lib/api", () => ({
  keys: {
    listCategories: vi.fn().mockResolvedValue({ categories: [] }),
    editorSchema: vi.fn().mockResolvedValue({
      data: {
        protocols: [
          {
            protocol: "shadowsocks",
            label: "Shadowsocks",
            supported_creations: ["raw", "structured"],
            fields: {
              method: { field_type: "select", required: true, can_clear: false },
              password: { field_type: "password", required: true, can_clear: false },
            },
          },
        ],
        exclusion_reason_codes: {},
      },
    }),
    get: vi.fn().mockResolvedValue({
      data: {
        id: 1,
        label: "Тестовый профиль",
        category: "",
        kind: "real",
        status: "active",
        ownership: "local",
        protocol: "shadowsocks",
        profile_revision: 1,
        safe_structured: {
          server: "example.com",
          port: "443",
          display_name: "Тестовый профиль",
          shadowsocks: {
            method: "2022-blake3-aes-128-gcm",
            password_present: true,
          },
        },
        capabilities: {
          plain: { status: "supported" },
        },
      },
    }),
    reveal: vi.fn().mockResolvedValue({
      data: {
        key_id: 1,
        confirmed_profile_revision: 1,
        target: "raw",
        raw_uri: "ss://2022-blake3-aes-128-gcm:secret@example.com:443#Test",
      },
    }),
    clone: vi.fn().mockResolvedValue({
      data: {
        id: 2,
        label: "Тестовый профиль (Копия)",
      },
    }),
    createProfile: vi.fn().mockResolvedValue({ data: { id: 2 } }),
    updateProfile: vi.fn().mockResolvedValue({ data: { id: 1 } }),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, msg: string) {
      super(msg);
      this.status = status;
    }
  },
}));

function renderEditor(keyId?: number) {
  const onClose = vi.fn();
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  const result = render(
    <ToastProvider>
      <KeyEditorModal
        open
        keyId={keyId}
        onClose={onClose}
        onRefresh={onRefresh}
      />
    </ToastProvider>
  );
  return { ...result, onClose, onRefresh };
}

describe("KeyEditorModal Stage 8", () => {
  it("renders new profile creation modal", async () => {
    renderEditor();
    await waitFor(() => {
      expect(screen.getByText("Добавить конфигурацию")).toBeInTheDocument();
    });
    expect(screen.getByLabelText("Название")).toBeInTheDocument();
  });

  it("loads and displays existing key safe details without exposing raw secret in initial view", async () => {
    renderEditor(1);
    await waitFor(() => {
      expect(screen.getByDisplayValue("Тестовый профиль")).toBeInTheDocument();
    });
    expect(screen.getByText("Секретные поля скрыты по умолчанию.")).toBeInTheDocument();
  });

  it("handles reveal secret call when requested", async () => {
    renderEditor(1);
    await waitFor(() => {
      expect(screen.getByDisplayValue("Тестовый профиль")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "XRAY-JSON" }));
    const revealBtn = await screen.findByRole("button", { name: "Раскрыть сырую ссылку" });
    fireEvent.click(revealBtn);

    await waitFor(() => {
      expect(screen.getByDisplayValue(/ss:\/\/2022-blake3-aes-128-gcm/)).toBeInTheDocument();
    });
  });
});
