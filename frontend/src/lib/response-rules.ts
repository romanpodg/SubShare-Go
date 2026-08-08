import type { ResponseRule, ResponseRuleCondition } from "@/lib/types";

export type ResponseRuleDraft = Omit<ResponseRule, "id" | "is_system" | "created_at" | "updated_at">;

const conditionOperators = new Set<ResponseRuleCondition["operator"]>([
  "EQUALS", "NOT_EQUALS", "CONTAINS", "NOT_CONTAINS", "STARTS_WITH",
  "NOT_STARTS_WITH", "ENDS_WITH", "NOT_ENDS_WITH", "REGEX", "NOT_REGEX",
]);
const responseTypes = new Set<ResponseRuleDraft["response_type"]>([
  "browser", "base64", "plain", "xray-json", "mihomo", "sing-box", "block", "not-found",
]);
const forbiddenHeaders = new Set([
  "set-cookie", "content-length", "transfer-encoding", "connection", "content-type",
  "content-disposition", "strict-transport-security", "access-control-allow-origin",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function expectString(value: unknown, field: string, maxLength: number, required = true): string {
  if (typeof value !== "string") throw new Error(`Field \"${field}\" must be a string.`);
  const trimmed = value.trim();
  if (required && trimmed.length === 0) throw new Error(`Field \"${field}\" cannot be empty.`);
  if (trimmed.length > maxLength) throw new Error(`Field \"${field}\" must not exceed ${maxLength} characters.`);
  return trimmed;
}

function parseCondition(value: unknown, index: number): ResponseRuleCondition {
  if (!isRecord(value)) throw new Error(`Condition ${index + 1} must be an object.`);
  const headerName = expectString(value.headerName, `conditions[${index}].headerName`, 80).toLowerCase();
  if (/[\r\n:]/.test(headerName)) throw new Error(`Condition ${index + 1} has an invalid header name.`);
  if (typeof value.operator !== "string" || !conditionOperators.has(value.operator as ResponseRuleCondition["operator"])) {
    throw new Error(`Condition ${index + 1} has an unsupported operator.`);
  }
  const conditionValue = expectString(value.value, `conditions[${index}].value`, 255);
  if (typeof value.caseSensitive !== "boolean") throw new Error(`Condition ${index + 1} caseSensitive must be boolean.`);
  if (value.operator === "REGEX" || value.operator === "NOT_REGEX") {
    try { new RegExp(conditionValue); } catch { throw new Error(`Condition ${index + 1} has an invalid regular expression.`); }
  }
  return { headerName, operator: value.operator as ResponseRuleCondition["operator"], value: conditionValue, caseSensitive: value.caseSensitive };
}

function parseHeader(value: unknown, index: number): { key: string; value: string } {
  if (!isRecord(value)) throw new Error(`Header ${index + 1} must be an object.`);
  const key = expectString(value.key, `headers[${index}].key`, 80);
  const headerValue = expectString(value.value, `headers[${index}].value`, 1024, false);
  if (/[\r\n]/.test(key + headerValue) || forbiddenHeaders.has(key.toLowerCase())) {
    throw new Error(`Header ${index + 1} is unsafe.`);
  }
  return { key, value: headerValue };
}

/** Parses JSON mode input into exactly the draft represented by the visual editor. */
export function parseResponseRuleDraftJSON(raw: string): ResponseRuleDraft {
  let parsed: unknown;
  try { parsed = JSON.parse(raw); } catch { throw new Error("JSON contains a syntax error."); }
  if (!isRecord(parsed)) throw new Error("Rule JSON must be an object, not an array or null.");

  const name = expectString(parsed.name, "name", 80);
  const description = expectString(parsed.description, "description", 250, false);
  if (typeof parsed.enabled !== "boolean") throw new Error("Field \"enabled\" must be boolean.");
  if (!Number.isInteger(parsed.priority) || (parsed.priority as number) < 0 || (parsed.priority as number) > 100000) {
    throw new Error("Field \"priority\" must be an integer from 0 to 100000.");
  }
  if (parsed.operator !== "AND" && parsed.operator !== "OR") throw new Error("Field \"operator\" must be AND or OR.");
  if (!Array.isArray(parsed.conditions) || parsed.conditions.length > 20) {
    throw new Error("Field \"conditions\" must be an array with at most 20 conditions.");
  }
  if (typeof parsed.response_type !== "string" || !responseTypes.has(parsed.response_type as ResponseRuleDraft["response_type"])) {
    throw new Error("Field \"response_type\" contains an unsupported response type.");
  }
  if (parsed.template_id !== null && (!Number.isInteger(parsed.template_id) || (parsed.template_id as number) < 1)) {
    throw new Error("Field \"template_id\" must be a positive integer or null.");
  }
  if (!Array.isArray(parsed.headers) || parsed.headers.length > 30) {
    throw new Error("Field \"headers\" must be an array with at most 30 headers.");
  }
  return {
    name,
    description,
    enabled: parsed.enabled,
    priority: parsed.priority as number,
    operator: parsed.operator,
    conditions: parsed.conditions.map(parseCondition),
    response_type: parsed.response_type as ResponseRuleDraft["response_type"],
    template_id: parsed.template_id as number | null,
    headers: parsed.headers.map(parseHeader),
  };
}
