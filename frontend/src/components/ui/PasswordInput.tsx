"use client";

import { useState, InputHTMLAttributes } from "react";
import { Eye, EyeOff } from "lucide-react";

interface PasswordInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: string;
  error?: string;
}

export function PasswordInput({ label, error, className = "", id, ...props }: PasswordInputProps) {
  const [showPassword, setShowPassword] = useState(false);
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");

  return (
    <div className="flex w-full min-w-0 flex-col gap-2">
      {label && (
        <label htmlFor={inputId} className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
          {label}
        </label>
      )}
      <div className="relative">
        <input
          id={inputId}
          type={showPassword ? "text" : "password"}
          className={`ui-text-input min-h-11 w-full rounded-sm border border-border bg-surface-2 px-3 py-2 pr-12 text-sm text-zinc-100 placeholder:text-zinc-600 transition-colors hover:border-[var(--border-strong)] focus-visible:border-accent focus-visible:outline-none ${error ? "border-danger" : ""} ${className}`}
          {...props}
        />
        <button
          type="button"
          onClick={() => setShowPassword(!showPassword)}
          className="absolute right-0 top-0 flex h-11 w-11 items-center justify-center border-l border-border text-zinc-500 transition-colors hover:bg-surface-3 hover:text-zinc-100"
          aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}
        >
          {showPassword ? (
            <EyeOff className="w-5 h-5" />
          ) : (
            <Eye className="w-5 h-5" />
          )}
        </button>
      </div>
      {error && <p className="font-mono text-[10px] uppercase tracking-wide text-danger">{error}</p>}
    </div>
  );
}
