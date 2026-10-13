import {
  asArray,
  asRecord,
  boolValue,
  textValue
} from "./configuration-values";
import { normalizeSecurity } from "./configuration-discriminators";

function securityMode(stream: Record<string, unknown>, tls: Record<string, unknown>, reality: Record<string, unknown>) {
  const mode = normalizeSecurity(textValue(stream.security));
  if (mode !== "none") return mode;
  if (Object.keys(reality).length > 0) return "reality";
  if (Object.keys(tls).length > 0) return "tls";
  return mode;
}
function tlsProjection(tls: Record<string, unknown>) {
  return {
    sni: textValue(tls.serverName),
    alpn: asArray(tls.alpn).map(textValue).filter(Boolean).join(","),
    allowInsecure: boolValue(tls.allowInsecure),
    fingerprint: textValue(tls.fingerprint)
  };
}
function realityProjection(reality: Record<string, unknown>) {
  return {
    sni: textValue(reality.serverName),
    fingerprint: textValue(reality.fingerprint),
    publicKey: textValue(reality.publicKey) || textValue(reality.password),
    shortId: textValue(reality.shortId),
    spiderX: textValue(reality.spiderX)
  };
}
export function readXraySecurity(stream: Record<string, unknown>) {
  const tls = asRecord(stream.tlsSettings) ?? {};
  const reality = asRecord(stream.realitySettings) ?? {};
  const security = securityMode(stream, tls, reality);
  const empty = {
    security,
    sni: "",
    alpn: "",
    fingerprint: "",
    publicKey: "",
    shortId: "",
    spiderX: "",
    allowInsecure: false
  };
  if (security === "tls") return { ...empty, ...tlsProjection(tls) };
  if (security === "reality") return { ...empty, ...realityProjection(reality) };
  return empty;
}
