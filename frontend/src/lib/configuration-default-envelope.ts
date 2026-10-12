function defaultInbound(tag: string, port: number, protocol: string, settings: Record<string, unknown>) {
  return {
    tag,
    port,
    listen: "127.0.0.1",
    protocol,
    settings,
    sniffing: {
      enabled: true,
      routeOnly: false,
      destOverride: [
        "http",
        "tls",
        "quic"
      ]
    }
  };
}

// Construct every nested default per call; each inbound owns its sniffing lists.
export function buildDefaultEnvelope(outbound: Record<string, unknown>) {
  return {
    dns: { servers: ["1.1.1.1", "1.0.0.1"], queryStrategy: "UseIP" },
    routing: {
      rules: [{
        type: "field",
        protocol: ["bittorrent"],
        outboundTag: "direct"
      }],
      domainMatcher: "hybrid",
      domainStrategy: "IPIfNonMatch"
    },
    inbounds: [defaultInbound("socks", 10808, "socks", { udp: true, auth: "noauth" }), defaultInbound("http", 10809, "http", { allowTransparent: false })],
    outbounds: [
      outbound,
      { tag: "direct", protocol: "freedom" },
      { tag: "block", protocol: "blackhole" }
    ],
  };
}
