import { buildXrayJSONConfiguration, createConfigurationFromXrayJSON, parseEditableConfiguration, parseXrayJSONConfiguration } from "@/lib/configuration";
import { inspectXrayJSONDocument } from "@/lib/xray-json-document";
import type { ExternalProfileProtocol } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import { isLegacyEditorProtocol } from "./protocol-editor-capabilities";

export function isXrayJSONRaw(protocol: ExternalProfileProtocol, raw: string) {
  return protocol === "xray-json" || raw.trim().startsWith("{");
}

export function validateEditorRaw(protocol: ExternalProfileProtocol, value: string) {
  if (!value.trim()) throw new Error("Сначала заполните raw-конфигурацию");
  if (isXrayJSONRaw(protocol, value)) {
    const inspection = inspectXrayJSONDocument(value);
    if (inspection.duplicateKeys.length > 0) {
      throw new Error("XRAY-JSON содержит повторяющиеся ключи. Устраните неоднозначность перед сохранением.");
    }
    parseXrayJSONConfiguration(value);
  } else if (isLegacyEditorProtocol(protocol)) parseEditableConfiguration(value);
}

export function buildLegacyCreateRaw(state: EditorCommandState) {
  return createConfigurationFromXrayJSON(buildXrayJSONConfiguration({
    ...state.legacyDraft,
    remark: state.displayName.trim() || state.label.trim(),
  }));
}
