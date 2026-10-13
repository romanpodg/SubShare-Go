import type { ConfigurationProtocol, XrayJSONNetwork, XrayJSONSecurity } from "./configuration-types";
import { DEFAULT_NETWORK, DEFAULT_SECURITY, SUPPORTED_PROTOCOLS } from "./configuration-values";

export function normalizeProtocol(value: string): ConfigurationProtocol {
  const parsed = parseSupportedProtocol(value);
  if (parsed) {
    return parsed;
  }
  return "vless";
}

export function parseSupportedProtocol(value: string): ConfigurationProtocol | null {
  const normalized = value.toLowerCase().trim() as ConfigurationProtocol;
  if (SUPPORTED_PROTOCOLS.includes(normalized)) {
    return normalized;
  }
  return null;
}

export function normalizeNetwork(value: string): XrayJSONNetwork {
  const normalized = value.toLowerCase().trim() as XrayJSONNetwork;
  return ["tcp", "ws", "grpc", "httpupgrade", "xhttp", "h2", "quic"].includes(normalized) ? normalized : DEFAULT_NETWORK;
}

export function normalizeSecurity(value: string): XrayJSONSecurity {
  const normalized = value.toLowerCase().trim() as XrayJSONSecurity;
  return ["none", "tls", "reality"].includes(normalized) ? normalized : DEFAULT_SECURITY;
}
