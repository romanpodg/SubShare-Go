import { describe, expect, it } from "vitest";
import {
  parseXrayJSONConfiguration,
  patchXrayJSONConfiguration,
} from "./configuration";
import {
  DuplicateJSONKeyError,
  formatXrayJSON,
  inspectXrayJSONDocument,
} from "./xray-json-document";

function richVlessConfig() {
  return {
    log: { loglevel: "debug", customLog: true },
    dns: { servers: ["9.9.9.9"], customDns: "keep" },
    routing: { domainStrategy: "AsIs", rules: [{ type: "field", outboundTag: "direct" }] },
    policy: { levels: { "0": { handshake: 4 } } },
    inbounds: [{ tag: "socks", protocol: "socks", customInbound: { keep: true } }],
    customTopLevel: { nested: [1, 2, 3] },
    outbounds: [
      { tag: "direct", protocol: "freedom", customDirect: true },
      {
        tag: "proxy",
        protocol: "vless",
        customOutbound: { keep: true },
        settings: {
          customSettings: "keep",
          vnext: [
            {
              address: "one.example.com",
              port: 443,
              customNode: { keep: true },
              users: [
                {
                  id: "user-one",
                  encryption: "none",
                  flow: "xtls-rprx-vision",
                  customUser: { keep: true },
                },
                { id: "user-two", encryption: "none", customSecondUser: true },
              ],
            },
            {
              address: "two.example.com",
              port: 8443,
              users: [{ id: "node-two-user", encryption: "none" }],
              customSecondNode: true,
            },
          ],
          servers: [
            {
              address: "dormant-one.example.com",
              port: 9443,
              password: "dormant-one",
              customServer: { keep: true },
            },
            {
              address: "dormant-two.example.com",
              port: 10443,
              password: "dormant-two",
              customSecondServer: true,
            },
          ],
        },
        streamSettings: {
          network: "ws",
          security: "tls",
          customStream: { keep: true },
          wsSettings: {
            path: "/old",
            headers: { Host: "old.example.com", "X-Custom": "keep" },
            customWs: true,
          },
          grpcSettings: { serviceName: "dormant", customGrpc: true },
          tlsSettings: {
            serverName: "old.example.com",
            alpn: ["h2"],
            customTls: { keep: true },
          },
          realitySettings: { publicKey: "dormant-key", customReality: true },
        },
      },
      { tag: "block", protocol: "blackhole", customBlock: true },
    ],
  };
}

function parsedObject(raw: string) {
  return JSON.parse(raw) as ReturnType<typeof richVlessConfig>;
}

interface TestUser {
  id: string;
  encryption?: string;
  flow?: string;
  customUser?: { keep: boolean };
}

interface TestSelectedOutbound {
  tag: string;
  protocol: string;
  customOutbound: { keep: boolean };
  settings: {
    customSettings: string;
    vnext: Array<{
      address: string;
      port: number;
      users: TestUser[];
      customNode?: { keep: boolean };
      customSecondNode?: boolean;
    }>;
    servers: Array<{
      address: string;
      port: number;
      password: string;
      customServer?: { keep: boolean };
      customSecondServer?: boolean;
    }>;
  };
  streamSettings: {
    network: string;
    security: string;
    customStream: { keep: boolean };
    wsSettings: {
      path: string;
      headers: Record<string, string>;
      customWs: boolean;
    };
    grpcSettings: { serviceName: string; customGrpc: boolean };
    tlsSettings: {
      serverName: string;
      alpn: string[];
      customTls: { keep: boolean };
    };
    realitySettings: { publicKey: string; customReality: boolean };
  };
}

function selectedOutbound(config: ReturnType<typeof richVlessConfig>) {
  return config.outbounds[1] as unknown as TestSelectedOutbound;
}

describe("lossless Xray JSON formatting", () => {
  it("formats with two spaces without rewriting value tokens", () => {
    const raw = '{"outbounds":[],"unknown":{"huge":1e400,"negativeZero":-0,"escaped":"\\u0061"}}';
    const formatted = formatXrayJSON(raw);

    expect(formatted).toContain('\n  "outbounds": []');
    expect(formatted).toContain('"huge": 1e400');
    expect(formatted).toContain('"negativeZero": -0');
    expect(formatted).toContain('"escaped": "\\u0061"');
    expect(JSON.parse(formatted)).toEqual(JSON.parse(raw));
  });

  it("detects literal and escaped duplicate keys and refuses to format them", () => {
    const raw = '{"outbounds":[],"nested":{"a":1,"\\u0061":2}}';
    const inspection = inspectXrayJSONDocument(raw);

    expect(inspection.duplicateKeys).toEqual([
      { key: "a", path: ["nested", "a"] },
    ]);
    expect(() => formatXrayJSON(raw)).toThrow(DuplicateJSONKeyError);
  });
});

describe("patchXrayJSONConfiguration", () => {
  it("patches only the represented first node and preserves all other data", () => {
    const original = richVlessConfig();
    const raw = JSON.stringify(original).replace('"customTopLevel":', '"huge":1e400,"customTopLevel":');
    const result = patchXrayJSONConfiguration(raw, { server: "changed.example.com" });
    const patched = parsedObject(result);

    expect(result).toContain('"huge": 1e400');
    expect(patched.outbounds).toHaveLength(3);
    expect(patched.outbounds[0]).toEqual(original.outbounds[0]);
    expect(patched.outbounds[2]).toEqual(original.outbounds[2]);
    expect(patched.inbounds).toEqual(original.inbounds);
    expect(patched.routing).toEqual(original.routing);
    expect(patched.dns).toEqual(original.dns);
    expect(patched.policy).toEqual(original.policy);
    expect(patched.customTopLevel).toEqual(original.customTopLevel);

    const selected = selectedOutbound(patched);
    const originalSelected = selectedOutbound(original);
    expect(selected.settings.vnext).toHaveLength(2);
    expect(selected.settings.vnext[0].address).toBe("changed.example.com");
    expect(selected.settings.vnext[0].users).toEqual(originalSelected.settings.vnext[0].users);
    expect(selected.settings.vnext[1]).toEqual(originalSelected.settings.vnext[1]);
    expect(selected.settings.servers).toEqual(originalSelected.settings.servers);
    expect(selected.customOutbound).toEqual(originalSelected.customOutbound);
    expect(selected.streamSettings).toEqual(originalSelected.streamSettings);
  });

  it("preserves extra Trojan servers and unknown fields while editing the first password", () => {
    const original = richVlessConfig();
    const selected = selectedOutbound(original);
    selected.protocol = "trojan";
    const raw = JSON.stringify(original);
    const result = patchXrayJSONConfiguration(raw, { identifier: "new-password" });
    const patched = selectedOutbound(parsedObject(result));

    expect(patched.settings.servers).toHaveLength(2);
    expect(patched.settings.servers[0]).toMatchObject({
      password: "new-password",
      customServer: { keep: true },
    });
    expect(patched.settings.servers[1]).toEqual(selected.settings.servers[1]);
    expect(patched.settings.vnext).toEqual(selected.settings.vnext);
  });

  it("clears only an explicitly changed field", () => {
    const original = richVlessConfig();
    const result = patchXrayJSONConfiguration(JSON.stringify(original), { flow: "" });
    const user = selectedOutbound(parsedObject(result)).settings.vnext[0].users[0];

    expect(user).not.toHaveProperty("flow");
    expect(user).toMatchObject({
      id: "user-one",
      encryption: "none",
      customUser: { keep: true },
    });
  });

  it("edits transport without deleting security and clears only represented host aliases", () => {
    const original = richVlessConfig();
    const pathResult = patchXrayJSONConfiguration(JSON.stringify(original), { path: "/new" });
    const pathStream = selectedOutbound(parsedObject(pathResult)).streamSettings;
    const originalStream = selectedOutbound(original).streamSettings;

    expect(pathStream.wsSettings.path).toBe("/new");
    expect(pathStream.wsSettings.headers).toEqual(originalStream.wsSettings.headers);
    expect(pathStream.tlsSettings).toEqual(originalStream.tlsSettings);
    expect(pathStream.realitySettings).toEqual(originalStream.realitySettings);

    const clearResult = patchXrayJSONConfiguration(pathResult, { host: "" });
    const clearStream = selectedOutbound(parsedObject(clearResult)).streamSettings;
    expect(clearStream.wsSettings.headers).toEqual({ "X-Custom": "keep" });
    expect(clearStream.wsSettings.customWs).toBe(true);
    expect(clearStream.tlsSettings).toEqual(originalStream.tlsSettings);
  });

  it("edits security without deleting transport settings", () => {
    const original = richVlessConfig();
    const result = patchXrayJSONConfiguration(JSON.stringify(original), { sni: "new-sni.example.com" });
    const stream = selectedOutbound(parsedObject(result)).streamSettings;
    const originalStream = selectedOutbound(original).streamSettings;

    expect(stream.tlsSettings).toMatchObject({
      serverName: "new-sni.example.com",
      alpn: ["h2"],
      customTls: { keep: true },
    });
    expect(stream.wsSettings).toEqual(originalStream.wsSettings);
    expect(stream.grpcSettings).toEqual(originalStream.grpcSettings);
    expect(stream.customStream).toEqual(originalStream.customStream);
  });

  it("changes network and security discriminators without deleting dormant branches", () => {
    const original = richVlessConfig();
    const originalStream = selectedOutbound(original).streamSettings;
    const networkResult = patchXrayJSONConfiguration(JSON.stringify(original), {
      network: "grpc",
    });
    const networkStream = selectedOutbound(parsedObject(networkResult)).streamSettings;

    expect(networkStream.network).toBe("grpc");
    expect(networkStream.wsSettings).toEqual(originalStream.wsSettings);
    expect(networkStream.grpcSettings).toEqual(originalStream.grpcSettings);
    expect(networkStream.tlsSettings).toEqual(originalStream.tlsSettings);
    expect(networkStream.realitySettings).toEqual(originalStream.realitySettings);

    const securityResult = patchXrayJSONConfiguration(networkResult, {
      security: "reality",
    });
    const securityStream = selectedOutbound(parsedObject(securityResult)).streamSettings;
    expect(securityStream.security).toBe("reality");
    expect(securityStream.wsSettings).toEqual(originalStream.wsSettings);
    expect(securityStream.grpcSettings).toEqual(originalStream.grpcSettings);
    expect(securityStream.tlsSettings).toEqual(originalStream.tlsSettings);
    expect(securityStream.realitySettings).toEqual(originalStream.realitySettings);
  });

  it("keeps dormant branches and every extra entry during protocol conversion", () => {
    const original = richVlessConfig();
    const originalSelected = selectedOutbound(original);
    const originalVnext = structuredClone(originalSelected.settings.vnext);
    const result = patchXrayJSONConfiguration(JSON.stringify(original), { protocol: "trojan" });
    const selected = selectedOutbound(parsedObject(result));

    expect(selected.protocol).toBe("trojan");
    expect(selected.settings.vnext).toEqual(originalVnext);
    expect(selected.settings.servers).toHaveLength(2);
    expect(selected.settings.servers[0]).toMatchObject({
      address: "one.example.com",
      port: 443,
      password: "user-one",
      customServer: { keep: true },
    });
    expect(selected.settings.servers[1]).toEqual(originalSelected.settings.servers[1]);
    expect(parseXrayJSONConfiguration(result)?.draft.protocol).toBe("trojan");
  });
});
