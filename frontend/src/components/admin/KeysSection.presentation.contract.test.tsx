import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { KeySummary } from "@/lib/types";
import { KeysSection } from "./KeysSection";

const mocks = vi.hoisted(() => ({ listCategories: vi.fn(), toast: vi.fn() }));
vi.mock("@/lib/api", () => ({ keys: mocks }));
vi.mock("@/components/ui/Toast", () => ({ useToast: () => ({ toast: mocks.toast }) }));
afterEach(cleanup);
beforeEach(() => { vi.resetAllMocks(); mocks.listCategories.mockResolvedValue({ categories: [] }); });
function presentationKey(id: number, changes: Partial<KeySummary> = {}): KeySummary {
  return { id, label: `key-${id}`, kind: "real", protocol: "vless", client_display_name: "  effective client  ", category: "", template_text: "fixture information", status: "active", check_status: "up", check_error: "", last_latency_ms: 37, last_checked_at: "2026-10-11T00:00:00Z", external_source_id: 0, external_source_name: "", created_at: "", ...changes };
}
function presentationCard(key: KeySummary) { return screen.getByRole("checkbox", { name: `Выбрать конфигурацию ${key.label}` }).closest(".ui-key-card") as HTMLElement; }
const variants = (["links", "xray-json"] as const).flatMap((format) => (["real", "informational"] as const).flatMap((kind) => (["active", "non-active"] as const).flatMap((status) => [false, true].map((external) => ({ format, kind, status, external })) )));

describe("KeysSection card and aligned-row presentation parity", () => {
  it.each(variants)("retains $format/$kind/$status/external=$external labels and actions", ({ format, kind, status, external }) => {
    const key = presentationKey(1, { kind, status, external_source_id: external ? 9 : 0, external_source_name: external ? "fixture source" : "" });
    render(<KeysSection keys={[key]} subscriptionFormat={format} onRefresh={vi.fn()} />);
    const card = presentationCard(key);
    expect(within(card).getByText("ID 1")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Изменить" })).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Удалить" })).toBeInTheDocument();
    expect(within(card).queryByRole("button", { name: "Проверить" }) !== null).toBe(kind === "real");
    expect(within(card).queryByText("Неактивен JSON") !== null).toBe(kind === "real" && format === "links");
    expect(within(card).queryByText("Источник: fixture source") !== null).toBe(external);
    if (kind === "real") {
      expect(card.querySelector(".ui-key-card-client-name")).toHaveTextContent("effective client");
      expect(card.querySelector('[data-health-state="up"]')).toHaveTextContent("37 ms");
    } else {
      expect(within(card).getByText("fixture information")).toBeInTheDocument();
      expect(card.querySelector("[data-health-state]")).toBeNull();
    }
  });
  it("keeps one subscription position across both columns with opposite gaps", () => {
    const keys = [presentationKey(1, { kind: "informational" }), presentationKey(2), presentationKey(3), presentationKey(4, { kind: "informational" })];
    const { container } = render(<KeysSection keys={keys} subscriptionFormat="xray-json" onRefresh={vi.fn()} />);
    for (const column of ["info", "real"]) {
      const slots = container.querySelectorAll(`.ui-key-order-column-${column} .ui-key-order-slot`);
      expect([...slots].map((slot) => slot.getAttribute("data-order-position"))).toEqual(["1", "2", "3", "4"]);
    }
    for (const key of keys) expect(presentationCard(key).closest("[data-order-position]")).toHaveAttribute("data-order-position", String(key.id));
    expect(container.querySelectorAll(".ui-key-order-gap")).toHaveLength(4);
  });
  it("uses the safe fallback source/client/info labels without injecting HTML", () => {
    const real = presentationKey(1, { label: "<script>fixture</script>", client_display_name: " ", external_source_id: 9 });
    const info = presentationKey(2, { kind: "informational", template_text: "", label: "fixture fallback info" });
    const { container } = render(<KeysSection keys={[real, info]} subscriptionFormat="xray-json" onRefresh={vi.fn()} />);
    expect(presentationCard(real).querySelector(".ui-key-card-client-name")).toHaveTextContent(real.label);
    expect(within(presentationCard(real)).getByText("Сторонняя подписка")).toBeInTheDocument();
    expect(presentationCard(info)).toHaveTextContent(info.label);
    expect(container.querySelector("script")).toBeNull();
  });
  it("retains alignment while one pane is hidden and restored", () => {
    const keys = [presentationKey(1, { kind: "informational" }), presentationKey(2)];
    const { container } = render(<KeysSection keys={keys} subscriptionFormat="xray-json" onRefresh={vi.fn()} />);
    fireEvent.click(screen.getAllByRole("button", { name: /Скрыть колонку/ })[0]);
    expect(container.querySelector(".ui-key-order-grid")).toHaveClass("ui-key-order-grid-info-hidden");
    fireEvent.click(screen.getByRole("button", { name: /Показать колонку/ }));
    expect(container.querySelector(".ui-key-order-grid")).not.toHaveClass("ui-key-order-grid-info-hidden");
  });
});
