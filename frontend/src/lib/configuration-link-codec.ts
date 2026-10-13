import type { ConfigurationDraft } from "./configuration-types";
import { DEFAULT_PORT, asRecord } from "./configuration-values";

function decodeBase64(value: string) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padding = normalized.length % 4;
  const padded = padding === 0 ? normalized : normalized + "=".repeat(4 - padding);
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function encodeBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

export function parseVmessExtraParams(paramsRaw: string) {
  const params = paramsRaw.trim();
  if (!params) return {};
  try {
    const parsed = JSON.parse(params);
    return asRecord(parsed) ?? {};
  } catch {
    throw new Error("JSON-параметры VMESS заполнены некорректно");
  }
}

function parseVmessLink(raw: string): ConfigurationDraft {
  const encoded = raw.slice("vmess://".length).trim();
  const payload = JSON.parse(decodeBase64(encoded)) as Record<string, unknown>;
  const { add, port, id, ps, ...rest } = payload;

  return {
    protocol: "vmess",
    server: String(add ?? "").trim(),
    port: String(port ?? DEFAULT_PORT).trim() || DEFAULT_PORT,
    identifier: String(id ?? "").trim(),
    params: Object.keys(rest).length > 0 ? JSON.stringify(rest, null, 2) : "",
    remark: String(ps ?? "").trim(),
  };
}

function parseUriLink(raw: string): ConfigurationDraft {
  const parsed = new URL(raw);
  const protocol = parsed.protocol.replace(":", "").toLowerCase();
  if (protocol !== "vless" && protocol !== "trojan") {
    throw new Error("Поддерживаются только vless://, vmess:// и trojan://");
  }

  return {
    protocol,
    server: parsed.hostname.trim(),
    port: parsed.port.trim() || DEFAULT_PORT,
    identifier: decodeURIComponent(parsed.username || "").trim(),
    params: parsed.search ? parsed.search.slice(1) : "",
    remark: decodeURIComponent(parsed.hash ? parsed.hash.slice(1) : "").trim(),
  };
}

export function parseConfiguration(raw: string): ConfigurationDraft | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  return trimmed.toLowerCase().startsWith("vmess://") ? parseVmessLink(trimmed) : parseUriLink(trimmed);
}

function normalizeLinkDraft(draft: ConfigurationDraft) {
  const protocol = draft.protocol;
  const server = draft.server.trim();
  const port = draft.port.trim() || DEFAULT_PORT;
  const identifier = draft.identifier.trim();
  const params = draft.params.trim();
  const remark = draft.remark.trim();

  if (!server) {
    throw new Error("Укажите сервер");
  }
  if (!identifier) {
    throw new Error(protocol === "trojan" ? "Укажите пароль" : "Укажите UUID");
  }

  return {
    protocol,
    server,
    port,
    identifier,
    params,
    remark
  };
}

function buildVmessLink(draft: ConfigurationDraft) {
  const { server, port, identifier, params, remark } = draft;
  let extra: Record<string, unknown> = {};
  if (params) {
    try {
      extra = JSON.parse(params) as Record<string, unknown>;
    } catch {
      throw new Error("JSON-параметры VMESS заполнены некорректно");
    }
  }

  const payload: Record<string, unknown> = {
    v: "2",
    ps: remark,
    add: server,
    port,
    id: identifier,
    ...extra,
  };

  return `vmess://${encodeBase64(JSON.stringify(payload))}`;
}

function buildUriLink(draft: ConfigurationDraft) {
  const { protocol, server, port, identifier, params, remark } = draft;
  const url = new URL(`${protocol}://${encodeURIComponent(identifier)}@${server}:${port}`);
  url.username = identifier;
  url.password = "";
  url.search = params ? `?${params}` : "";
  url.hash = remark ? `#${remark}` : "";
  return url.toString();
}

export function buildConfiguration(draft: ConfigurationDraft) {
  const normalized = normalizeLinkDraft(draft);
  return normalized.protocol === "vmess" ? buildVmessLink(normalized) : buildUriLink(normalized);
}

export function extractLabelFromConfiguration(raw: string) {
  const parsed = parseConfiguration(raw);
  if (!parsed) {
    return "";
  }
  return parsed.remark.trim();
}
