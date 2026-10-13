import type { XrayJSONDraft } from "./configuration-types";
import { parseConfiguration } from "./configuration-link-codec";
import { projectLinkDraft } from "./configuration-link-projection";
import { serializeXrayDraftLink } from "./configuration-link-serialization";
import { parseXrayJSONConfiguration } from "./configuration-xray-reader";
import { buildXrayJSONConfiguration } from "./configuration-xray-builder";
import { formatXrayJSON } from "./xray-json-document";

export function createXrayJSONFromConfiguration(raw: string) {
  const parsed = parseConfiguration(raw);
  if (!parsed) throw new Error("Сначала заполните конфигурацию в формате ссылки");
  return buildXrayJSONConfiguration(projectLinkDraft(parsed));
}

export function parseEditableConfiguration(raw: string): XrayJSONDraft {
  const converted = createXrayJSONFromConfiguration(raw);
  const parsed = parseXrayJSONConfiguration(converted);
  if (!parsed) throw new Error("Не удалось разобрать конфигурацию");
  return parsed.draft;
}

export function createConfigurationFromXrayJSON(raw: string) {
  const parsed = parseXrayJSONConfiguration(formatXrayJSON(raw));
  if (!parsed) throw new Error("Сначала заполните XRAY-JSON конфигурацию");
  return serializeXrayDraftLink(parsed.draft);
}

export function isXrayJSONConfiguration(raw: string) {
  try { return Boolean(parseXrayJSONConfiguration(raw)); }
  catch { return false; }
}
