export const ADMIN_PASSWORD_MIN_CODE_POINTS = 15;
export const ADMIN_PASSWORD_MAX_CODE_POINTS = 256;
export const ADMIN_PASSWORD_MAX_ENCODED_BYTES = 1024;

// HTML maxlength counts UTF-16 code units. Two code units per allowed Unicode
// code point keeps supplementary characters usable; the validator below is
// the authoritative code-point and UTF-8 byte check.
export const ADMIN_PASSWORD_INPUT_MAX_LENGTH = ADMIN_PASSWORD_MAX_CODE_POINTS * 2;

export const ADMIN_PASSWORD_TOO_SHORT_MESSAGE =
  "password must contain at least 15 Unicode characters";
export const ADMIN_PASSWORD_TOO_LONG_MESSAGE =
  "password must contain at most 256 Unicode characters";
export const ADMIN_PASSWORD_TOO_MANY_BYTES_MESSAGE =
  "password encoded length must not exceed 1024 bytes";

export function validateNewAdministratorPassword(value: string): string | null {
  if (new TextEncoder().encode(value).byteLength > ADMIN_PASSWORD_MAX_ENCODED_BYTES) {
    return ADMIN_PASSWORD_TOO_MANY_BYTES_MESSAGE;
  }
  const codePoints = Array.from(value).length;
  if (codePoints < ADMIN_PASSWORD_MIN_CODE_POINTS) {
    return ADMIN_PASSWORD_TOO_SHORT_MESSAGE;
  }
  if (codePoints > ADMIN_PASSWORD_MAX_CODE_POINTS) {
    return ADMIN_PASSWORD_TOO_LONG_MESSAGE;
  }
  return null;
}
