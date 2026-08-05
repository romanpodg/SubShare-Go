"use client";

import { AlertTriangle } from "lucide-react";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import type { Hysteria2StructuredPatch, ProtocolSchemaDTO } from "@/lib/types";

interface Props {
  schema?: ProtocolSchemaDTO;
  authentication?: string;
  onAuthChange: (val?: string) => void;
  sni: string;
  onSniChange: (val: string) => void;
  insecure: boolean;
  onInsecureChange: (val: boolean) => void;
  certSha256: string;
  onCertSha256Change: (val: string) => void;
  obfsType: string;
  onObfsTypeChange: (val: string) => void;
  obfsPassword?: string;
  onObfsPasswordChange: (val?: string) => void;
  readOnly?: boolean;
}

export function Hysteria2Fields({
  schema,
  authentication,
  onAuthChange,
  sni,
  onSniChange,
  insecure,
  onInsecureChange,
  certSha256,
  onCertSha256Change,
  obfsType,
  onObfsTypeChange,
  obfsPassword,
  onObfsPasswordChange,
  readOnly = false,
}: Props) {
  const obfsOptions = schema?.fields.obfuscation_type?.options || [
    { value: "", label: "Без маскировки" },
    { value: "salamander", label: "Salamander" },
    { value: "gecko", label: "Gecko (требует совместимый клиент)" },
  ];

  return (
    <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
      <Input
        label="Пароль авторизации (Auth)"
        type="password"
        value={authentication ?? ""}
        placeholder={authentication === undefined ? "(не изменён)" : "Пароль Hysteria 2"}
        onChange={(e) => onAuthChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="SNI (Server Name)"
        value={sni}
        placeholder="example.com"
        onChange={(e) => onSniChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="Certificate SHA256 Pin"
        value={certSha256}
        placeholder="64 hex characters"
        onChange={(e) => onCertSha256Change(e.target.value)}
        disabled={readOnly}
      />
      <Select
        label="Тип обфускации (Obfs)"
        value={obfsType}
        onChange={(e) => onObfsTypeChange(e.target.value)}
        disabled={readOnly}
        options={obfsOptions}
      />
      {obfsType && (
        <Input
          label="Пароль обфускации"
          type="password"
          value={obfsPassword ?? ""}
          placeholder={obfsPassword === undefined ? "(не изменён)" : "Пароль обфускации"}
          onChange={(e) => onObfsPasswordChange(e.target.value)}
          disabled={readOnly}
        />
      )}
      <div className="sm:col-span-2">
        <label className="flex items-center gap-3 rounded-sm border border-border bg-surface-2 p-3 text-xs text-zinc-300">
          <input
            type="checkbox"
            checked={insecure}
            onChange={(e) => onInsecureChange(e.target.checked)}
            disabled={readOnly}
            className="h-4 w-4 accent-[var(--accent)]"
          />
          Allow Insecure TLS (skip certificate verification)
        </label>
        {insecure && (
          <div role="status" className="mt-2 flex items-start gap-2 rounded-sm border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-200">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>ВНИМАНИЕ: Отключение проверки сертификата TLS снижает уровень безопасности соединения!</span>
          </div>
        )}
      </div>
    </div>
  );
}

export function buildHysteria2Patch(
  initialSni: string, currentSni: string,
  initialInsecure: boolean, currentInsecure: boolean,
  initialCertSha: string, currentCertSha: string,
  initialObfsType: string, currentObfsType: string,
  authVal?: string,
  obfsPassVal?: string
): Hysteria2StructuredPatch | undefined {
  const patch: Hysteria2StructuredPatch = {};
  let touched = false;

  if (authVal !== undefined) {
    patch.authentication = { operation: "set", value: authVal };
    touched = true;
  }
  if (currentSni !== initialSni) {
    patch.sni = currentSni ? { operation: "set", value: currentSni } : { operation: "clear" };
    touched = true;
  }
  if (currentInsecure !== initialInsecure) {
    patch.insecure = { operation: "set", value: currentInsecure };
    touched = true;
  }
  if (currentCertSha !== initialCertSha) {
    patch.certificate_sha256 = currentCertSha ? { operation: "set", value: currentCertSha } : { operation: "clear" };
    touched = true;
  }
  if (currentObfsType !== initialObfsType) {
    patch.obfuscation_type = currentObfsType ? { operation: "set", value: currentObfsType } : { operation: "clear" };
    touched = true;
  }
  if (obfsPassVal !== undefined) {
    patch.obfuscation_password = obfsPassVal ? { operation: "set", value: obfsPassVal } : { operation: "clear" };
    touched = true;
  }

  return touched ? patch : undefined;
}
