"use client";

import { useEffect, useRef, useState } from "react";
import appleData from "@emoji-mart/data/sets/15/apple.json";
import ruI18n from "@emoji-mart/data/i18n/ru.json";

interface Props {
  onSelect: (emoji: string) => void;
  className?: string;
  inline?: boolean;
}

export function EmojiPickerButton({ onSelect, className = "", inline = false }: Props) {
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
    <div ref={rootRef} className={className}>
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="h-9 rounded-lg border border-border bg-surface-2 px-3 text-sm text-zinc-200 transition-colors hover:bg-surface-1"
        aria-haspopup="dialog"
        aria-expanded={open}
      >
        {open ? "Скрыть" : "Эмодзи"}
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
