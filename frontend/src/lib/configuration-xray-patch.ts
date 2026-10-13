import type {
  ParsedXrayJSONConfiguration,
  XrayJSONDraft,
  XrayJSONPatch
} from "./configuration-types";
import { DEFAULT_PORT, asRecord, asArray, toPortNumber, parseHeaderType, splitCommaValues } from "./configuration-values";
import { normalizeProtocol, normalizeNetwork, normalizeSecurity } from "./configuration-discriminators";
import { parseXrayJSONConfiguration } from "./configuration-xray-reader";
import { appendSecurityPlan } from "./configuration-security-plan";
import { hasPatchField, optionalText } from "./configuration-patch-values";
import { formatXrayJSON, modifyXrayJSONPath } from "./xray-json-document";

type MutableJSONPath = Array<string | number>;


function patchJSONPath(
  raw: string,
  path: MutableJSONPath,
  value: unknown
) {
  return modifyXrayJSONPath(raw, path, value);
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
  if (context.security === "none") return;
  const branch = context.security === "tls" ? "tlsSettings" : "realitySettings";
  const path = [...context.outboundPath, "streamSettings", branch];
  const value = context.streamSettings?.[branch];
  appendSecurityPlan({
    mode: context.security,
    patch: context.patch,
    path,
    original: asRecord(value),
    initialize: () => appendObjectInitialization(context.operations, path, value),
    append: (editPath, editValue) => appendDocumentEdit(context.operations, editPath, editValue),
  });
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
