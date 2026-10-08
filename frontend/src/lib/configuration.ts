import {
  formatXrayJSON,
  modifyXrayJSONPath,
} from "@/lib/xray-json-document";

export type ConfigurationProtocol = "vless" | "trojan" | "vmess";
export type RealConfigurationMode = "link" | "xray-json";
export type XrayJSONNetwork = "tcp" | "ws" | "grpc" | "httpupgrade" | "xhttp" | "h2" | "quic";
export type XrayJSONSecurity = "none" | "tls" | "reality";

export interface ConfigurationDraft {
  protocol: ConfigurationProtocol;
  server: string;
  port: string;
  identifier: string;
  params: string;
  remark: string;
}

export interface XrayJSONDraft {
  protocol: ConfigurationProtocol;
  server: string;
  port: string;
  identifier: string;
  network: XrayJSONNetwork;
  security: XrayJSONSecurity;
  path: string;
  host: string;
  sni: string;
  alpn: string;
  remark: string;
  flow?: string;
  encryption?: string;
  fingerprint?: string;
  publicKey?: string;
  shortId?: string;
  spiderX?: string;
  allowInsecure?: boolean;
  grpcAuthority?: string;
  headerType?: string;
  vmessSecurity?: string;
  vmessAlterId?: string;
}

export interface ParsedXrayJSONConfiguration {
  draft: XrayJSONDraft;
  config: Record<string, unknown>;
  outboundIndex: number;
}

const DEFAULT_PORT = "443";
const DEFAULT_NETWORK: XrayJSONNetwork = "tcp";
const DEFAULT_SECURITY: XrayJSONSecurity = "none";
const SUPPORTED_PROTOCOLS: ConfigurationProtocol[] = ["vless", "vmess", "trojan"];

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

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function textValue(value: unknown): string {
  if (typeof value === "string") {
    return value.trim();
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return "";
}

function portValue(value: unknown): string {
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

function boolValue(value: unknown): boolean {
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

function splitCommaValues(raw: string) {
  return raw
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function parseHeaderType(value: unknown) {
  const normalized = textValue(value).toLowerCase();
  if (normalized === "none" || normalized === "http") {
    return normalized;
  }
  return "";
}

function normalizeProtocol(value: string): ConfigurationProtocol {
  const parsed = parseSupportedProtocol(value);
  if (parsed) {
    return parsed;
  }
  return "vless";
}

function parseSupportedProtocol(value: string): ConfigurationProtocol | null {
  const normalized = value.toLowerCase().trim() as ConfigurationProtocol;
  if (SUPPORTED_PROTOCOLS.includes(normalized)) {
    return normalized;
  }
  return null;
}

function normalizeNetwork(value: string): XrayJSONNetwork {
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

function normalizeSecurity(value: string): XrayJSONSecurity {
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

function findSupportedOutbound(config: Record<string, unknown>) {
  const outbounds = asArray(config.outbounds);
  for (let index = 0; index < outbounds.length; index += 1) {
    const outbound = asRecord(outbounds[index]);
    if (!outbound) continue;
    const protocol = parseSupportedProtocol(textValue(outbound.protocol));
    if (protocol) {
      return { outbound, outboundIndex: index, protocol };
    }
  }
  return null;
}

function parseVmessExtraParams(paramsRaw: string) {
  const params = paramsRaw.trim();
  if (!params) return {};
  try {
    const parsed = JSON.parse(params);
    return asRecord(parsed) ?? {};
  } catch {
    throw new Error("JSON-параметры VMESS заполнены некорректно");
  }
}

function parseQueryParams(paramsRaw: string) {
  const search = new URLSearchParams(paramsRaw);
  return {
    network: normalizeNetwork(search.get("type") || DEFAULT_NETWORK),
    security: normalizeSecurity(search.get("security") || DEFAULT_SECURITY),
    path: (search.get("path") || "").trim(),
    host: (search.get("host") || "").trim(),
    sni: (search.get("sni") || "").trim(),
    alpn: (search.get("alpn") || "").trim(),
    encryption: (search.get("encryption") || "").trim(),
    flow: (search.get("flow") || "").trim(),
    fingerprint: (search.get("fp") || search.get("fingerprint") || "").trim(),
    publicKey: (search.get("pbk") || search.get("publicKey") || search.get("password") || "").trim(),
    shortId: (search.get("sid") || search.get("shortId") || "").trim(),
    spiderX: (search.get("spx") || search.get("spiderX") || "").trim(),
    allowInsecure: boolValue(search.get("allowInsecure") || search.get("allowinsecure") || search.get("allow_insecure")),
    headerType: parseHeaderType(search.get("headerType") || search.get("header_type") || search.get("typeHeader")),
  };
}

export function extractLabelFromConfiguration(raw: string) {
  const parsed = parseConfiguration(raw);
  if (!parsed) {
    return "";
  }
  return parsed.remark.trim();
}

export function parseConfiguration(raw: string): ConfigurationDraft | null {
  const trimmed = raw.trim();
  if (!trimmed) {
    return null;
  }

  if (trimmed.toLowerCase().startsWith("vmess://")) {
    const encoded = trimmed.slice("vmess://".length).trim();
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

  const parsed = new URL(trimmed);
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

export function buildConfiguration(draft: ConfigurationDraft) {
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

  if (protocol === "vmess") {
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

  const url = new URL(`${protocol}://${encodeURIComponent(identifier)}@${server}:${port}`);
  url.username = identifier;
  url.password = "";
  url.search = params ? `?${params}` : "";
  url.hash = remark ? `#${remark}` : "";
  return url.toString();
}

export function parseXrayJSONConfiguration(raw: string): ParsedXrayJSONConfiguration | null {
  const trimmed = raw.trim();
  if (!trimmed) {
    return null;
  }

  let parsedUnknown: unknown;
  try {
    parsedUnknown = JSON.parse(trimmed);
  } catch {
    throw new Error("XRAY-JSON содержит ошибку синтаксиса");
  }

  const config = asRecord(parsedUnknown);
  if (!config) {
    throw new Error("XRAY-JSON должен быть JSON-объектом");
  }

  const supportedOutbound = findSupportedOutbound(config);
  if (!supportedOutbound) {
    throw new Error("Не найден outbound с протоколом vless/vmess/trojan");
  }

  const { outbound, outboundIndex, protocol } = supportedOutbound;
  const settings = asRecord(outbound.settings) ?? {};
  const streamSettings = asRecord(outbound.streamSettings) ?? {};

  let server = "";
  let port = DEFAULT_PORT;
  let identifier = "";

  if (protocol === "trojan") {
    const serverNode = asRecord(asArray(settings.servers)[0]);
    server = textValue(serverNode?.address);
    port = portValue(serverNode?.port);
    identifier = textValue(serverNode?.password);
  } else {
    const vnextNode = asRecord(asArray(settings.vnext)[0]);
    const userNode = asRecord(asArray(vnextNode?.users)[0]);
    server = textValue(vnextNode?.address);
    port = portValue(vnextNode?.port);
    identifier = textValue(userNode?.id);
  }

  const network = normalizeNetwork(textValue(streamSettings.network));

  let path = "";
  let host = "";
  let sni = "";
  let alpn = "";
  let flow = "";
  let encryption = "";
  let fingerprint = "";
  let publicKey = "";
  let shortId = "";
  let spiderX = "";
  let grpcAuthority = "";
  let headerType = "";
  let allowInsecure = false;
  let vmessSecurity = "";
  let vmessAlterId = "";

  const wsSettings = asRecord(streamSettings.wsSettings);
  const grpcSettings = asRecord(streamSettings.grpcSettings);
  const httpUpgradeSettings = asRecord(streamSettings.httpupgradeSettings);
  const xhttpSettings = asRecord(streamSettings.xhttpSettings);
  const tlsSettings = asRecord(streamSettings.tlsSettings);
  const realitySettings = asRecord(streamSettings.realitySettings);
  const rawSettings = asRecord(streamSettings.rawSettings);
  const tcpSettings = asRecord(streamSettings.tcpSettings);
  const tcpHeader = asRecord(rawSettings?.header) ?? asRecord(tcpSettings?.header);

  let security = normalizeSecurity(textValue(streamSettings.security));
  if (security === "none" && realitySettings && Object.keys(realitySettings).length > 0) {
    security = "reality";
  } else if (security === "none" && tlsSettings && Object.keys(tlsSettings).length > 0) {
    security = "tls";
  }

  if (protocol !== "trojan") {
    const vnextNode = asRecord(asArray(settings.vnext)[0]);
    const userNode = asRecord(asArray(vnextNode?.users)[0]);
    flow = textValue(userNode?.flow);
    encryption = textValue(userNode?.encryption);
    vmessSecurity = textValue(userNode?.security);
    vmessAlterId = textValue(userNode?.alterId);
  }

  if (network === "ws") {
    path = textValue(wsSettings?.path);
    const headers = asRecord(wsSettings?.headers);
    host = textValue(headers?.Host) || textValue(headers?.host);
  } else if (network === "grpc") {
    path = textValue(grpcSettings?.serviceName);
    grpcAuthority = textValue(grpcSettings?.authority);
  } else if (network === "httpupgrade") {
    path = textValue(httpUpgradeSettings?.path);
    host = textValue(httpUpgradeSettings?.host);
  } else if (network === "xhttp") {
    path = textValue(xhttpSettings?.path);
    host = textValue(xhttpSettings?.host);
  } else if (network === "tcp") {
    headerType = parseHeaderType(tcpHeader?.type);
    if (headerType === "http") {
      const request = asRecord(tcpHeader?.request);
      const paths = asArray(request?.path).map((entry) => textValue(entry)).filter(Boolean);
      if (paths.length > 0) {
        path = paths.join(",");
      }
      const headers = asRecord(request?.headers);
      const hosts = asArray(headers?.Host ?? headers?.host).map((entry) => textValue(entry)).filter(Boolean);
      if (hosts.length > 0) {
        host = hosts.join(",");
      }
    }
  }

  if (security === "tls") {
    sni = textValue(tlsSettings?.serverName);
    const alpnValues = asArray(tlsSettings?.alpn).map((entry) => textValue(entry)).filter(Boolean);
    alpn = alpnValues.join(",");
    allowInsecure = boolValue(tlsSettings?.allowInsecure);
    fingerprint = textValue(tlsSettings?.fingerprint);
  } else if (security === "reality") {
    sni = textValue(realitySettings?.serverName);
    fingerprint = textValue(realitySettings?.fingerprint);
    publicKey = textValue(realitySettings?.publicKey) || textValue(realitySettings?.password);
    shortId = textValue(realitySettings?.shortId);
    spiderX = textValue(realitySettings?.spiderX);
  }

  const remark = textValue(outbound.tag);

  return {
    config,
    outboundIndex,
    draft: {
      protocol,
      server,
      port,
      identifier,
      network,
      security,
      path,
      host,
      sni,
      alpn,
      remark,
      flow,
      encryption,
      fingerprint,
      publicKey,
      shortId,
      spiderX,
      allowInsecure,
      grpcAuthority,
      headerType,
      vmessSecurity,
      vmessAlterId,
    },
  };
}

function buildALPNValues(raw: string) {
  return raw
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

function toPortNumber(rawPort: string) {
  const parsedPort = Number.parseInt(rawPort.trim(), 10);
  if (!Number.isFinite(parsedPort) || parsedPort <= 0 || parsedPort > 65535) {
    return Number.parseInt(DEFAULT_PORT, 10);
  }
  return parsedPort;
}

function buildDefaultXrayJSONConfig(draft: XrayJSONDraft) {
  const port = toPortNumber(draft.port || DEFAULT_PORT);
  const outbound: Record<string, unknown> = {
    tag: draft.remark.trim() || "proxy",
    protocol: draft.protocol,
    settings: {},
    streamSettings: {
      network: draft.network || DEFAULT_NETWORK,
      security: draft.security || DEFAULT_SECURITY,
    },
  };

  if (draft.protocol === "trojan") {
    outbound.settings = {
      servers: [
        {
          address: draft.server.trim(),
          port,
          password: draft.identifier.trim(),
        },
      ],
    };
  } else {
    const userNode: Record<string, unknown> = {
      id: draft.identifier.trim(),
    };
    if (draft.protocol === "vless") {
      userNode.encryption = draft.encryption?.trim() || "none";
      if (draft.flow?.trim()) {
        userNode.flow = draft.flow.trim();
      }
    }
    if (draft.protocol === "vmess") {
      userNode.security = draft.vmessSecurity?.trim() || "auto";
      if (draft.vmessAlterId?.trim()) {
        const parsedAlterID = Number.parseInt(draft.vmessAlterId.trim(), 10);
        if (Number.isFinite(parsedAlterID) && parsedAlterID >= 0) {
          userNode.alterId = parsedAlterID;
        } else {
          userNode.alterId = draft.vmessAlterId.trim();
        }
      }
    }
    outbound.settings = {
      vnext: [
        {
          address: draft.server.trim(),
          port,
          users: [userNode],
        },
      ],
    };
  }

  const streamSettings = asRecord(outbound.streamSettings) ?? {};
  if (draft.network === "ws") {
    streamSettings.wsSettings = {
      path: draft.path.trim(),
      headers: draft.host.trim() ? { Host: draft.host.trim() } : {},
    };
  } else if (draft.network === "grpc") {
    streamSettings.grpcSettings = {
      serviceName: draft.path.trim(),
      ...(draft.grpcAuthority?.trim() ? { authority: draft.grpcAuthority.trim() } : {}),
    };
  } else if (draft.network === "httpupgrade") {
    streamSettings.httpupgradeSettings = {
      path: draft.path.trim(),
      host: draft.host.trim(),
    };
  } else if (draft.network === "xhttp") {
    streamSettings.xhttpSettings = {
      path: draft.path.trim(),
      host: draft.host.trim(),
    };
  } else if (draft.network === "tcp") {
    const normalizedHeaderType = parseHeaderType(draft.headerType);
    if (normalizedHeaderType) {
      const header: Record<string, unknown> = {
        type: normalizedHeaderType,
      };
      if (normalizedHeaderType === "http") {
        const request: Record<string, unknown> = {};
        const paths = splitCommaValues(draft.path);
        if (paths.length > 0) {
          request.path = paths;
        }
        const hosts = splitCommaValues(draft.host);
        if (hosts.length > 0) {
          request.headers = { Host: hosts };
        }
        if (Object.keys(request).length > 0) {
          header.request = request;
        }
      }
      streamSettings.tcpSettings = { header };
      streamSettings.rawSettings = { header };
    } else {
      streamSettings.tcpSettings = {};
      delete streamSettings.rawSettings;
    }
  }
  if (draft.security === "tls") {
    const alpnValues = buildALPNValues(draft.alpn);
    streamSettings.tlsSettings = {
      serverName: draft.sni.trim(),
      ...(alpnValues.length > 0 ? { alpn: alpnValues } : {}),
      ...(draft.allowInsecure ? { allowInsecure: true } : {}),
      ...(draft.fingerprint?.trim() ? { fingerprint: draft.fingerprint.trim() } : {}),
    };
  }
  if (draft.security === "reality") {
    streamSettings.realitySettings = {
      serverName: draft.sni.trim(),
      show: false,
      ...(draft.fingerprint?.trim() ? { fingerprint: draft.fingerprint.trim() } : {}),
      ...(draft.publicKey?.trim() ? { publicKey: draft.publicKey.trim(), password: draft.publicKey.trim() } : {}),
      ...(draft.shortId?.trim() ? { shortId: draft.shortId.trim() } : {}),
      ...(draft.spiderX?.trim() ? { spiderX: draft.spiderX.trim() } : {}),
    };
  }
  outbound.streamSettings = streamSettings;

  return {
    dns: {
      servers: ["1.1.1.1", "1.0.0.1"],
      queryStrategy: "UseIP",
    },
    routing: {
      rules: [
        {
          type: "field",
          protocol: ["bittorrent"],
          outboundTag: "direct",
        },
      ],
      domainMatcher: "hybrid",
      domainStrategy: "IPIfNonMatch",
    },
    inbounds: [
      {
        tag: "socks",
        port: 10808,
        listen: "127.0.0.1",
        protocol: "socks",
        settings: {
          udp: true,
          auth: "noauth",
        },
        sniffing: {
          enabled: true,
          routeOnly: false,
          destOverride: ["http", "tls", "quic"],
        },
      },
      {
        tag: "http",
        port: 10809,
        listen: "127.0.0.1",
        protocol: "http",
        settings: {
          allowTransparent: false,
        },
        sniffing: {
          enabled: true,
          routeOnly: false,
          destOverride: ["http", "tls", "quic"],
        },
      },
    ],
    outbounds: [
      outbound,
      {
        tag: "direct",
        protocol: "freedom",
      },
      {
        tag: "block",
        protocol: "blackhole",
      },
    ],
  } as Record<string, unknown>;
}

export function buildXrayJSONConfiguration(
  draft: XrayJSONDraft
) {
  const protocol = normalizeProtocol(draft.protocol);
  const server = draft.server.trim();
  const identifier = draft.identifier.trim();

  if (!server) {
    throw new Error("Укажите сервер");
  }
  if (!identifier) {
    throw new Error(protocol === "trojan" ? "Укажите пароль" : "Укажите UUID / ID");
  }

  return JSON.stringify(buildDefaultXrayJSONConfig({ ...draft, protocol }), null, 2);
}

export type XrayJSONPatch = Partial<XrayJSONDraft>;

type MutableJSONPath = Array<string | number>;

function hasPatchField<Key extends keyof XrayJSONDraft>(
  patch: XrayJSONPatch,
  key: Key
) {
  return Object.prototype.hasOwnProperty.call(patch, key);
}

function patchJSONPath(
  raw: string,
  path: MutableJSONPath,
  value: unknown
) {
  return modifyXrayJSONPath(raw, path, value);
}





function optionalText(value: unknown) {
  const normalized = typeof value === "string" ? value.trim() : "";
  return normalized || undefined;
}

function alterIDValue(value: unknown) {
  const normalized = optionalText(value);
  if (!normalized) return undefined;
  const parsed = Number.parseInt(normalized, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : normalized;
}

// Ordered edits contain only represented values and parent initialization.
// Applying them to the original tokens preserves unknown data and lexemes.
interface XrayDocumentEdit { path: MutableJSONPath; value: unknown; }
function appendDocumentEdit(operations: XrayDocumentEdit[], path: MutableJSONPath, value: unknown) { operations.push({ path, value }); }
function appendObjectInitialization(operations: XrayDocumentEdit[], path: MutableJSONPath, currentValue: unknown) { if (!asRecord(currentValue)) appendDocumentEdit(operations, path, {}); }
function appendFirstObjectInitialization(operations: XrayDocumentEdit[], path: MutableJSONPath, currentValue: unknown) {
  const entries = asArray(currentValue);
  if (entries.length === 0) { appendDocumentEdit(operations, path, [{}]); return; }
  if (!asRecord(entries[0])) appendDocumentEdit(operations, [...path, 0], {});
}
function hasAnyPatchField(patch: XrayJSONPatch, fields: readonly (keyof XrayJSONDraft)[]) { return fields.some(field => hasPatchField(patch, field)); }
function readXrayPatchProjection(parsed: ParsedXrayJSONConfiguration) {
  const outboundPath: MutableJSONPath = ["outbounds", parsed.outboundIndex];
  const outbound = asRecord(asArray(parsed.config.outbounds)[parsed.outboundIndex]) ?? {};
  const settings = asRecord(outbound.settings);
  const streamSettings = asRecord(outbound.streamSettings);
  const originalVnext = asArray(settings?.vnext);
  const originalVnextNode = asRecord(originalVnext[0]);
  const originalUsers = asArray(originalVnextNode?.users);
  const originalUser = asRecord(originalUsers[0]);
  return { outboundPath, outbound, settings, streamSettings, originalVnextNode, originalUser };
}
function createXrayPatchContext(parsed: ParsedXrayJSONConfiguration, patch: XrayJSONPatch) {
  const operations: XrayDocumentEdit[] = [];
  const nextDraft = { ...parsed.draft, ...patch };
  const protocol = normalizeProtocol(nextDraft.protocol);
  const server = nextDraft.server.trim();
  const identifier = nextDraft.identifier.trim();
  const port = toPortNumber(nextDraft.port || DEFAULT_PORT);
  const network = normalizeNetwork(nextDraft.network);
  const security = normalizeSecurity(nextDraft.security);
  if (!server) {
    throw new Error("Укажите сервер");
  }
  if (!identifier) {
    throw new Error(protocol === "trojan" ? "Укажите пароль" : "Укажите UUID / ID");
  }
  const { outboundPath, outbound, settings, streamSettings, originalVnextNode, originalUser } = readXrayPatchProjection(parsed);
  let settingsReady = false;
  let connectionReady = false;
  const ensureSettings = () => {
    if (!settingsReady) {
      appendObjectInitialization(operations, [...outboundPath, "settings"], outbound.settings);
      settingsReady = true;
    }
  };
  const ensureConnection = () => {
    if (connectionReady)
      return;
    ensureSettings();
    if (protocol === "trojan") {
      appendFirstObjectInitialization(operations, [...outboundPath, "settings", "servers"], settings?.servers);
    }
    else {
      appendFirstObjectInitialization(operations, [...outboundPath, "settings", "vnext"], settings?.vnext);
      appendFirstObjectInitialization(operations, [...outboundPath, "settings", "vnext", 0, "users"], originalVnextNode?.users);
    }
    connectionReady = true;
  };
  const connectionBasePath = () => protocol === "trojan"
    ? [...outboundPath, "settings", "servers", 0]
    : [...outboundPath, "settings", "vnext", 0];
  const userBasePath = () => [...outboundPath, "settings", "vnext", 0, "users", 0];
  let streamSettingsReady = false;
  const ensureStreamSettings = () => {
    if (!streamSettingsReady) {
      appendObjectInitialization(operations, [...outboundPath, "streamSettings"], outbound.streamSettings);
      streamSettingsReady = true;
    }
  };
  const transportFieldsChanged = hasAnyPatchField(patch, ["path", "host", "grpcAuthority", "headerType"]);
  const securityFieldsChanged = hasAnyPatchField(patch, ["sni", "alpn", "allowInsecure", "fingerprint", "publicKey", "shortId", "spiderX"]);
  return { patch, nextDraft, protocol, server, identifier, port, network, security, outboundPath, outbound, settings, streamSettings, originalVnextNode, originalUser, ensureSettings, ensureConnection, connectionBasePath, userBasePath, ensureStreamSettings, transportFieldsChanged, securityFieldsChanged, operations };
}
type XrayPatchContext = ReturnType<typeof createXrayPatchContext>;
function appendProtocolEdits(context: XrayPatchContext) {
  if (!hasPatchField(context.patch, "protocol")) return;
  appendDocumentEdit(context.operations, [...context.outboundPath, "protocol"], context.protocol);
  context.ensureConnection();
  appendDocumentEdit(context.operations, [...context.connectionBasePath(), "address"], context.server);
  appendDocumentEdit(context.operations, [...context.connectionBasePath(), "port"], context.port);
  appendDocumentEdit(context.operations, connectionCredentialPath(context), context.identifier);
  appendInitialUserDefaults(context);
}
function connectionCredentialPath(context: XrayPatchContext) {
  if (context.protocol === "trojan") return [...context.connectionBasePath(), "password"];
  return [...context.userBasePath(), "id"];
}
function appendInitialUserDefaults(context: XrayPatchContext) {
  if (context.originalUser) return;
  if (context.protocol === "vless") appendDocumentEdit(context.operations, [...context.userBasePath(), "encryption"], context.nextDraft.encryption?.trim() || "none");
  else if (context.protocol === "vmess") appendDocumentEdit(context.operations, [...context.userBasePath(), "security"], context.nextDraft.vmessSecurity?.trim() || "auto");
}
function appendConnectionEdits(context: XrayPatchContext) {
  const { patch, ensureConnection, operations, connectionBasePath, server, port, protocol, userBasePath, identifier, outboundPath } = context;
  if (hasPatchField(patch, "server")) {
    ensureConnection();
    appendDocumentEdit(operations, [...connectionBasePath(), "address"], server);
  }
  if (hasPatchField(patch, "port")) {
    ensureConnection();
    appendDocumentEdit(operations, [...connectionBasePath(), "port"], port);
  }
  if (hasPatchField(patch, "identifier")) {
    ensureConnection();
    const path = protocol === "trojan"
      ? [...connectionBasePath(), "password"]
      : [...userBasePath(), "id"];
    appendDocumentEdit(operations, path, identifier);
  }
  if (hasPatchField(patch, "remark")) {
    appendDocumentEdit(operations, [...outboundPath, "tag"], optionalText(patch.remark));
  }
}
function appendUserEdits(context: XrayPatchContext) {

  if (context.protocol !== "trojan" && hasAnyPatchField(context.patch, ["flow", "encryption", "vmessSecurity", "vmessAlterId"])) context.ensureConnection();
  const fields = [{ protocol: "vless", field: "flow", target: "flow", value: optionalText }, { protocol: "vless", field: "encryption", target: "encryption", value: optionalText }, { protocol: "vmess", field: "vmessSecurity", target: "security", value: optionalText }, { protocol: "vmess", field: "vmessAlterId", target: "alterId", value: alterIDValue }] as const;
  for (const field of fields) {
    if (context.protocol === field.protocol && hasPatchField(context.patch, field.field)) appendDocumentEdit(context.operations, [...context.userBasePath(), field.target], field.value(context.patch[field.field]));
  }
}
function appendStreamDiscriminators(context: XrayPatchContext) {
  const { patch, ensureStreamSettings, operations, outboundPath, network, security } = context;
  if (hasPatchField(patch, "network")) {
    ensureStreamSettings();
    appendDocumentEdit(operations, [...outboundPath, "streamSettings", "network"], network);
  }
  if (hasPatchField(patch, "security")) {
    ensureStreamSettings();
    appendDocumentEdit(operations, [...outboundPath, "streamSettings", "security"], security);
  }
}
function appendWebSocketEdits(context: XrayPatchContext) {
  const settings = asRecord(context.streamSettings?.wsSettings);
  const path = [...context.outboundPath, "streamSettings", "wsSettings"];
  appendObjectInitialization(context.operations, path, context.streamSettings?.wsSettings);
  if (hasPatchField(context.patch, "path")) appendDocumentEdit(context.operations, [...path, "path"], optionalText(context.patch.path));
  if (hasPatchField(context.patch, "host")) appendWebSocketHost(context, settings, path);
}
function existingHostAlias(headers: Record<string, unknown> | null) {
  if (Object.prototype.hasOwnProperty.call(headers ?? {}, "Host")) return "Host";
  if (Object.prototype.hasOwnProperty.call(headers ?? {}, "host")) return "host";
  return "Host";
}
function appendWebSocketHost(context: XrayPatchContext, settings: Record<string, unknown> | null, path: MutableJSONPath) {
  const headers = asRecord(settings?.headers);
  const headerPath = [...path, "headers"];
  appendObjectInitialization(context.operations, headerPath, settings?.headers);
  const host = optionalText(context.patch.host);
  if (host) appendDocumentEdit(context.operations, [...headerPath, existingHostAlias(headers)], host);
  else appendHostClear(context.operations, headerPath);
}
function appendHostClear(operations: XrayDocumentEdit[], path: MutableJSONPath) {
  appendDocumentEdit(operations, [...path, "Host"], undefined);
  appendDocumentEdit(operations, [...path, "host"], undefined);
}
function appendGrpcEdits(context: XrayPatchContext) {
  const { operations, outboundPath, streamSettings, patch } = context;

  appendObjectInitialization(operations, [...outboundPath, "streamSettings", "grpcSettings"], streamSettings?.grpcSettings);
  if (hasPatchField(patch, "path")) {
    appendDocumentEdit(operations, [...outboundPath, "streamSettings", "grpcSettings", "serviceName"], optionalText(patch.path));
  }
  if (hasPatchField(patch, "grpcAuthority")) {
    appendDocumentEdit(operations, [...outboundPath, "streamSettings", "grpcSettings", "authority"], optionalText(patch.grpcAuthority));
  }

}
function appendHttpTransportEdits(context: XrayPatchContext) {
  const { network, operations, outboundPath, streamSettings, patch } = context;

  const branch = network === "httpupgrade" ? "httpupgradeSettings" : "xhttpSettings";
  appendObjectInitialization(operations, [...outboundPath, "streamSettings", branch], streamSettings?.[branch]);
  if (hasPatchField(patch, "path")) {
    appendDocumentEdit(operations, [...outboundPath, "streamSettings", branch, "path"], optionalText(patch.path));
  }
  if (hasPatchField(patch, "host")) {
    appendDocumentEdit(operations, [...outboundPath, "streamSettings", branch, "host"], optionalText(patch.host));
  }

}
function appendTcpEdits(context: XrayPatchContext) {
  const scope = readTcpPatchScope(context);
  appendObjectInitialization(context.operations, scope.branchPath, scope.settings);
  appendObjectInitialization(context.operations, scope.headerPath, scope.settings?.header);
  if (hasPatchField(context.patch, "headerType")) appendDocumentEdit(context.operations, [...scope.headerPath, "type"], optionalText(parseHeaderType(context.patch.headerType)));
  if (hasAnyPatchField(context.patch, ["path", "host"])) appendObjectInitialization(context.operations, scope.requestPath, scope.header?.request);
  appendTcpPath(context, scope);
  appendTcpHost(context, scope);
}
function readTcpPatchScope(context: XrayPatchContext) {
  const raw = asRecord(context.streamSettings?.rawSettings);
  const tcp = asRecord(context.streamSettings?.tcpSettings);
  const useRaw = Boolean(asRecord(raw?.header));
  const branch = useRaw ? "rawSettings" : "tcpSettings";
  const settings = useRaw ? raw : tcp;
  const header = asRecord(settings?.header);
  const request = asRecord(header?.request);
  const branchPath = [...context.outboundPath, "streamSettings", branch];
  const headerPath = [...branchPath, "header"];
  return { settings, header, request, branchPath, headerPath, requestPath: [...headerPath, "request"] };
}
type TcpPatchScope = ReturnType<typeof readTcpPatchScope>;
function appendTcpPath(context: XrayPatchContext, scope: TcpPatchScope) {
  if (!hasPatchField(context.patch, "path")) return;
  const values = splitCommaValues(context.patch.path ?? "");
  appendDocumentEdit(context.operations, [...scope.requestPath, "path"], values.length > 0 ? values : undefined);
}
function appendTcpHost(context: XrayPatchContext, scope: TcpPatchScope) {
  if (!hasPatchField(context.patch, "host")) return;
  const headers = asRecord(scope.request?.headers);
  const path = [...scope.requestPath, "headers"];
  appendObjectInitialization(context.operations, path, scope.request?.headers);
  const values = splitCommaValues(context.patch.host ?? "");
  if (values.length > 0) appendDocumentEdit(context.operations, [...path, existingHostAlias(headers)], values);
  else appendHostClear(context.operations, path);
}
function appendTlsEdits(context: XrayPatchContext) {
  const { operations, outboundPath, streamSettings, patch } = context;

  appendObjectInitialization(operations, [...outboundPath, "streamSettings", "tlsSettings"], streamSettings?.tlsSettings);
  const tlsPath = [...outboundPath, "streamSettings", "tlsSettings"];
  if (hasPatchField(patch, "sni")) {
    appendDocumentEdit(operations, [...tlsPath, "serverName"], optionalText(patch.sni));
  }
  if (hasPatchField(patch, "alpn")) {
    const values = buildALPNValues(patch.alpn ?? "");
    appendDocumentEdit(operations, [...tlsPath, "alpn"], values.length > 0 ? values : undefined);
  }
  if (hasPatchField(patch, "allowInsecure")) {
    appendDocumentEdit(operations, [...tlsPath, "allowInsecure"], patch.allowInsecure ? true : undefined);
  }
  if (hasPatchField(patch, "fingerprint")) {
    appendDocumentEdit(operations, [...tlsPath, "fingerprint"], optionalText(patch.fingerprint));
  }

}
function appendRealityEdits(context: XrayPatchContext) {
  const { streamSettings, operations, outboundPath, patch } = context;

  const realitySettings = asRecord(streamSettings?.realitySettings);
  appendObjectInitialization(operations, [...outboundPath, "streamSettings", "realitySettings"], streamSettings?.realitySettings);
  const realityPath = [...outboundPath, "streamSettings", "realitySettings"];
  if (hasPatchField(patch, "sni")) {
    appendDocumentEdit(operations, [...realityPath, "serverName"], optionalText(patch.sni));
  }
  if (hasPatchField(patch, "fingerprint")) {
    appendDocumentEdit(operations, [...realityPath, "fingerprint"], optionalText(patch.fingerprint));
  }
  if (hasPatchField(patch, "publicKey")) appendRealityKey(context, realitySettings, realityPath);
  if (hasPatchField(patch, "shortId")) {
    appendDocumentEdit(operations, [...realityPath, "shortId"], optionalText(patch.shortId));
  }
  if (hasPatchField(patch, "spiderX")) {
    appendDocumentEdit(operations, [...realityPath, "spiderX"], optionalText(patch.spiderX));
  }

}
function appendRealityKey(context: XrayPatchContext, settings: Record<string, unknown> | null, path: MutableJSONPath) {
  const value = optionalText(context.patch.publicKey);
  if (!value) {
    appendDocumentEdit(context.operations, [...path, "publicKey"], undefined);
    appendDocumentEdit(context.operations, [...path, "password"], undefined);
    return;
  }
  const modern = Object.prototype.hasOwnProperty.call(settings ?? {}, "publicKey");
  const legacy = Object.prototype.hasOwnProperty.call(settings ?? {}, "password");
  if (modern || !legacy) appendDocumentEdit(context.operations, [...path, "publicKey"], value);
  if (legacy) appendDocumentEdit(context.operations, [...path, "password"], value);
}
function appendTransportEdits(context: XrayPatchContext) {
  if (context.transportFieldsChanged) context.ensureStreamSettings();
  const handlers = [
    { networks: ["ws"], fields: ["path", "host"], append: appendWebSocketEdits },
    { networks: ["grpc"], fields: ["path", "grpcAuthority"], append: appendGrpcEdits },
    { networks: ["httpupgrade", "xhttp"], fields: ["path", "host"], append: appendHttpTransportEdits },
    { networks: ["tcp"], fields: ["path", "host", "headerType"], append: appendTcpEdits },
  ] as const;
  const handler = handlers.find(entry => (entry.networks as readonly string[]).includes(context.network));
  if (handler && hasAnyPatchField(context.patch, handler.fields)) handler.append(context);
}
function appendSecurityEdits(context: XrayPatchContext) {
  if (!context.securityFieldsChanged) return;
  context.ensureStreamSettings();
  if (context.security === "tls") appendTlsEdits(context);
  else if (context.security === "reality") appendRealityEdits(context);
}
export function patchXrayJSONConfiguration(raw: string, patch: XrayJSONPatch) {
  const formatted = formatXrayJSON(raw);
  const parsed = parseXrayJSONConfiguration(formatted);
  if (!parsed) throw new Error("Сначала заполните XRAY-JSON конфигурацию");
  return applyXrayDocumentEdits(formatted, buildXrayPatchPlan(parsed, patch));
}
function buildXrayPatchPlan(parsed: ParsedXrayJSONConfiguration, patch: XrayJSONPatch): readonly XrayDocumentEdit[] {
  const context = createXrayPatchContext(parsed, patch);
  appendProtocolEdits(context);
  appendConnectionEdits(context);
  appendUserEdits(context);
  appendStreamDiscriminators(context);
  appendTransportEdits(context);
  appendSecurityEdits(context);
  return context.operations;
}
function applyXrayDocumentEdits(raw: string, operations: readonly XrayDocumentEdit[]) {
  let nextRaw = raw;
  for (const operation of operations) nextRaw = patchJSONPath(nextRaw, operation.path, operation.value);
  return formatXrayJSON(nextRaw);
}


export function createXrayJSONFromConfiguration(raw: string) {
  const parsed = parseConfiguration(raw);
  if (!parsed) {
    throw new Error("Сначала заполните конфигурацию в формате ссылки");
  }

  const protocol = parsed.protocol;
  let network: XrayJSONNetwork = DEFAULT_NETWORK;
  let security: XrayJSONSecurity = DEFAULT_SECURITY;
  let path = "";
  let host = "";
  let sni = "";
  let alpn = "";
  let flow = "";
  let encryption = "";
  let fingerprint = "";
  let publicKey = "";
  let shortId = "";
  let spiderX = "";
  let allowInsecure = false;
  let grpcAuthority = "";
  let headerType = "";
  let vmessSecurity = "";
  let vmessAlterId = "";

  if (protocol === "vmess") {
    const vmessParams = parseVmessExtraParams(parsed.params);
    network = normalizeNetwork(textValue(vmessParams.net) || textValue(vmessParams.type) || DEFAULT_NETWORK);

    const tlsFlag = textValue(vmessParams.tls).toLowerCase();
    if (tlsFlag === "tls") {
      security = "tls";
    } else {
      security = normalizeSecurity(textValue(vmessParams.security) || DEFAULT_SECURITY);
    }

    path = textValue(vmessParams.path);
    host = textValue(vmessParams.host);
    sni = textValue(vmessParams.sni);
    alpn = textValue(vmessParams.alpn);
    flow = textValue(vmessParams.flow);
    fingerprint = textValue(vmessParams.fp) || textValue(vmessParams.fingerprint);
    publicKey = textValue(vmessParams.pbk) || textValue(vmessParams.publicKey) || textValue(vmessParams.password);
    shortId = textValue(vmessParams.sid) || textValue(vmessParams.shortId);
    spiderX = textValue(vmessParams.spx) || textValue(vmessParams.spiderX);
    allowInsecure = boolValue(vmessParams.allowInsecure) || boolValue(vmessParams.allowinsecure);
    grpcAuthority = textValue(vmessParams.authority);
    headerType = parseHeaderType(vmessParams.type) || parseHeaderType(vmessParams.headerType);
    vmessSecurity = textValue(vmessParams.scy) || textValue(vmessParams.encryption) || textValue(vmessParams.securityType);
    vmessAlterId = textValue(vmessParams.aid);
  } else {
    const parsedParams = parseQueryParams(parsed.params);
    network = parsedParams.network;
    security = parsedParams.security;
    path = parsedParams.path;
    host = parsedParams.host;
    sni = parsedParams.sni;
    alpn = parsedParams.alpn;
    flow = parsedParams.flow;
    encryption = parsedParams.encryption;
    fingerprint = parsedParams.fingerprint;
    publicKey = parsedParams.publicKey;
    shortId = parsedParams.shortId;
    spiderX = parsedParams.spiderX;
    allowInsecure = parsedParams.allowInsecure;
    headerType = parsedParams.headerType;
  }

  return buildXrayJSONConfiguration({
    protocol,
    server: parsed.server,
    port: parsed.port,
    identifier: parsed.identifier,
    network,
    security,
    path,
    host,
    sni,
    alpn,
    remark: parsed.remark,
    flow,
    encryption,
    fingerprint,
    publicKey,
    shortId,
    spiderX,
    allowInsecure,
    grpcAuthority,
    headerType,
    vmessSecurity,
    vmessAlterId,
  });
}

export function parseEditableConfiguration(raw: string): XrayJSONDraft {
  const converted = createXrayJSONFromConfiguration(raw);
  const parsed = parseXrayJSONConfiguration(converted);
  if (!parsed) {
    throw new Error("Не удалось разобрать конфигурацию");
  }
  return parsed.draft;
}

function patchSearchValue(
  search: URLSearchParams,
  aliases: string[],
  value: string | undefined,
  fallbackKey = aliases[0]
) {
  aliases.forEach((key) => search.delete(key));
  if (value?.trim()) search.set(fallbackKey, value.trim());
}

function patchObjectValue(
  object: Record<string, unknown>,
  aliases: string[],
  value: string | boolean | undefined,
  fallbackKey = aliases[0]
) {
  aliases.forEach((key) => delete object[key]);
  if (typeof value === "boolean") {
    if (value) object[fallbackKey] = "1";
  } else if (value?.trim()) {
    object[fallbackKey] = value.trim();
  }
}

// Patches only represented legacy share-link fields. Unknown query entries,
// duplicate unrelated entries, and VMess payload properties remain intact.
export function patchEditableConfiguration(raw: string, patch: XrayJSONPatch) {
  const parsed = parseConfiguration(raw);
  if (!parsed) throw new Error("Не удалось разобрать конфигурацию");
  const next = { ...parsed };
  if (hasPatchField(patch, "server")) next.server = patch.server ?? "";
  if (hasPatchField(patch, "port")) next.port = patch.port ?? "";
  if (hasPatchField(patch, "identifier")) next.identifier = patch.identifier ?? "";
  if (hasPatchField(patch, "remark")) next.remark = patch.remark ?? "";

  if (parsed.protocol === "vmess") {
    const extra = parseVmessExtraParams(parsed.params);
    if (hasPatchField(patch, "network")) patchObjectValue(extra, ["net"], patch.network === "tcp" ? "" : patch.network);
    if (hasPatchField(patch, "security")) {
      delete extra.tls;
      delete extra.security;
      if (patch.security === "tls") extra.tls = "tls";
      else if (patch.security && patch.security !== "none") extra.security = patch.security;
    }
    if (hasPatchField(patch, "path")) patchObjectValue(extra, ["path"], patch.path);
    if (hasPatchField(patch, "host")) patchObjectValue(extra, ["host"], patch.host);
    if (hasPatchField(patch, "sni")) patchObjectValue(extra, ["sni"], patch.sni);
    if (hasPatchField(patch, "alpn")) patchObjectValue(extra, ["alpn"], patch.alpn);
    if (hasPatchField(patch, "flow")) patchObjectValue(extra, ["flow"], patch.flow);
    if (hasPatchField(patch, "fingerprint")) patchObjectValue(extra, ["fp", "fingerprint"], patch.fingerprint);
    if (hasPatchField(patch, "publicKey")) patchObjectValue(extra, ["pbk", "publicKey", "password"], patch.publicKey);
    if (hasPatchField(patch, "shortId")) patchObjectValue(extra, ["sid", "shortId"], patch.shortId);
    if (hasPatchField(patch, "spiderX")) patchObjectValue(extra, ["spx", "spiderX"], patch.spiderX);
    if (hasPatchField(patch, "allowInsecure")) patchObjectValue(extra, ["allowInsecure", "allowinsecure"], patch.allowInsecure);
    if (hasPatchField(patch, "grpcAuthority")) patchObjectValue(extra, ["authority"], patch.grpcAuthority);
    if (hasPatchField(patch, "headerType")) patchObjectValue(extra, ["type", "headerType"], patch.headerType);
    if (hasPatchField(patch, "vmessSecurity")) patchObjectValue(extra, ["scy", "encryption", "securityType"], patch.vmessSecurity);
    if (hasPatchField(patch, "vmessAlterId")) patchObjectValue(extra, ["aid"], patch.vmessAlterId);
    next.params = Object.keys(extra).length > 0 ? JSON.stringify(extra) : "";
    return buildConfiguration(next);
  }

  const search = new URLSearchParams(parsed.params);
  if (hasPatchField(patch, "network")) patchSearchValue(search, ["type"], patch.network === "tcp" ? "" : patch.network);
  if (hasPatchField(patch, "security")) patchSearchValue(search, ["security"], patch.security === "none" ? "" : patch.security);
  if (hasPatchField(patch, "path")) patchSearchValue(search, ["path"], patch.path);
  if (hasPatchField(patch, "host")) patchSearchValue(search, ["host"], patch.host);
  if (hasPatchField(patch, "sni")) patchSearchValue(search, ["sni"], patch.sni);
  if (hasPatchField(patch, "alpn")) patchSearchValue(search, ["alpn"], patch.alpn);
  if (hasPatchField(patch, "encryption")) patchSearchValue(search, ["encryption"], patch.encryption);
  if (hasPatchField(patch, "flow")) patchSearchValue(search, ["flow"], patch.flow);
  if (hasPatchField(patch, "fingerprint")) patchSearchValue(search, ["fp", "fingerprint"], patch.fingerprint, "fp");
  if (hasPatchField(patch, "publicKey")) patchSearchValue(search, ["pbk", "publicKey", "password"], patch.publicKey, "pbk");
  if (hasPatchField(patch, "shortId")) patchSearchValue(search, ["sid", "shortId"], patch.shortId, "sid");
  if (hasPatchField(patch, "spiderX")) patchSearchValue(search, ["spx", "spiderX"], patch.spiderX, "spx");
  if (hasPatchField(patch, "allowInsecure")) patchSearchValue(search, ["allowInsecure", "allowinsecure", "allow_insecure"], patch.allowInsecure ? "1" : "");
  if (hasPatchField(patch, "grpcAuthority")) patchSearchValue(search, ["authority"], patch.grpcAuthority);
  if (hasPatchField(patch, "headerType")) patchSearchValue(search, ["headerType", "header_type", "typeHeader"], patch.headerType);
  next.params = search.toString();
  return buildConfiguration(next);
}

export function createConfigurationFromXrayJSON(raw: string) {
  const parsed = parseXrayJSONConfiguration(formatXrayJSON(raw));
  if (!parsed) {
    throw new Error("Сначала заполните XRAY-JSON конфигурацию");
  }

  const draft = parsed.draft;
  if (draft.protocol === "vmess") {
    const vmessExtra: Record<string, unknown> = {};
    if (draft.network && draft.network !== "tcp") {
      vmessExtra.net = draft.network;
    }
    if (draft.security === "tls") {
      vmessExtra.tls = "tls";
    } else if (draft.security !== "none") {
      vmessExtra.security = draft.security;
    }
    if (draft.path.trim()) vmessExtra.path = draft.path.trim();
    if (draft.host.trim()) vmessExtra.host = draft.host.trim();
    if (draft.sni.trim()) vmessExtra.sni = draft.sni.trim();
    if (draft.alpn.trim()) vmessExtra.alpn = draft.alpn.trim();
    if (draft.flow?.trim()) vmessExtra.flow = draft.flow.trim();
    if (draft.fingerprint?.trim()) vmessExtra.fp = draft.fingerprint.trim();
    if (draft.publicKey?.trim()) vmessExtra.pbk = draft.publicKey.trim();
    if (draft.shortId?.trim()) vmessExtra.sid = draft.shortId.trim();
    if (draft.spiderX?.trim()) vmessExtra.spx = draft.spiderX.trim();
    if (draft.allowInsecure) vmessExtra.allowInsecure = "1";
    if (draft.network === "tcp" && draft.headerType?.trim()) vmessExtra.type = draft.headerType.trim();
    if (draft.grpcAuthority?.trim()) vmessExtra.authority = draft.grpcAuthority.trim();
    if (draft.vmessSecurity?.trim()) vmessExtra.scy = draft.vmessSecurity.trim();
    if (draft.vmessAlterId?.trim()) vmessExtra.aid = draft.vmessAlterId.trim();

    return buildConfiguration({
      protocol: draft.protocol,
      server: draft.server,
      port: draft.port,
      identifier: draft.identifier,
      params: Object.keys(vmessExtra).length > 0 ? JSON.stringify(vmessExtra, null, 2) : "",
      remark: draft.remark,
    });
  }

  const query = new URLSearchParams();
  if (draft.network && draft.network !== "tcp") {
    query.set("type", draft.network);
  }
  if (draft.security && draft.security !== "none") {
    query.set("security", draft.security);
  }
  if (draft.path.trim()) query.set("path", draft.path.trim());
  if (draft.host.trim()) query.set("host", draft.host.trim());
  if (draft.sni.trim()) query.set("sni", draft.sni.trim());
  if (draft.alpn.trim()) query.set("alpn", draft.alpn.trim());
  if (draft.encryption?.trim()) query.set("encryption", draft.encryption.trim());
  if (draft.flow?.trim()) query.set("flow", draft.flow.trim());
  if (draft.fingerprint?.trim()) query.set("fp", draft.fingerprint.trim());
  if (draft.publicKey?.trim()) query.set("pbk", draft.publicKey.trim());
  if (draft.shortId?.trim()) query.set("sid", draft.shortId.trim());
  if (draft.spiderX?.trim()) query.set("spx", draft.spiderX.trim());
  if (draft.allowInsecure) query.set("allowInsecure", "1");
  if (draft.network === "tcp" && draft.headerType?.trim()) query.set("headerType", draft.headerType.trim());
  if (draft.network === "grpc" && draft.grpcAuthority?.trim()) query.set("authority", draft.grpcAuthority.trim());

  return buildConfiguration({
    protocol: draft.protocol,
    server: draft.server,
    port: draft.port,
    identifier: draft.identifier,
    params: query.toString(),
    remark: draft.remark,
  });
}

export function isXrayJSONConfiguration(raw: string) {
  try {
    return Boolean(parseXrayJSONConfiguration(raw));
  } catch {
    return false;
  }
}

export function getConfigurationPlaceholder() {
  return [
    "vless://uuid@example.com:443?type=ws&security=tls#My Server",
    "vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6InV1aWQifQ==",
    "trojan://password@example.com:443?security=tls#Trojan Server",
  ].join("\n");
}

export function getXrayJSONPlaceholder() {
  return JSON.stringify(
    {
      log: {
        loglevel: "warning",
      },
      inbounds: [],
      outbounds: [
        {
          tag: "My Server",
          protocol: "vless",
          settings: {
            vnext: [
              {
                address: "example.com",
                port: 443,
                users: [
                  {
                    id: "00000000-0000-0000-0000-000000000000",
                    encryption: "none",
                  },
                ],
              },
            ],
          },
          streamSettings: {
            network: "ws",
            security: "tls",
            wsSettings: {
              path: "/",
              headers: {
                Host: "example.com",
              },
            },
            tlsSettings: {
              serverName: "example.com",
            },
          },
        },
      ],
    },
    null,
    2
  );
}
