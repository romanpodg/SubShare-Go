import type { RefObject } from "react";
import { getText, toHtml } from "./emoji-input-dom";
import { restoreCursor, saveSelection, type PlainTextSelection } from "./emoji-input-selection";
import { truncateText } from "./emoji-input-text";
import type { AppleEmojiInputHandle, AppleEmojiInputProps } from "./emoji-input-types";

export interface EmojiInputState {
  element: RefObject<HTMLDivElement | null>;
  value: RefObject<string>;
  selection: RefObject<PlainTextSelection>;
  pendingCursor: RefObject<number | null>;
}

type InputOptions = Pick<AppleEmojiInputProps, "onChange" | "maxLength">;
type EmitValue = (value: string, cursor?: number) => void;

function rememberCaret(state: EmojiInputState, position: number) {
  state.selection.current = { start: position, end: position };
}

export function syncEmojiSelection(state: EmojiInputState) {
  const element = state.element.current;
  if (!element) return;
  const selection = saveSelection(element);
  if (selection) state.selection.current = selection;
}

export function emitEmojiValue(state: EmojiInputState, options: InputOptions, value: string, cursor?: number) {
  const limited = truncateText(value, options.maxLength);
  state.value.current = limited;
  if (typeof cursor === "number") {
    state.pendingCursor.current = Math.min(cursor, limited.length);
    rememberCaret(state, state.pendingCursor.current);
  }
  options.onChange?.({ target: { value: limited } });
}

function activeSelection(state: EmojiInputState) {
  const element = state.element.current;
  if (!element) return null;
  if (document.activeElement !== element) return null;
  return saveSelection(element);
}

function insertEmojiText(state: EmojiInputState, text: string, emit: EmitValue) {
  const base = state.value.current;
  const selection = activeSelection(state) ?? state.selection.current;
  const start = Math.min(Math.max(selection.start, 0), base.length);
  const end = Math.min(Math.max(selection.end, start), base.length);
  emit(`${base.slice(0, start)}${text}${base.slice(end)}`, start + text.length);
}

export function emojiInputHandle(state: EmojiInputState, emit: EmitValue): AppleEmojiInputHandle {
  const insertText = (text: string) => insertEmojiText(state, text, emit);
  return { insertEmoji: insertText, insertText, focus: () => state.element.current?.focus() };
}

function restoreValueCaret(state: EmojiInputState, element: HTMLElement, position: number) {
  if (state.pendingCursor.current !== null) element.focus();
  restoreCursor(element, position);
  rememberCaret(state, position);
  state.pendingCursor.current = null;
}

export function syncEmojiValue(state: EmojiInputState, value: string) {
  const element = state.element.current;
  if (!element) return;
  const position = state.pendingCursor.current;
  if (value === state.value.current && position === null) return;
  state.value.current = value;
  const focused = element === document.activeElement || position !== null;
  element.innerHTML = toHtml(value);
  if (focused) restoreValueCaret(state, element, position ?? value.length);
}

export function initializeEmojiValue(state: EmojiInputState, value: string) {
  const element = state.element.current;
  if (!element) return;
  element.innerHTML = toHtml(value);
  state.value.current = value;
  rememberCaret(state, value.length);
}

function limitEditableDOM(state: EmojiInputState, element: HTMLElement, value: string) {
  const position = saveSelection(element)?.start ?? value.length;
  element.innerHTML = toHtml(value);
  restoreCursor(element, position);
  rememberCaret(state, Math.min(position, value.length));
}

export function readEmojiInput(state: EmojiInputState, options: InputOptions) {
  const element = state.element.current;
  if (!element) return;
  const text = getText(element).replace(/\u00a0/g, " ").replace(/\n{3,}/g, "\n\n");
  const value = truncateText(text, options.maxLength);
  state.value.current = value;
  if (value !== text) limitEditableDOM(state, element, value);
  else syncEmojiSelection(state);
  options.onChange?.({ target: { value } });
}
