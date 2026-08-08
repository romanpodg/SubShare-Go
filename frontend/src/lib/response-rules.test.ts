import { describe, expect, it } from "vitest";
import { parseResponseRuleDraftJSON } from "./response-rules";

const validRule = {
  name: "Desktop clients",
  description: "Client-specific response",
  enabled: true,
  priority: 100,
  operator: "AND",
  conditions: [{ headerName: "User-Agent", operator: "CONTAINS", value: "Happ", caseSensitive: false }],
  response_type: "base64",
  template_id: null,
  headers: [{ key: "X-Subscription-Title", value: "SubShare" }],
};

describe("parseResponseRuleDraftJSON", () => {
  it("normalizes a valid visual-editor draft", () => {
    expect(parseResponseRuleDraftJSON(JSON.stringify(validRule))).toEqual({
      ...validRule,
      name: "Desktop clients",
      description: "Client-specific response",
      conditions: [{ ...validRule.conditions[0], headerName: "user-agent" }],
    });
  });

  it.each([
    ["array", []],
    ["null", null],
    ["missing required field", { ...validRule, conditions: undefined }],
    ["bad condition shape", { ...validRule, conditions: [{ headerName: "user-agent" }] }],
    ["unsupported condition operator", { ...validRule, conditions: [{ ...validRule.conditions[0], operator: "MATCHES" }] }],
    ["invalid priority", { ...validRule, priority: 1.5 }],
    ["unsupported response type", { ...validRule, response_type: "yaml" }],
    ["unsafe header", { ...validRule, headers: [{ key: "Content-Type", value: "text/plain" }] }],
  ])("rejects %s", (_label, value) => {
    expect(() => parseResponseRuleDraftJSON(JSON.stringify(value))).toThrow();
  });
});
