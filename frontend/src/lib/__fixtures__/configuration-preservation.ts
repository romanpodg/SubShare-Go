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
const dormantID = "33333333-3333-4333-8333-333333333333";

function lexicalTokens() {
  return { hugeNumber: "big-number-slot", preciseNumber: "precision-number-slot", escapedText: "escape-slot" };
}

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
        protocol, tag: "proxy", customOutbound: { future: true, ...lexicalTokens() },
        settings: {
          customSettings: { future: true },
          vnext: [
            { address: nodeAddress, port: protocol === "trojan" ? 9443 : 2443, customNode: { future: true, ...lexicalTokens() }, users: [
              { id: protocol === "trojan" ? dormantID : primaryID, encryption: "custom", security: "auto", alterId: 7, flow: "old-flow", customUser: { future: true, ...lexicalTokens() } },
              { id: extraID, encryption: "none", customExtraUser: true },
            ] },
            { address: "extra-node.example", port: 8443, users: [{ id: extraID }], customExtraNode: true },
          ],
          servers: [
            { address: serverAddress, port: protocol === "trojan" ? 2443 : 9443, password: protocol === "trojan" ? primaryID : dormantID, customServer: { future: true, ...lexicalTokens() } },
            { address: "extra-server.example", port: 8443, password: extraID, customExtraServer: true },
          ],
        },
        streamSettings: {
          network, security, customStream: true,
          tcpSettings: { customTcp: { future: true, ...lexicalTokens() }, header: { customHeader: { future: true, ...lexicalTokens() }, type: "http", request: { path: ["/old"], headers: { Host: ["host.matrix.example"], "X-Future": ["keep"] }, customRequest: { future: true, ...lexicalTokens() } } } },
          wsSettings: { path: "/old", headers: { Host: "host.matrix.example", host: "lower.matrix.example", "X-Future": "keep" }, customWs: { future: true, ...lexicalTokens() } },
          grpcSettings: { serviceName: "old-service", authority: "authority.matrix.example", multiMode: false, customGrpc: { future: true, ...lexicalTokens() } },
          httpupgradeSettings: { path: "/old", host: "host.matrix.example", customUpgrade: { future: true, ...lexicalTokens() } },
          xhttpSettings: { path: "/old", host: "host.matrix.example", mode: "auto", extra: { future: true, ...lexicalTokens() }, customXhttp: true },
          httpSettings: { path: "/old", host: ["host.matrix.example"], customHttp: { future: true, ...lexicalTokens() } },
          quicSettings: { security: "none", key: "public", header: { type: "none" }, customQuic: { future: true, ...lexicalTokens() } },
          tlsSettings: { serverName: "tls.matrix.example", alpn: ["h2"], allowInsecure: false, fingerprint: "chrome", customTLS: { future: true, ...lexicalTokens() } },
          realitySettings: { serverName: "reality.matrix.example", publicKey: "public", password: "public", shortId: "0123456789", spiderX: "/spider", fingerprint: "chrome", customReality: { future: true, ...lexicalTokens() } },
        },
      },
      { protocol: "blackhole", tag: "blocked", customBlocked: true },
      { protocol: "trojan", tag: "extra-proxy", settings: { servers: [{ address: "extra-proxy.example", port: 443, password: extraID, customExtraProxy: true }] } },
    ] as const,
  };
}

export function serializeCorpusDocument(document: object) {
  return JSON.stringify(document, null, 2)
    .replaceAll('"big-number-slot"', "1e400")
    .replaceAll('"precision-number-slot"', "900719925474099312345")
    .replaceAll('"escape-slot"', '"\\u0061"');
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
