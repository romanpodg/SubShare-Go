"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, Check } from "lucide-react";

interface SelectOption {
  value: string;
  label: string;
}

interface SelectProps {
  label?: string;
  error?: string;
  options: SelectOption[];
  value?: string;
  onChange?: (event: { target: { name?: string; value: string } }) => void;
  className?: string;
  id?: string;
  name?: string;
  disabled?: boolean;
}

export function Select({
  label,
  error,
  options,
  value,
  onChange,
  className = "",
  id,
  name,
  disabled = false,
}: SelectProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const selectId = id || label?.toLowerCase().replace(/\s+/g, "-");

  // Find currently selected option
  const selectedOption = options.find((opt) => String(opt.value) === String(value));

  // Close dropdown on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isOpen]);

  // Handle option select
  const handleSelect = (option: SelectOption) => {
    if (disabled) return;
    setIsOpen(false);
    if (onChange) {
      onChange({
        target: {
          name,
          value: option.value,
        },
      });
    }
  };

  // Close dropdown on Escape key
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    }
    if (isOpen) {
      document.addEventListener("keydown", handleKeyDown);
    }
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  return (
    <div className={`relative flex w-full flex-col gap-2 ${className}`} ref={containerRef}>
      {label && (
        <label htmlFor={selectId} className="select-none font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
          {label}
        </label>
      )}
      
      <div className="relative">
        <button
          id={selectId}
          type="button"
          disabled={disabled}
          onClick={() => !disabled && setIsOpen(!isOpen)}
          className={`flex min-h-11 w-full cursor-pointer items-center justify-between rounded-sm border border-border bg-surface-2 px-3 py-2.5 text-left text-sm text-zinc-100 transition-colors duration-200 hover:border-[var(--border-strong)] focus-visible:border-accent focus-visible:outline-none ${
            disabled ? "opacity-50 cursor-not-allowed" : ""
          } ${error ? "ring-1 ring-red-500 border-red-500" : ""}`}
          aria-haspopup="listbox"
          aria-expanded={isOpen}
        >
          <span className="truncate">
            {selectedOption ? selectedOption.label : "Выберите значение..."}
          </span>
          <ChevronDown
            className={`w-4 h-4 text-zinc-400 transition-transform duration-200 flex-shrink-0 ${
              isOpen ? "rotate-180" : ""
            }`}
          />
        </button>

        {isOpen && (
          <ul
            className="absolute left-0 right-0 z-50 mt-1 max-h-60 overflow-y-auto rounded-sm border border-border bg-surface-1 py-1 animate-fade-in focus:outline-none"
            role="listbox"
          >
            {options.length === 0 ? (
              <li className="px-3 py-2 text-sm text-zinc-500 select-none">
                Нет доступных вариантов
              </li>
            ) : (
              options.map((opt) => {
                const isSelected = String(opt.value) === String(value);
                return (
                  <li
                    key={opt.value}
                    role="option"
                    aria-selected={isSelected}
                    onClick={() => handleSelect(opt)}
                    className={`px-3 py-2 text-sm cursor-pointer transition-colors duration-150 flex items-center justify-between select-none ${
                      isSelected
                        ? "bg-accent/8 font-medium text-accent"
                        : "text-zinc-300 hover:bg-surface-2 hover:text-white"
                    }`}
                  >
                    <span className="truncate">{opt.label}</span>
                    {isSelected && (
                      <Check className="w-4 h-4 text-zinc-300 flex-shrink-0" />
                    )}
                  </li>
                );
              })
            )}
          </ul>
        )}
      </div>
      
      {error && <p className="text-xs text-red-400 mt-1">{error}</p>}
    </div>
  );
}
