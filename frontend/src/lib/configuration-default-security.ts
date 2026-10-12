import type { XrayJSONDraft } from "./configuration-types";
import { buildALPNValues } from "./configuration-patch-values";

function tlsSettings(draft: XrayJSONDraft) {
  const settings: Record<string, unknown> = { serverName: draft.sni.trim() };
  const alpn = buildALPNValues(draft.alpn);
  if (alpn.length > 0) settings.alpn = alpn;
  if (draft.allowInsecure) settings.allowInsecure = true;
  if (draft.fingerprint?.trim()) settings.fingerprint = draft.fingerprint.trim();
  return settings;
}

function realitySettings(draft: XrayJSONDraft) {
  const settings: Record<string, unknown> = { serverName: draft.sni.trim(), show: false };
  if (draft.fingerprint?.trim()) settings.fingerprint = draft.fingerprint.trim();
  if (draft.publicKey?.trim()) {
    settings.publicKey = draft.publicKey.trim();
    settings.password = draft.publicKey.trim();
  }
  if (draft.shortId?.trim()) settings.shortId = draft.shortId.trim();
  if (draft.spiderX?.trim()) settings.spiderX = draft.spiderX.trim();
  return settings;
}

export function buildDefaultSecurity(draft: XrayJSONDraft): Record<string, unknown> {
  if (draft.security === "tls") return { tlsSettings: tlsSettings(draft) };
  if (draft.security === "reality") return { realitySettings: realitySettings(draft) };
  return {};
}
