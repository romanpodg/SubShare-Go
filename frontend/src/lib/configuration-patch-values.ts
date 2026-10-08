import type { XrayJSONDraft, XrayJSONPatch } from "./configuration";

export function buildALPNValues(raw: string) {
  return raw
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

export function optionalText(value: unknown) {
  const normalized = typeof value === "string" ? value.trim() : "";
  return normalized || undefined;
}

export function hasPatchField<Key extends keyof XrayJSONDraft>(
  patch: XrayJSONPatch,
  key: Key
) {
  return Object.prototype.hasOwnProperty.call(patch, key);
}
