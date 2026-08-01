import { describe, expect, it } from "vitest";
import {
  calculateEmojiPopoverLayout,
  EMOJI_POPOVER_DESKTOP_WIDTH,
  EMOJI_POPOVER_HEIGHT,
} from "./emoji-popover";

describe("calculateEmojiPopoverLayout", () => {
  it("uses stable desktop dimensions", () => {
    const layout = calculateEmojiPopoverLayout(
      { top: 60, bottom: 104, left: 800, right: 900 },
      { width: 1280, height: 900 }
    );
    expect(layout).toMatchObject({
      width: EMOJI_POPOVER_DESKTOP_WIDTH,
      height: EMOJI_POPOVER_HEIGHT,
      placement: "bottom",
      perLine: 9,
    });
  });

  it("uses the available width on a narrow viewport", () => {
    const layout = calculateEmojiPopoverLayout(
      { top: 60, bottom: 104, left: 260, right: 304 },
      { width: 320, height: 700 }
    );
    expect(layout.width).toBe(296);
    expect(layout.left).toBe(12);
    expect(layout.perLine).toBe(8);
  });

  it("stays inside the right edge and flips above a bottom-edge trigger", () => {
    const layout = calculateEmojiPopoverLayout(
      { top: 700, bottom: 744, left: 970, right: 1018 },
      { width: 1024, height: 768 }
    );
    expect(layout.placement).toBe("top");
    expect(layout.left + layout.width).toBeLessThanOrEqual(1012);
    expect(layout.top).toBeGreaterThanOrEqual(12);
    expect(layout.top + layout.height).toBeLessThanOrEqual(756);
  });
});
