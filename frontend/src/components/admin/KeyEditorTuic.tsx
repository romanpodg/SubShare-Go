"use client";

import { TuicV5Fields } from "./protocol-editors/TuicV5Fields";
import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorTuic({ model }: { model: KeyEditorModel; }) {
  const {
    isSourceOwned,
    markStructuredChange,
    schemaForProtocol,
    setTuicAlpn,
    setTuicCc,
    setTuicHeartbeat,
    setTuicPassword,
    setTuicSkipCert,
    setTuicSni,
    setTuicUdpOverStream,
    setTuicUdpRelay,
    setTuicUuid,
    setTuicZeroRtt,
    tuicAlpn,
    tuicCc,
    tuicHeartbeat,
    tuicPassword,
    tuicSkipCert,
    tuicSni,
    tuicUdpOverStream,
    tuicUdpRelay,
    tuicUuid,
    tuicZeroRtt
  } = model;
  return (
    <TuicV5Fields
      schema={schemaForProtocol}
      uuid={tuicUuid}
      onUuidChange={(v) => { setTuicUuid(v); markStructuredChange(); }}
      password={tuicPassword}
      onPasswordChange={(v) => { setTuicPassword(v); markStructuredChange(); }}
      sni={tuicSni}
      onSniChange={(v) => { setTuicSni(v); markStructuredChange(); }}
      alpn={tuicAlpn}
      onAlpnChange={(v) => { setTuicAlpn(v); markStructuredChange(); }}
      skipCertVerify={tuicSkipCert}
      onSkipCertVerifyChange={(v) => { setTuicSkipCert(v); markStructuredChange(); }}
      congestionControl={tuicCc}
      onCongestionControlChange={(v) => { setTuicCc(v); markStructuredChange(); }}
      udpRelayMode={tuicUdpRelay}
      onUdpRelayModeChange={(v) => { setTuicUdpRelay(v); markStructuredChange(); }}
      udpOverStream={tuicUdpOverStream}
      onUdpOverStreamChange={(v) => { setTuicUdpOverStream(v); markStructuredChange(); }}
      zeroRtt={tuicZeroRtt}
      onZeroRttChange={(v) => { setTuicZeroRtt(v); markStructuredChange(); }}
      heartbeat={tuicHeartbeat}
      onHeartbeatChange={(v) => { setTuicHeartbeat(v); markStructuredChange(); }}
      readOnly={isSourceOwned}
    />
  );
}
