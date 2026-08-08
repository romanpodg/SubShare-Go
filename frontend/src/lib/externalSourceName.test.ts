import { describe, expect, it } from "vitest";
import {
  EXTERNAL_SOURCE_NAME_INPUT_MAX_LENGTH,
  validateExternalSourceName,
} from "./externalSourceName";

describe("validateExternalSourceName", () => {
  it("counts Unicode code points instead of UTF-16 code units", () => {
    expect(validateExternalSourceName("a".repeat(64))).toBeNull();
    expect(validateExternalSourceName("a".repeat(65))).toMatch(/max 64/);
    expect(validateExternalSourceName("Я".repeat(64))).toBeNull();
    expect(validateExternalSourceName("Я".repeat(65))).toMatch(/max 64/);
    expect(validateExternalSourceName("😀".repeat(64))).toBeNull();
    expect(validateExternalSourceName("😀".repeat(65))).toMatch(/max 64/);
    expect(EXTERNAL_SOURCE_NAME_INPUT_MAX_LENGTH).toBe(128);
  });

  it("rejects blank names after trimming whitespace", () => {
    expect(validateExternalSourceName("  Provider  ")).toBeNull();
    expect(validateExternalSourceName(" \t\n ")).toBe("Name is required");
  });
});
