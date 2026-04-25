export function normalizeDateTimeLocalValue(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  const isoLikeMatch = trimmed.match(/^(\d{4}-\d{2}-\d{2})[T\s](\d{2}:\d{2})/);
  if (isoLikeMatch) {
    return `${isoLikeMatch[1]}T${isoLikeMatch[2]}`;
  }

  const slashMatch = trimmed.match(/^(\d{2})\/(\d{2})\/(\d{4})\s+(\d{2}):(\d{2})$/);
  if (slashMatch) {
    const [, day, month, year, hours, minutes] = slashMatch;
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  }

  return trimmed;
}
