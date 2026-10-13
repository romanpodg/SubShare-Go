import type { ConfigurationProtocol } from "./configuration-types";
import {
  asArray,
  asRecord,
  portValue,
  textValue
} from "./configuration-values";

function primaryServer(settings: Record<string, unknown>, protocol: ConfigurationProtocol) {
  const servers = protocol === "trojan" ? settings.servers : settings.vnext;
  return asRecord(asArray(servers)[0]) ?? {};
}
function primaryUser(server: Record<string, unknown>, protocol: ConfigurationProtocol): Record<string, unknown> {
  if (protocol === "trojan") return {};
  return asRecord(asArray(server.users)[0]) ?? {};
}
export function readXrayConnection(outbound: Record<string, unknown>, protocol: ConfigurationProtocol) {
  const settings = asRecord(outbound.settings) ?? {};
  const server = primaryServer(settings, protocol);
  const user = primaryUser(server, protocol);
  return {
    server: textValue(server.address),
    port: portValue(server.port),
    identifier: textValue(protocol === "trojan" ? server.password : user.id),
    flow: textValue(user.flow),
    encryption: textValue(user.encryption),
    vmessSecurity: textValue(user.security),
    vmessAlterId: textValue(user.alterId),
  };
}
