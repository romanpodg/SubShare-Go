"use client";

import { AlertTriangle, RefreshCw } from "lucide-react";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

export function InitialLoading({ label = "Загрузка…" }: { label?: string }) {
  return (
    <div
      className="flex min-h-[36vh] flex-col items-center justify-center gap-3 text-sm text-zinc-500"
      role="status"
      aria-live="polite"
    >
      <LoadingSpinner />
      <span>{label}</span>
    </div>
  );
}

export function ResourceError({
  error,
  onRetry,
  compact = false,
  title = "Не удалось загрузить данные",
}: {
  error: unknown;
  onRetry?: () => void;
  compact?: boolean;
  title?: string;
}) {
  const apiError = error instanceof ApiError ? error : null;
  const message = error instanceof Error ? error.message : "Неизвестная ошибка";

  return (
    <div
      className={`rounded-2xl border border-rose-500/25 bg-rose-500/5 ${
        compact ? "p-4" : "p-7"
      }`}
      role="alert"
      aria-live="assertive"
    >
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-rose-300" />
        <div className="min-w-0 flex-1">
          <div className="font-medium text-rose-200">{title}</div>
          <div className="mt-1 break-words text-sm text-rose-300/80">{message}</div>
          {apiError?.requestId && (
            <div className="mt-2 font-mono text-xs text-zinc-600">
              Request ID: {apiError.requestId}
            </div>
          )}
        </div>
        {onRetry && (
          <Button variant="outline" onClick={onRetry}>
            <RefreshCw className="h-4 w-4" />
            Повторить
          </Button>
        )}
      </div>
    </div>
  );
}
