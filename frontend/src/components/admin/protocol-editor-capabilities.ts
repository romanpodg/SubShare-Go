import type { ExternalProfileProtocol } from "@/lib/types";

export type EditableProtocol =
  | "vless"
  | "vmess"
  | "trojan"
  | "shadowsocks"
  | "hysteria2"
  | "tuic";

export interface ProtocolEditorCapability {
  protocol: EditableProtocol;
  label: string;
  structured: true;
  editor: "legacy" | "shadowsocks" | "hysteria2" | "tuic";
}

export const PROTOCOL_EDITOR_CAPABILITIES: readonly ProtocolEditorCapability[] = [
  { protocol: "vless", label: "VLESS", structured: true, editor: "legacy" },
  { protocol: "vmess", label: "VMess", structured: true, editor: "legacy" },
  { protocol: "trojan", label: "Trojan", structured: true, editor: "legacy" },
  { protocol: "shadowsocks", label: "Shadowsocks", structured: true, editor: "shadowsocks" },
  { protocol: "hysteria2", label: "Hysteria 2", structured: true, editor: "hysteria2" },
  { protocol: "tuic", label: "TUIC v5", structured: true, editor: "tuic" },
] as const;

export function protocolEditorCapability(protocol: ExternalProfileProtocol | undefined) {
  return PROTOCOL_EDITOR_CAPABILITIES.find((item) => item.protocol === protocol);
}

export function isLegacyEditorProtocol(protocol: ExternalProfileProtocol): protocol is "vless" | "vmess" | "trojan" {
  return protocol === "vless" || protocol === "vmess" || protocol === "trojan";
}
