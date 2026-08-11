import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { KeySummary } from "@/lib/types";
import { KeyHealthBadge } from "./KeyHealthBadge";

function healthKey(overrides: Partial<KeySummary> = {}): KeySummary {
  return {
    id: 1,
    label: "Node",
    client_display_name: "Node",
    category: "",
    kind: "real",
    status: "active",
    check_status: "up",
    check_error: "",
    last_latency_ms: 2,
    last_checked_at: "10/08/2026 23:48",
    external_source_id: 0,
    external_source_name: "",
    created_at: "",
    ...overrides,
  };
}

describe("KeyHealthBadge", () => {
  it("shows a compact healthy badge for an active profile", () => {
    const { container } = render(<KeyHealthBadge healthKey={healthKey()} />);
    const badge = screen.getByText("ДОСТУПЕН · 2 ms");

    expect(badge).toHaveAttribute("data-health-state", "up");
    expect(badge).toHaveAttribute("title", "Последняя проверка: 10/08/2026 23:48");
    expect(container.textContent).not.toContain("10/08/2026 23:48");
  });

  it("shows a healthy result with latency for an inactive profile", () => {
    render(<KeyHealthBadge healthKey={healthKey({ status: "non-active", last_latency_ms: 24 })} />);

    expect(screen.getByText("ДОСТУПЕН · 24 ms")).toHaveAttribute("data-health-state", "up");
  });

  it("shows an unavailable result with the error tone for an inactive profile", () => {
    render(
      <KeyHealthBadge
        healthKey={healthKey({ status: "non-active", check_status: "down", last_latency_ms: 0 })}
      />
    );

    expect(screen.getByText("НЕДОСТУПЕН")).toHaveAttribute("data-health-state", "down");
    expect(screen.getByText("НЕДОСТУПЕН")).toHaveClass("text-red-300");
  });

  it("shows a checked unknown result as a check error", () => {
    render(<KeyHealthBadge healthKey={healthKey({ check_status: "unknown", check_error: "probe unsupported" })} />);

    expect(screen.getByText("ОШИБКА ПРОВЕРКИ")).toHaveAttribute("data-health-state", "unknown");
  });

  it("shows an unsupported check as a neutral state without latency", () => {
    render(<KeyHealthBadge healthKey={healthKey({ check_status: "unsupported_check", last_latency_ms: 99 })} />);

    const badge = screen.getByText("ПРОВЕРКА НЕ ПОДДЕРЖИВАЕТСЯ");
    expect(badge).toHaveAttribute("data-health-state", "unsupported_check");
    expect(badge).toHaveClass("text-slate-300");
    expect(badge).not.toHaveTextContent("ms");
  });

  it.each([
    ["active profile never checked", { last_checked_at: "" }],
    ["inactive profile never checked", { status: "non-active" as const, last_checked_at: "" }],
  ])("renders nothing for %s", (_name, overrides) => {
    const { container } = render(<KeyHealthBadge healthKey={healthKey(overrides)} />);
    expect(container).toBeEmptyDOMElement();
  });
});
