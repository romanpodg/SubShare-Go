import type { ConfigurationProtocol, ParsedXrayJSONConfiguration } from "./configuration-types";
import {
  asArray,
  asRecord,
  textValue
} from "./configuration-values";
import { normalizeNetwork, parseSupportedProtocol } from "./configuration-discriminators";
import { readXrayConnection } from "./configuration-xray-connection-reader";
import { readXrayTransport } from "./configuration-xray-transport-reader";
import { readXraySecurity } from "./configuration-xray-security-reader";

function findSupportedOutbound(config: Record<string, unknown>) {
  const outbounds = asArray(config.outbounds);
  for (let index = 0;index < outbounds.length;index += 1) {
    const outbound = asRecord(outbounds[index]);
    if (!outbound) continue;
    const protocol = parseSupportedProtocol(textValue(outbound.protocol));
    if (protocol) return {
      outbound,
      outboundIndex: index,
      protocol
    };
  }
  throw new Error("Не найден outbound с протоколом vless/vmess/trojan");
}

function readRoot(raw: string) {
  let parsed: unknown;
  try { parsed = JSON.parse(raw); }
  catch { throw new Error("XRAY-JSON содержит ошибку синтаксиса"); }
  const root = asRecord(parsed);
  if (!root) throw new Error("XRAY-JSON должен быть JSON-объектом");
  return root;
}

function readDraft(outbound: Record<string, unknown>, protocol: ConfigurationProtocol) {
  const connection = readXrayConnection(outbound, protocol);
  const stream = asRecord(outbound.streamSettings) ?? {};
  const network = normalizeNetwork(textValue(stream.network));
  const transport = readXrayTransport(stream, network);
  const security = readXraySecurity(stream);
  // Keep the public draft's original key order as well as its values.
  return {
    protocol,
    server: connection.server,
    port: connection.port,
    identifier: connection.identifier,
    network,
    security: security.security,
    path: transport.path,
    host: transport.host,
    sni: security.sni,
    alpn: security.alpn,
    remark: textValue(outbound.tag),
    flow: connection.flow,
    encryption: connection.encryption,
    fingerprint: security.fingerprint,
    publicKey: security.publicKey,
    shortId: security.shortId,
    spiderX: security.spiderX,
    allowInsecure: security.allowInsecure,
    grpcAuthority: transport.grpcAuthority,
    headerType: transport.headerType,
    vmessSecurity: connection.vmessSecurity,
    vmessAlterId: connection.vmessAlterId,
  };
}

export function parseXrayJSONConfiguration(raw: string): ParsedXrayJSONConfiguration | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  const config = readRoot(trimmed);
  const { outbound, outboundIndex, protocol } = findSupportedOutbound(config);
  return {
    config,
    outboundIndex,
    draft: readDraft(outbound, protocol)
  };
}
