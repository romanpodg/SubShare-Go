import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  createNumericGlobePoints,
  NumericGlobe,
  projectNumericGlobePoint,
} from "./NumericGlobe";

const context = {
  clearRect: vi.fn(),
  fillText: vi.fn(),
  setTransform: vi.fn(),
  fillStyle: "",
  font: "",
  globalAlpha: 1,
  textAlign: "center",
  textBaseline: "middle",
};

describe("NumericGlobe", () => {
  let reducedMotion = false;
  let requestAnimationFrameSpy: ReturnType<typeof vi.spyOn>;
  let cancelAnimationFrameSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    reducedMotion = false;
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(
      context as unknown as CanvasRenderingContext2D
    );
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 400,
      bottom: 400,
      width: 400,
      height: 400,
      toJSON: () => ({}),
    });
    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockImplementation(() => ({
        matches: reducedMotion,
        media: "(prefers-reduced-motion: reduce)",
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }))
    );
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        disconnect() {}
      }
    );
    requestAnimationFrameSpy = vi
      .spyOn(window, "requestAnimationFrame")
      .mockImplementation(() => 11);
    cancelAnimationFrameSpy = vi
      .spyOn(window, "cancelAnimationFrame")
      .mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    context.clearRect.mockClear();
    context.fillText.mockClear();
    context.setTransform.mockClear();
  });

  it("generates a deterministic sphere of digits", () => {
    const first = createNumericGlobePoints(32);
    const second = createNumericGlobePoints(32);

    expect(first).toEqual(second);
    expect(first).toHaveLength(32);
    expect(first.every((point) => /^[0-9]$/.test(point.digit))).toBe(true);
    expect(first.every((point) => Math.abs(Math.hypot(point.x, point.y, point.z) - 1) < 0.00001)).toBe(true);
  });

  it("projects foreground digits larger and brighter than background digits", () => {
    const front = projectNumericGlobePoint(
      { x: 0, y: 0, z: 1, digit: "1", accent: false },
      0
    );
    const back = projectNumericGlobePoint(
      { x: 0, y: 0, z: -1, digit: "2", accent: false },
      0
    );

    expect(front.scale).toBeGreaterThan(back.scale);
    expect(front.opacity).toBeGreaterThan(back.opacity);
  });

  it("starts animation and cancels it on unmount", () => {
    const { unmount } = render(<NumericGlobe />);

    expect(screen.getByRole("img", { name: "Цифровой глобус сети SubShare" })).toHaveAttribute(
      "data-motion",
      "animated"
    );
    expect(requestAnimationFrameSpy).toHaveBeenCalled();

    unmount();
    expect(cancelAnimationFrameSpy).toHaveBeenCalledWith(11);
  });

  it("renders a static frame when reduced motion is enabled", () => {
    reducedMotion = true;

    render(<NumericGlobe />);

    act(() => undefined);
    expect(screen.getByTestId("numeric-globe")).toHaveAttribute("data-motion", "static");
    expect(requestAnimationFrameSpy).not.toHaveBeenCalled();
    expect(context.fillText).toHaveBeenCalled();
  });
});
