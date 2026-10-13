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
  if (typeof value !== "object") {
    return null;
  }
  if (value === null) return null;
  if (Array.isArray(value)) return null;
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
  if (!validPort(numeric)) {
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





export function toPortNumber(rawPort: string) {
  const parsedPort = Number.parseInt(rawPort.trim(), 10);
  if (!validPort(parsedPort)) {
    return Number.parseInt(DEFAULT_PORT, 10);
  }
  return parsedPort;
}

function validPort(value: number) {
  if (!Number.isFinite(value)) return false;
  if (value <= 0) return false;
  return value <= 65535;
}
