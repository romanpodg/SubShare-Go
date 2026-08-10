"use client";

import { forwardRef, useRef, useCallback, useEffect, useImperativeHandle, type ReactNode } from "react";
import emojiRegex from "emoji-regex";
import { getEmojiAssetURL } from "@/lib/emoji-assets";

function esc(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/** Plain text → safe editor HTML with native Unicode emoji fallback. */
function toHtml(text: string): string {
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

function boundaryOffset(root: HTMLElement, target: Node, targetOffset: number): number {
  let pos = 0;
  const walk = (node: Node): boolean => {
    if (node === target) {
      if (node.nodeType === Node.TEXT_NODE) {
        pos += targetOffset;
      } else {
        for (let i = 0; i < targetOffset; i++) {
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
  return walk(root) ? pos : -1;
}

interface PlainTextSelection {
  start: number;
  end: number;
}

/** Save the selection as plain-text UTF-16 offsets. */
function saveSelection(root: HTMLElement): PlainTextSelection | null {
  const selection = window.getSelection();
  if (!selection?.rangeCount) return null;
  const range = selection.getRangeAt(0);
  if (!root.contains(range.startContainer) || !root.contains(range.endContainer)) return null;
  const start = boundaryOffset(root, range.startContainer, range.startOffset);
  const end = boundaryOffset(root, range.endContainer, range.endOffset);
  if (start < 0 || end < 0) return null;
  return { start: Math.min(start, end), end: Math.max(start, end) };
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

const graphemeSegmenter = typeof Intl.Segmenter === "function"
  ? new Intl.Segmenter(undefined, { granularity: "grapheme" })
  : null;

function textGraphemes(text: string): string[] {
  if (graphemeSegmenter) return Array.from(graphemeSegmenter.segment(text), (item) => item.segment);
  return Array.from(text);
}

function truncateText(text: string, maxLength?: number): string {
  if (typeof maxLength !== "number") return text;
  return textGraphemes(text).slice(0, maxLength).join("");
}

function textLength(text: string): number {
  return textGraphemes(text).length;
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
  const savedSelectionRef = useRef<PlainTextSelection>({ start: value.length, end: value.length });
  const pendingCursorPosRef = useRef<number | null>(null);
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");
  const labelId = label && inputId ? `${inputId}-label` : undefined;

  const syncSelection = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    const nextSelection = saveSelection(el);
    if (nextSelection) {
      savedSelectionRef.current = nextSelection;
    }
  }, []);

  const emitNextValue = useCallback((nextValue: string, nextCursorPos?: number) => {
    const limitedValue = truncateText(nextValue, maxLength);
    lastVal.current = limitedValue;
    if (typeof nextCursorPos === "number") {
      pendingCursorPosRef.current = Math.min(nextCursorPos, limitedValue.length);
      savedSelectionRef.current = { start: pendingCursorPosRef.current, end: pendingCursorPosRef.current };
    }
    onChange?.({ target: { value: limitedValue } });
  }, [maxLength, onChange]);

  useImperativeHandle(forwardedRef, () => {
    const insertText = (text: string) => {
      const el = ref.current;
      const baseValue = lastVal.current;
      const activeSelection = el && document.activeElement === el ? saveSelection(el) : null;
      const selection = activeSelection ?? savedSelectionRef.current;
      const start = Math.min(Math.max(selection.start, 0), baseValue.length);
      const end = Math.min(Math.max(selection.end, start), baseValue.length);
      emitNextValue(
        `${baseValue.slice(0, start)}${text}${baseValue.slice(end)}`,
        start + text.length
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
        savedSelectionRef.current = { start: restorePos, end: restorePos };
        pendingCursorPosRef.current = null;
      } else {
        restoreCursor(el, value.length);
        savedSelectionRef.current = { start: value.length, end: value.length };
      }
    }
  }, [value]);

  /* Initial render */
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.innerHTML = toHtml(value);
    lastVal.current = value;
    savedSelectionRef.current = { start: value.length, end: value.length };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /* On user input: extract plain text and only rewrite when length limiting requires it. */
  const onInput = useCallback(() => {
    const el = ref.current;
    if (!el) return;

    const text = getText(el).replace(/\u00a0/g, " ").replace(/\n{3,}/g, "\n\n");
    const nextValue = truncateText(text, maxLength);
    lastVal.current = nextValue;

    if (nextValue !== text) {
      const pos = saveSelection(el)?.start ?? nextValue.length;
      el.innerHTML = toHtml(nextValue);
      restoreCursor(el, pos);
      savedSelectionRef.current = { start: Math.min(pos, nextValue.length), end: Math.min(pos, nextValue.length) };
    } else {
      const selection = saveSelection(el);
      if (selection) {
        savedSelectionRef.current = selection;
      }
    }

    onChange?.({ target: { value: nextValue } });
  }, [maxLength, onChange]);

  const hasValue = value.length > 0;
  const currentLength = textLength(value);
  const hasRightSlot = Boolean(rightSlot);
  const inputPaddingRight = hasRightSlot ? `${rightSlotWidth + 12}px` : undefined;

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label id={labelId} htmlFor={inputId} className="text-sm text-zinc-400">
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
          aria-labelledby={labelId}
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
                ? truncateText(normalized, Math.max(0, maxLength - textLength(getText(ref.current ?? document.createElement("div")))))
                : normalized;
            document.execCommand(
              multiline ? "insertHTML" : "insertText",
              false,
              multiline ? esc(truncated).replace(/\n/g, "<br>") : truncated
            );
          }}
          onErrorCapture={(event) => {
            const image = event.target;
            if (!(image instanceof HTMLImageElement)) return;
            image.replaceWith(document.createTextNode(image.alt));
            onInput();
          }}
          className={`apple-emoji-input w-full rounded-sm border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 hover:border-[var(--border-strong)] focus:border-accent focus-visible:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg ${multiline ? "apple-emoji-input--multiline min-h-[8.5rem]" : ""} ${error ? "ring-1 ring-red-500" : ""} ${className}`}
          style={hasRightSlot ? { paddingRight: inputPaddingRight } : undefined}
        />

        {hasRightSlot && (
          <div
            className="absolute right-0 top-0 bottom-0 flex w-[46px] items-center justify-center rounded-r-sm border-l border-border bg-surface-1/70"
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
