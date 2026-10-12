import type { XrayJSONDraft } from "./configuration-types";
import { parseHeaderType, splitCommaValues } from "./configuration-values";

function httpHeaderRequest(draft: XrayJSONDraft) {
  const request: Record<string, unknown> = {};
  const paths = splitCommaValues(draft.path);
  if (paths.length > 0) request.path = paths;
  const hosts = splitCommaValues(draft.host);
  if (hosts.length > 0) request.headers = { Host: hosts };
  return request;
}

function tcpSettings(draft: XrayJSONDraft) {
  const type = parseHeaderType(draft.headerType);
  if (!type) return { tcpSettings: {} };
  const header: Record<string, unknown> = { type };
  if (type === "http") {
    const request = httpHeaderRequest(draft);
    if (Object.keys(request).length > 0) header.request = request;
  }
  // The two aliases share their header within one build, as before; each build
  // constructs its own header, lists and request objects.
  return { tcpSettings: { header }, rawSettings: { header } };
}

export function buildDefaultTransport(draft: XrayJSONDraft): Record<string, unknown> {
  switch (draft.network) {
    case "ws": return { wsSettings: { path: draft.path.trim(), headers: draft.host.trim() ? { Host: draft.host.trim() } : {} } };
    case "grpc": return { grpcSettings: { serviceName: draft.path.trim(), ...(draft.grpcAuthority?.trim() ? { authority: draft.grpcAuthority.trim() } : {}) } };
    case "httpupgrade": return { httpupgradeSettings: { path: draft.path.trim(), host: draft.host.trim() } };
    case "xhttp": return { xhttpSettings: { path: draft.path.trim(), host: draft.host.trim() } };
    case "tcp": return tcpSettings(draft);
    default: return {};
  }
}
