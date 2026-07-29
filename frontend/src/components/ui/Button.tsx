"use client";

import { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "ghost" | "danger" | "outline";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
}

const variants: Record<Variant, string> = {
  primary: "border-accent bg-accent text-accent-fg hover:border-accent-hover hover:bg-accent-hover",
  ghost: "border-transparent bg-transparent text-zinc-400 hover:border-border hover:bg-surface-2 hover:text-zinc-100",
  danger: "border-danger/30 bg-danger/8 text-danger hover:bg-danger/14",
  outline: "border-border bg-transparent text-zinc-200 hover:border-[var(--border-strong)] hover:bg-surface-2",
};

export function Button({
  variant = "primary",
  loading,
  className = "",
  children,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`relative inline-flex min-h-11 items-center justify-center gap-2 rounded-sm border px-4 py-2 font-mono text-xs font-semibold uppercase tracking-[0.08em] transition-[background-color,border-color,color] duration-200 disabled:cursor-not-allowed disabled:opacity-45 ${variants[variant]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading && (
        <span className="absolute inset-0 flex items-center justify-center">
          <svg
            className="animate-spin h-4 w-4"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
        </span>
      )}
      <span className={`inline-flex items-center justify-center gap-2 ${loading ? "opacity-0" : ""}`}>{children}</span>
    </button>
  );
}
