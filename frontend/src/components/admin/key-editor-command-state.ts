import type { XrayJSONDraft } from "@/lib/configuration";
import type { ExternalProfileProtocol } from "@/lib/types";

export function editorInitialText(value: string | undefined, fallback = "") { return value || fallback; }
export function editorInitialFlag(value: boolean | undefined) { return value || false; }

export function emptyLegacyDraft(protocol: "vless" | "vmess" | "trojan" = "vless"): XrayJSONDraft {
  return {
    protocol,
    server: "",
    port: "443",
    identifier: "",
    network: "tcp",
    security: "none",
    path: "",
    host: "",
    sni: "",
    alpn: "",
    remark: "",
  };
}

export interface EditorCommandState {
  label: string;
  displayName: string;
  category: string;
  status: "active" | "non-active";
  kind: "real" | "informational";
  templateText: string;
  mode: "raw" | "structured";
  protocol: ExternalProfileProtocol;
  server: string;
  port: string;
  rawUri: string;
  authoritativeRaw: string;
  revealedRaw: boolean;
  structuredEdited: boolean;
  rawEdited: boolean;
  legacyDraft: XrayJSONDraft;
  ssMethod: string;
  ssPassword?: string;
  ssPluginName?: string;
  ssPluginOptions?: string;
  hy2Auth?: string;
  hy2Sni: string;
  hy2Insecure: boolean;
  hy2CertSha: string;
  hy2ObfsType: string;
  hy2ObfsPassword?: string;
  tuicUuid?: string;
  tuicPassword?: string;
  tuicSni: string;
  tuicAlpn: string;
  tuicSkipCert: boolean;
  tuicCc: string;
  tuicUdpRelay: string;
  tuicUdpOverStream: boolean;
  tuicZeroRtt: boolean;
  tuicHeartbeat: string;
}
