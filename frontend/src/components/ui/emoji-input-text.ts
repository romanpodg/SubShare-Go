const graphemeSegmenter = typeof Intl.Segmenter === "function"
  ? new Intl.Segmenter(undefined, { granularity: "grapheme" })
  : null;

function textGraphemes(text: string): string[] {
  if (graphemeSegmenter) return Array.from(graphemeSegmenter.segment(text), (item) => item.segment);
  return Array.from(text);
}

export function truncateText(text: string, maxLength?: number): string {
  if (typeof maxLength !== "number") return text;
  return textGraphemes(text).slice(0, maxLength).join("");
}

export function textLength(text: string): number {
  return textGraphemes(text).length;
}

