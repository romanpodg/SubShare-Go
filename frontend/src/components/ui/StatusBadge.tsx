import { EmojiText } from "./EmojiText";

interface StatusBadgeProps {
  status: string;
  className?: string;
}

const statusConfig: Record<string, { label: string; className: string }> = {
  active: { label: "Активен", className: "bg-green-500/10 text-green-400" },
  paused: { label: "Приостановлен", className: "bg-yellow-500/10 text-yellow-400" },
  blocked: { label: "Заблокирован", className: "bg-red-500/10 text-red-400" },
  "non-active": { label: "Неактивен", className: "bg-zinc-500/10 text-zinc-400" },
};

export function StatusBadge({ status, className = "" }: StatusBadgeProps) {
  const config = statusConfig[status] || { label: status, className: "bg-zinc-500/10 text-zinc-400" };
  return (
    <span
      role="status"
      className={`text-xs px-2 py-0.5 rounded-full ${config.className} ${className}`}
    >
      <EmojiText text={config.label} />
    </span>
  );
}
