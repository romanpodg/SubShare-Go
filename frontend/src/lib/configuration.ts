// Stable public API for the key editor. Implementations depend on types/values,
// never on this facade, so the interpretation and token-editing layers stay directed.
export type {
  ConfigurationProtocol,
  RealConfigurationMode,
  XrayJSONNetwork,
  XrayJSONSecurity,
  ConfigurationDraft,
  XrayJSONDraft,
  ParsedXrayJSONConfiguration,
  XrayJSONPatch
} from "./configuration-types";
export {
  extractLabelFromConfiguration,
  parseConfiguration,
  buildConfiguration
} from "./configuration-link-codec";
export { parseXrayJSONConfiguration } from "./configuration-xray-reader";
export { buildXrayJSONConfiguration } from "./configuration-xray-builder";
export { patchXrayJSONConfiguration } from "./configuration-xray-patch";
export {
  createXrayJSONFromConfiguration,
  parseEditableConfiguration,
  createConfigurationFromXrayJSON,
  isXrayJSONConfiguration
} from "./configuration-conversion";
export { patchEditableConfiguration } from "./configuration-link-patch";

export function getConfigurationPlaceholder() {
  return [
    "vless://uuid@example.com:443?type=ws&security=tls#My Server",
    "vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6InV1aWQifQ==",
    "trojan://password@example.com:443?security=tls#Trojan Server",
  ].join("\n");
}

export function getXrayJSONPlaceholder() {
  return JSON.stringify(
    {
      log: {
        loglevel: "warning",
      },
      inbounds: [],
      outbounds: [
        {
          tag: "My Server",
          protocol: "vless",
          settings: {
            vnext: [
              {
                address: "example.com",
                port: 443,
                users: [
                  {
                    id: "00000000-0000-0000-0000-000000000000",
                    encryption: "none",
                  },
                ],
              },
            ],
          },
          streamSettings: {
            network: "ws",
            security: "tls",
            wsSettings: {
              path: "/",
              headers: {
                Host: "example.com",
              },
            },
            tlsSettings: {
              serverName: "example.com",
            },
          },
        },
      ],
    },
    null,
    2
  );
}
