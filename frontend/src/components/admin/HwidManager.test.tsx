import { describe, expect, it } from "vitest";
import { connectedDevicesHeading } from "./HwidManager";

describe("connectedDevicesHeading", () => {
  it.each([
    { connected: 0, maximum: 0, expected: "Подключенные устройства: 0 — без ограничений" },
    { connected: 3, maximum: 0, expected: "Подключенные устройства: 3 — без ограничений" },
    { connected: 0, maximum: 5, expected: "Подключенные устройства (0/5)" },
    { connected: 2, maximum: 5, expected: "Подключенные устройства (2/5)" },
  ])("formats connected=$connected maximum=$maximum", ({ connected, maximum, expected }) => {
    expect(connectedDevicesHeading(connected, maximum)).toBe(expected);
  });

  it("switches immediately between finite and unlimited wording", () => {
    expect(connectedDevicesHeading(2, 5)).toContain("2/5");
    expect(connectedDevicesHeading(2, 0)).toContain("без ограничений");
    expect(connectedDevicesHeading(2, 3)).toContain("2/3");
  });
});
