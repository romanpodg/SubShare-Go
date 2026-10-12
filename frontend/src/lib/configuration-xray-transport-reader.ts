import type { XrayJSONNetwork } from "./configuration-types";
import {
  asArray,
  asRecord,
  parseHeaderType,
  textValue
} from "./configuration-values";

function commaArray(value: unknown) {
  return asArray(value).map(textValue).filter(Boolean).join(",");
}

function readTcp(stream: Record<string, unknown>) {
  const raw = asRecord(stream.rawSettings);
  const tcp = asRecord(stream.tcpSettings);
  const header = asRecord(raw?.header) ?? asRecord(tcp?.header);
  const headerType = parseHeaderType(header?.type);
  if (headerType !== "http") return {
    path: "",
    host: "",
    headerType
  };
  const request = asRecord(header?.request);
  const headers = asRecord(request?.headers);
  return {
    path: commaArray(request?.path),
    host: commaArray(headers?.Host ?? headers?.host),
    headerType
  };
}

export function readXrayTransport(stream: Record<string, unknown>, network: XrayJSONNetwork) {
  const empty = {
    path: "",
    host: "",
    grpcAuthority: "",
    headerType: ""
  };
  if (network === "tcp") return { ...empty, ...readTcp(stream) };
  if (network === "ws") {
    const settings = asRecord(stream.wsSettings);
    const headers = asRecord(settings?.headers);
    return {
      ...empty,
      path: textValue(settings?.path),
      host: textValue(headers?.Host) || textValue(headers?.host)
    };
  }
  if (network === "grpc") {
    const settings = asRecord(stream.grpcSettings);
    return {
      ...empty,
      path: textValue(settings?.serviceName),
      grpcAuthority: textValue(settings?.authority)
    };
  }
  const keys: Partial<Record<XrayJSONNetwork, string>> = { httpupgrade: "httpupgradeSettings", xhttp: "xhttpSettings" };
  const key = keys[network];
  if (!key) return empty;
  const settings = asRecord(stream[key]);
  return {
    ...empty,
    path: textValue(settings?.path),
    host: textValue(settings?.host)
  };
}
