import { describe, expect, it } from "vitest";
import {
  ADMIN_PASSWORD_TOO_LONG_MESSAGE,
  ADMIN_PASSWORD_TOO_MANY_BYTES_MESSAGE,
  ADMIN_PASSWORD_TOO_SHORT_MESSAGE,
  validateNewAdministratorPassword,
} from "./adminPassword";

describe("administrator password policy", () => {
  it("counts Unicode code points at the lower boundary", () => {
    expect(validateNewAdministratorPassword("я".repeat(14))).toBe(
      ADMIN_PASSWORD_TOO_SHORT_MESSAGE,
    );
    expect(validateNewAdministratorPassword("я".repeat(15))).toBeNull();
  });

  it("accepts 256 supplementary characters and rejects values above the limit", () => {
    expect(validateNewAdministratorPassword("😀".repeat(256))).toBeNull();
    expect(validateNewAdministratorPassword("a".repeat(257))).toBe(
      ADMIN_PASSWORD_TOO_LONG_MESSAGE,
    );
  });

  it("preserves Unicode and surrounding spaces instead of trimming", () => {
    expect(validateNewAdministratorPassword("  пароль с пробелами  ")).toBeNull();
    expect(validateNewAdministratorPassword("             a ")).toBeNull();
    expect(validateNewAdministratorPassword("a")).toBe(ADMIN_PASSWORD_TOO_SHORT_MESSAGE);
  });

  it("enforces the defensive UTF-8 byte limit", () => {
    expect(validateNewAdministratorPassword("😀".repeat(257))).toBe(
      ADMIN_PASSWORD_TOO_MANY_BYTES_MESSAGE,
    );
  });
});
