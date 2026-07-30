"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronDown, Check } from "lucide-react";

interface SelectOption {
  value: string;
  label: string;
}

interface SelectProps {
  label?: string;
  ariaLabel?: string;
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
  ariaLabel,
  error,
  options,
  value,
  onChange,
  className = "w-full",
  id,
  name,
  disabled = false,
}: SelectProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [menuStyle, setMenuStyle] = useState<{
    bottom?: number;
    left: number;
    maxHeight: number;
    top?: number;
    width: number;
  } | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLUListElement>(null);
  const selectId = id || label?.toLowerCase().replace(/\s+/g, "-");

  // Find currently selected option
  const selectedOption = options.find((opt) => String(opt.value) === String(value));

  const updateMenuPosition = useCallback(() => {
    const button = buttonRef.current;
    if (!button) return;

    const rect = button.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const gap = 4;
    const edgePadding = 8;
    const width = Math.min(rect.width, viewportWidth - edgePadding * 2);
    const left = Math.min(Math.max(rect.left, edgePadding), viewportWidth - width - edgePadding);
    const spaceBelow = viewportHeight - rect.bottom - edgePadding;
    const spaceAbove = rect.top - edgePadding;
    const openAbove = spaceBelow < 160 && spaceAbove > spaceBelow;
    const maxHeight = Math.max(72, Math.min(240, openAbove ? spaceAbove - gap : spaceBelow - gap));

    setMenuStyle(
      openAbove
        ? { bottom: viewportHeight - rect.top + gap, left, maxHeight, width }
        : { top: rect.bottom + gap, left, maxHeight, width }
    );
  }, []);

  // Close dropdown on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      const isInTrigger = containerRef.current?.contains(target) ?? false;
      const isInMenu = menuRef.current?.contains(target) ?? false;
      if (!isInTrigger && !isInMenu) {
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

  useEffect(() => {
    if (!isOpen) return;

    const frame = window.requestAnimationFrame(updateMenuPosition);
    window.addEventListener("resize", updateMenuPosition);
    window.addEventListener("scroll", updateMenuPosition, true);

    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("resize", updateMenuPosition);
      window.removeEventListener("scroll", updateMenuPosition, true);
    };
  }, [isOpen, updateMenuPosition]);

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
    <div className={`relative flex flex-col gap-2 ${className}`} ref={containerRef}>
      {label && (
        <label htmlFor={selectId} className="ui-field-label-wrap select-none">
          {label}
        </label>
      )}
      
      <div className="relative">
        <button
          ref={buttonRef}
          id={selectId}
          type="button"
          disabled={disabled}
          onClick={() => {
            if (disabled) return;
            if (!isOpen) updateMenuPosition();
            setIsOpen((current) => !current);
          }}
          className={`ui-select-trigger flex h-11 min-h-11 w-full cursor-pointer items-center justify-between rounded-sm border border-border bg-surface-2 px-3 py-2.5 text-left text-sm text-zinc-100 transition-colors duration-200 hover:border-[var(--border-strong)] focus-visible:border-accent focus-visible:outline-none ${
            disabled ? "opacity-50 cursor-not-allowed" : ""
          } ${error ? "ring-1 ring-red-500 border-red-500" : ""}`}
          aria-haspopup="listbox"
          aria-expanded={isOpen}
          aria-label={ariaLabel}
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

        {isOpen && menuStyle && typeof document !== "undefined" &&
          createPortal(
            <ul
              ref={menuRef}
              className="ui-select-menu fixed overflow-y-auto rounded-sm border border-[var(--border-strong)] bg-surface-1 py-1 animate-fade-in focus:outline-none"
              role="listbox"
              style={{
                bottom: menuStyle.bottom,
                left: menuStyle.left,
                maxHeight: menuStyle.maxHeight,
                top: menuStyle.top,
                width: menuStyle.width,
                zIndex: 1000,
              }}
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
                      className={`ui-select-option flex cursor-pointer select-none items-center justify-between gap-3 px-3 py-2 text-sm transition-colors duration-150 ${
                        isSelected
                          ? "bg-accent/8 font-medium text-accent shadow-[inset_2px_0_0_var(--accent)]"
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
            </ul>,
            document.body
          )}
      </div>
      
      {error && <p className="text-xs text-red-400 mt-1">{error}</p>}
    </div>
  );
}
