import type { XrayJSONDraft } from "./configuration-types";
import { buildConfiguration } from "./configuration-link-codec";
import {
  uriTextFields,
  vmessTextFields,
  type LinkFieldSpec
} from "./configuration-link-fields";

function objectTextFields(object: Record<string, unknown>, draft: XrayJSONDraft, fields: readonly LinkFieldSpec[]) {
  for (const [field, aliases] of fields) {
    const value = draft[field]?.trim();
    if (value) object[aliases[0]] = value;
  }
}

function queryTextFields(query: URLSearchParams, draft: XrayJSONDraft, fields: readonly LinkFieldSpec[]) {
  for (const [field, aliases] of fields) {
    const value = draft[field]?.trim();
    if (value) query.set(aliases[0], value);
  }
}

function vmessDiscriminators(extra: Record<string, unknown>, draft: XrayJSONDraft) {
  if (draft.network && draft.network !== "tcp") extra.net = draft.network;
  if (draft.security === "tls") extra.tls = "tls";
  else if (draft.security !== "none") extra.security = draft.security;
}

function vmessHeader(extra: Record<string, unknown>, draft: XrayJSONDraft) {
  if (draft.network === "tcp") objectTextFields(extra, draft, [["headerType", ["type"]]]);
}

function vmessParams(draft: XrayJSONDraft) {
  const extra: Record<string, unknown> = {};
  vmessDiscriminators(extra, draft);
  objectTextFields(extra, draft, vmessTextFields);
  if (draft.allowInsecure) extra.allowInsecure = "1";
  vmessHeader(extra, draft);
  objectTextFields(extra, draft, [
    ["grpcAuthority", ["authority"]],
    ["vmessSecurity", ["scy"]],
    ["vmessAlterId", ["aid"]]
  ]);
  return Object.keys(extra).length > 0 ? JSON.stringify(extra, null, 2) : "";
}

function uriDiscriminators(query: URLSearchParams, draft: XrayJSONDraft) {
  if (draft.network && draft.network !== "tcp") query.set("type", draft.network);
  if (draft.security && draft.security !== "none") query.set("security", draft.security);
}

function uriNetworkFields(query: URLSearchParams, draft: XrayJSONDraft) {
  if (draft.network === "tcp") queryTextFields(query, draft, [["headerType", ["headerType"]]]);
  if (draft.network === "grpc") queryTextFields(query, draft, [["grpcAuthority", ["authority"]]]);
}

function uriParams(draft: XrayJSONDraft) {
  const query = new URLSearchParams();
  uriDiscriminators(query, draft);
  queryTextFields(query, draft, uriTextFields);
  if (draft.allowInsecure) query.set("allowInsecure", "1");
  uriNetworkFields(query, draft);
  return query.toString();
}

export function serializeXrayDraftLink(draft: XrayJSONDraft) {
  const params = draft.protocol === "vmess" ? vmessParams(draft) : uriParams(draft);
  return buildConfiguration({
    protocol: draft.protocol,
    server: draft.server,
    port: draft.port,
    identifier: draft.identifier,
    params,
    remark: draft.remark
  });
}
