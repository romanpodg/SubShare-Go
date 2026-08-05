"use client";

import { AlertTriangle } from "lucide-react";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import type { ProtocolSchemaDTO, TUICStructuredPatch } from "@/lib/types";

interface Props {
  schema?: ProtocolSchemaDTO;
  uuid?: string;
  onUuidChange: (val?: string) => void;
  password?: string;
  onPasswordChange: (val?: string) => void;
  sni: string;
  onSniChange: (val: string) => void;
  alpn: string;
  onAlpnChange: (val: string) => void;
  skipCertVerify: boolean;
  onSkipCertVerifyChange: (val: boolean) => void;
  congestionControl: string;
  onCongestionControlChange: (val: string) => void;
  udpRelayMode: string;
  onUdpRelayModeChange: (val: string) => void;
  udpOverStream: boolean;
  onUdpOverStreamChange: (val: boolean) => void;
  zeroRtt: boolean;
  onZeroRttChange: (val: boolean) => void;
  heartbeat: string;
  onHeartbeatChange: (val: string) => void;
  readOnly?: boolean;
}

export function TuicV5Fields({
  schema,
  uuid,
  onUuidChange,
  password,
  onPasswordChange,
  sni,
  onSniChange,
  alpn,
  onAlpnChange,
  skipCertVerify,
  onSkipCertVerifyChange,
  congestionControl,
  onCongestionControlChange,
  udpRelayMode,
  onUdpRelayModeChange,
  udpOverStream,
  onUdpOverStreamChange,
  zeroRtt,
  onZeroRttChange,
  heartbeat,
  onHeartbeatChange,
  readOnly = false,
}: Props) {
  const ccOptions = schema?.fields.congestion_controller?.options || [
    { value: "bbr", label: "BBR" },
    { value: "cubic", label: "CUBIC" },
    { value: "new_reno", label: "New Reno" },
  ];
  const udpOptions = schema?.fields.udp_relay_mode?.options || [
    { value: "native", label: "Native" },
    { value: "quic", label: "QUIC" },
  ];

  return (
    <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
      <Input
        label="UUID"
        type="password"
        value={uuid ?? ""}
        placeholder={uuid === undefined ? "(не изменён)" : "UUID"}
        onChange={(e) => onUuidChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="Пароль"
        type="password"
        value={password ?? ""}
        placeholder={password === undefined ? "(не изменён)" : "Пароль"}
        onChange={(e) => onPasswordChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="SNI"
        value={sni}
        placeholder="example.com"
        onChange={(e) => onSniChange(e.target.value)}
        disabled={readOnly}
      />
      <Input
        label="ALPN (через запятую)"
        value={alpn}
        placeholder="h3"
        onChange={(e) => onAlpnChange(e.target.value)}
        disabled={readOnly}
      />
      <Select
        label="Управление перегрузкой (Congestion)"
        value={congestionControl}
        onChange={(e) => onCongestionControlChange(e.target.value)}
        disabled={readOnly}
        options={ccOptions}
      />
      <Select
        label="Режим UDP Relay"
        value={udpRelayMode}
        onChange={(e) => onUdpRelayModeChange(e.target.value)}
        disabled={readOnly}
        options={udpOptions}
      />
      <Input
        label="Интервал Heartbeat"
        value={heartbeat}
        placeholder="10s"
        onChange={(e) => onHeartbeatChange(e.target.value)}
        disabled={readOnly}
      />
      <div className="flex flex-col justify-center space-y-2 rounded-sm border border-border bg-surface-2 p-3 text-xs text-zinc-300 sm:col-span-2">
        <label className="flex items-center gap-3">
          <input
            type="checkbox"
            checked={udpOverStream}
            onChange={(e) => onUdpOverStreamChange(e.target.checked)}
            disabled={readOnly}
            className="h-4 w-4 accent-[var(--accent)]"
          />
          UDP over Stream
        </label>
        <label className="flex items-center gap-3">
          <input
            type="checkbox"
            checked={zeroRtt}
            onChange={(e) => onZeroRttChange(e.target.checked)}
            disabled={readOnly}
            className="h-4 w-4 accent-[var(--accent)]"
          />
          Zero RTT Handshake
        </label>
        <label className="flex items-center gap-3">
          <input
            type="checkbox"
            checked={skipCertVerify}
            onChange={(e) => onSkipCertVerifyChange(e.target.checked)}
            disabled={readOnly}
            className="h-4 w-4 accent-[var(--accent)]"
          />
          Skip Certificate Verification
        </label>
      </div>
      {skipCertVerify && (
        <div role="status" className="flex items-start gap-2 rounded-sm border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-200 sm:col-span-2">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>ВНИМАНИЕ: Отключение проверки сертификата TLS снижает уровень безопасности соединения!</span>
        </div>
      )}
    </div>
  );
}

export function buildTUICPatch(
  initialSni: string, currentSni: string,
  initialAlpn: string, currentAlpn: string,
  initialSkipCert: boolean, currentSkipCert: boolean,
  initialCc: string, currentCc: string,
  initialUdpRelay: string, currentUdpRelay: string,
  initialUdpOverStream: boolean, currentUdpOverStream: boolean,
  initialZeroRtt: boolean, currentZeroRtt: boolean,
  initialHeartbeat: string, currentHeartbeat: string,
  uuidVal?: string,
  passVal?: string
): TUICStructuredPatch | undefined {
  const patch: TUICStructuredPatch = {};
  let touched = false;

  if (uuidVal !== undefined) {
    patch.uuid = { operation: "set", value: uuidVal };
    touched = true;
  }
  if (passVal !== undefined) {
    patch.password = { operation: "set", value: passVal };
    touched = true;
  }
  if (currentSni !== initialSni) {
    patch.sni = currentSni ? { operation: "set", value: currentSni } : { operation: "clear" };
    touched = true;
  }
  if (currentAlpn !== initialAlpn) {
    const list = currentAlpn.split(",").map((s) => s.trim()).filter(Boolean);
    patch.alpn = list.length > 0 ? { operation: "set", value: list } : { operation: "clear" };
    touched = true;
  }
  if (currentSkipCert !== initialSkipCert) {
    patch.skip_cert_verify = { operation: "set", value: currentSkipCert };
    touched = true;
  }
  if (currentCc !== initialCc) {
    patch.congestion_controller = currentCc ? { operation: "set", value: currentCc } : { operation: "clear" };
    touched = true;
  }
  if (currentUdpRelay !== initialUdpRelay) {
    patch.udp_relay_mode = currentUdpRelay ? { operation: "set", value: currentUdpRelay } : { operation: "clear" };
    touched = true;
  }
  if (currentUdpOverStream !== initialUdpOverStream) {
    patch.udp_over_stream = { operation: "set", value: currentUdpOverStream };
    touched = true;
  }
  if (currentZeroRtt !== initialZeroRtt) {
    patch.zero_rtt = { operation: "set", value: currentZeroRtt };
    touched = true;
  }
  if (currentHeartbeat !== initialHeartbeat) {
    patch.heartbeat = currentHeartbeat ? { operation: "set", value: currentHeartbeat } : { operation: "clear" };
    touched = true;
  }

  return touched ? patch : undefined;
}
