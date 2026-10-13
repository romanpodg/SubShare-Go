import type { XrayJSONDraft } from "./configuration-types";
import { buildALPNValues } from "./configuration-patch-values";

function tlsSettings(draft: XrayJSONDraft) {
  const settings: Record<string, unknown> = { serverName: draft.sni.trim() };
  const alpn = buildALPNValues(draft.alpn);
  if (alpn.length > 0) settings.alpn = alpn;
  if (draft.allowInsecure) settings.allowInsecure = true;
  appendSecurityText(settings, draft, ["fingerprint"]);
  return settings;
}

function realitySettings(draft: XrayJSONDraft) {
  const settings: Record<string, unknown> = { serverName: draft.sni.trim(), show: false };
  appendSecurityText(settings, draft, ["fingerprint"]);
  if (draft.publicKey?.trim()) {
    settings.publicKey = draft.publicKey.trim();
    settings.password = draft.publicKey.trim();
  }
  appendSecurityText(settings, draft, ["shortId", "spiderX"]);
  return settings;
}

export function buildDefaultSecurity(draft: XrayJSONDraft): Record<string, unknown> {
  if (draft.security === "tls") return { tlsSettings: tlsSettings(draft) };
  if (draft.security === "reality") return { realitySettings: realitySettings(draft) };
  return {};
}

function appendSecurityText(settings: Record<string, unknown>, draft: XrayJSONDraft, fields: readonly ("fingerprint" | "shortId" | "spiderX")[]) {
  for (const field of fields) {
    const value = draft[field]?.trim();
    if (value) settings[field] = value;
  }
}
