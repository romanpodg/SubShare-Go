import type { CreateKeyProfileInput, StructuredProfilePatch } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import { buildLegacyCreateRaw, validateEditorRaw } from "./key-editor-raw-policy";
import { isLegacyEditorProtocol } from "./protocol-editor-capabilities";

function createShadowsocks(state: EditorCommandState) {
  return {
    method: { operation: "set" as const, value: state.ssMethod },
    password: { operation: "set" as const, value: state.ssPassword || "" },
    plugin_name: state.ssPluginName ? { operation: "set" as const, value: state.ssPluginName } : undefined,
    plugin_options: state.ssPluginOptions ? { operation: "set" as const, value: state.ssPluginOptions } : undefined,
  };
}

function createHysteria2(state: EditorCommandState) {
  return {
    authentication: { operation: "set" as const, value: state.hy2Auth || "" },
    sni: state.hy2Sni ? { operation: "set" as const, value: state.hy2Sni } : undefined,
    insecure: { operation: "set" as const, value: state.hy2Insecure },
    certificate_sha256: state.hy2CertSha ? { operation: "set" as const, value: state.hy2CertSha } : undefined,
    obfuscation_type: state.hy2ObfsType ? { operation: "set" as const, value: state.hy2ObfsType } : undefined,
    obfuscation_password: state.hy2ObfsPassword ? { operation: "set" as const, value: state.hy2ObfsPassword } : undefined,
  };
}

function createTuic(state: EditorCommandState) {
  return {
    uuid: { operation: "set" as const, value: state.tuicUuid || "" },
    password: { operation: "set" as const, value: state.tuicPassword || "" },
    sni: state.tuicSni ? { operation: "set" as const, value: state.tuicSni } : undefined,
    alpn: state.tuicAlpn ? { operation: "set" as const, value: state.tuicAlpn.split(",").map((s) => s.trim()).filter(Boolean) } : undefined,
    skip_cert_verify: { operation: "set" as const, value: state.tuicSkipCert },
    congestion_controller: { operation: "set" as const, value: state.tuicCc },
    udp_relay_mode: { operation: "set" as const, value: state.tuicUdpRelay },
    udp_over_stream: { operation: "set" as const, value: state.tuicUdpOverStream },
    zero_rtt: { operation: "set" as const, value: state.tuicZeroRtt },
    heartbeat: { operation: "set" as const, value: state.tuicHeartbeat },
  };
}

function createStructuredPatch(state: EditorCommandState): StructuredProfilePatch | undefined {
  if (state.mode !== "structured" || state.kind !== "real") return undefined;
  const patch: StructuredProfilePatch = {
    server: { operation: "set", value: state.server },
    port: { operation: "set", value: state.port },
    display_name: { operation: "set", value: state.displayName || state.label },
  };
  if (state.protocol === "shadowsocks") patch.shadowsocks = createShadowsocks(state);
  else if (state.protocol === "hysteria2") patch.hysteria2 = createHysteria2(state);
  else if (state.protocol === "tuic") patch.tuic = createTuic(state);
  return patch;
}

function createContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined) {
  const content = { mode: state.mode, raw: state.mode === "raw" ? state.rawUri : undefined, patch };
  if (state.kind !== "real") return content;
  if (isLegacyEditorProtocol(state.protocol) && state.mode === "structured") {
    content.raw = buildLegacyCreateRaw(state);
    validateEditorRaw(state.protocol, content.raw);
    content.mode = "raw";
    content.patch = undefined;
  } else if (state.mode === "raw") validateEditorRaw(state.protocol, state.rawUri);
  return content;
}

export function buildEditorCreateCommand(state: EditorCommandState): CreateKeyProfileInput {
  const content = createContent(state, createStructuredPatch(state));
  return {
    label: state.label.trim(),
    client_display_name: state.displayName.trim() || state.label.trim(),
    category: state.category,
    status: state.status,
    kind: state.kind,
    template_text: state.templateText,
    creation_mode: content.mode,
    raw_uri: content.mode === "raw" ? content.raw : undefined,
    protocol: state.protocol,
    structured: content.mode === "structured" ? content.patch : undefined,
  };
}
