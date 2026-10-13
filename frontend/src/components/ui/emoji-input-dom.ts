import { getEmojiAssetURL } from "@/lib/emoji-assets";
import emojiRegex from "emoji-regex";

export function esc(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/** Plain text → safe editor HTML with native Unicode emoji fallback. */
export function toHtml(text: string): string {
  if (!text) return "";
  const re = emojiRegex();
  let out = "";
  let last = 0;
  for (const m of text.matchAll(re)) {
    const i = m.index!;
    if (i > last) out += esc(text.slice(last, i)).replace(/\n/g, "<br>");
    const e = m[0];
    const assetURL = getEmojiAssetURL(e);
    out += assetURL
      ? `<img src="${assetURL}" alt="${esc(e)}" class="emoji-image" draggable="false" loading="lazy">`
      : `<span class="emoji-native">${esc(e)}</span>`;
    last = i + e.length;
  }
  if (last < text.length) out += esc(text.slice(last)).replace(/\n/g, "<br>");
  return out;
}

/** Extract plain text from contentEditable (img.alt → native emoji) */
function blockText(node: Node): string {
  const text = getText(node);
  if (!text) return text;
  return text.endsWith("\n") ? text : `${text}\n`;
}

function childText(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? "";
  if (node instanceof HTMLBRElement) return "\n";
  if (node instanceof HTMLImageElement) return node.alt;
  if (node instanceof HTMLDivElement) return blockText(node);
  if (node instanceof HTMLParagraphElement) return blockText(node);
  if (node instanceof HTMLElement) return getText(node);
  return "";
}

export function getText(node: Node): string {
  return Array.from(node.childNodes, childText).join("");
}
