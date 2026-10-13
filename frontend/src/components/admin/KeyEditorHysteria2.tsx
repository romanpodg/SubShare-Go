"use client";

import { Hysteria2Fields } from "./protocol-editors/Hysteria2Fields";
import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorHysteria2({ model }: { model: KeyEditorModel; }) {
  const {
    hy2Auth,
    hy2CertSha,
    hy2Insecure,
    hy2ObfsPassword,
    hy2ObfsType,
    hy2Sni,
    isSourceOwned,
    markStructuredChange,
    schemaForProtocol,
    setHy2Auth,
    setHy2CertSha,
    setHy2Insecure,
    setHy2ObfsPassword,
    setHy2ObfsType,
    setHy2Sni
  } = model;
  return (
    <Hysteria2Fields
      schema={schemaForProtocol}
      authentication={hy2Auth}
      onAuthChange={(v) => { setHy2Auth(v); markStructuredChange(); }}
      sni={hy2Sni}
      onSniChange={(v) => { setHy2Sni(v); markStructuredChange(); }}
      insecure={hy2Insecure}
      onInsecureChange={(v) => { setHy2Insecure(v); markStructuredChange(); }}
      certSha256={hy2CertSha}
      onCertSha256Change={(v) => { setHy2CertSha(v); markStructuredChange(); }}
      obfsType={hy2ObfsType}
      onObfsTypeChange={(v) => { setHy2ObfsType(v); markStructuredChange(); }}
      obfsPassword={hy2ObfsPassword}
      onObfsPasswordChange={(v) => { setHy2ObfsPassword(v); markStructuredChange(); }}
      readOnly={isSourceOwned}
    />
  );
}
