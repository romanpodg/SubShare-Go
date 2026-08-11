"use client";

import { createContext, useContext, useState, useCallback, ReactNode } from "react";
import { Check, CircleAlert, Info, TriangleAlert } from "lucide-react";
import { EmojiText } from "@/components/ui/EmojiText";

type ToastType = "success" | "error" | "warning" | "info";

interface Toast {
  id: number;
  message: string;
  type: ToastType;
  exiting?: boolean;
}

interface ToastContextType {
  toast: (message: string, type?: ToastType) => void;
}

const ToastContext = createContext<ToastContextType>({ toast: () => {} });

export function useToast() {
  return useContext(ToastContext);
}

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const toast = useCallback((message: string, type: ToastType = "info") => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts((prev) =>
        prev.map((t) => (t.id === id ? { ...t, exiting: true } : t))
      );
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
      }, 300);
    }, 4000);
  }, []);

  const presentation = {
    success: {
      Icon: Check,
      border: "border-success/35",
      marker: "border-success/25 bg-success/8 text-success",
    },
    error: {
      Icon: CircleAlert,
      border: "border-danger/35",
      marker: "border-danger/25 bg-danger/8 text-danger",
    },
    warning: {
      Icon: TriangleAlert,
      border: "border-warning/35",
      marker: "border-warning/25 bg-warning/8 text-warning",
    },
    info: {
      Icon: Info,
      border: "border-info/35",
      marker: "border-info/25 bg-info/8 text-info",
    },
  };

  return (
    <ToastContext value={{ toast }}>
      {children}
      <div
        className="fixed bottom-[max(1rem,env(safe-area-inset-bottom))] right-[max(1rem,env(safe-area-inset-right))] z-50 flex max-w-[calc(100vw-2rem)] flex-col items-end gap-2"
        role="region"
        aria-label="Уведомления"
      >
        {toasts.map((t) => {
          const { Icon, border, marker } = presentation[t.type];

          return (
            <div
              key={t.id}
              role={t.type === "error" ? "alert" : "status"}
              aria-atomic="true"
              data-toast-type={t.type}
              className={`flex max-w-sm items-start gap-2.5 rounded-sm border bg-surface-1 px-3 py-2.5 text-sm leading-5 text-zinc-100 shadow-lg ${
                t.exiting ? "animate-fade-out" : "animate-fade-in"
              } ${border}`}
            >
              <span
                className={`mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-sm border ${marker}`}
                aria-hidden="true"
              >
                <Icon className="size-3" strokeWidth={2} />
              </span>
              <EmojiText text={t.message} className="min-w-0 break-words text-pretty" />
            </div>
          );
        })}
      </div>
    </ToastContext>
  );
}
