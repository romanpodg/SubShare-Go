import type {
  ConfigurationProtocol,
  XrayJSONNetwork,
  XrayJSONSecurity,
} from "../configuration";

export const corpusProtocols: ConfigurationProtocol[] = ["vless", "vmess", "trojan"];
const networks: XrayJSONNetwork[] = ["tcp", "ws", "grpc", "httpupgrade", "xhttp", "h2", "quic"];
const securities: XrayJSONSecurity[] = ["none", "tls", "reality"];
const primaryID = "11111111-1111-4111-8111-111111111111";
const extraID = "22222222-2222-4222-8222-222222222222";

function corpusDocument(protocol: ConfigurationProtocol, network: XrayJSONNetwork, security: XrayJSONSecurity) {
  const nodeAddress = protocol === "trojan" ? "dormant-node.example" : "edge.matrix.example";
  const serverAddress = protocol === "trojan" ? "edge.matrix.example" : "dormant-server.example";
  return {
    hugeToken: "big-number-slot",
    preciseToken: "precision-number-slot",
    escapedToken: "escape-slot",
    customTop: { future: true, ordered: ["first", "second"] },
    dns: { servers: ["9.9.9.9"], customDNS: true },
    routing: { rules: [{ type: "field", outboundTag: "direct" }] },
    inbounds: [{ protocol: "socks", tag: "inbound", customInbound: true }],
    outbounds: [
      { protocol: "freedom", tag: "direct", customDirect: true },
      {
        protocol, tag: "proxy", customOutbound: { future: true },
        settings: {
          customSettings: { future: true },
          vnext: [
            { address: nodeAddress, port: 443, customNode: true, users: [
              { id: primaryID, encryption: "none", security: "auto", alterId: 0, flow: "xtls-rprx-vision", customUser: true },
              { id: extraID, encryption: "none", customExtraUser: true },
            ] },
            { address: "extra-node.example", port: 8443, users: [{ id: extraID }], customExtraNode: true },
          ],
          servers: [
            { address: serverAddress, port: 443, password: primaryID, customServer: true },
            { address: "extra-server.example", port: 8443, password: extraID, customExtraServer: true },
          ],
        },
        streamSettings: {
          network, security, customStream: true,
          tcpSettings: { header: { type: "http", request: { path: ["/old"], headers: { Host: ["host.matrix.example"], "X-Future": ["keep"] }, customRequest: true } } },
          wsSettings: { path: "/old", headers: { Host: "host.matrix.example", host: "lower.matrix.example", "X-Future": "keep" }, customWs: true },
          grpcSettings: { serviceName: "old-service", authority: "authority.matrix.example", multiMode: false, customGrpc: true },
          httpupgradeSettings: { path: "/old", host: "host.matrix.example", customUpgrade: true },
          xhttpSettings: { path: "/old", host: "host.matrix.example", mode: "auto", extra: { future: true }, customXhttp: true },
          httpSettings: { path: "/old", host: ["host.matrix.example"], customHttp: true },
          quicSettings: { security: "none", key: "public", header: { type: "none" }, customQuic: true },
          tlsSettings: { serverName: "tls.matrix.example", alpn: ["h2"], allowInsecure: true, fingerprint: "chrome", customTLS: true },
          realitySettings: { serverName: "reality.matrix.example", publicKey: "public", password: "public", shortId: "0123456789", spiderX: "/spider", fingerprint: "chrome", customReality: true },
        },
      },
      { protocol: "blackhole", tag: "blocked", customBlocked: true },
      { protocol: "trojan", tag: "extra-proxy", settings: { servers: [{ address: "extra-proxy.example", port: 443, password: extraID, customExtraProxy: true }] } },
    ] as const,
  };
}

export function serializeCorpusDocument(document: object) {
  return JSON.stringify(document, null, 2)
    .replace('"big-number-slot"', "1e400")
    .replace('"precision-number-slot"', "900719925474099312345")
    .replace('"escape-slot"', '"\\u0061"');
}

function corpusCase(protocol: ConfigurationProtocol, network: XrayJSONNetwork, security: XrayJSONSecurity) {
  const document = corpusDocument(protocol, network, security);
  const raw = serializeCorpusDocument(document);
  return { name: `${protocol}/${network}/${security}`, protocol, network, security, raw, document };
}

export const preservationCorpus = corpusProtocols.flatMap((protocol) =>
  networks.flatMap((network) => securities.map((security) => corpusCase(protocol, network, security)))
);

export type PreservationCase = (typeof preservationCorpus)[number];
