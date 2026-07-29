"use client";

import { ReactNode, useEffect, useId, useRef } from "react";
import { X } from "lucide-react";

export function Drawer({
  open,
  title,
  description,
  onClose,
  children,
  footer,
}: {
  open: boolean;
  title: string;
  description?: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
}) {
  const titleId = useId();
  const descriptionId = useId();
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = previousOverflow;
      previous?.focus();
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex justify-end bg-black/85"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        className="technical-grid flex h-full w-full max-w-2xl flex-col border-l border-border bg-bg"
      >
        <header className="flex items-start justify-between gap-4 border-b border-border px-5 py-4 sm:px-6">
          <div>
            <div className="system-label mb-1">DETAIL PANEL / ACTIVE</div>
            <h2 id={titleId} className="font-display text-xl font-medium tracking-tight text-zinc-100">{title}</h2>
            {description && <p id={descriptionId} className="mt-1 text-sm text-zinc-500">{description}</p>}
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            className="flex h-11 w-11 items-center justify-center rounded-sm border border-border text-zinc-500 hover:border-[var(--border-strong)] hover:bg-surface-2 hover:text-zinc-100"
            aria-label="Закрыть"
          >
            <X className="h-5 w-5" />
          </button>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5 sm:px-6">{children}</div>
        {footer && <footer className="border-t border-border px-5 py-4 sm:px-6">{footer}</footer>}
      </section>
    </div>
  );
}
