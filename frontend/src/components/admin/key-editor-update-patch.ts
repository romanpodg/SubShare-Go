import type { KeyProfileDetailResponse, SafeTUICDetail, StructuredProfilePatch } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import { buildShadowsocksPatch } from "./protocol-editors/ShadowsocksFields";
import { buildHysteria2Patch } from "./protocol-editors/Hysteria2Fields";
import { buildTUICPatch } from "./protocol-editors/TuicV5Fields";

function initialText(value: string | undefined, fallback = "") { return value || fallback; }

function updateShadowsocks(state: EditorCommandState, detail: KeyProfileDetailResponse) {
  const initial = detail.safe_structured?.shadowsocks;
  return buildShadowsocksPatch(
    initialText(initial?.method, "2022-blake3-aes-128-gcm"), state.ssMethod, state.ssPassword,
    initialText(initial?.plugin_name), state.ssPluginName, "", state.ssPluginOptions
  );
}

function updateHysteria2(state: EditorCommandState, detail: KeyProfileDetailResponse) {
  const initial = detail.safe_structured?.hysteria2;
  return buildHysteria2Patch(
    initialText(initial?.sni), state.hy2Sni,
    initial?.insecure || false, state.hy2Insecure,
    initialText(initial?.certificate_sha256), state.hy2CertSha,
    initialText(initial?.obfuscation_type), state.hy2ObfsType,
    state.hy2Auth, state.hy2ObfsPassword
  );
}

function updateTuic(state: EditorCommandState, detail: KeyProfileDetailResponse) {
  const initial: Partial<SafeTUICDetail> = detail.safe_structured?.tuic || {};
  return buildTUICPatch(
    initialText(initial.sni), state.tuicSni,
    (initial.alpn || []).join(","), state.tuicAlpn,
    initial.skip_cert_verify || false, state.tuicSkipCert,
    initialText(initial.congestion_controller, "bbr"), state.tuicCc,
    initialText(initial.udp_relay_mode, "native"), state.tuicUdpRelay,
    initial.udp_over_stream || false, state.tuicUdpOverStream,
    initial.zero_rtt || false, state.tuicZeroRtt,
    initialText(initial.heartbeat, "10s"), state.tuicHeartbeat,
    state.tuicUuid, state.tuicPassword
  );
}

export function buildEditorUpdatePatch(state: EditorCommandState, detail: KeyProfileDetailResponse) {
  if (state.mode !== "structured" || state.kind !== "real") return undefined;
  const patch: StructuredProfilePatch = {};
  appendConnectionPatch(patch, state, detail);
  if (state.protocol === "shadowsocks") patch.shadowsocks = updateShadowsocks(state, detail);
  else if (state.protocol === "hysteria2") patch.hysteria2 = updateHysteria2(state, detail);
  else if (state.protocol === "tuic") patch.tuic = updateTuic(state, detail);
  return patch;
}

function appendConnectionPatch(patch: StructuredProfilePatch, state: EditorCommandState, detail: KeyProfileDetailResponse) {
  if (state.server !== (detail.safe_structured?.server || "")) patch.server = { operation: "set", value: state.server };
  if (state.port !== (detail.safe_structured?.port || "")) patch.port = { operation: "set", value: state.port };
}
