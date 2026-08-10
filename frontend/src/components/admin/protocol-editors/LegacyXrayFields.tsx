import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import type { XrayJSONDraft, XrayJSONPatch } from "@/lib/configuration";

interface Props {
  draft: XrayJSONDraft;
  onChange: (patch: XrayJSONPatch) => void;
  readOnly?: boolean;
  secretsRevealed: boolean;
  onReveal: () => void;
}

export function LegacyXrayFields({ draft, onChange, readOnly = false, secretsRevealed, onReveal }: Props) {
  const secretLabel = draft.protocol === "trojan" ? "Пароль" : "UUID / ID";
  return (
    <div className="space-y-4" data-testid="legacy-structured-editor">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Input label="Сервер (Host)" value={draft.server} onChange={(e) => onChange({ server: e.target.value })} disabled={readOnly} />
        <Input label="Порт" value={draft.port} onChange={(e) => onChange({ port: e.target.value })} disabled={readOnly} />
        <Input
          label={secretLabel}
          value={secretsRevealed ? draft.identifier : ""}
          placeholder={secretsRevealed ? "" : "Скрыто до раскрытия"}
          onChange={(e) => onChange({ identifier: e.target.value })}
          disabled={readOnly || !secretsRevealed}
        />
      </div>

      {!readOnly && !secretsRevealed && (
        <button type="button" onClick={onReveal} className="text-xs font-medium text-accent hover:underline">
          Раскрыть учетные данные и параметры транспорта
        </button>
      )}

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Select label="Транспорт" value={draft.network} onChange={(e) => onChange({ network: e.target.value as XrayJSONDraft["network"] })} disabled={readOnly || !secretsRevealed} options={[
          { value: "tcp", label: "TCP / RAW" }, { value: "ws", label: "WebSocket" }, { value: "grpc", label: "gRPC" },
          { value: "httpupgrade", label: "HTTP Upgrade" }, { value: "xhttp", label: "XHTTP" }, { value: "h2", label: "HTTP/2" }, { value: "quic", label: "QUIC" },
        ]} />
        <Select label="Защита" value={draft.security} onChange={(e) => onChange({ security: e.target.value as XrayJSONDraft["security"] })} disabled={readOnly || !secretsRevealed} options={[
          { value: "none", label: "Нет" }, { value: "tls", label: "TLS" }, { value: "reality", label: "Reality" },
        ]} />
        <Input label={draft.network === "grpc" ? "Service name" : "Path"} value={draft.path} onChange={(e) => onChange({ path: e.target.value })} disabled={readOnly || !secretsRevealed} />
        <Input label="Host / Authority" value={draft.network === "grpc" ? draft.grpcAuthority || "" : draft.host} onChange={(e) => onChange(draft.network === "grpc" ? { grpcAuthority: e.target.value } : { host: e.target.value })} disabled={readOnly || !secretsRevealed} />
        <Input label="SNI" value={draft.sni} onChange={(e) => onChange({ sni: e.target.value })} disabled={readOnly || !secretsRevealed} />
        <Input label="ALPN" value={draft.alpn} onChange={(e) => onChange({ alpn: e.target.value })} disabled={readOnly || !secretsRevealed} />
        <Input label="Fingerprint" value={draft.fingerprint || ""} onChange={(e) => onChange({ fingerprint: e.target.value })} disabled={readOnly || !secretsRevealed} />
        {draft.network === "tcp" && <Input label="TCP header" value={draft.headerType || ""} onChange={(e) => onChange({ headerType: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.protocol === "vless" && <Input label="Flow" value={draft.flow || ""} onChange={(e) => onChange({ flow: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.protocol === "vless" && <Input label="Encryption" value={draft.encryption || ""} onChange={(e) => onChange({ encryption: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.protocol === "vmess" && <Input label="VMess security" value={draft.vmessSecurity || ""} onChange={(e) => onChange({ vmessSecurity: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.protocol === "vmess" && <Input label="Alter ID" value={draft.vmessAlterId || ""} onChange={(e) => onChange({ vmessAlterId: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.security === "reality" && <Input label="Reality public key" value={draft.publicKey || ""} onChange={(e) => onChange({ publicKey: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.security === "reality" && <Input label="Reality short ID" value={draft.shortId || ""} onChange={(e) => onChange({ shortId: e.target.value })} disabled={readOnly || !secretsRevealed} />}
        {draft.security === "reality" && <Input label="Reality SpiderX" value={draft.spiderX || ""} onChange={(e) => onChange({ spiderX: e.target.value })} disabled={readOnly || !secretsRevealed} />}
      </div>
      <label className="flex items-center gap-2 text-xs text-zinc-300">
        <input type="checkbox" checked={Boolean(draft.allowInsecure)} onChange={(e) => onChange({ allowInsecure: e.target.checked })} disabled={readOnly || !secretsRevealed} className="accent-accent" />
        Разрешить небезопасный TLS
      </label>
    </div>
  );
}
