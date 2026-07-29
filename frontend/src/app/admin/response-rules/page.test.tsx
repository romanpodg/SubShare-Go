import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ResponseRulesPage from "./page";

const mocks = vi.hoisted(() => ({
  rulesList: vi.fn(),
  templatesList: vi.fn(),
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
  useToast: () => ({ toast: vi.fn() }),
}));
vi.mock("@/hooks/useAuth", () => ({
  useAuth: () => ({ role: "owner" }),
}));

describe("ResponseRulesPage", () => {
  beforeEach(() => {
    mocks.rulesList.mockResolvedValue({ data: [] });
    mocks.templatesList.mockResolvedValue({ data: [] });
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
});
