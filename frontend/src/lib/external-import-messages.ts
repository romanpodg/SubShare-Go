import type { ExternalSourcePreviewKey } from "@/lib/types";

const warningMessages: Record<string, string> = {
  partial_import: "Часть профилей не будет импортирована. Проверьте строки ниже.",
  base64_decoded: "Подписка была автоматически декодирована из Base64.",
};

const statusMessages: Record<string, string> = {
  accepted: "Готов к импорту",
  added: "Добавлен",
  rejected: "Отклонён",
  duplicate: "Дубликат",
  updated: "Будет обновлён",
  unchanged: "Без изменений",
  compatibility_only: "Только совместимый режим",
  ambiguous: "Неоднозначные параметры",
  unsupported: "Не поддерживается",
};

const reasonMessages: Record<string, string> = {
  unsupported_hysteria_v1: "Hysteria v1 пока не поддерживается.",
  unsupported_hysteria_version: "Распознана неподдерживаемая версия Hysteria.",
  ambiguous_hysteria_version: "Версию Hysteria определить не удалось: требуется settings.version или hysteriaSettings.version.",
  invalid_hysteria2_json: "Объект Hysteria2 содержит неполные или неподдерживаемые параметры.",
  unsupported_xray_protocol: "Протокол Xray распознан, но не поддерживается для импорта.",
  invalid_vless_json: "Некорректная конфигурация VLESS в XRAY-JSON.",
  invalid_vmess_json: "Некорректная конфигурация VMess в XRAY-JSON.",
  invalid_trojan_json: "Некорректная конфигурация Trojan в XRAY-JSON.",
  invalid_xray_json: "JSON не является поддерживаемой конфигурацией XRAY-JSON.",
  unsupported_protocol: "Протокол распознан, но не поддерживается.",
  unsupported_scheme: "Схема ссылки не поддерживается.",
  unsupported_sip008_format: "Формат SIP008 распознан, но импорт пока не поддерживается.",
};

const protocolLabels: Record<string, string> = {
  vless: "VLESS", vmess: "VMess", trojan: "Trojan", shadowsocks: "Shadowsocks",
  hysteria: "Hysteria v1", "hysteria-unknown": "Hysteria (версия не определена)", hysteria2: "Hysteria 2", tuic: "TUIC", "xray-json": "XRAY-JSON",
};

export function externalImportWarningMessage(code: string) {
  return warningMessages[code] || `Предупреждение импорта: ${code}`;
}

export function externalImportStatusMessage(status?: string) {
  return status ? statusMessages[status] || `Статус: ${status}` : "";
}

export function externalImportReasonMessage(code?: string) {
  return code ? reasonMessages[code] || `Причина: ${code}` : "";
}

export function externalImportProtocolLabel(protocol?: string) {
  return protocol ? protocolLabels[protocol] || protocol : "Неизвестный протокол";
}

export function safeExternalPreviewEndpoint(item: ExternalSourcePreviewKey) {
  if (!item.host) return "";
  const host = item.host.includes(":") && !item.host.startsWith("[") ? `[${item.host}]` : item.host;
  return item.port ? `${host}:${item.port}` : host;
}
