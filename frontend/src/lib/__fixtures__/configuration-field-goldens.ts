import type { XrayJSONPatch } from "../configuration";
import { preservationCorpus, serializeCorpusDocument, type PreservationCase } from "./configuration-preservation";

type Path = Array<string | number>;
export type FixtureChange = { path: Path; value: unknown };
type Change = FixtureChange;
type Model = Record<string | number, unknown>;

// This oracle changes plain fixture objects, never production JSON tokens.
// Lexical marker strings survive cloning and are encoded independently later.
function applyModelChange(root: object, change: Change) {
  let parent = root as Model;
  for (let index = 0; index < change.path.length - 1; index += 1) {
    const segment = change.path[index];
    if (!isFixtureContainer(parent[segment], typeof change.path[index + 1] === "number")) parent[segment] = typeof change.path[index + 1] === "number" ? [] : {};
    parent = parent[segment] as Model;
  }
  const last = change.path[change.path.length - 1];
  if (change.value === undefined) delete parent[last];
  else parent[last] = change.value;
}

function isFixtureContainer(value: unknown, array: boolean) {
  if (array) return Array.isArray(value);
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function fixtureGolden(corpus: PreservationCase, sourceChanges: Change[], expectedChanges: Change[]) {
  const source = structuredClone(corpus.document);
  sourceChanges.forEach((change) => applyModelChange(source, change));
  const expected = structuredClone(source);
  expectedChanges.forEach((change) => applyModelChange(expected, change));
  return { raw: serializeCorpusDocument(source), expected: serializeCorpusDocument(expected) };
}

function modelGolden(corpus: PreservationCase, changes: Change[], removedBranch?: string) {
  const source = structuredClone(corpus.document);
  if (removedBranch) delete (source.outbounds[1].streamSettings as unknown as Model)[removedBranch];
  const expected = structuredClone(source);
  changes.forEach((change) => applyModelChange(expected, change));
  return { raw: serializeCorpusDocument(source), expected: serializeCorpusDocument(expected) };
}

const base: Path = ["outbounds", 1];
const stream: Path = [...base, "streamSettings"];
const transportFields = ["path", "host", "grpcAuthority", "headerType"] as const;
const transportValues = { path: "/single", host: "single-host.example", grpcAuthority: "single-authority.example", headerType: "none" };

function transportChanges(corpus: PreservationCase, field: (typeof transportFields)[number], clear: boolean): Change[] {
  const value = clear ? undefined : transportValues[field];
  if (corpus.network === "ws") {
    if (field === "path") return [{ path: [...stream, "wsSettings", "path"], value }];
    if (field === "host") return clear ? [
      { path: [...stream, "wsSettings", "headers", "Host"], value }, { path: [...stream, "wsSettings", "headers", "host"], value },
    ] : [{ path: [...stream, "wsSettings", "headers", "Host"], value }];
  }
  if (corpus.network === "grpc") {
    if (field === "path") return [{ path: [...stream, "grpcSettings", "serviceName"], value }];
    if (field === "grpcAuthority") return [{ path: [...stream, "grpcSettings", "authority"], value }];
  }
  if (corpus.network === "httpupgrade" || corpus.network === "xhttp") {
    if (field === "path" || field === "host") return [{ path: [...stream, `${corpus.network}Settings`, field], value }];
  }
  if (corpus.network === "tcp") return tcpChanges(field, value);
  return [];
}

function tcpChanges(field: (typeof transportFields)[number], value: unknown): Change[] {
  const header = [...stream, "tcpSettings", "header"];
  if (field === "headerType") return [{ path: [...header, "type"], value }];
  if (field === "path") return [{ path: [...header, "request", "path"], value: value === undefined ? undefined : [value] }];
  if (field === "host") return [{ path: [...header, "request", "headers", "Host"], value: value === undefined ? undefined : [value] }];
  return [];
}

export const singleTransportGoldens = preservationCorpus.flatMap((corpus) => transportFields.flatMap((field) => [false, true].map((clear) => {
  const changes = transportChanges(corpus, field, clear);
  return { name: `${corpus.name}/${field}/${clear ? "clear" : "set"}`, corpus, field, changes, patch: { [field]: clear ? "" : transportValues[field] } as XrayJSONPatch, ...modelGolden(corpus, changes) };
})));

export const absentTransportGoldens = singleTransportGoldens.filter(({ changes }) => changes.length > 0).map((entry) => {
  const branch = String(entry.changes[0].path[3]);
  return { ...entry, name: `${entry.name}/absent-${branch}`, ...modelGolden(entry.corpus, entry.changes, branch) };
});

const securityFields = ["sni", "alpn", "allowInsecure", "fingerprint", "publicKey", "shortId", "spiderX"] as const;
const securityValues = { sni: "single-sni.example", alpn: "http/1.1", allowInsecure: true, fingerprint: "firefox", publicKey: "updated", shortId: "1234", spiderX: "/single-spider" };

function securityChanges(corpus: PreservationCase, field: (typeof securityFields)[number], clear: boolean, missing: boolean): Change[] {
  const value = clear ? undefined : securityValues[field];
  if (corpus.security === "tls") return tlsChanges(field, value);
  const reality = [...stream, "realitySettings"];
  if (field === "sni") return [{ path: [...reality, "serverName"], value }];
  if (field === "publicKey") return missing ? [{ path: [...reality, "publicKey"], value }] : [
    { path: [...reality, "publicKey"], value }, { path: [...reality, "password"], value },
  ];
  if (field === "fingerprint" || field === "shortId" || field === "spiderX") return [{ path: [...reality, field], value }];
  return [];
}

function tlsChanges(field: (typeof securityFields)[number], value: unknown): Change[] {
  const tls = [...stream, "tlsSettings"];
  if (field === "sni") return [{ path: [...tls, "serverName"], value }];
  if (field === "alpn") return [{ path: [...tls, "alpn"], value: value === undefined ? undefined : [value] }];
  if (field === "allowInsecure" || field === "fingerprint") return [{ path: [...tls, field], value }];
  return [];
}

export const singleSecurityGoldens = preservationCorpus.flatMap((corpus) => securityFields.flatMap((field) => [false, true].map((clear) => ({
  name: `${corpus.name}/${field}/${clear ? "clear" : "set"}`, corpus, field, clear,
  patch: { [field]: clear ? (field === "allowInsecure" ? false : "") : securityValues[field] } as XrayJSONPatch,
  ...modelGolden(corpus, securityChanges(corpus, field, clear, false)),
}))));

export const absentSecurityGoldens = singleSecurityGoldens.filter((entry) => entry.corpus.security !== "none" && securityChanges(entry.corpus, entry.field, entry.clear, true).length > 0).map((entry) => {
  const branch = entry.corpus.security === "tls" ? "tlsSettings" : "realitySettings";
  return { ...entry, name: `${entry.name}/absent-${branch}`, ...modelGolden(entry.corpus, securityChanges(entry.corpus, entry.field, entry.clear, true), branch) };
});

const connectionFields = ["port", "remark", "identifier", "flow", "encryption", "vmessSecurity", "vmessAlterId"] as const;
const connectionValues = { port: "8443", remark: "Single", identifier: "44444444-4444-4444-8444-444444444444", flow: "new-flow", encryption: "none", vmessSecurity: "none", vmessAlterId: "2" };

function connectionChanges(corpus: PreservationCase, field: (typeof connectionFields)[number], clear: boolean): Change[] {
  const value = clear ? undefined : connectionValues[field];
  const endpoint = corpus.protocol === "trojan" ? [...base, "settings", "servers", 0] : [...base, "settings", "vnext", 0];
  const user = [...base, "settings", "vnext", 0, "users", 0];
  if (field === "remark") return [{ path: [...base, "tag"], value }];
  if (field === "port") return [{ path: [...endpoint, "port"], value: clear ? 443 : 8443 }];
  if (field === "identifier") return [{ path: corpus.protocol === "trojan" ? [...endpoint, "password"] : [...user, "id"], value }];
  if (corpus.protocol === "vless" && (field === "flow" || field === "encryption")) return [{ path: [...user, field], value }];
  if (corpus.protocol === "vmess") return vmessChanges(field, value);
  return [];
}

function vmessChanges(field: (typeof connectionFields)[number], value: unknown): Change[] {
  const user = [...base, "settings", "vnext", 0, "users", 0];
  if (field === "vmessSecurity") return [{ path: [...user, "security"], value }];
  if (field === "vmessAlterId") return [{ path: [...user, "alterId"], value: value === undefined ? undefined : 2 }];
  return [];
}

export const singleConnectionGoldens = preservationCorpus.flatMap((corpus) => connectionFields.flatMap((field) => [false, true].filter((clear) => field !== "identifier" || !clear).map((clear) => ({
  name: `${corpus.name}/${field}/${clear ? "clear" : "set"}`,
  patch: { [field]: clear ? "" : connectionValues[field] } as XrayJSONPatch,
  ...modelGolden(corpus, connectionChanges(corpus, field, clear)),
}))));

export const tlsOnlyNoneGoldens = preservationCorpus.filter(({ security }) => security === "none").map((corpus) => ({
  name: `${corpus.name}/tls-only`, ...modelGolden(corpus, [], "realitySettings"),
}));
