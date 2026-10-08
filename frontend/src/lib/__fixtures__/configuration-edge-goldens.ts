import { preservationCorpus } from "./configuration-preservation";
import { fixtureGolden, type FixtureChange } from "./configuration-field-goldens";
import type { XrayJSONPatch } from "../configuration";

const base = ["outbounds", 1] as const;
const stream = [...base, "streamSettings"];
const sampled = preservationCorpus.filter(({ network, security }) => network === "tcp" && security === "tls");
const identity = "44444444-4444-4444-8444-444444444444";

function change(path: Array<string | number>, value: unknown): FixtureChange { return { path, value }; }

const nestedParents = [
  { name: "ws headers", network: "ws", parent: [...stream, "wsSettings", "headers"], target: [...stream, "wsSettings", "headers", "Host"], value: "edge-host.example", patch: { host: "edge-host.example" } },
  ...["header", "request", "headers"].map((parent) => {
    const header = [...stream, "tcpSettings", "header"];
    const path = parent === "header" ? header : parent === "request" ? [...header, "request"] : [...header, "request", "headers"];
    return { name: `tcp ${parent}`, network: "tcp", parent: path, target: [...header, "request", "headers", "Host"], value: ["edge-host.example"], patch: { host: "edge-host.example" } };
  }),
] as const;

export const partialTransportGoldens = sampled.flatMap((corpus) => nestedParents.map((entry) => ({
  name: `${corpus.protocol}/${entry.name}`, patch: entry.patch as XrayJSONPatch,
  ...fixtureGolden(corpus, [change([...stream, "network"], entry.network), change([...entry.parent], undefined)], [change([...entry.target], entry.value)]),
})));

const malformedParents = [
  { name: "stream", path: stream, patch: { network: "ws", security: "tls", path: "/edge", sni: "edge-sni.example" }, outcomes: [change([...stream, "network"], "ws"), change([...stream, "security"], "tls"), change([...stream, "wsSettings", "path"], "/edge"), change([...stream, "tlsSettings", "serverName"], "edge-sni.example")] },
  { name: "ws", path: [...stream, "wsSettings"], patch: { network: "ws", path: "/edge" }, outcomes: [change([...stream, "network"], "ws"), change([...stream, "wsSettings", "path"], "/edge")] },
  { name: "tls", path: [...stream, "tlsSettings"], patch: { sni: "edge-sni.example" }, outcomes: [change([...stream, "tlsSettings", "serverName"], "edge-sni.example")] },
] as const;

export const nonObjectGoldens = sampled.flatMap((corpus) => malformedParents.flatMap((entry) => [null, [], "invalid", 7].map((value) => ({
  name: `${corpus.protocol}/${entry.name}/${JSON.stringify(value)}`, patch: entry.patch as XrayJSONPatch,
  ...fixtureGolden(corpus, [change([...entry.path], value)], [...entry.outcomes]),
}))));

export const lowercaseHostGoldens = sampled.flatMap((corpus) => ["ws", "tcp"].flatMap((network) => [false, true].map((clear) => {
  const headers = network === "ws" ? [...stream, "wsSettings", "headers"] : [...stream, "tcpSettings", "header", "request", "headers"];
  const oldValue = network === "ws" ? "lower.example" : ["lower.example"];
  const nextValue = clear ? undefined : network === "ws" ? "edge-host.example" : ["edge-host.example"];
  return { name: `${corpus.protocol}/${network}/${clear ? "clear" : "set"}`, patch: { host: clear ? "" : "edge-host.example" },
    ...fixtureGolden(corpus, [change([...stream, "network"], network), change([...headers, "Host"], undefined), change([...headers, "host"], oldValue)], [change([...headers, "host"], nextValue)]),
  };
})));

export const combinedModeGoldens = sampled.flatMap((corpus) => [
  { name: "grpc path", patch: { network: "grpc", path: "new-service" }, changes: [change([...stream, "network"], "grpc"), change([...stream, "grpcSettings", "serviceName"], "new-service")] },
  { name: "ws path", patch: { network: "ws", path: "/edge" }, changes: [change([...stream, "network"], "ws"), change([...stream, "wsSettings", "path"], "/edge")] },
  { name: "reality key", patch: { security: "reality", publicKey: "updated" }, changes: [change([...stream, "security"], "reality"), change([...stream, "realitySettings", "publicKey"], "updated"), change([...stream, "realitySettings", "password"], "updated")] },
  { name: "none ignores SNI", patch: { security: "none", sni: "ignored.example" }, changes: [change([...stream, "security"], "none")] },
].map((entry) => ({ name: `${corpus.protocol}/${entry.name}`, patch: entry.patch as XrayJSONPatch, ...fixtureGolden(corpus, [], entry.changes) })));

export const combinedProtocolGoldens = sampled.flatMap((corpus) => (["vless", "vmess", "trojan"] as const).filter((target) => target !== corpus.protocol).map((target) => {
  const endpoint = target === "trojan" ? [...base, "settings", "servers", 0] : [...base, "settings", "vnext", 0];
  const credential = target === "trojan" ? [...endpoint, "password"] : [...endpoint, "users", 0, "id"];
  return { name: `${corpus.protocol}/${target}`, patch: { protocol: target, server: "combined.example", port: "7443", identifier: identity },
    ...fixtureGolden(corpus, [], [change([...base, "protocol"], target), change([...endpoint, "address"], "combined.example"), change([...endpoint, "port"], 7443), change(credential, identity)]),
  };
}));

export const trueNoneGoldens = sampled.map((corpus) => ({ name: corpus.protocol,
  ...fixtureGolden(corpus, [change([...stream, "security"], "none"), change([...stream, "tlsSettings"], undefined), change([...stream, "realitySettings"], undefined)], []),
}));

export const trueNoneTransitions = sampled.flatMap((corpus) => [
  { name: "tls", patch: { security: "tls", sni: "edge-sni.example" }, changes: [change([...stream, "security"], "tls"), change([...stream, "tlsSettings", "serverName"], "edge-sni.example")] },
  { name: "reality", patch: { security: "reality", publicKey: "updated" }, changes: [change([...stream, "security"], "reality"), change([...stream, "realitySettings", "publicKey"], "updated")] },
].map((entry) => ({ name: `${corpus.protocol}/${entry.name}`, patch: entry.patch as XrayJSONPatch,
  ...fixtureGolden(corpus, [change([...stream, "security"], "none"), change([...stream, "tlsSettings"], undefined), change([...stream, "realitySettings"], undefined)], entry.changes),
})));

export const missingConnectionGoldens = sampled.flatMap((corpus) => {
  const endpoint = corpus.protocol === "trojan" ? [...base, "settings", "servers", 0] : [...base, "settings", "vnext", 0];
  const credential = corpus.protocol === "trojan" ? [...endpoint, "password"] : [...endpoint, "users", 0, "id"];
  return [undefined, null, [], "invalid"].map((shape) => {
    const node = corpus.protocol === "trojan" ? { address: "combined.example", port: 7443, password: identity } : { users: [{ id: identity }], address: "combined.example", port: 7443 };
    const branch = corpus.protocol === "trojan" ? "servers" : "vnext";
    return { name: `${corpus.protocol}/settings/${JSON.stringify(shape)}`, patch: { server: "combined.example", port: "7443", identifier: identity },
      ...fixtureGolden(corpus, [change([...base, "settings"], shape)], [change([...base, "settings"], { [branch]: [node] })]),
    };
  }).concat(corpus.protocol === "trojan" ? [] : [{ name: `${corpus.protocol}/users`, patch: { server: "combined.example", port: "7443", identifier: identity },
    ...fixtureGolden(corpus, [change([...endpoint, "users"], undefined)], [change([...endpoint, "users"], [{ id: identity }]), change([...endpoint, "address"], "combined.example"), change([...endpoint, "port"], 7443), change(credential, identity)]),
  }]);
});

export const commaListGoldens = sampled.flatMap((corpus) => [
  { name: "tcp paths", patch: { path: " /one , /two " }, changes: [change([...stream, "tcpSettings", "header", "request", "path"], ["/one", "/two"])] },
  { name: "tcp hosts", patch: { host: " one.example , two.example " }, changes: [change([...stream, "tcpSettings", "header", "request", "headers", "Host"], ["one.example", "two.example"])] },
  { name: "tls alpn", patch: { alpn: " h2 , http/1.1 " }, changes: [change([...stream, "tlsSettings", "alpn"], ["h2", "http/1.1"])] },
].map((entry) => ({ name: `${corpus.protocol}/${entry.name}`, patch: entry.patch as XrayJSONPatch, ...fixtureGolden(corpus, [], entry.changes) })));
