export type ConfigurationProtocol = "vless" | "trojan" | "vmess";

export interface ConfigurationDraft {
  protocol: ConfigurationProtocol;
  server: string;
  port: string;
  identifier: string;
  params: string;
  remark: string;
}

const DEFAULT_PORT = "443";

function decodeBase64(value: string) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padding = normalized.length % 4;
  const padded = padding === 0 ? normalized : normalized + "=".repeat(4 - padding);
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function encodeBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

export function extractLabelFromConfiguration(raw: string) {
  const parsed = parseConfiguration(raw);
  if (!parsed) {
    return "";
  }
  return parsed.remark.trim();
}

export function parseConfiguration(raw: string): ConfigurationDraft | null {
  const trimmed = raw.trim();
  if (!trimmed) {
    return null;
  }

  if (trimmed.toLowerCase().startsWith("vmess://")) {
    const encoded = trimmed.slice("vmess://".length).trim();
    const payload = JSON.parse(decodeBase64(encoded)) as Record<string, unknown>;
    const { add, port, id, ps, ...rest } = payload;

    return {
      protocol: "vmess",
      server: String(add ?? "").trim(),
      port: String(port ?? DEFAULT_PORT).trim() || DEFAULT_PORT,
      identifier: String(id ?? "").trim(),
      params: Object.keys(rest).length > 0 ? JSON.stringify(rest, null, 2) : "",
      remark: String(ps ?? "").trim(),
    };
  }

  const parsed = new URL(trimmed);
  const protocol = parsed.protocol.replace(":", "").toLowerCase();
  if (protocol !== "vless" && protocol !== "trojan") {
    throw new Error("Поддерживаются только vless://, vmess:// и trojan://");
  }

  return {
    protocol,
    server: parsed.hostname.trim(),
    port: parsed.port.trim() || DEFAULT_PORT,
    identifier: decodeURIComponent(parsed.username || "").trim(),
    params: parsed.search ? parsed.search.slice(1) : "",
    remark: decodeURIComponent(parsed.hash ? parsed.hash.slice(1) : "").trim(),
  };
}

export function buildConfiguration(draft: ConfigurationDraft) {
  const protocol = draft.protocol;
  const server = draft.server.trim();
  const port = draft.port.trim() || DEFAULT_PORT;
  const identifier = draft.identifier.trim();
  const params = draft.params.trim();
  const remark = draft.remark.trim();

  if (!server) {
    throw new Error("Укажите сервер");
  }
  if (!identifier) {
    throw new Error(protocol === "trojan" ? "Укажите пароль" : "Укажите UUID");
  }

  if (protocol === "vmess") {
    let extra: Record<string, unknown> = {};
    if (params) {
      try {
        extra = JSON.parse(params) as Record<string, unknown>;
      } catch {
        throw new Error("JSON-параметры VMESS заполнены некорректно");
      }
    }

    const payload: Record<string, unknown> = {
      v: "2",
      ps: remark,
      add: server,
      port,
      id: identifier,
      ...extra,
    };

    return `vmess://${encodeBase64(JSON.stringify(payload))}`;
  }

  const url = new URL(`${protocol}://${encodeURIComponent(identifier)}@${server}:${port}`);
  url.username = identifier;
  url.password = "";
  url.search = params ? `?${params}` : "";
  url.hash = remark ? `#${remark}` : "";
  return url.toString();
}

export function getConfigurationPlaceholder() {
  return [
    "vless://uuid@example.com:443?type=ws&security=tls#My Server",
    "vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6InV1aWQifQ==",
    "trojan://password@example.com:443?security=tls#Trojan Server",
  ].join("\n");
}
