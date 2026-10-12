import { expect, it } from "vitest";
import fixtures from "../../../testdata/configuration/xray-interpretation.json";
import { parseXrayJSONConfiguration } from "./configuration";

it.each(fixtures)("preserves the frontend interpretation boundary: $name", (fixture) => {
  if (fixture.frontend.error) {
    expect(() => parseXrayJSONConfiguration(fixture.raw)).toThrow(fixture.frontend.error);
    return;
  }
  const parsed = parseXrayJSONConfiguration(fixture.raw);
  expect(parsed?.draft ?? null).toEqual(fixture.frontend.draft);
  expect(parsed?.outboundIndex ?? null).toBe(fixture.frontend.outbound_index);
});
