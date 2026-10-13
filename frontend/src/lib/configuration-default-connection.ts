import type { XrayJSONDraft } from "./configuration-types";
import { toPortNumber, DEFAULT_PORT } from "./configuration-values";

function vmessUser(draft: XrayJSONDraft) {
  const user: Record<string, unknown> = { id: draft.identifier.trim(), security: draft.vmessSecurity?.trim() || "auto" };
  const alterId = draft.vmessAlterId?.trim();
  if (alterId) {
    const parsed = Number.parseInt(alterId, 10);
    user.alterId = Number.isFinite(parsed) && parsed >= 0 ? parsed : alterId;
  }
  return user;
}

function vlessUser(draft: XrayJSONDraft) {
  const user: Record<string, unknown> = { id: draft.identifier.trim(), encryption: draft.encryption?.trim() || "none" };
  if (draft.flow?.trim()) user.flow = draft.flow.trim();
  return user;
}

export function buildDefaultConnection(draft: XrayJSONDraft, port = toPortNumber(draft.port || DEFAULT_PORT)) {
  const address = draft.server.trim();
  if (draft.protocol === "trojan") {
    return {
      servers: [{
        address,
        port,
        password: draft.identifier.trim()
      }]
    };
  }
  const user = draft.protocol === "vmess" ? vmessUser(draft) : vlessUser(draft);
  return {
    vnext: [{
      address,
      port,
      users: [user]
    }]
  };
}
