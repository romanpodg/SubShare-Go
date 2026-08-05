import { describe, expect, it } from "vitest";
import type { KeySummary } from "@/lib/types";
import { alignKeysBySubscriptionOrder, dragAutoScrollVelocity } from "./KeysSection";

function key(id: number, kind: KeySummary["kind"]): KeySummary {
  return {
    id,
    kind,
    label: `key-${id}`,
    category: "",
    template_text: "",
    status: "active",
    check_status: "unknown",
    check_error: "",
    last_latency_ms: 0,
    last_checked_at: "",
    external_source_id: 0,
    external_source_name: "",
    created_at: "",
  };
}

describe("alignKeysBySubscriptionOrder", () => {
  it("keeps one shared position and leaves the opposite column empty", () => {
    const rows = alignKeysBySubscriptionOrder([
      key(24, "informational"),
      key(1, "real"),
      key(2, "real"),
      key(25, "informational"),
    ]);

    expect(rows.map((row) => ({
      key: row.key.id,
      informational: row.informational?.id ?? null,
      real: row.real?.id ?? null,
    }))).toEqual([
      { key: 24, informational: 24, real: null },
      { key: 1, informational: null, real: 1 },
      { key: 2, informational: null, real: 2 },
      { key: 25, informational: 25, real: null },
    ]);
  });
});

describe("dragAutoScrollVelocity", () => {
  it("scrolls toward the nearest edge and accelerates close to it", () => {
    expect(dragAutoScrollVelocity(500, 0, 1000)).toBe(0);

    const nearTop = dragAutoScrollVelocity(80, 0, 1000);
    const atTop = dragAutoScrollVelocity(0, 0, 1000);
    expect(nearTop).toBeLessThan(0);
    expect(atTop).toBeLessThan(nearTop);

    const nearBottom = dragAutoScrollVelocity(920, 0, 1000);
    const atBottom = dragAutoScrollVelocity(1000, 0, 1000);
    expect(nearBottom).toBeGreaterThan(0);
    expect(atBottom).toBeGreaterThan(nearBottom);
  });

  it("adapts the edge zone for short nested scroll containers", () => {
    expect(dragAutoScrollVelocity(205, 200, 300)).toBeLessThan(0);
    expect(dragAutoScrollVelocity(250, 200, 300)).toBe(0);
    expect(dragAutoScrollVelocity(295, 200, 300)).toBeGreaterThan(0);
  });
});
