"use client";

import { InputHTMLAttributes } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export function Input({ label, error, className = "", id, ...props }: InputProps) {
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");
  return (
    <div className="flex flex-col gap-2">
      {label && (
        <label htmlFor={inputId} className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`min-h-11 rounded-sm border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-600 transition-colors hover:border-[var(--border-strong)] focus-visible:border-accent focus-visible:outline-none ${error ? "border-danger" : ""} ${className}`}
        {...props}
      />
      {error && <p className="font-mono text-[10px] uppercase tracking-wide text-danger">{error}</p>}
    </div>
  );
}
