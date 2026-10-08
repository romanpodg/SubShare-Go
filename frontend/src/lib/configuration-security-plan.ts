import type { XrayJSONPatch } from "./configuration";
import { buildALPNValues, hasPatchField, optionalText } from "./configuration-patch-values";

type SecurityPath = Array<string | number>;

export interface SecurityPatchPlan {
  mode: "tls" | "reality";
  patch: XrayJSONPatch;
  path: SecurityPath;
  original: Readonly<Record<string, unknown>> | null;
  initialize: () => void;
  append: (path: SecurityPath, value: unknown) => void;
}

// The caller owns ordered parent initialization and document application.
// This phase records only represented security changes, normalizing lazily.
export function appendSecurityPlan(plan: SecurityPatchPlan) {
  plan.initialize();
  if (plan.mode === "tls") appendTls(plan);
  else appendReality(plan);
}

function appendTls(plan: SecurityPatchPlan) {
  const { patch, path, append } = plan;
  if (hasPatchField(patch, "sni")) append([...path, "serverName"], optionalText(patch.sni));
  if (hasPatchField(patch, "alpn")) {
    const values = buildALPNValues(patch.alpn ?? "");
    append([...path, "alpn"], values.length > 0 ? values : undefined);
  }
  if (hasPatchField(patch, "allowInsecure")) append([...path, "allowInsecure"], patch.allowInsecure ? true : undefined);
  if (hasPatchField(patch, "fingerprint")) append([...path, "fingerprint"], optionalText(patch.fingerprint));
}

function appendReality(plan: SecurityPatchPlan) {
  const { patch, path, append } = plan;
  if (hasPatchField(patch, "sni")) append([...path, "serverName"], optionalText(patch.sni));
  if (hasPatchField(patch, "fingerprint")) append([...path, "fingerprint"], optionalText(patch.fingerprint));
  if (hasPatchField(patch, "publicKey")) appendRealityKey(plan);
  if (hasPatchField(patch, "shortId")) append([...path, "shortId"], optionalText(patch.shortId));
  if (hasPatchField(patch, "spiderX")) append([...path, "spiderX"], optionalText(patch.spiderX));
}

function appendRealityKey(plan: SecurityPatchPlan) {
  const { path, append } = plan;
  const value = optionalText(plan.patch.publicKey);
  if (!value) {
    append([...path, "publicKey"], undefined);
    append([...path, "password"], undefined);
    return;
  }
  const modern = Object.prototype.hasOwnProperty.call(plan.original ?? {}, "publicKey");
  const legacy = Object.prototype.hasOwnProperty.call(plan.original ?? {}, "password");
  if (modern || !legacy) append([...path, "publicKey"], value);
  if (legacy) append([...path, "password"], value);
}
