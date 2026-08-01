import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ResponseRulesPage from "./page";

const mocks = vi.hoisted(() => ({
  rulesList: vi.fn(),
  templatesList: vi.fn(),
  toast: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  apiV1: {
    responseRules: {
      list: mocks.rulesList,
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
    },
    templates: { list: mocks.templatesList },
  },
}));
vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast: mocks.toast }),
}));
vi.mock("@/hooks/useAuth", () => ({
  useAuth: () => ({ role: "owner" }),
}));

describe("ResponseRulesPage", () => {
  beforeEach(() => {
    mocks.rulesList.mockResolvedValue({ data: [] });
    mocks.templatesList.mockResolvedValue({ data: [] });
    mocks.toast.mockReset();
  });

  it("allows conditions and safe response headers to be assembled visually", async () => {
    render(<ResponseRulesPage />);

    expect(await screen.findByText("Порядок выполнения")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "Добавить" })[0]);
    expect(screen.getAllByLabelText("Заголовок")).toHaveLength(2);

    fireEvent.click(screen.getAllByRole("button", { name: "Добавить" })[1]);
    expect(screen.getByLabelText("Имя заголовка")).toBeInTheDocument();
    expect(screen.getByLabelText("Значение заголовка")).toBeInTheDocument();
  });

  it("uses one semantic surface for JSON mode and only applies validated drafts", async () => {
    render(<ResponseRulesPage />);
    await screen.findByText("Порядок выполнения");
    fireEvent.click(screen.getByRole("button", { name: "JSON-режим" }));

    const editor = screen.getByTestId("response-rule-json-editor");
    const textarea = screen.getByLabelText("JSON правила");
    expect(editor).toHaveClass("ui-json-editor");
    expect(textarea).toHaveClass("ui-json-editor-textarea");

    fireEvent.change(textarea, { target: { value: "[]" } });
    fireEvent.click(screen.getByRole("button", { name: "Применить JSON" }));
    expect(screen.getByTestId("response-rule-json-editor")).toBeInTheDocument();
    expect(mocks.toast).toHaveBeenLastCalledWith("Rule JSON must be an object, not an array or null.", "error");

    fireEvent.change(textarea, {
      target: {
        value: JSON.stringify({
          name: "JSON rule",
          description: "normalized",
          enabled: true,
          priority: 10,
          operator: "OR",
          conditions: [],
          response_type: "base64",
          template_id: null,
          headers: [],
        }),
      },
    });
    fireEvent.click(screen.getByRole("button", { name: "Применить JSON" }));
    expect(screen.getByLabelText("Название")).toHaveValue("JSON rule");
  });

  it("uses informational and success tokens for rule format and enabled state", async () => {
    mocks.rulesList.mockResolvedValue({
      data: [{
        id: 1,
        name: "Default",
        description: "",
        enabled: true,
        priority: 100,
        operator: "AND",
        conditions: [],
        response_type: "base64",
        template_id: null,
        headers: [],
        is_system: false,
        created_at: "",
        updated_at: "",
      }],
    });
    render(<ResponseRulesPage />);

    expect(await screen.findByText("base64")).toHaveClass("text-info");
    expect(screen.getByText("включено")).toHaveClass("text-success");
  });
});
