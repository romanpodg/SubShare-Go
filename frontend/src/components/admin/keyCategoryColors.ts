"use client";

export const DEFAULT_KEY_CATEGORY_COLOR = "#d8b33d";

export const KEY_CATEGORY_COLOR_PRESETS = [
  "#d8b33d",
  "#ef4444",
  "#f97316",
  "#22c55e",
  "#06b6d4",
  "#3b82f6",
  "#8b5cf6",
  "#ec4899",
  "#94a3b8",
];

export function normalizeKeyCategoryColor(value?: string | null) {
  const trimmed = (value || "").trim();
  if (/^#[0-9a-fA-F]{6}$/.test(trimmed)) {
    return trimmed.toUpperCase();
  }
  return DEFAULT_KEY_CATEGORY_COLOR;
}

export function alphaHexColor(hex: string, alphaHex: string) {
  return `${normalizeKeyCategoryColor(hex)}${alphaHex}`;
}
