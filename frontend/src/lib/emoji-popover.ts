export const EMOJI_POPOVER_DESKTOP_WIDTH = 352;
export const EMOJI_POPOVER_HEIGHT = 400;
export const EMOJI_POPOVER_MARGIN = 12;
export const EMOJI_POPOVER_GAP = 8;

export interface EmojiPopoverLayout {
  height: number;
  left: number;
  perLine: number;
  placement: "top" | "bottom";
  top: number;
  width: number;
}

interface ViewportSize {
  height: number;
  width: number;
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(value, minimum), maximum);
}

export function emojiPerLine(width: number): number {
  return clamp(Math.floor((width - 24) / 34), 6, 9);
}

export function calculateEmojiPopoverLayout(
  trigger: Pick<DOMRect, "bottom" | "left" | "right" | "top">,
  viewport: ViewportSize
): EmojiPopoverLayout {
  const availableWidth = Math.max(0, viewport.width - EMOJI_POPOVER_MARGIN * 2);
  const availableHeight = Math.max(0, viewport.height - EMOJI_POPOVER_MARGIN * 2);
  const width = Math.min(EMOJI_POPOVER_DESKTOP_WIDTH, availableWidth);
  const height = Math.min(EMOJI_POPOVER_HEIGHT, availableHeight);
  const spaceBelow = viewport.height - trigger.bottom - EMOJI_POPOVER_GAP - EMOJI_POPOVER_MARGIN;
  const spaceAbove = trigger.top - EMOJI_POPOVER_GAP - EMOJI_POPOVER_MARGIN;
  const placement = spaceBelow < height && spaceAbove > spaceBelow ? "top" : "bottom";
  const requestedTop = placement === "top"
    ? trigger.top - EMOJI_POPOVER_GAP - height
    : trigger.bottom + EMOJI_POPOVER_GAP;
  const top = clamp(requestedTop, EMOJI_POPOVER_MARGIN, Math.max(EMOJI_POPOVER_MARGIN, viewport.height - height - EMOJI_POPOVER_MARGIN));
  const requestedLeft = trigger.right - width;
  const left = clamp(requestedLeft, EMOJI_POPOVER_MARGIN, Math.max(EMOJI_POPOVER_MARGIN, viewport.width - width - EMOJI_POPOVER_MARGIN));

  return { height, left, perLine: emojiPerLine(width), placement, top, width };
}
