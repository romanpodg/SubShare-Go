import { EmojiText } from "./EmojiText";

interface StatusBadgeProps {
  status: string;
  className?: string;
}

const statusConfig: Record<string, { label: string; className: string }> = {
  active: { label: "Активен", className: "border-accent/25 bg-accent/7 text-accent" },
  paused: { label: "Приостановлен", className: "border-warning/25 bg-warning/7 text-warning" },
  blocked: { label: "Заблокирован", className: "border-danger/25 bg-danger/7 text-danger" },
  expired: { label: "Истёк", className: "border-danger/25 bg-danger/7 text-danger" },
  limited: { label: "Лимит устройств", className: "border-blue-400/25 bg-blue-400/7 text-blue-300" },
  "non-active": { label: "Неактивен", className: "border-border bg-surface-2 text-zinc-500" },
};

export function StatusBadge({ status, className = "" }: StatusBadgeProps) {
  const config = statusConfig[status] || {
    label: status,
    className: "border-border bg-surface-2 text-zinc-500",
  };

  return (
    <span
      role="status"
      className={`inline-flex min-h-6 items-center gap-1.5 rounded-sm border px-2 font-mono text-[9px] font-semibold uppercase tracking-[0.12em] before:h-1 before:w-1 before:bg-current ${config.className} ${className}`}
    >
      <EmojiText text={config.label} />
    </span>
  );
}
