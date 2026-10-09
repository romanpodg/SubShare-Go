import { buildXrayJSONConfiguration, createConfigurationFromXrayJSON, parseEditableConfiguration, parseXrayJSONConfiguration } from "@/lib/configuration";
import { formatXrayJSONWithinLimit, inspectXrayJSONDocument } from "@/lib/xray-json-document";
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

export function projectEditorRawDisplay(protocol: ExternalProfileProtocol, raw: string) {
  const original = { value: raw, warning: "" };
  if (!isXrayJSONRaw(protocol, raw)) return original;
  try {
    const inspection = inspectXrayJSONDocument(raw);
    if (inspection.duplicateKeys.length > 0) {
      return { value: raw, warning: "JSON оставлен без форматирования: обнаружены повторяющиеся ключи." };
    }
    const formatted = formatXrayJSONWithinLimit(raw);
    return {
      value: formatted.value,
      warning: formatted.exceededLimit ? "JSON оставлен без форматирования: форматированная версия превышает допустимый размер." : "",
    };
  } catch {
    return original;
  }
}
