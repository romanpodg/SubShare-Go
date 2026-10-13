"use client";

import { KeyEditorConnection, KeyEditorNativeReveal, KeyEditorTuicReadOnly } from "./KeyEditorConnection";
import { KeyEditorHysteria2 } from "./KeyEditorHysteria2";
import { KeyEditorShadowsocks } from "./KeyEditorShadowsocks";
import { KeyEditorTuic } from "./KeyEditorTuic";
import { isLegacyEditorProtocol } from "./protocol-editor-capabilities";
import { LegacyXrayFields } from "./protocol-editors/LegacyXrayFields";
import type { KeyEditorModel } from "./use-key-editor-model";

function NativeProtocolFields({ model }: { model: KeyEditorModel; }) {
  switch (model.protocol) {
    case "shadowsocks": return <KeyEditorShadowsocks model={model} />;
    case "hysteria2": return <KeyEditorHysteria2 model={model} />;
    case "tuic": return <TuicProfileFields model={model} />;
    default: return null;
  }
}

function TuicProfileFields({ model }: { model: KeyEditorModel; }) {
  if (model.detail?.safe_structured?.tuic?.generation === 4) return null;
  return <KeyEditorTuic model={model} />;
}

export function KeyEditorStructuredFields({ model }: { model: KeyEditorModel; }) {
  const { mode, protocol, legacyDraft, changeLegacyDraft, isSourceOwned, legacySecretsRevealed, handleReveal } = model;
  return (
    <div className={mode === "structured" ? "space-y-4" : "hidden space-y-4"}>
      <KeyEditorTuicReadOnly model={model} />
      {isLegacyEditorProtocol(protocol) && <LegacyXrayFields draft={legacyDraft} onChange={changeLegacyDraft} readOnly={Boolean(isSourceOwned)} secretsRevealed={legacySecretsRevealed} onReveal={() => void handleReveal("raw")} />}
      <KeyEditorConnection model={model} />
      <KeyEditorNativeReveal model={model} />
      <NativeProtocolFields model={model} />
    </div>
  );
}
