"use client";

import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import type { ProtocolSchemaDTO, ShadowsocksStructuredPatch } from "@/lib/types";

interface Props {
  schema?: ProtocolSchemaDTO;
  method: string;
  onMethodChange: (val: string) => void;
  password?: string;
  onPasswordChange: (val?: string) => void;
  pluginName?: string;
  onPluginNameChange: (val?: string) => void;
  pluginOptions?: string;
  onPluginOptionsChange: (val?: string) => void;
  readOnly?: boolean;
}

export function ShadowsocksFields({
  schema,
  method,
  onMethodChange,
  password,
  onPasswordChange,
  pluginName,
  onPluginNameChange,
  pluginOptions,
  onPluginOptionsChange,
  readOnly = false,
}: Props) {
  const methodOptions = schema?.fields.method?.options || [
    { value: "aes-128-gcm", label: "AES-128-GCM" },
    { value: "aes-256-gcm", label: "AES-256-GCM" },
    { value: "chacha20-ietf-poly1305", label: "ChaCha20-IETF-Poly1305" },
    { value: "2022-blake3-aes-128-gcm", label: "Shadowsocks 2022 (Blake3-AES-128-GCM)" },
    { value: "2022-blake3-aes-256-gcm", label: "Shadowsocks 2022 (Blake3-AES-256-GCM)" },
    { value: "2022-blake3-chacha20-poly1305", label: "Shadowsocks 2022 (Blake3-ChaCha20-Poly1305)" },
  ];

  return (
    <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
      <Select
        label="Шифр (Method)"
        value={method}
        onChange={(e) => onMethodChange(e.target.value)}
        disabled={readOnly}
        options={methodOptions}
      />
      <Input
        label="Пароль / PSK"
        type="password"
        value={password ?? ""}
        placeholder={password === undefined ? "(не изменён)" : "Введите пароль"}
        onChange={(e) => onPasswordChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="Имя плагина (Plugin)"
        value={pluginName ?? ""}
        placeholder="v2ray-plugin"
        onChange={(e) => onPluginNameChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="Параметры плагина"
        value={pluginOptions ?? ""}
        placeholder="mux=0;tls;host=example.com"
        onChange={(e) => onPluginOptionsChange(e.target.value)}
        disabled={readOnly}
      />
    </div>
  );
}

export function buildShadowsocksPatch(
  initialMethod: string,
  currentMethod: string,
  passwordVal?: string,
  initialPluginName?: string,
  currentPluginName?: string,
  initialPluginOpts?: string,
  currentPluginOpts?: string
): ShadowsocksStructuredPatch | undefined {
  const patch: ShadowsocksStructuredPatch = {};
  let touched = false;

  if (currentMethod !== initialMethod) {
    patch.method = { operation: "set", value: currentMethod };
    touched = true;
  }
  if (passwordVal !== undefined) {
    patch.password = { operation: "set", value: passwordVal };
    touched = true;
  }
  if (currentPluginName !== undefined && currentPluginName !== initialPluginName) {
    if (currentPluginName === "") {
      patch.plugin_name = { operation: "clear" };
    } else {
      patch.plugin_name = { operation: "set", value: currentPluginName };
    }
    touched = true;
  }
  if (currentPluginOpts !== undefined && currentPluginOpts !== initialPluginOpts) {
    if (currentPluginOpts === "") {
      patch.plugin_options = { operation: "clear" };
    } else {
      patch.plugin_options = { operation: "set", value: currentPluginOpts };
    }
    touched = true;
  }

  return touched ? patch : undefined;
}
