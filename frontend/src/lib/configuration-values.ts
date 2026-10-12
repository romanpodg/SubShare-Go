import type {
  ConfigurationProtocol,
  XrayJSONNetwork,
  XrayJSONSecurity
} from "./configuration-types";

export const DEFAULT_PORT = "443";
export const DEFAULT_NETWORK: XrayJSONNetwork = "tcp";
export const DEFAULT_SECURITY: XrayJSONSecurity = "none";
export const SUPPORTED_PROTOCOLS: ConfigurationProtocol[] = [
  "vless",
  "vmess",
  "trojan"
];

export function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

export function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

export function textValue(value: unknown): string {
  if (typeof value === "string") {
    return value.trim();
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return "";
}

export function portValue(value: unknown): string {
  const text = textValue(value);
  if (!text) {
    return DEFAULT_PORT;
  }
  const numeric = Number.parseInt(text, 10);
  if (!Number.isFinite(numeric) || numeric <= 0 || numeric > 65535) {
    return DEFAULT_PORT;
  }
  return String(numeric);
}

export function boolValue(value: unknown): boolean {
  if (typeof value === "boolean") return value;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "1" || normalized === "true" || normalized === "yes" || normalized === "on";
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  return false;
}

export function splitCommaValues(raw: string) {
  return raw
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function parseHeaderType(value: unknown) {
  const normalized = textValue(value).toLowerCase();
  if (normalized === "none" || normalized === "http") {
    return normalized;
  }
  return "";
}

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
  switch (normalized) {
    case "tcp":
    case "ws":
    case "grpc":
    case "httpupgrade":
    case "xhttp":
    case "h2":
    case "quic":
      return normalized;
    default:
      return DEFAULT_NETWORK;
  }
}

export function normalizeSecurity(value: string): XrayJSONSecurity {
  const normalized = value.toLowerCase().trim() as XrayJSONSecurity;
  switch (normalized) {
    case "none":
    case "tls":
    case "reality":
      return normalized;
    default:
      return DEFAULT_SECURITY;
  }
}

export function toPortNumber(rawPort: string) {
  const parsedPort = Number.parseInt(rawPort.trim(), 10);
  if (!Number.isFinite(parsedPort) || parsedPort <= 0 || parsedPort > 65535) {
    return Number.parseInt(DEFAULT_PORT, 10);
  }
  return parsedPort;
}
