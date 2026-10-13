import type { XrayJSONDraft } from "./configuration-types";
import {
  DEFAULT_PORT,
  DEFAULT_NETWORK,
  DEFAULT_SECURITY,
  toPortNumber
} from "./configuration-values";
import { normalizeProtocol } from "./configuration-discriminators";
import { buildDefaultConnection } from "./configuration-default-connection";
import { buildDefaultTransport } from "./configuration-default-transport";
import { buildDefaultSecurity } from "./configuration-default-security";
import { buildDefaultEnvelope } from "./configuration-default-envelope";

export function buildXrayJSONConfiguration(draft: XrayJSONDraft) {
  const protocol = normalizeProtocol(draft.protocol);
  const server = draft.server.trim();
  const identifier = draft.identifier.trim();
  if (!server) throw new Error("Укажите сервер");
  if (!identifier) throw new Error(protocol === "trojan" ? "Укажите пароль" : "Укажите UUID / ID");
  const normalized = { ...draft, protocol };
  const port = toPortNumber(draft.port || DEFAULT_PORT);
  const outbound = buildOutbound(normalized, port);
  return JSON.stringify(buildDefaultEnvelope(outbound), null, 2);
}

function buildOutbound(draft: XrayJSONDraft, port: number) {
  const protocol = draft.protocol;
  const normalized = draft;
  return {
    tag: draft.remark.trim() || "proxy",
    protocol,
    settings: buildDefaultConnection(normalized, port),
    streamSettings: {
      network: draft.network || DEFAULT_NETWORK,
      security: draft.security || DEFAULT_SECURITY,
      ...buildDefaultTransport(normalized),
      ...buildDefaultSecurity(normalized),
    },
  };
}
