import type { KeySummary } from "@/lib/types";

function formatHealthCheckTime(value: string): string {
  const isoMatch = value.match(/^(\d{4}-\d{2}-\d{2})(?:[T\s](\d{2}:\d{2})(?::\d{2})?)?/);
  if (isoMatch) {
    const parsed = new Date(value);
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
        .toLocaleString("ru-RU", {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
          hour: isoMatch[2] ? "2-digit" : undefined,
          minute: isoMatch[2] ? "2-digit" : undefined,
        })
        .replace(",", "");
    }
  }
  return value;
}

export function KeyHealthBadge({ healthKey }: { healthKey: KeySummary }) {
  if (!healthKey.last_checked_at) {
    return null;
  }

  const healthy = healthKey.check_status === "up";
  const unavailable = healthKey.check_status === "down";
  const unsupported = healthKey.check_status === "unsupported_check";
  const label = healthy
    ? `ДОСТУПЕН${healthKey.last_latency_ms > 0 ? ` · ${healthKey.last_latency_ms} ms` : ""}`
    : unavailable
      ? "НЕДОСТУПЕН"
      : unsupported
        ? "ПРОВЕРКА НЕ ПОДДЕРЖИВАЕТСЯ"
        : "ОШИБКА ПРОВЕРКИ";
  const checkedAt = formatHealthCheckTime(healthKey.last_checked_at);

  return (
    <span
      className={`mb-2 inline-flex w-fit items-center rounded-sm border px-2 py-0.5 text-[11px] font-semibold tracking-[0.04em] ${
        healthy
          ? "border-emerald-500/35 bg-emerald-500/10 text-emerald-300"
          : unsupported
            ? "border-slate-500/35 bg-slate-500/10 text-slate-300"
          : "border-red-500/35 bg-red-500/10 text-red-300"
      }`}
      title={`Последняя проверка: ${checkedAt}`}
      data-health-state={healthKey.check_status}
    >
      {label}
    </span>
  );
}
