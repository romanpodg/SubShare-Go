import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { emojiToUnified, getEmojiAssetURL } from "./emoji-assets";

describe("emoji asset strategy", () => {
  it("generates only an explicitly configured same-origin asset URL", () => {
    expect(emojiToUnified("😀")).toBe("1f600");
    expect(getEmojiAssetURL("😀")).toBeNull();
    expect(getEmojiAssetURL("😀", "/emoji/native/15/")).toBe("/emoji/native/15/1f600.png");
    expect(() => getEmojiAssetURL("😀", "https://cdn.example/emoji")).toThrow(/same-origin/);
  });

  it("contains no jsDelivr runtime references in emoji components", () => {
    const sources = [
      "src/components/ui/EmojiPickerButton.tsx",
      "src/components/ui/AppleEmojiInput.tsx",
      "src/components/ui/EmojiText.tsx",
    ].map((path) => readFileSync(resolve(process.cwd(), path), "utf8"));
    expect(sources.join("\n")).not.toContain("cdn.jsdelivr.net");
  });
});
