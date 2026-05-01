"use client";

import { useEffect, useRef, useState } from "react";
import appleData from "@emoji-mart/data/sets/15/apple.json";
import ruI18n from "@emoji-mart/data/i18n/ru.json";

interface Props {
  onSelect: (emoji: string) => void;
  className?: string;
  inline?: boolean;
  iconOnly?: boolean;
}

export function EmojiPickerButton({ onSelect, className = "", inline = false, iconOnly = false }: Props) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const pickerContainerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open || inline) {
      return;
    }

    const handleOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };

    document.addEventListener("mousedown", handleOutsideClick);
    document.addEventListener("keydown", handleEscape);

    return () => {
      document.removeEventListener("mousedown", handleOutsideClick);
      document.removeEventListener("keydown", handleEscape);
    };
  }, [open, inline]);

  useEffect(() => {
    const container = pickerContainerRef.current;
    if (!open || !container) {
      return;
    }

    let destroyed = false;

    const mountPicker = async () => {
      const { Picker } = await import("emoji-mart");

      if (destroyed) {
        return;
      }

      const picker = new Picker({
        data: appleData,
        i18n: ruI18n,
        set: "apple",
        theme: "dark",
        locale: "ru",
        spritesheet: false,
        getImageURL: (set: string, unified: string) =>
          `https://cdn.jsdelivr.net/npm/emoji-datasource-${set}@15.0.1/img/${set}/64/${unified}.png`,
        previewPosition: "none",
        skinTonePosition: "none",
        maxFrequentRows: 1,
        perLine: 11,
        emojiSize: 28,
        emojiButtonSize: 32,
        dynamicWidth: true,
        onEmojiSelect: (emoji: { native?: string }) => {
          const native = typeof emoji?.native === "string" ? emoji.native : "";
          if (native) {
            onSelect(native);
          }
          if (!inline) {
            setOpen(false);
          }
        },
      });

      container.replaceChildren();
      container.appendChild(picker as unknown as Node);
    };

    mountPicker();

    return () => {
      destroyed = true;
      container.replaceChildren();
    };
  }, [open, onSelect, inline]);

  return (
    <div ref={rootRef} className={`relative inline-block ${className}`}>
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className={
          iconOnly
            ? `flex h-8 w-8 items-center justify-center rounded-md border border-border bg-surface-2 text-zinc-100 transition-colors hover:bg-surface-1 ${open ? "bg-surface-1" : ""}`
            : "h-9 rounded-lg border border-border bg-surface-2 px-3 text-sm text-zinc-200 transition-colors hover:bg-surface-1"
        }
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={open ? "Скрыть эмодзи" : "Показать эмодзи"}
      >
        {iconOnly ? (
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="h-4 w-4"
            aria-hidden="true"
          >
            <circle cx="12" cy="12" r="9" />
            <path d="M8.5 10h.01" />
            <path d="M15.5 10h.01" />
            <path d="M8.5 14.5c.9 1.2 2.2 1.8 3.5 1.8s2.6-.6 3.5-1.8" />
          </svg>
        ) : open ? "Скрыть" : "Эмодзи"}
      </button>
      {open && (
        <div
          className={inline
            ? "mt-3 h-[280px] overflow-hidden rounded-xl border border-border bg-surface-2"
            : "absolute right-0 top-full z-50 mt-2 overflow-hidden rounded-xl border border-border bg-surface-2 shadow-lg"
          }
        >
          <div ref={pickerContainerRef} className="w-full h-full" />
        </div>
      )}
    </div>
  );
}
