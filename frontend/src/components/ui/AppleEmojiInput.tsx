"use client";

import { forwardRef } from "react";
import { esc, getText } from "./emoji-input-dom";
import { textLength, truncateText } from "./emoji-input-text";
import type { AppleEmojiInputHandle, AppleEmojiInputProps } from "./emoji-input-types";
import { useEmojiInput } from "./use-emoji-input";
export type { AppleEmojiInputHandle } from "./emoji-input-types";

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
  const { ref, syncSelection, onInput } = useEmojiInput({ value, onChange, maxLength }, forwardedRef);
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");
  const labelId = label && inputId ? `${inputId}-label` : undefined;
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
            if (e.key !== "Enter") return;
            if (!multiline) {
              e.preventDefault();
              return;
            }
            if (!e.shiftKey) {
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
