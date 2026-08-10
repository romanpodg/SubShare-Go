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
  it("shows a compact healthy badge and keeps the timestamp in the tooltip", () => {
    const { container } = render(<KeyHealthBadge healthKey={healthKey()} />);
    const badge = screen.getByText("ДОСТУПЕН · 2 ms");

    expect(badge).toHaveAttribute("data-health-state", "up");
    expect(badge).toHaveAttribute("title", "Последняя проверка: 10/08/2026 23:48");
    expect(container.textContent).not.toContain("10/08/2026 23:48");
  });

  it("shows an unavailable result with the error tone", () => {
    render(<KeyHealthBadge healthKey={healthKey({ check_status: "down", last_latency_ms: 0 })} />);
    expect(screen.getByText("НЕДОСТУПЕН")).toHaveAttribute("data-health-state", "down");
  });

  it.each([
    ["disabled with an old result", { status: "non-active" as const }],
    ["never checked", { last_checked_at: "" }],
    ["unknown result", { check_status: "unknown" as const }],
  ])("renders nothing for %s", (_name, overrides) => {
    const { container } = render(<KeyHealthBadge healthKey={healthKey(overrides)} />);
    expect(container).toBeEmptyDOMElement();
  });
});
