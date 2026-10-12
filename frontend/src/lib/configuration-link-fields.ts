import type { XrayJSONDraft } from "./configuration-types";

export type TextDraftField = Exclude<keyof XrayJSONDraft, "allowInsecure">;
export type LinkFieldSpec = readonly [TextDraftField, readonly string[]];

// These lists are ordered wire policies. URI and VMess differ in represented
// fields and alias sets, so each format keeps its own explicit specification.
export const vmessTextFields: readonly LinkFieldSpec[] = [
  ["path", ["path"]],
  ["host", ["host"]],
  ["sni", ["sni"]],
  ["alpn", ["alpn"]],
  ["flow", ["flow"]],
  ["fingerprint", ["fp", "fingerprint"]],
  ["publicKey", [
    "pbk",
    "publicKey",
    "password"
  ]],
  ["shortId", ["sid", "shortId"]],
  ["spiderX", ["spx", "spiderX"]],
];
export const uriTextFields: readonly LinkFieldSpec[] = [
  ["path", ["path"]],
  ["host", ["host"]],
  ["sni", ["sni"]],
  ["alpn", ["alpn"]],
  ["encryption", ["encryption"]],
  ["flow", ["flow"]],
  ["fingerprint", ["fp", "fingerprint"]],
  ["publicKey", [
    "pbk",
    "publicKey",
    "password"
  ]],
  ["shortId", ["sid", "shortId"]],
  ["spiderX", ["spx", "spiderX"]],
];
export const vmessTrailingFields: readonly LinkFieldSpec[] = [
  ["grpcAuthority", ["authority"]],
  ["headerType", ["type", "headerType"]],
  ["vmessSecurity", [
    "scy",
    "encryption",
    "securityType"
  ]],
  ["vmessAlterId", ["aid"]],
];
export const uriTrailingFields: readonly LinkFieldSpec[] = [
  ["grpcAuthority", ["authority"]], ["headerType", [
    "headerType",
    "header_type",
    "typeHeader"
  ]],
];
