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

function ensureJSONObject(
  raw: string,
  path: MutableJSONPath,
  currentValue: unknown
) {
  return asRecord(currentValue) ? raw : patchJSONPath(raw, path, {});
}

function ensureFirstJSONObject(
  raw: string,
  path: MutableJSONPath,
  currentValue: unknown
) {
  const entries = asArray(currentValue);
  if (entries.length === 0) {
    return patchJSONPath(raw, path, [{}]);
  }
  return asRecord(entries[0]) ? raw : patchJSONPath(raw, [...path, 0], {});
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

export function patchXrayJSONConfiguration(raw: string, patch: XrayJSONPatch) {
  let nextRaw = formatXrayJSON(raw);
  const parsed = parseXrayJSONConfiguration(nextRaw);
  if (!parsed) {
    throw new Error("Сначала заполните XRAY-JSON конфигурацию");
  }

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

  const outboundPath: MutableJSONPath = ["outbounds", parsed.outboundIndex];
  const outbound = asRecord(asArray(parsed.config.outbounds)[parsed.outboundIndex]) ?? {};
  const settings = asRecord(outbound.settings);
  const streamSettings = asRecord(outbound.streamSettings);
  const originalVnext = asArray(settings?.vnext);
  const originalVnextNode = asRecord(originalVnext[0]);
  const originalUsers = asArray(originalVnextNode?.users);
  const originalUser = asRecord(originalUsers[0]);

  let settingsReady = false;
  let connectionReady = false;
  const ensureSettings = () => {
    if (!settingsReady) {
      nextRaw = ensureJSONObject(nextRaw, [...outboundPath, "settings"], outbound.settings);
      settingsReady = true;
    }
  };
  const ensureConnection = () => {
    if (connectionReady) return;
    ensureSettings();
    if (protocol === "trojan") {
      nextRaw = ensureFirstJSONObject(
        nextRaw,
        [...outboundPath, "settings", "servers"],
        settings?.servers
      );
    } else {
      nextRaw = ensureFirstJSONObject(
        nextRaw,
        [...outboundPath, "settings", "vnext"],
        settings?.vnext
      );
      nextRaw = ensureFirstJSONObject(
        nextRaw,
        [...outboundPath, "settings", "vnext", 0, "users"],
        originalVnextNode?.users
      );
    }
    connectionReady = true;
  };

  const connectionBasePath = () =>
    protocol === "trojan"
      ? [...outboundPath, "settings", "servers", 0]
      : [...outboundPath, "settings", "vnext", 0];
  const userBasePath = () => [...outboundPath, "settings", "vnext", 0, "users", 0];

  if (hasPatchField(patch, "protocol")) {
    nextRaw = patchJSONPath(nextRaw, [...outboundPath, "protocol"], protocol);
    ensureConnection();
    const basePath = connectionBasePath();
    nextRaw = patchJSONPath(nextRaw, [...basePath, "address"], server);
    nextRaw = patchJSONPath(nextRaw, [...basePath, "port"], port);
    nextRaw = patchJSONPath(
      nextRaw,
      [...(protocol === "trojan" ? basePath : userBasePath()), protocol === "trojan" ? "password" : "id"],
      identifier
    );
    if (protocol === "vless" && !originalUser) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...userBasePath(), "encryption"],
        nextDraft.encryption?.trim() || "none"
      );
    } else if (protocol === "vmess" && !originalUser) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...userBasePath(), "security"],
        nextDraft.vmessSecurity?.trim() || "auto"
      );
    }
  }

  if (hasPatchField(patch, "server")) {
    ensureConnection();
    nextRaw = patchJSONPath(nextRaw, [...connectionBasePath(), "address"], server);
  }
  if (hasPatchField(patch, "port")) {
    ensureConnection();
    nextRaw = patchJSONPath(nextRaw, [...connectionBasePath(), "port"], port);
  }
  if (hasPatchField(patch, "identifier")) {
    ensureConnection();
    const path = protocol === "trojan"
      ? [...connectionBasePath(), "password"]
      : [...userBasePath(), "id"];
    nextRaw = patchJSONPath(nextRaw, path, identifier);
  }
  if (hasPatchField(patch, "remark")) {
    nextRaw = patchJSONPath(
      nextRaw,
      [...outboundPath, "tag"],
      optionalText(patch.remark)
    );
  }

  if (protocol !== "trojan" && (hasPatchField(patch, "flow") || hasPatchField(patch, "encryption") || hasPatchField(patch, "vmessSecurity") || hasPatchField(patch, "vmessAlterId"))) {
    ensureConnection();
  }
  if (protocol === "vless" && hasPatchField(patch, "flow")) {
    nextRaw = patchJSONPath(nextRaw, [...userBasePath(), "flow"], optionalText(patch.flow));
  }
  if (protocol === "vless" && hasPatchField(patch, "encryption")) {
    nextRaw = patchJSONPath(
      nextRaw,
      [...userBasePath(), "encryption"],
      optionalText(patch.encryption)
    );
  }
  if (protocol === "vmess" && hasPatchField(patch, "vmessSecurity")) {
    nextRaw = patchJSONPath(
      nextRaw,
      [...userBasePath(), "security"],
      optionalText(patch.vmessSecurity)
    );
  }
  if (protocol === "vmess" && hasPatchField(patch, "vmessAlterId")) {
    nextRaw = patchJSONPath(
      nextRaw,
      [...userBasePath(), "alterId"],
      alterIDValue(patch.vmessAlterId)
    );
  }

  let streamSettingsReady = false;
  const ensureStreamSettings = () => {
    if (!streamSettingsReady) {
      nextRaw = ensureJSONObject(
        nextRaw,
        [...outboundPath, "streamSettings"],
        outbound.streamSettings
      );
      streamSettingsReady = true;
    }
  };

  if (hasPatchField(patch, "network")) {
    ensureStreamSettings();
    nextRaw = patchJSONPath(nextRaw, [...outboundPath, "streamSettings", "network"], network);
  }
  if (hasPatchField(patch, "security")) {
    ensureStreamSettings();
    nextRaw = patchJSONPath(nextRaw, [...outboundPath, "streamSettings", "security"], security);
  }

  const transportFieldsChanged =
    hasPatchField(patch, "path") ||
    hasPatchField(patch, "host") ||
    hasPatchField(patch, "grpcAuthority") ||
    hasPatchField(patch, "headerType");

  if (transportFieldsChanged) {
    ensureStreamSettings();
  }

  if (network === "ws" && (hasPatchField(patch, "path") || hasPatchField(patch, "host"))) {
    const wsSettings = asRecord(streamSettings?.wsSettings);
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", "wsSettings"],
      streamSettings?.wsSettings
    );
    if (hasPatchField(patch, "path")) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", "wsSettings", "path"],
        optionalText(patch.path)
      );
    }
    if (hasPatchField(patch, "host")) {
      const headers = asRecord(wsSettings?.headers);
      nextRaw = ensureJSONObject(
        nextRaw,
        [...outboundPath, "streamSettings", "wsSettings", "headers"],
        wsSettings?.headers
      );
      const host = optionalText(patch.host);
      if (host) {
        const key = Object.prototype.hasOwnProperty.call(headers ?? {}, "Host")
          ? "Host"
          : Object.prototype.hasOwnProperty.call(headers ?? {}, "host")
            ? "host"
            : "Host";
        nextRaw = patchJSONPath(
          nextRaw,
          [...outboundPath, "streamSettings", "wsSettings", "headers", key],
          host
        );
      } else {
        nextRaw = patchJSONPath(
          nextRaw,
          [...outboundPath, "streamSettings", "wsSettings", "headers", "Host"],
          undefined
        );
        nextRaw = patchJSONPath(
          nextRaw,
          [...outboundPath, "streamSettings", "wsSettings", "headers", "host"],
          undefined
        );
      }
    }
  } else if (network === "grpc" && (hasPatchField(patch, "path") || hasPatchField(patch, "grpcAuthority"))) {
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", "grpcSettings"],
      streamSettings?.grpcSettings
    );
    if (hasPatchField(patch, "path")) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", "grpcSettings", "serviceName"],
        optionalText(patch.path)
      );
    }
    if (hasPatchField(patch, "grpcAuthority")) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", "grpcSettings", "authority"],
        optionalText(patch.grpcAuthority)
      );
    }
  } else if ((network === "httpupgrade" || network === "xhttp") && (hasPatchField(patch, "path") || hasPatchField(patch, "host"))) {
    const branch = network === "httpupgrade" ? "httpupgradeSettings" : "xhttpSettings";
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", branch],
      streamSettings?.[branch]
    );
    if (hasPatchField(patch, "path")) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", branch, "path"],
        optionalText(patch.path)
      );
    }
    if (hasPatchField(patch, "host")) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", branch, "host"],
        optionalText(patch.host)
      );
    }
  } else if (network === "tcp" && (hasPatchField(patch, "path") || hasPatchField(patch, "host") || hasPatchField(patch, "headerType"))) {
    const rawSettings = asRecord(streamSettings?.rawSettings);
    const tcpSettings = asRecord(streamSettings?.tcpSettings);
    const useRawSettings = Boolean(asRecord(rawSettings?.header));
    const branch = useRawSettings ? "rawSettings" : "tcpSettings";
    const branchSettings = useRawSettings ? rawSettings : tcpSettings;
    const header = asRecord(branchSettings?.header);
    const request = asRecord(header?.request);
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", branch],
      branchSettings
    );
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", branch, "header"],
      branchSettings?.header
    );
    if (hasPatchField(patch, "headerType")) {
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", branch, "header", "type"],
        optionalText(parseHeaderType(patch.headerType))
      );
    }
    if (hasPatchField(patch, "path") || hasPatchField(patch, "host")) {
      nextRaw = ensureJSONObject(
        nextRaw,
        [...outboundPath, "streamSettings", branch, "header", "request"],
        header?.request
      );
    }
    if (hasPatchField(patch, "path")) {
      const paths = splitCommaValues(patch.path ?? "");
      nextRaw = patchJSONPath(
        nextRaw,
        [...outboundPath, "streamSettings", branch, "header", "request", "path"],
        paths.length > 0 ? paths : undefined
      );
    }
    if (hasPatchField(patch, "host")) {
      const headers = asRecord(request?.headers);
      nextRaw = ensureJSONObject(
        nextRaw,
        [...outboundPath, "streamSettings", branch, "header", "request", "headers"],
        request?.headers
      );
      const hosts = splitCommaValues(patch.host ?? "");
      if (hosts.length > 0) {
        const key = Object.prototype.hasOwnProperty.call(headers ?? {}, "Host")
          ? "Host"
          : Object.prototype.hasOwnProperty.call(headers ?? {}, "host")
            ? "host"
            : "Host";
        nextRaw = patchJSONPath(
          nextRaw,
          [...outboundPath, "streamSettings", branch, "header", "request", "headers", key],
          hosts
        );
      } else {
        nextRaw = patchJSONPath(
          nextRaw,
          [...outboundPath, "streamSettings", branch, "header", "request", "headers", "Host"],
          undefined
        );
        nextRaw = patchJSONPath(
          nextRaw,
          [...outboundPath, "streamSettings", branch, "header", "request", "headers", "host"],
          undefined
        );
      }
    }
  }

  const securityFieldsChanged =
    hasPatchField(patch, "sni") ||
    hasPatchField(patch, "alpn") ||
    hasPatchField(patch, "allowInsecure") ||
    hasPatchField(patch, "fingerprint") ||
    hasPatchField(patch, "publicKey") ||
    hasPatchField(patch, "shortId") ||
    hasPatchField(patch, "spiderX");

  if (securityFieldsChanged) {
    ensureStreamSettings();
  }

  if (security === "tls" && securityFieldsChanged) {
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", "tlsSettings"],
      streamSettings?.tlsSettings
    );
    const tlsPath = [...outboundPath, "streamSettings", "tlsSettings"];
    if (hasPatchField(patch, "sni")) {
      nextRaw = patchJSONPath(nextRaw, [...tlsPath, "serverName"], optionalText(patch.sni));
    }
    if (hasPatchField(patch, "alpn")) {
      const values = buildALPNValues(patch.alpn ?? "");
      nextRaw = patchJSONPath(nextRaw, [...tlsPath, "alpn"], values.length > 0 ? values : undefined);
    }
    if (hasPatchField(patch, "allowInsecure")) {
      nextRaw = patchJSONPath(nextRaw, [...tlsPath, "allowInsecure"], patch.allowInsecure ? true : undefined);
    }
    if (hasPatchField(patch, "fingerprint")) {
      nextRaw = patchJSONPath(nextRaw, [...tlsPath, "fingerprint"], optionalText(patch.fingerprint));
    }
  } else if (security === "reality" && securityFieldsChanged) {
    const realitySettings = asRecord(streamSettings?.realitySettings);
    nextRaw = ensureJSONObject(
      nextRaw,
      [...outboundPath, "streamSettings", "realitySettings"],
      streamSettings?.realitySettings
    );
    const realityPath = [...outboundPath, "streamSettings", "realitySettings"];
    if (hasPatchField(patch, "sni")) {
      nextRaw = patchJSONPath(nextRaw, [...realityPath, "serverName"], optionalText(patch.sni));
    }
    if (hasPatchField(patch, "fingerprint")) {
      nextRaw = patchJSONPath(nextRaw, [...realityPath, "fingerprint"], optionalText(patch.fingerprint));
    }
    if (hasPatchField(patch, "publicKey")) {
      const value = optionalText(patch.publicKey);
      if (value) {
        const hasPublicKey = Object.prototype.hasOwnProperty.call(realitySettings ?? {}, "publicKey");
        const hasPassword = Object.prototype.hasOwnProperty.call(realitySettings ?? {}, "password");
        if (hasPublicKey || !hasPassword) {
          nextRaw = patchJSONPath(nextRaw, [...realityPath, "publicKey"], value);
        }
        if (hasPassword) {
          nextRaw = patchJSONPath(nextRaw, [...realityPath, "password"], value);
        }
      } else {
        nextRaw = patchJSONPath(nextRaw, [...realityPath, "publicKey"], undefined);
        nextRaw = patchJSONPath(nextRaw, [...realityPath, "password"], undefined);
      }
    }
    if (hasPatchField(patch, "shortId")) {
      nextRaw = patchJSONPath(nextRaw, [...realityPath, "shortId"], optionalText(patch.shortId));
    }
    if (hasPatchField(patch, "spiderX")) {
      nextRaw = patchJSONPath(nextRaw, [...realityPath, "spiderX"], optionalText(patch.spiderX));
    }
  }

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
