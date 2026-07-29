import { ReactNode } from "react";

export function StatCard({
  label,
  value,
  icon,
  tone = "cyan",
  hint,
}: {
  label: string;
  value: number | string;
  icon: ReactNode;
  tone?: "cyan" | "emerald" | "amber" | "rose" | "zinc";
  hint?: string;
}) {
  const tones = {
    cyan: "border-cyan-400/20 bg-cyan-400/10 text-cyan-300",
    emerald: "border-emerald-400/20 bg-emerald-400/10 text-emerald-300",
    amber: "border-amber-400/20 bg-amber-400/10 text-amber-300",
    rose: "border-rose-400/20 bg-rose-400/10 text-rose-300",
    zinc: "border-zinc-700 bg-zinc-800/50 text-zinc-400",
  };
  return (
    <div className="technical-frame min-h-36 border border-border bg-surface-1 p-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="font-mono text-[9px] font-semibold uppercase tracking-[0.14em] text-zinc-600">{label}</div>
          <div className="mt-4 font-display text-3xl font-medium tabular-nums tracking-[-0.04em] text-zinc-100">{value}</div>
          {hint && <div className="mt-2 font-mono text-[10px] text-zinc-600">{hint}</div>}
        </div>
        <div className={`flex h-9 w-9 items-center justify-center rounded-sm border ${tones[tone]}`}>
          {icon}
        </div>
      </div>
    </div>
  );
}
