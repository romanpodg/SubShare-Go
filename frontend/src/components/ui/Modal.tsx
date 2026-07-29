"use client";

import { ReactNode, useEffect, useRef, useCallback } from "react";
import { EmojiText } from "./EmojiText";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  className?: string;
}

export function Modal({ open, onClose, title, children, className = "" }: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null);
  const modalRef = useRef<HTMLDivElement>(null);
  const previousFocusRef = useRef<Element | null>(null);
  const sizeClasses = className ? className : "max-w-lg max-h-[88vh] overflow-y-auto";

  const getFocusableElements = useCallback(() => {
    if (!modalRef.current) return [];
    return Array.from(
      modalRef.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )
    );
  }, []);

  useEffect(() => {
    if (!open) return;

    previousFocusRef.current = document.activeElement;

    document.body.classList.add("overflow-hidden");

    const timer = setTimeout(() => {
      const focusable = getFocusableElements();
      if (focusable.length > 0) {
        focusable[0].focus();
      }
    }, 0);

    return () => {
      clearTimeout(timer);
      document.body.classList.remove("overflow-hidden");

      if (previousFocusRef.current instanceof HTMLElement) {
        previousFocusRef.current.focus();
      }
    };
  }, [open, getFocusableElements]);

  useEffect(() => {
    if (!open) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Tab") {
        const focusable = getFocusableElements();
        if (focusable.length === 0) {
          e.preventDefault();
          return;
        }

        const first = focusable[0];
        const last = focusable[focusable.length - 1];

        if (e.shiftKey) {
          if (document.activeElement === first) {
            e.preventDefault();
            last.focus();
          }
        } else {
          if (document.activeElement === last) {
            e.preventDefault();
            first.focus();
          }
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose, getFocusableElements]);

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 p-3"
    >
      <div
        ref={modalRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`technical-frame w-full border border-border bg-surface-1 p-5 animate-fade-in ${sizeClasses}`}
      >
        <div className="flex items-center justify-between mb-3">
          <div>
            <div className="system-label mb-1">DIALOG / ACTIVE</div>
            <h2 className="font-display text-xl font-medium tracking-tight text-zinc-100"><EmojiText text={title} /></h2>
          </div>
          <button
            onClick={onClose}
            aria-label="Закрыть"
            className="flex h-11 w-11 items-center justify-center rounded-sm border border-border text-xl leading-none text-zinc-500 transition-colors hover:border-[var(--border-strong)] hover:bg-surface-2 hover:text-zinc-100"
          >
            &times;
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
