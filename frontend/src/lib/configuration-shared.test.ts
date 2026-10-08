import { describe, expect, it } from "vitest";
import fixtures from "../../../testdata/configuration/xray-shared.json";
import { parseXrayJSONConfiguration, patchXrayJSONConfiguration } from "./configuration";
import {
  configurationByteLength,
  CONFIG_LIMIT,
  formatXrayJSON,
  formatXrayJSONWithinLimit,
} from "./xray-json-document";

describe("shared Go and frontend supported Xray projection", () => {
  it.each(fixtures)("projects $name from the same fixture", ({ raw, expected }) => {
    const parsed = parseXrayJSONConfiguration(raw);
    expect(parsed?.outboundIndex).toBe(1);
    expect(parsed?.draft).toMatchObject(expected);
    // Go expands all supported outbounds; the editor selects the first.
    expect(parsed?.config.outbounds).toHaveLength(3);
  });
});

function boundaryDocument(bytes: number, formatted: boolean) {
  const raw = '{"outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"edge.example","port":2443,"password":"fixture-password"}]}}],"unicode":"Ж🌍","precision":900719925474099312345,"escaped":"\\u2603","padding":""}';
  const base = formatted ? formatXrayJSON(raw) : raw;
  return base.replace('"padding": ""', '"padding": "' + "x".repeat(bytes - configurationByteLength(base)) + '"')
    .replace('"padding":""', '"padding":"' + "x".repeat(bytes - configurationByteLength(base)) + '"');
}

describe("Xray document UTF-8 size boundary", () => {
  it.each([CONFIG_LIMIT - 1, CONFIG_LIMIT, CONFIG_LIMIT + 1])("formats a %i-byte document according to formatted size", (bytes) => {
    const document = boundaryDocument(bytes, true);
    expect(configurationByteLength(document)).toBe(bytes);
    const result = formatXrayJSONWithinLimit(document);
    expect(result.formatted).toBe(bytes <= CONFIG_LIMIT);
    expect(result.exceededLimit).toBe(bytes > CONFIG_LIMIT);
    expect(result.value).toBe(document);
  });

  it.each([CONFIG_LIMIT - 1, CONFIG_LIMIT, CONFIG_LIMIT + 1])("preserves raw tokens while patching a %i-byte document", (bytes) => {
    const raw = boundaryDocument(bytes, false);
    expect(configurationByteLength(raw)).toBe(bytes);
    const expected = formatXrayJSON(raw.replace("edge.example", "next.example"));
    const result = patchXrayJSONConfiguration(raw, { server: "next.example" });
    expect(result).toBe(expected);
    expect(result).toContain('"precision": 900719925474099312345');
    expect(result).toContain('"escaped": "\\u2603"');
    expect(result).toContain('"unicode": "Ж🌍"');
    expect(patchXrayJSONConfiguration(result, { server: "next.example" })).toBe(result);
    expect(formatXrayJSONWithinLimit(raw)).toMatchObject({ formatted: false, exceededLimit: true, value: raw });
  });
});
