"use client";

import { useCallback, useEffect, useImperativeHandle, useRef, type ForwardedRef } from "react";
import { getText, toHtml } from "./emoji-input-dom";
import { restoreCursor, saveSelection, type PlainTextSelection } from "./emoji-input-selection";
import { truncateText } from "./emoji-input-text";
import type { AppleEmojiInputHandle, AppleEmojiInputProps } from "./emoji-input-types";

export function useEmojiInput({ value = "", onChange, maxLength }: AppleEmojiInputProps, forwardedRef: ForwardedRef<AppleEmojiInputHandle>) {
  const ref = useRef<HTMLDivElement>(null);
  const lastVal = useRef(value);
  const initialValue = useRef(value);
  const savedSelectionRef = useRef<PlainTextSelection>({ start: value.length, end: value.length });
  const pendingCursorPosRef = useRef<number | null>(null);

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
    const initial = initialValue.current;
    el.innerHTML = toHtml(initial);
    lastVal.current = initial;
    savedSelectionRef.current = { start: initial.length, end: initial.length };
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

  return { ref, syncSelection, onInput };
}
