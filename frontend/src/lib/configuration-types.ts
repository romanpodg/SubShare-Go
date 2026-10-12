export type ConfigurationProtocol = "vless" | "trojan" | "vmess";
export type RealConfigurationMode = "link" | "xray-json";
export type XrayJSONNetwork = "tcp" | "ws" | "grpc" | "httpupgrade" | "xhttp" | "h2" | "quic";
export type XrayJSONSecurity = "none" | "tls" | "reality";

export interface ConfigurationDraft {
  protocol: ConfigurationProtocol;
  server: string;
  port: string;
  identifier: string;
  params: string;
  remark: string;
}

export interface XrayJSONDraft {
  protocol: ConfigurationProtocol;
  server: string;
  port: string;
  identifier: string;
  network: XrayJSONNetwork;
  security: XrayJSONSecurity;
  path: string;
  host: string;
  sni: string;
  alpn: string;
  remark: string;
  flow?: string;
  encryption?: string;
  fingerprint?: string;
  publicKey?: string;
  shortId?: string;
  spiderX?: string;
  allowInsecure?: boolean;
  grpcAuthority?: string;
  headerType?: string;
  vmessSecurity?: string;
  vmessAlterId?: string;
}

export interface ParsedXrayJSONConfiguration {
  draft: XrayJSONDraft;
  config: Record<string, unknown>;
  outboundIndex: number;
}

export type XrayJSONPatch = Partial<XrayJSONDraft>;
