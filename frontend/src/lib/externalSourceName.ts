export const MAX_EXTERNAL_SOURCE_NAME_CODE_POINTS = 64;

// The HTML maxLength property counts UTF-16 code units, so this allows the
// longest valid 64-code-point name while validation below enforces the API limit.
export const EXTERNAL_SOURCE_NAME_INPUT_MAX_LENGTH = MAX_EXTERNAL_SOURCE_NAME_CODE_POINTS * 2;

export function validateExternalSourceName(value: string): string | null {
  const name = value.trim();
  if (!name) return "Name is required";
  if (Array.from(name).length > MAX_EXTERNAL_SOURCE_NAME_CODE_POINTS) {
    return `Name is too long (max ${MAX_EXTERNAL_SOURCE_NAME_CODE_POINTS} characters)`;
  }
  return null;
}
