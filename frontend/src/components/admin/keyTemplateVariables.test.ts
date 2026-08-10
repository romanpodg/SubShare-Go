import { describe, expect, it } from "vitest";
import { informationalTemplatePreviewParts, keyTemplateVariables } from "./keyTemplateVariables";

describe("informational key template variables", () => {
  it("documents exactly the backend renderer placeholders", () => {
    expect(keyTemplateVariables.map((variable) => variable.token)).toEqual([
      "{user_name}",
      "{telegram}",
      "{subscription_id}",
      "{expires_date}",
      "{expires_at}",
      "{real_keys_count}",
    ]);
  });

  it("renders multiple sample variables and preserves unknown placeholders", () => {
    expect(informationalTemplatePreviewParts("Hi {user_name}: {expires_date} {unknown}"))
      .toEqual([
        { text: "Hi ", unknown: false },
        { text: "Иван", unknown: false },
        { text: ": ", unknown: false },
        { text: "25/08/2026", unknown: false },
        { text: " ", unknown: false },
        { text: "{unknown}", unknown: true },
      ]);
  });

  it("keeps an empty template empty", () => {
    expect(informationalTemplatePreviewParts("")).toEqual([]);
  });
});
