import type { XrayJSONNetwork } from "./configuration-types";
import {
  asArray,
  asRecord,
  parseHeaderType,
  textValue
} from "./configuration-values";

function commaArray(value: unknown) { return asArray(value).map(textValue).filter(Boolean).join(","); }
function tcpHeader(stream: Record<string, unknown>) {
  const raw = asRecord(stream.rawSettings) ?? {};
  const tcp = asRecord(stream.tcpSettings) ?? {};
  return asRecord(raw.header) ?? asRecord(tcp.header) ?? {};
}
function readTcp(stream: Record<string, unknown>) {
  const header = tcpHeader(stream);
  const headerType = parseHeaderType(header.type);
  if (headerType !== "http") return {
    path: "",
    host: "",
    headerType
  };
  const request = asRecord(header.request) ?? {};
  const headers = asRecord(request.headers) ?? {};
  return {
    path: commaArray(request.path),
    host: commaArray(headers.Host ?? headers.host),
    headerType
  };
}
function readWebSocket(stream: Record<string, unknown>) {
  const settings = asRecord(stream.wsSettings) ?? {};
  const headers = asRecord(settings.headers) ?? {};
  return { path: textValue(settings.path), host: textValue(headers.Host) || textValue(headers.host) };
}
function readGrpc(stream: Record<string, unknown>) {
  const settings = asRecord(stream.grpcSettings) ?? {};
  return { path: textValue(settings.serviceName), grpcAuthority: textValue(settings.authority) };
}
function readHttpTransport(stream: Record<string, unknown>, network: XrayJSONNetwork) {
  const keys: Partial<Record<XrayJSONNetwork, string>> = { httpupgrade: "httpupgradeSettings", xhttp: "xhttpSettings" };
  const key = keys[network];
  if (!key) return {};
  const settings = asRecord(stream[key]) ?? {};
  return { path: textValue(settings.path), host: textValue(settings.host) };
}
export function readXrayTransport(stream: Record<string, unknown>, network: XrayJSONNetwork) {
  const empty = {
    path: "",
    host: "",
    grpcAuthority: "",
    headerType: ""
  };
  if (network === "tcp") return { ...empty, ...readTcp(stream) };
  if (network === "ws") return { ...empty, ...readWebSocket(stream) };
  if (network === "grpc") return { ...empty, ...readGrpc(stream) };
  return { ...empty, ...readHttpTransport(stream, network) };
}
