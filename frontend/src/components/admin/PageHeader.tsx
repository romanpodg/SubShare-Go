import { ReactNode } from "react";

export function PageHeader({
  title,
  description,
  icon,
  actions,
}: {
  title: string;
  description?: string;
  icon?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="technical-frame mb-0 flex flex-col justify-between gap-5 border border-border bg-surface-1 p-5 sm:flex-row sm:items-center lg:p-6">
      <div className="flex items-center gap-4">
        {icon && (
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-sm border border-accent/20 bg-accent/5 text-accent">
            {icon}
          </div>
        )}
        <div>
          <div className="mb-1 font-mono text-[9px] font-semibold uppercase tracking-[0.16em] text-accent/70">CONTROL MODULE</div>
          <h1 className="font-display text-2xl font-medium tracking-[-0.035em] text-zinc-100">{title}</h1>
          {description && <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-500">{description}</p>}
        </div>
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}
