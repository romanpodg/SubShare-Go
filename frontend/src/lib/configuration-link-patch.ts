import type { XrayJSONPatch } from "./configuration-types";
import {
  buildConfiguration,
  parseConfiguration,
  parseVmessExtraParams
} from "./configuration-link-codec";
import { hasPatchField } from "./configuration-patch-values";
import {
  vmessTextFields,
  vmessTrailingFields,
  uriTextFields,
  uriTrailingFields,
  type LinkFieldSpec
} from "./configuration-link-fields";

function patchSearchValue(search: URLSearchParams, aliases: readonly string[], value: string | undefined) {
  aliases.forEach(key => search.delete(key));
  if (value?.trim()) search.set(aliases[0], value.trim());
}

function patchObjectValue(object: Record<string, unknown>, aliases: readonly string[], value: string | boolean | undefined) {
  aliases.forEach(key => delete object[key]);
  if (typeof value === "boolean") {
    if (value) object[aliases[0]] = "1";
  } else if (value?.trim()) object[aliases[0]] = value.trim();
}

function patchObjectFields(extra: Record<string, unknown>, patch: XrayJSONPatch, fields: readonly LinkFieldSpec[]) {
  for (const [field, aliases] of fields) {
    if (hasPatchField(patch, field)) patchObjectValue(extra, aliases, patch[field]);
  }
}

function patchQueryFields(search: URLSearchParams, patch: XrayJSONPatch, fields: readonly LinkFieldSpec[]) {
  for (const [field, aliases] of fields) {
    if (hasPatchField(patch, field)) patchSearchValue(search, aliases, patch[field]);
  }
}

function patchVmessSecurity(extra: Record<string, unknown>, patch: XrayJSONPatch) {
  if (!hasPatchField(patch, "security")) return;
  delete extra.tls;
  delete extra.security;
  if (patch.security === "tls") extra.tls = "tls";
  else if (patch.security && patch.security !== "none") extra.security = patch.security;
}

function patchVmessParams(params: string, patch: XrayJSONPatch) {
  const extra = parseVmessExtraParams(params);
  if (hasPatchField(patch, "network")) patchObjectValue(extra, ["net"], patch.network === "tcp" ? "" : patch.network);
  patchVmessSecurity(extra, patch);
  patchObjectFields(extra, patch, vmessTextFields);
  if (hasPatchField(patch, "allowInsecure")) patchObjectValue(extra, ["allowInsecure", "allowinsecure"], patch.allowInsecure);
  patchObjectFields(extra, patch, vmessTrailingFields);
  return Object.keys(extra).length > 0 ? JSON.stringify(extra) : "";
}

function patchUriParams(params: string, patch: XrayJSONPatch) {
  const search = new URLSearchParams(params);
  if (hasPatchField(patch, "network")) patchSearchValue(search, ["type"], patch.network === "tcp" ? "" : patch.network);
  if (hasPatchField(patch, "security")) patchSearchValue(search, ["security"], patch.security === "none" ? "" : patch.security);
  patchQueryFields(search, patch, uriTextFields);
  if (hasPatchField(patch, "allowInsecure")) patchSearchValue(search, [
    "allowInsecure",
    "allowinsecure",
    "allow_insecure"
  ], patch.allowInsecure ? "1" : "");
  patchQueryFields(search, patch, uriTrailingFields);
  return search.toString();
}

// Patches only represented fields. Unrelated duplicate query entries and
// unrepresented VMess properties retain their original ordering and values.
export function patchEditableConfiguration(raw: string, patch: XrayJSONPatch) {
  const parsed = parseConfiguration(raw);
  if (!parsed) throw new Error("Не удалось разобрать конфигурацию");
  const next = { ...parsed };
  for (const field of [
    "server",
    "port",
    "identifier",
    "remark"
  ] as const) {
    if (hasPatchField(patch, field)) next[field] = patch[field] ?? "";
  }
  next.params = parsed.protocol === "vmess" ? patchVmessParams(parsed.params, patch) : patchUriParams(parsed.params, patch);
  return buildConfiguration(next);
}
