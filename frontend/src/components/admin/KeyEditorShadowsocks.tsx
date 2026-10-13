"use client";

import { ShadowsocksFields } from "./protocol-editors/ShadowsocksFields";
import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorShadowsocks({ model }: { model: KeyEditorModel; }) {
  const {
    isSourceOwned,
    markStructuredChange,
    schemaForProtocol,
    setSsMethod,
    setSsPassword,
    setSsPluginName,
    setSsPluginOptions,
    ssMethod,
    ssPassword,
    ssPluginName,
    ssPluginOptions
  } = model;
  return (
    <ShadowsocksFields
      schema={schemaForProtocol}
      method={ssMethod}
      onMethodChange={(v) => { setSsMethod(v); markStructuredChange(); }}
      password={ssPassword}
      onPasswordChange={(v) => { setSsPassword(v); markStructuredChange(); }}
      pluginName={ssPluginName}
      onPluginNameChange={(v) => { setSsPluginName(v); markStructuredChange(); }}
      pluginOptions={ssPluginOptions}
      onPluginOptionsChange={(v) => { setSsPluginOptions(v); markStructuredChange(); }}
      readOnly={isSourceOwned}
    />
  );
}
