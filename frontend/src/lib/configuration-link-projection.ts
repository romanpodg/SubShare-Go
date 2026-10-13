import type { ConfigurationDraft, XrayJSONDraft } from "./configuration-types";
import {
  DEFAULT_NETWORK,
  DEFAULT_SECURITY,
  boolValue,
  parseHeaderType,
  textValue
} from "./configuration-values";
import { normalizeNetwork, normalizeSecurity } from "./configuration-discriminators";
import { parseVmessExtraParams } from "./configuration-link-codec";

function firstQueryValue(search: URLSearchParams, aliases: readonly string[]) {
  // URI aliases choose a raw truthy value before trimming. Whitespace in the
  // first alias therefore blocks later aliases, unlike VMess text coercion.
  return aliases.map(key => search.get(key)).find(Boolean) || "";
}

function queryText(search: URLSearchParams, ...aliases: string[]) {
  return firstQueryValue(search, aliases).trim();
}

function firstObjectText(object: Record<string, unknown>, ...aliases: string[]) {
  return aliases.map(key => textValue(object[key])).find(Boolean) || "";
}

function queryProjection(params: string) {
  const search = new URLSearchParams(params);
  return {
    network: normalizeNetwork(firstQueryValue(search, ["type"]) || DEFAULT_NETWORK),
    security: normalizeSecurity(firstQueryValue(search, ["security"]) || DEFAULT_SECURITY),
    path: queryText(search, "path"),
    host: queryText(search, "host"),
    sni: queryText(search, "sni"),
    alpn: queryText(search, "alpn"),
    encryption: queryText(search, "encryption"),
    flow: queryText(search, "flow"),
    fingerprint: queryText(search, "fp", "fingerprint"),
    publicKey: queryText(search, "pbk", "publicKey", "password"),
    shortId: queryText(search, "sid", "shortId"),
    spiderX: queryText(search, "spx", "spiderX"),
    allowInsecure: boolValue(firstQueryValue(search, [
      "allowInsecure",
      "allowinsecure",
      "allow_insecure"
    ])),
    headerType: parseHeaderType(firstQueryValue(search, [
      "headerType",
      "header_type",
      "typeHeader"
    ])),
    grpcAuthority: "",
    vmessSecurity: "",
    vmessAlterId: "",
  };
}

function vmessSecurity(params: Record<string, unknown>) {
  if (textValue(params.tls).toLowerCase() === "tls") return "tls";
  return normalizeSecurity(textValue(params.security) || DEFAULT_SECURITY);
}

function vmessProjection(raw: string) {
  const params = parseVmessExtraParams(raw);
  return {
    network: normalizeNetwork(firstObjectText(params, "net", "type") || DEFAULT_NETWORK),
    security: vmessSecurity(params),
    path: textValue(params.path),
    host: textValue(params.host),
    sni: textValue(params.sni),
    alpn: textValue(params.alpn),
    flow: textValue(params.flow),
    encryption: "",
    fingerprint: firstObjectText(params, "fp", "fingerprint"),
    publicKey: firstObjectText(params, "pbk", "publicKey", "password"),
    shortId: firstObjectText(params, "sid", "shortId"),
    spiderX: firstObjectText(params, "spx", "spiderX"),
    allowInsecure: boolValue(params.allowInsecure) || boolValue(params.allowinsecure),
    grpcAuthority: textValue(params.authority),
    headerType: parseHeaderType(params.type) || parseHeaderType(params.headerType),
    vmessSecurity: firstObjectText(params, "scy", "encryption", "securityType"),
    vmessAlterId: textValue(params.aid),
  };
}

export function projectLinkDraft(parsed: ConfigurationDraft): XrayJSONDraft {
  const fields = parsed.protocol === "vmess" ? vmessProjection(parsed.params) : queryProjection(parsed.params);
  return {
    protocol: parsed.protocol,
    server: parsed.server,
    port: parsed.port,
    identifier: parsed.identifier,
    ...fields,
    remark: parsed.remark
  };
}
