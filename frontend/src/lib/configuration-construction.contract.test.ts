import {
  describe,
  expect,
  it
} from "vitest";
import {
  buildXrayJSONConfiguration,
  createConfigurationFromXrayJSON,
  createXrayJSONFromConfiguration,
  parseXrayJSONConfiguration,
  type XrayJSONDraft
} from "./configuration";
import { buildDefaultEnvelope } from "./configuration-default-envelope";
import { buildDefaultTransport } from "./configuration-default-transport";

const draft: XrayJSONDraft = {
  protocol: "vless",
  server: " edge.example ",
  port: "443tail",
  identifier: " id ",
  network: "tcp",
  security: "none",
  path: "",
  host: "",
  sni: "",
  alpn: "",
  remark: ""
};
const object = (value: XrayJSONDraft) => JSON.parse(buildXrayJSONConfiguration(value));
const first = (config: ReturnType<typeof object>) => config.outbounds[0];

describe("configuration construction contracts", () => {
  it("retains literal envelope defaults and outbound order", () => {
    const config = object(draft);
    expect(config.dns).toEqual({ servers: ["1.1.1.1", "1.0.0.1"], queryStrategy: "UseIP" });
    expect(config.routing).toEqual({
      rules: [{
        type: "field",
        protocol: ["bittorrent"],
        outboundTag: "direct"
      }],
      domainMatcher: "hybrid",
      domainStrategy: "IPIfNonMatch"
    });
    expect(config.inbounds.map((entry: { tag: string; port: number }) => [entry.tag, entry.port])).toEqual([["socks", 10808], ["http", 10809]]);
    expect(config.outbounds).toEqual([
      {
        tag: "proxy",
        protocol: "vless",
        settings: {
          vnext: [{
            address: "edge.example",
            port: 443,
            users: [{ id: "id", encryption: "none" }]
          }]
        },
        streamSettings: {
          network: "tcp",
          security: "none",
          tcpSettings: {}
        }
      },
      { tag: "direct", protocol: "freedom" },
      { tag: "block", protocol: "blackhole" }
    ]);
    expect(Object.keys(config)).toEqual([
      "dns",
      "routing",
      "inbounds",
      "outbounds"
    ]);
    expect(Object.keys(first(config))).toEqual([
      "tag",
      "protocol",
      "settings",
      "streamSettings"
    ]);
  });

  it.each([
    ["0", 0],
    ["12tail", 12],
    ["-1", "-1"],
    ["invalid", "invalid"]
  ])("retains VMess alter-ID %s", (value, expected) => {
    expect(first(object({
      ...draft,
      protocol: "vmess",
      vmessAlterId: String(value)
    })).settings.vnext[0].users).toEqual([{
      id: "id",
      security: "auto",
      alterId: expected
    }]);
  });

  it("retains TCP/raw headers and trimmed lists", () => {
    const input = {
      ...draft,
      headerType: "http",
      path: " /one, , /two ",
      host: " a.example, b.example "
    };
    const a = object(input);
    expect(first(a).streamSettings).toEqual({
      network: "tcp",
      security: "none",
      tcpSettings: { header: { type: "http", request: { path: ["/one", "/two"], headers: { Host: ["a.example", "b.example"] } } } },
      rawSettings: { header: { type: "http", request: { path: ["/one", "/two"], headers: { Host: ["a.example", "b.example"] } } } }
    });
  });

  it("constructs independent nested envelope defaults", () => {
    const a = buildDefaultEnvelope({});
    const b = buildDefaultEnvelope({});
    a.dns.servers.push("changed");
    a.inbounds[0].sniffing.destOverride.push("changed");
    expect(b.dns.servers).toEqual(["1.1.1.1", "1.0.0.1"]);
    expect(b.inbounds[0].sniffing.destOverride).toEqual([
      "http",
      "tls",
      "quic"
    ]);
    expect(a.inbounds[1].sniffing.destOverride).toEqual([
      "http",
      "tls",
      "quic"
    ]);
  });

  it("shares TCP/raw header aliases within one build and isolates subsequent builds", () => {
    const input = {
      ...draft,
      headerType: "http",
      path: "/one"
    };
    type Transport = { tcpSettings: { header: { request: { path: string[] } } }; rawSettings: { header: { request: { path: string[] } } } };
    const a = buildDefaultTransport(input) as Transport;
    expect(a.tcpSettings.header).toBe(a.rawSettings.header);
    a.rawSettings.header.request.path.push("changed");
    const b = buildDefaultTransport(input) as Transport;
    expect(b.rawSettings.header.request.path).toEqual(["/one"]);
    expect(b.rawSettings.header).not.toBe(a.rawSettings.header);
  });

  it("retains Reality aliases and TLS false omission", () => {
    expect(first(object({
      ...draft,
      security: "reality",
      sni: " tls.example ",
      publicKey: " key ",
      shortId: " 01 ",
      spiderX: " / ",
      fingerprint: " chrome "
    })).streamSettings.realitySettings).toEqual({
      serverName: "tls.example",
      show: false,
      fingerprint: "chrome",
      publicKey: "key",
      password: "key",
      shortId: "01",
      spiderX: "/"
    });
    expect(first(object({
      ...draft,
      security: "tls",
      allowInsecure: false
    })).streamSettings.tlsSettings).toEqual({ serverName: "" });
  });

  it("retains first raw truthy URI alias before trimming and ignored URI authority", () => {
    const config = JSON.parse(createXrayJSONFromConfiguration("vless://id@edge.example:443?type=grpc&fp=%20%20&fingerprint=chrome&authority=ignored"));
    expect(first(config).streamSettings).toEqual({
      network: "grpc",
      security: "none",
      grpcSettings: { serviceName: "" }
    });
    expect(parseXrayJSONConfiguration(JSON.stringify(config))?.draft.fingerprint).toBe("");
  });

  it("retains URI conversion query order and explicit network-specific omissions", () => {
    const raw = '{"outbounds":[{"protocol":"vless","tag":"Name","settings":{"vnext":[{"address":"edge.example","port":443,"users":[{"id":"id","encryption":"none","flow":"flow"}]}]},"streamSettings":{"network":"grpc","security":"tls","grpcSettings":{"serviceName":"svc","authority":"auth"},"tlsSettings":{"serverName":"tls.example","alpn":["h2","http/1.1"],"fingerprint":"chrome","allowInsecure":true}}}]}';
    expect(createConfigurationFromXrayJSON(raw)).toBe("vless://id@edge.example:443?type=grpc&security=tls&path=svc&sni=tls.example&alpn=h2%2Chttp%2F1.1&encryption=none&flow=flow&fp=chrome&allowInsecure=1&authority=auth#Name");
  });
});
