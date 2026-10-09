import type { KeyProfileDetailResponse } from "@/lib/types";
import { emptyLegacyDraft, editorInitialText as initialText, editorInitialFlag as initialFlag } from "./key-editor-command-state";
import { isLegacyEditorProtocol } from "./protocol-editor-capabilities";

type SafeDetail = NonNullable<KeyProfileDetailResponse["safe_structured"]>;

function shadowsocksProjection(initial: NonNullable<SafeDetail["shadowsocks"]>) {
  return { method: initialText(initial.method, "2022-blake3-aes-128-gcm"), pluginName: initialText(initial.plugin_name) };
}

function hysteria2Projection(initial: NonNullable<SafeDetail["hysteria2"]>) {
  return {
    sni: initialText(initial.sni), insecure: initialFlag(initial.insecure),
    certificate: initialText(initial.certificate_sha256), obfuscation: initialText(initial.obfuscation_type),
  };
}

function tuicProjection(initial: NonNullable<SafeDetail["tuic"]>) {
  return {
    sni: initialText(initial.sni), alpn: (initial.alpn || []).join(","),
    skipCert: initialFlag(initial.skip_cert_verify),
    ...tuicTransportProjection(initial),
  };
}

function tuicTransportProjection(initial: NonNullable<SafeDetail["tuic"]>) {
  return {
    cc: initialText(initial.congestion_controller, "bbr"),
    relay: initialText(initial.udp_relay_mode, "native"), udpOverStream: initialFlag(initial.udp_over_stream),
    zeroRtt: initialFlag(initial.zero_rtt), heartbeat: initialText(initial.heartbeat, "10s"),
  };
}

function displayNameProjection(detail: KeyProfileDetailResponse, safe: SafeDetail) {
  if (detail.ownership === "external_source" && !detail.client_display_name_overridden) return "";
  return detail.client_display_name || safe.display_name || detail.label;
}

function legacyProjection(detail: KeyProfileDetailResponse, safe: SafeDetail) {
  if (!isLegacyEditorProtocol(detail.protocol)) return undefined;
  return {
    ...emptyLegacyDraft(detail.protocol), server: initialText(safe.server),
    port: safe.port || "443", remark: safe.display_name || detail.label,
  };
}

// Missing protocol projections leave existing fields untouched on a reload.
export function buildEditorDetailProjection(detail: KeyProfileDetailResponse) {
  const safe = detail.safe_structured;
  if (!safe) return undefined;
  return {
    server: initialText(safe.server), port: initialText(safe.port), displayName: displayNameProjection(detail, safe),
    legacy: legacyProjection(detail, safe),
    shadowsocks: safe.shadowsocks ? shadowsocksProjection(safe.shadowsocks) : undefined,
    hysteria2: safe.hysteria2 ? hysteria2Projection(safe.hysteria2) : undefined,
    tuic: safe.tuic ? tuicProjection(safe.tuic) : undefined,
  };
}
