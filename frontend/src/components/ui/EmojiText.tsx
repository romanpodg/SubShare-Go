"use client";

import { type ReactNode, useState } from "react";
import emojiRegex from "emoji-regex";

interface EmojiTextProps {
  text: string;
  className?: string;
}

const APPLE_EMOJI_BASE_URL = "https://cdn.jsdelivr.net/npm/emoji-datasource-apple@15.0.1/img/apple/64/";

function toUnified(emoji: string): string {
  return Array.from(emoji)
    .map((char) => char.codePointAt(0)?.toString(16) ?? "")
    .filter(Boolean)
    .join("-");
}

function AppleEmojiImg({ emoji, unified }: { emoji: string; unified: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return <span>{emoji}</span>;
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={`${APPLE_EMOJI_BASE_URL}${unified}.png`}
      alt={emoji}
      className="emoji-image"
      draggable={false}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}

export function EmojiText({ text, className = "" }: EmojiTextProps) {
  const regex = emojiRegex();
  const nodes: ReactNode[] = [];
  let lastIndex = 0;
  let keyIndex = 0;

  for (const match of text.matchAll(regex)) {
    const matchIndex = typeof match.index === "number" ? match.index : -1;
    if (matchIndex < 0) {
      continue;
    }

    if (matchIndex > lastIndex) {
      nodes.push(
        <span key={`text-${keyIndex++}`}>
          {text.slice(lastIndex, matchIndex)}
        </span>
      );
    }

    const nativeEmoji = match[0];
    const unified = toUnified(nativeEmoji);
    nodes.push(
      <AppleEmojiImg key={`emoji-${keyIndex++}`} emoji={nativeEmoji} unified={unified} />
    );

    lastIndex = matchIndex + nativeEmoji.length;
  }

  if (lastIndex < text.length) {
    nodes.push(
      <span key={`text-${keyIndex++}`}>
        {text.slice(lastIndex)}
      </span>
    );
  }

  if (nodes.length === 0) {
    return <span className={`emoji-text ${className}`.trim()}>{text}</span>;
  }

  return <span className={`emoji-text ${className}`.trim()}>{nodes}</span>;
}