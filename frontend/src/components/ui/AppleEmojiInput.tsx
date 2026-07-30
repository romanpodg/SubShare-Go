"use client";

import { forwardRef, useRef, useCallback, useEffect, useImperativeHandle, type ReactNode } from "react";
import emojiRegex from "emoji-regex";

const APPLE_CDN =
  "https://cdn.jsdelivr.net/npm/emoji-datasource-apple@15.0.1/img/apple/64/";

function toUnified(emoji: string): string {
  return [...emoji]
    .map((c) => c.codePointAt(0)!.toString(16))
    .filter(Boolean)
    .join("-");
}

function esc(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/** Plain text → HTML with Apple emoji <img> tags */
function toHtml(text: string): string {
  if (!text) return "";
  const re = emojiRegex();
  let out = "";
  let last = 0;
  for (const m of text.matchAll(re)) {
    const i = m.index!;
    if (i > last) out += esc(text.slice(last, i)).replace(/\n/g, "<br>");
    const e = m[0];
    out += `<img src="${APPLE_CDN}${toUnified(e)}.png" alt="${e}" class="emoji-image" draggable="false" loading="lazy" onerror="this.outerHTML=this.alt">`;
    last = i + e.length;
  }
  if (last < text.length) out += esc(text.slice(last)).replace(/\n/g, "<br>");
  return out;
}

/** Extract plain text from contentEditable (img.alt → native emoji) */
function getText(node: Node): string {
  let s = "";
  for (const c of node.childNodes) {
    if (c.nodeType === Node.TEXT_NODE) s += c.textContent ?? "";
    else if (c instanceof HTMLBRElement) s += "\n";
    else if (c instanceof HTMLImageElement) s += c.alt;
    else if (c instanceof HTMLDivElement || c instanceof HTMLParagraphElement) {
      const nested = getText(c);
      s += nested;
      if (nested && !nested.endsWith("\n")) s += "\n";
    } else if (c instanceof HTMLElement) s += getText(c);
  }
  return s;
}

/** Count "plain text length" of a DOM node */
function nodeLen(n: Node): number {
  if (n.nodeType === Node.TEXT_NODE) return (n.textContent ?? "").length;
  if (n instanceof HTMLImageElement) return (n.alt ?? "").length;
  let l = 0;
  for (const c of n.childNodes) l += nodeLen(c);
  return l;
}

/** Save cursor position as a plain‑text offset */
function saveCursor(root: HTMLElement): number {
  const sel = window.getSelection();
  if (!sel?.rangeCount || !root.contains(sel.anchorNode)) return -1;
  const { startContainer, startOffset } = sel.getRangeAt(0);
  let pos = 0;
  const walk = (node: Node): boolean => {
    if (node === startContainer) {
      if (node.nodeType === Node.TEXT_NODE) {
        pos += startOffset;
      } else {
        for (let i = 0; i < startOffset; i++) {
          pos += nodeLen(node.childNodes[i]);
        }
      }
      return true;
    }
    if (node.nodeType === Node.TEXT_NODE) {
      pos += (node.textContent ?? "").length;
      return false;
    }
    if (node instanceof HTMLImageElement) {
      pos += (node.alt ?? "").length;
      return false;
    }
    for (const c of node.childNodes) {
      if (walk(c)) return true;
    }
    return false;
  };
  walk(root);
  return pos;
}

/** Restore cursor to a plain‑text offset after innerHTML replacement */
function restoreCursor(root: HTMLElement, target: number) {
  const sel = window.getSelection();
  if (!sel || target < 0) return;

  const walk = (node: Node, left: number): [Node, number] | null => {
    if (node.nodeType === Node.TEXT_NODE) {
      const len = (node.textContent ?? "").length;
      return left <= len ? [node, left] : null;
    }
    if (node instanceof HTMLImageElement) {
      const len = (node.alt ?? "").length;
      if (left <= 0) {
        const p = node.parentNode!;
        return [p, [...p.childNodes].indexOf(node)];
      }
      if (left <= len) {
        const p = node.parentNode!;
        return [p, [...p.childNodes].indexOf(node) + 1];
      }
      return null;
    }
    let rem = left;
    for (let i = 0; i < node.childNodes.length; i++) {
      const c = node.childNodes[i];
      const cLen = nodeLen(c);
      if (rem <= cLen) {
        const r = walk(c, rem);
        if (r) return r;
      }
      rem -= cLen;
    }
    return [node, node.childNodes.length];
  };

  const r = walk(root, target);
  if (r) {
    const range = document.createRange();
    range.setStart(r[0], r[1]);
    range.collapse(true);
    sel.removeAllRanges();
    sel.addRange(range);
  }
}

/* ------------------------------------------------------------------ */

interface AppleEmojiInputProps {
  label?: string;
  error?: string;
  value?: string;
  placeholder?: string;
  onChange?: (e: { target: { value: string } }) => void;
  className?: string;
  id?: string;
  multiline?: boolean;
  maxLength?: number;
  showCounter?: boolean;
  rightSlot?: ReactNode;
  rightSlotWidth?: number;
}

export interface AppleEmojiInputHandle {
  insertEmoji: (emoji: string) => void;
  insertText: (text: string) => void;
  focus: () => void;
}

export const AppleEmojiInput = forwardRef<AppleEmojiInputHandle, AppleEmojiInputProps>(function AppleEmojiInput({
  label,
  error,
  value = "",
  placeholder,
  onChange,
  className = "",
  id,
  multiline = false,
  maxLength,
  showCounter = false,
  rightSlot,
  rightSlotWidth = 46,
}, forwardedRef) {
  const ref = useRef<HTMLDivElement>(null);
  const lastVal = useRef(value);
  const savedCursorPosRef = useRef<number>(value.length);
  const pendingCursorPosRef = useRef<number | null>(null);
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");

  const syncSelection = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    const nextPos = saveCursor(el);
    if (nextPos >= 0) {
      savedCursorPosRef.current = nextPos;
    }
  }, []);

  const emitNextValue = useCallback((nextValue: string, nextCursorPos?: number) => {
    const limitedValue =
      typeof maxLength === "number" ? nextValue.slice(0, maxLength) : nextValue;
    lastVal.current = limitedValue;
    if (typeof nextCursorPos === "number") {
      pendingCursorPosRef.current = Math.min(nextCursorPos, limitedValue.length);
      savedCursorPosRef.current = pendingCursorPosRef.current;
    }
    onChange?.({ target: { value: limitedValue } });
  }, [maxLength, onChange]);

  useImperativeHandle(forwardedRef, () => {
    const insertText = (text: string) => {
      const el = ref.current;
      const baseValue = lastVal.current;
      const insertionPoint = el && document.activeElement === el
        ? Math.max(0, saveCursor(el))
        : savedCursorPosRef.current;
      const normalizedPoint = Math.min(Math.max(insertionPoint, 0), baseValue.length);
      emitNextValue(
        `${baseValue.slice(0, normalizedPoint)}${text}${baseValue.slice(normalizedPoint)}`,
        normalizedPoint + text.length
      );
    };

    return {
      insertEmoji: insertText,
      insertText,
      focus: () => {
        ref.current?.focus();
      },
    };
  }, [emitNextValue]);

  /* Sync external value → DOM (e.g. emoji picker appends emoji) */
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const restorePos = pendingCursorPosRef.current;
    if (value === lastVal.current && restorePos === null) return;
    lastVal.current = value;
    const isFocused = el === document.activeElement || restorePos !== null;
    el.innerHTML = toHtml(value);
    if (isFocused) {
      if (restorePos !== null) {
        el.focus();
        restoreCursor(el, restorePos);
        savedCursorPosRef.current = restorePos;
        pendingCursorPosRef.current = null;
      } else {
        restoreCursor(el, value.length);
        savedCursorPosRef.current = value.length;
      }
    }
  }, [value]);

  /* Initial render */
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.innerHTML = toHtml(value);
    lastVal.current = value;
    savedCursorPosRef.current = value.length;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /* On user input: extract text, replace raw emoji text‑nodes → img */
  const onInput = useCallback(() => {
    const el = ref.current;
    if (!el) return;

    const text = getText(el).replace(/\u00a0/g, " ").replace(/\n{3,}/g, "\n\n");
    const nextValue = typeof maxLength === "number" ? text.slice(0, maxLength) : text;
    lastVal.current = nextValue;

    // Only rewrite the DOM if raw emoji glyphs exist as text nodes
    let needsUpdate = false;
    const tw = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    let tn = tw.nextNode();
    while (tn) {
      if (emojiRegex().test(tn.textContent ?? "")) {
        needsUpdate = true;
        break;
      }
      tn = tw.nextNode();
    }

    if (needsUpdate || nextValue !== text) {
      const pos = saveCursor(el);
      el.innerHTML = toHtml(nextValue);
      restoreCursor(el, pos);
      savedCursorPosRef.current = Math.min(pos, nextValue.length);
    } else {
      const pos = saveCursor(el);
      if (pos >= 0) {
        savedCursorPosRef.current = Math.min(pos, nextValue.length);
      }
    }

    onChange?.({ target: { value: nextValue } });
  }, [maxLength, onChange]);

  const hasValue = value.length > 0;
  const currentLength = value.length;
  const hasRightSlot = Boolean(rightSlot);
  const inputPaddingRight = hasRightSlot ? `${rightSlotWidth + 12}px` : undefined;

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={inputId} className="text-sm text-zinc-400">
          {label}
        </label>
      )}

      <div className="relative">
        {/* Placeholder (visible only when empty) */}
        {!hasValue && placeholder && (
          <div
            className={`pointer-events-none absolute inset-0 px-3 text-sm text-zinc-500 ${multiline ? "pt-2.5" : "flex items-center"}`}
            style={hasRightSlot ? { paddingRight: inputPaddingRight } : undefined}
            aria-hidden="true"
          >
            {placeholder}
          </div>
        )}

        <div
          ref={ref}
          id={inputId}
          contentEditable
          role="textbox"
          aria-multiline={multiline}
          suppressContentEditableWarning
          onInput={onInput}
          onBlur={syncSelection}
          onKeyUp={syncSelection}
          onMouseUp={syncSelection}
          onFocus={syncSelection}
          onKeyDown={(e) => {
            if (!multiline && e.key === "Enter") {
              e.preventDefault();
              return;
            }
            if (multiline && e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              document.execCommand("insertLineBreak");
            }
          }}
          onPaste={(e) => {
            e.preventDefault();
            const text = e.clipboardData.getData("text/plain");
            const normalized = multiline ? text.replace(/\r\n/g, "\n") : text.replace(/[\r\n]+/g, " ");
            const truncated =
              typeof maxLength === "number"
                ? normalized.slice(0, Math.max(0, maxLength - getText(ref.current ?? document.createElement("div")).length))
                : normalized;
            document.execCommand(
              multiline ? "insertHTML" : "insertText",
              false,
              multiline ? esc(truncated).replace(/\n/g, "<br>") : truncated
            );
          }}
          className={`apple-emoji-input w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg ${multiline ? "apple-emoji-input--multiline min-h-[8.5rem]" : ""} ${error ? "ring-1 ring-red-500" : ""} ${className}`}
          style={hasRightSlot ? { paddingRight: inputPaddingRight } : undefined}
        />

        {hasRightSlot && (
          <div
            className="absolute right-0 top-0 bottom-0 flex w-[46px] items-center justify-center rounded-r-lg border-l border-border bg-surface-1/70"
            style={{ width: `${rightSlotWidth}px` }}
          >
            {rightSlot}
          </div>
        )}
      </div>

      {error && <p className="text-xs text-red-400">{error}</p>}
      {showCounter && typeof maxLength === "number" && (
        <div className="-mt-0.5 text-right text-xs text-zinc-500">
          {currentLength}/{maxLength}
        </div>
      )}
    </div>
  );
});
