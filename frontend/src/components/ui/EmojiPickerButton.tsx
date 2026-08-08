"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { EMOJI_PRESENTATION_SET } from "@/lib/emoji-assets";
import { calculateEmojiPopoverLayout, emojiPerLine, type EmojiPopoverLayout } from "@/lib/emoji-popover";

interface Props {
  onSelect: (emoji: string) => void;
  className?: string;
  inline?: boolean;
  iconOnly?: boolean;
}

type EmojiMartPicker = HTMLElement & { shadowRoot: ShadowRoot | null };
type PickerModule = Awaited<ReturnType<typeof importPickerModule>>;

let pickerModulePromise: Promise<PickerModule> | null = null;

async function importPickerModule() {
  const [{ Picker }, { default: data }, { default: i18n }] = await Promise.all([
    import("emoji-mart"),
    import("@emoji-mart/data/sets/15/native.json"),
    import("@emoji-mart/data/i18n/ru.json"),
  ]);
  return { Picker, data, i18n };
}

export function preloadEmojiPicker(): Promise<PickerModule> {
  if (!pickerModulePromise) {
    pickerModulePromise = importPickerModule().catch((error) => {
      pickerModulePromise = null;
      throw error;
    });
  }
  return pickerModulePromise;
}

function applyEmojiPickerShadowTheme(picker: EmojiMartPicker) {
  const shadowRoot = picker.shadowRoot;
  if (!shadowRoot || shadowRoot.querySelector("[data-subshare-emoji-theme]")) return;

  const style = document.createElement("style");
  style.dataset.subshareEmojiTheme = "true";
  style.textContent = `
    :host { width: 100%; height: 100%; min-height: 0; border-radius: 0; box-shadow: none; }
    #root { --padding: 8px; --sidebar-width: var(--scrollbar-size); border-radius: 0; }
    .search input[type="search"] { border-radius: var(--radius-sm); }
    .search input[type="search"]:focus { box-shadow: inset 0 0 0 1px var(--accent); }
    .sticky { background: var(--surface-2); backdrop-filter: none; font-family: var(--font-mono); font-size: 11px; letter-spacing: .06em; text-transform: uppercase; }
    .category > div:last-child:empty::after { display: block; padding: 16px 8px; color: var(--text-dim); content: "Эмодзи не найден"; font-family: var(--font-mono); font-size: 11px; }
    #nav .bar { height: 2px; border-radius: 0; }
    .category button .background, .menu, .option { border-radius: var(--radius-sm); }
    .scroll { scrollbar-color: var(--scrollbar-thumb) var(--scrollbar-track); scrollbar-width: thin; }
    .scroll::-webkit-scrollbar { width: var(--scrollbar-size); height: var(--scrollbar-size); }
    .scroll::-webkit-scrollbar-track { background: var(--scrollbar-track); }
    .scroll::-webkit-scrollbar-thumb { min-height: 32px; border: 2px solid var(--scrollbar-track); border-radius: 0; background: var(--scrollbar-thumb); background-clip: padding-box; }
    .scroll::-webkit-scrollbar-thumb:hover { background: var(--scrollbar-thumb-hover) !important; }
    .scroll::-webkit-scrollbar-thumb:active { background: var(--scrollbar-thumb-active) !important; }
  `;
  shadowRoot.appendChild(style);
}

export function EmojiPickerButton({ onSelect, className = "", inline = false, iconOnly = false }: Props) {
  const [open, setOpen] = useState(false);
  const [layout, setLayout] = useState<EmojiPopoverLayout | null>(null);
  const [loadState, setLoadState] = useState<"loading" | "ready" | "error">("loading");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const pickerContainerRef = useRef<HTMLDivElement | null>(null);
  const onSelectRef = useRef(onSelect);
  const popoverId = useId();
  const pickerWidth = layout?.width;

  useEffect(() => {
    onSelectRef.current = onSelect;
  }, [onSelect]);

  const restoreTriggerFocus = useCallback(() => {
    window.requestAnimationFrame(() => triggerRef.current?.focus());
  }, []);

  const close = useCallback((restoreFocus: boolean) => {
    setOpen(false);
    if (restoreFocus) restoreTriggerFocus();
  }, [restoreTriggerFocus]);

  const updatePosition = useCallback(() => {
    if (inline || !triggerRef.current) return;
    setLayout(calculateEmojiPopoverLayout(triggerRef.current.getBoundingClientRect(), {
      height: window.innerHeight,
      width: window.innerWidth,
    }));
  }, [inline]);

  useEffect(() => {
    const idleWindow = window as Window & {
      cancelIdleCallback?: (handle: number) => void;
      requestIdleCallback?: (callback: () => void, options?: { timeout: number }) => number;
    };
    const preload = () => void preloadEmojiPicker().catch(() => undefined);
    if (idleWindow.requestIdleCallback) {
      const handle = idleWindow.requestIdleCallback(preload, { timeout: 1500 });
      return () => idleWindow.cancelIdleCallback?.(handle);
    }
    const handle = window.setTimeout(preload, 600);
    return () => window.clearTimeout(handle);
  }, []);

  useEffect(() => {
    if (!open || inline) return;
    updatePosition();
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!triggerRef.current?.contains(target) && !popoverRef.current?.contains(target)) close(false);
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close(true);
      }
    };
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [close, inline, open, updatePosition]);

  useEffect(() => {
    const container = pickerContainerRef.current;
    if (!open || !container || (!inline && pickerWidth === undefined)) return;
    let cancelled = false;

    void preloadEmojiPicker().then(({ Picker, data, i18n }) => {
      if (cancelled) return;
      const width = inline ? (container.clientWidth || 352) : pickerWidth!;
      const picker = new Picker({
        data,
        i18n,
        set: EMOJI_PRESENTATION_SET,
        theme: "dark",
        locale: "ru",
        previewPosition: "none",
        skinTonePosition: "none",
        maxFrequentRows: 1,
        perLine: emojiPerLine(width),
        emojiSize: 26,
        emojiButtonSize: 34,
        dynamicWidth: false,
        onEmojiSelect: (emoji: { native?: string }) => {
          if (typeof emoji.native === "string" && emoji.native) onSelectRef.current(emoji.native);
          if (!inline) close(true);
        },
      }) as unknown as EmojiMartPicker;
      picker.classList.add("emoji-picker-host");
      picker.setAttribute("aria-label", "Выбор эмодзи");
      container.replaceChildren(picker);
      applyEmojiPickerShadowTheme(picker);
      setLoadState("ready");
      window.requestAnimationFrame(() => {
        const search = picker.shadowRoot?.querySelector<HTMLInputElement>('input[type="search"]');
        search?.focus();
      });
    }).catch(() => {
      if (!cancelled) setLoadState("error");
    });

    return () => {
      cancelled = true;
      container.replaceChildren();
    };
  }, [close, inline, loadAttempt, open, pickerWidth]);

  const pickerSurface = (
    <div
      ref={popoverRef}
      id={popoverId}
      role="dialog"
      aria-label="Выбор эмодзи"
      tabIndex={-1}
      data-placement={layout?.placement}
      className={inline ? "emoji-picker-surface emoji-picker-surface--inline mt-3" : "emoji-picker-surface emoji-picker-popover"}
      style={inline ? undefined : {
        height: layout?.height,
        left: layout?.left,
        top: layout?.top,
        visibility: layout ? "visible" : "hidden",
        width: layout?.width,
      }}
    >
      {loadState === "loading" && <div className="emoji-picker-state" role="status">Загрузка эмодзи…</div>}
      {loadState === "error" && (
        <div className="emoji-picker-state" role="alert">
          <span>Не удалось загрузить эмодзи.</span>
          <button type="button" className="ui-control px-3" onClick={() => {
            setLoadState("loading");
            setLoadAttempt((value) => value + 1);
          }}>Повторить</button>
        </div>
      )}
      <div ref={pickerContainerRef} className="emoji-picker-container" hidden={loadState === "error"} />
    </div>
  );

  return (
    <div className={`inline-block ${className}`}>
      <button
        ref={triggerRef}
        type="button"
        onPointerEnter={() => void preloadEmojiPicker().catch(() => undefined)}
        onFocus={() => void preloadEmojiPicker().catch(() => undefined)}
        onClick={() => {
          if (open) close(true);
          else {
            setLoadState("loading");
            setOpen(true);
          }
        }}
        className={iconOnly ? `ui-icon-button ${open ? "border-accent bg-surface-1" : ""}` : "ui-control px-3 text-sm text-zinc-200"}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? popoverId : undefined}
        aria-label={open ? "Скрыть эмодзи" : "Показать эмодзи"}
      >
        {iconOnly ? (
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4" aria-hidden="true">
            <circle cx="12" cy="12" r="9" /><path d="M8.5 10h.01" /><path d="M15.5 10h.01" /><path d="M8.5 14.5c.9 1.2 2.2 1.8 3.5 1.8s2.6-.6 3.5-1.8" />
          </svg>
        ) : open ? "Скрыть" : "Эмодзи"}
      </button>
      {open && (inline ? pickerSurface : createPortal(pickerSurface, document.body))}
    </div>
  );
}
