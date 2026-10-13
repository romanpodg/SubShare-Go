"use client";

import { useCallback, useEffect, useImperativeHandle, useMemo, useRef, type ForwardedRef } from "react";
import { emitEmojiValue, emojiInputHandle, initializeEmojiValue, readEmojiInput, syncEmojiSelection, syncEmojiValue } from "./emoji-input-controller";
import type { PlainTextSelection } from "./emoji-input-selection";
import type { AppleEmojiInputHandle, AppleEmojiInputProps } from "./emoji-input-types";

export function useEmojiInput({ value = "", onChange, maxLength }: AppleEmojiInputProps, forwardedRef: ForwardedRef<AppleEmojiInputHandle>) {
  const ref = useRef<HTMLDivElement>(null);
  const lastValue = useRef(value);
  const initialValue = useRef(value);
  const selection = useRef<PlainTextSelection>({ start: value.length, end: value.length });
  const pendingCursor = useRef<number | null>(null);
  const state = useMemo(() => ({ element: ref, value: lastValue, selection, pendingCursor }), [lastValue, selection, pendingCursor]);
  const syncSelection = useCallback(() => syncEmojiSelection(state), [state]);
  const emit = useCallback((text: string, cursor?: number) => emitEmojiValue(state, { maxLength, onChange }, text, cursor), [state, maxLength, onChange]);
  const onInput = useCallback(() => readEmojiInput(state, { maxLength, onChange }), [state, maxLength, onChange]);

  useImperativeHandle(forwardedRef, () => emojiInputHandle(state, emit), [state, emit]);
  useEffect(() => syncEmojiValue(state, value), [state, value]);
  useEffect(() => initializeEmojiValue(state, initialValue.current), [state]);
  return { ref, syncSelection, onInput };
}
