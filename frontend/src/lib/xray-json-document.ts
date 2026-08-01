import {
  applyEdits,
  format,
  modify,
  parseTree,
  printParseErrorCode,
  type JSONPath,
  type Node as JSONNode,
  type ParseError,
} from "jsonc-parser";

export const CONFIG_LIMIT = 65_535;

const STRICT_PARSE_OPTIONS = {
  allowEmptyContent: false,
  allowTrailingComma: false,
  disallowComments: true,
} as const;

const FORMATTING_OPTIONS = {
  eol: "\n",
  insertFinalNewline: false,
  insertSpaces: true,
  keepLines: false,
  tabSize: 2,
} as const;

export interface DuplicateJSONKey {
  key: string;
  path: JSONPath;
}

export interface XrayJSONDocumentInspection {
  duplicateKeys: DuplicateJSONKey[];
  root: JSONNode;
}

export class DuplicateJSONKeyError extends Error {
  readonly duplicateKeys: DuplicateJSONKey[];

  constructor(duplicateKeys: DuplicateJSONKey[]) {
    const first = duplicateKeys[0];
    const location = first ? formatJSONPath(first.path) : "$";
    super(`XRAY-JSON содержит повторяющийся ключ ${location}`);
    this.name = "DuplicateJSONKeyError";
    this.duplicateKeys = duplicateKeys;
  }
}

function formatJSONPath(path: JSONPath) {
  return path.reduce<string>((result, segment) => {
    if (typeof segment === "number") {
      return `${result}[${segment}]`;
    }
    if (/^[A-Za-z_$][\w$]*$/.test(segment)) {
      return `${result}.${segment}`;
    }
    return `${result}[${JSON.stringify(segment)}]`;
  }, "$");
}

function collectDuplicateKeys(
  node: JSONNode,
  path: JSONPath,
  duplicates: DuplicateJSONKey[]
) {
  if (node.type === "object") {
    const seen = new Set<string>();
    for (const property of node.children ?? []) {
      const keyNode = property.children?.[0];
      const valueNode = property.children?.[1];
      if (!keyNode || typeof keyNode.value !== "string") continue;

      const key = keyNode.value;
      const propertyPath = [...path, key];
      if (seen.has(key)) {
        duplicates.push({ key, path: propertyPath });
      } else {
        seen.add(key);
      }
      if (valueNode) {
        collectDuplicateKeys(valueNode, propertyPath, duplicates);
      }
    }
    return;
  }

  if (node.type === "array") {
    (node.children ?? []).forEach((child, index) => {
      collectDuplicateKeys(child, [...path, index], duplicates);
    });
  }
}

function syntaxError(errors: ParseError[]) {
  const first = errors[0];
  const detail = first ? printParseErrorCode(first.error) : "Invalid JSON";
  return new Error(`XRAY-JSON содержит ошибку синтаксиса (${detail})`);
}

export function inspectXrayJSONDocument(raw: string): XrayJSONDocumentInspection {
  const errors: ParseError[] = [];
  const root = parseTree(raw, errors, STRICT_PARSE_OPTIONS);
  if (!root || errors.length > 0) {
    throw syntaxError(errors);
  }
  if (root.type !== "object") {
    throw new Error("XRAY-JSON должен быть JSON-объектом");
  }

  const duplicateKeys: DuplicateJSONKey[] = [];
  collectDuplicateKeys(root, [], duplicateKeys);
  return { duplicateKeys, root };
}

export function configurationByteLength(value: string) {
  return new TextEncoder().encode(value).length;
}

export function formatXrayJSON(raw: string) {
  const trimmed = raw.trim();
  const { duplicateKeys } = inspectXrayJSONDocument(trimmed);
  if (duplicateKeys.length > 0) {
    throw new DuplicateJSONKeyError(duplicateKeys);
  }
  return applyEdits(trimmed, format(trimmed, undefined, FORMATTING_OPTIONS));
}

export function formatXrayJSONWithinLimit(raw: string, limit = CONFIG_LIMIT) {
  const original = raw.trim();
  const formatted = formatXrayJSON(original);
  if (configurationByteLength(formatted) <= limit) {
    return {
      exceededLimit: false,
      formatted: true,
      value: formatted,
    } as const;
  }
  return {
    exceededLimit: true,
    formatted: false,
    value: original,
  } as const;
}

export function modifyXrayJSONPath(raw: string, path: JSONPath, value: unknown) {
  const { duplicateKeys } = inspectXrayJSONDocument(raw);
  if (duplicateKeys.length > 0) {
    throw new DuplicateJSONKeyError(duplicateKeys);
  }
  return applyEdits(
    raw,
    modify(raw, path, value, {
      formattingOptions: FORMATTING_OPTIONS,
    })
  );
}
