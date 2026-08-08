"use client";

import { type ReactNode, useState } from "react";
import emojiRegex from "emoji-regex";
import { getEmojiAssetURL } from "@/lib/emoji-assets";

interface EmojiTextProps {
  text: string;
  className?: string;
  truncate?: boolean;
  title?: string;
}

export function EmojiGlyph({ emoji, assetURL = getEmojiAssetURL(emoji) }: { emoji: string; assetURL?: string | null }) {
  const [failed, setFailed] = useState(false);
  if (!assetURL || failed) return <span className="emoji-native">{emoji}</span>;
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={assetURL} alt={emoji} className="emoji-image" draggable={false} loading="lazy" onError={() => setFailed(true)} />
  );
}

export function EmojiText({ text, className = "", truncate = false, title }: EmojiTextProps) {
  const regex = emojiRegex();
  const nodes: ReactNode[] = [];
  let lastIndex = 0;
  let keyIndex = 0;
  const rootClassName = `emoji-text ${truncate ? "emoji-text--truncate" : ""} ${className}`.trim();
  const computedTitle = title ?? (truncate ? text : undefined);

  for (const match of text.matchAll(regex)) {
    const matchIndex = typeof match.index === "number" ? match.index : -1;
    if (matchIndex < 0) continue;
    if (matchIndex > lastIndex) nodes.push(<span key={`text-${keyIndex++}`}>{text.slice(lastIndex, matchIndex)}</span>);
    const nativeEmoji = match[0];
    nodes.push(<EmojiGlyph key={`emoji-${keyIndex++}`} emoji={nativeEmoji} />);
    lastIndex = matchIndex + nativeEmoji.length;
  }

  if (lastIndex < text.length) nodes.push(<span key={`text-${keyIndex++}`}>{text.slice(lastIndex)}</span>);
  if (nodes.length === 0) return <span className={rootClassName} title={computedTitle}>{text}</span>;
  return <span className={rootClassName} title={computedTitle}>{nodes}</span>;
}
