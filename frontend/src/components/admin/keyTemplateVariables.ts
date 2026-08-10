export const keyTemplateVariables = [
  { token: "{user_name}", description: "Имя пользователя", sample: "Иван" },
  { token: "{telegram}", description: "Telegram username без @", sample: "ivan_example" },
  { token: "{subscription_id}", description: "ID подписки", sample: "example-subscription" },
  { token: "{expires_date}", description: "Дата окончания", sample: "25/08/2026" },
  { token: "{expires_at}", description: "Дата и время окончания", sample: "25/08/2026 12:00" },
  { token: "{real_keys_count}", description: "Количество доступных конфигураций", sample: "6" },
] as const;

const templateTokenPattern = /\{[^{}\s]+\}/g;
const sampleByToken = new Map(keyTemplateVariables.map((variable) => [variable.token, variable.sample]));

export interface InformationalPreviewPart {
  text: string;
  unknown: boolean;
}

export function informationalTemplatePreviewParts(template: string): InformationalPreviewPart[] {
  const parts: InformationalPreviewPart[] = [];
  let offset = 0;
  for (const match of template.matchAll(templateTokenPattern)) {
    const index = match.index ?? 0;
    if (index > offset) parts.push({ text: template.slice(offset, index), unknown: false });
    const token = match[0];
    const sample = sampleByToken.get(token as typeof keyTemplateVariables[number]["token"]);
    parts.push({ text: sample ?? token, unknown: sample === undefined });
    offset = index + token.length;
  }
  if (offset < template.length) parts.push({ text: template.slice(offset), unknown: false });
  return parts;
}
