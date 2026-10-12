import type { ConfigurationProtocol } from "./configuration-types";
import {
  asArray,
  asRecord,
  portValue,
  textValue
} from "./configuration-values";

export function readXrayConnection(outbound: Record<string, unknown>, protocol: ConfigurationProtocol) {
  const settings = asRecord(outbound.settings) ?? {};
  const server = protocol === "trojan" ? asRecord(asArray(settings.servers)[0]) : asRecord(asArray(settings.vnext)[0]);
  const user = protocol === "trojan" ? null : asRecord(asArray(server?.users)[0]);
  return {
    server: textValue(server?.address),
    port: portValue(server?.port),
    identifier: textValue(protocol === "trojan" ? server?.password : user?.id),
    flow: textValue(user?.flow),
    encryption: textValue(user?.encryption),
    vmessSecurity: textValue(user?.security),
    vmessAlterId: textValue(user?.alterId),
  };
}
