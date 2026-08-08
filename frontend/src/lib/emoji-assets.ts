export const EMOJI_PRESENTATION_SET = "native" as const;

export function emojiToUnified(emoji: string): string {
  return Array.from(emoji)
    .map((character) => character.codePointAt(0)?.toString(16) ?? "")
    .filter(Boolean)
    .join("-");
}

/**
 * Native Unicode is the production strategy, so no image URL is returned by
 * default. A future vendored set must opt into an absolute same-origin path.
 */
export function getEmojiAssetURL(emoji: string, localBasePath?: string): string | null {
  if (!localBasePath) return null;
  if (!localBasePath.startsWith("/") || localBasePath.startsWith("//") || localBasePath.includes("://")) {
    throw new Error("Emoji asset paths must be same-origin absolute paths.");
  }
  const basePath = localBasePath.replace(/\/+$/, "");
  return `${basePath}/${emojiToUnified(emoji)}.png`;
}
