import type { KeySummary } from "@/lib/types";

export interface AlignedKeyRow {
  key: KeySummary;
  informational: KeySummary | null;
  real: KeySummary | null;
}

export function alignKeysBySubscriptionOrder(keys: KeySummary[]): AlignedKeyRow[] {
  return keys.map((key) => ({
    key,
    informational: key.kind === "informational" ? key : null,
    real: key.kind === "informational" ? null : key,
  }));
}

export function keyCardClientDisplayName(key: KeySummary): string {
  return key.client_display_name.trim() || key.label;
}
