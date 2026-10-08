import type { ConfigurationProtocol } from "../configuration";
import { serializeCorpusDocument, type PreservationCase } from "./configuration-preservation";

const identifier = "11111111-1111-4111-8111-111111111111";

function withSelectedOutbound(corpus: PreservationCase, changes: object) {
  return { ...corpus.document, outbounds: [
    corpus.document.outbounds[0],
    { ...corpus.document.outbounds[1], ...changes },
    ...corpus.document.outbounds.slice(2),
  ] };
}

function sourceOnlySettings(corpus: PreservationCase): Record<string, unknown> {
  const original = corpus.document.outbounds[1].settings;
  if (corpus.protocol === "trojan") {
    return { customSettings: original.customSettings, servers: original.servers };
  }
  return { customSettings: original.customSettings, vnext: original.vnext };
}

export function sourceOnlyConversionGolden(corpus: PreservationCase, target: ConfigurationProtocol) {
  const settings = sourceOnlySettings(corpus);
  const raw = serializeCorpusDocument(withSelectedOutbound(corpus, { settings }));
  const expectedSettings = { ...settings };
  if (target === "trojan") {
    expectedSettings.servers = [{ address: "edge.matrix.example", port: 443, password: identifier }];
  } else if (corpus.protocol === "trojan") {
    const user = target === "vless" ? { id: identifier, encryption: "none" } : { id: identifier, security: "auto" };
    // New vnext nodes receive their users before their endpoint properties.
    expectedSettings.vnext = [{ users: [user], address: "edge.matrix.example", port: 443 }];
  }
  return { raw, expected: serializeCorpusDocument(withSelectedOutbound(corpus, { protocol: target, settings: expectedSettings })) };
}

export const transportModes = {
  set: { path: "/new", host: "changed-host.example", authority: "changed-authority.example", header: "none" },
  clear: { path: undefined, host: undefined, authority: undefined, header: undefined },
} as const;
export type TransportMode = keyof typeof transportModes;

function expectedWsSettings(corpus: PreservationCase, mode: TransportMode) {
  const before = corpus.document.outbounds[1].streamSettings.wsSettings;
  const values = transportModes[mode];
  const headers: Record<string, unknown> = { ...before.headers, Host: values.host };
  if (mode === "clear") delete headers.host;
  return { ...before, path: values.path, headers };
}

function expectedTcpSettings(corpus: PreservationCase, mode: TransportMode) {
  const before = corpus.document.outbounds[1].streamSettings.tcpSettings;
  const values = transportModes[mode];
  return { ...before, header: { ...before.header, type: values.header, request: {
    ...before.header.request,
    path: values.path ? [values.path] : undefined,
    headers: { ...before.header.request.headers, Host: values.host ? [values.host] : undefined },
  } } };
}

function expectedTransportStream(corpus: PreservationCase, mode: TransportMode) {
  const before = corpus.document.outbounds[1].streamSettings;
  const values = transportModes[mode];
  switch (corpus.network) {
    case "ws": return { ...before, wsSettings: expectedWsSettings(corpus, mode) };
    case "tcp": return { ...before, tcpSettings: expectedTcpSettings(corpus, mode) };
    case "grpc": return { ...before, grpcSettings: { ...before.grpcSettings, serviceName: values.path, authority: values.authority } };
    case "httpupgrade": return { ...before, httpupgradeSettings: { ...before.httpupgradeSettings, path: values.path, host: values.host } };
    case "xhttp": return { ...before, xhttpSettings: { ...before.xhttpSettings, path: values.path, host: values.host } };
    // The current patcher ignores represented hints for H2 and QUIC.
    default: return before;
  }
}

export function transportGolden(corpus: PreservationCase, mode: TransportMode) {
  const values = transportModes[mode];
  return {
    patch: { path: values.path ?? "", host: values.host ?? "", grpcAuthority: values.authority ?? "", headerType: values.header ?? "" },
    expected: serializeCorpusDocument(withSelectedOutbound(corpus, { streamSettings: expectedTransportStream(corpus, mode) })),
  };
}

export function rawTcpGolden(corpus: PreservationCase, mode: TransportMode) {
  const before = corpus.document.outbounds[1].streamSettings;
  const sourceStream = { ...before, rawSettings: before.tcpSettings };
  const expectedStream = { ...sourceStream, rawSettings: expectedTcpSettings(corpus, mode) };
  return {
    raw: serializeCorpusDocument(withSelectedOutbound(corpus, { streamSettings: sourceStream })),
    expected: serializeCorpusDocument(withSelectedOutbound(corpus, { streamSettings: expectedStream })),
  };
}

export const realityAliasShapes = ["modern", "legacy", "both", "neither"] as const;
export type RealityAliasShape = (typeof realityAliasShapes)[number];

const expectedAliases = {
  modern: { publicKey: "updated" }, legacy: { password: "updated" },
  both: { publicKey: "updated", password: "updated" }, neither: { publicKey: "updated" },
};

export function realityAliasGolden(corpus: PreservationCase, shape: RealityAliasShape) {
  const before = corpus.document.outbounds[1].streamSettings;
  const sourceSettings: Record<string, unknown> = { ...before.realitySettings };
  if (shape === "legacy" || shape === "neither") delete sourceSettings.publicKey;
  if (shape === "modern" || shape === "neither") delete sourceSettings.password;
  return {
    raw: serializeCorpusDocument(withSelectedOutbound(corpus, { streamSettings: { ...before, realitySettings: sourceSettings } })),
    expected: serializeCorpusDocument(withSelectedOutbound(corpus, { streamSettings: { ...before, realitySettings: { ...sourceSettings, ...expectedAliases[shape] } } })),
  };
}
