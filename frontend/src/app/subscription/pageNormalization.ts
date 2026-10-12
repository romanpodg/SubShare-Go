import {
  subscriptionPageConfig,
  type ActivationStepBlockConfig,
  type PageBlock,
  type SubscriptionPageConfig,
  type SubscriptionTheme,
} from "./pageConfig";

function applyTemplate(source: string, vars: Record<string, string>): string {
  return source.replace(/\{([a-zA-Z0-9_]+)\}/g, (_, key: string) => vars[key] ?? "");
}

export function resolveTemplateValues<T>(value: T, vars: Record<string, string>): T {
  if (typeof value === "string") {
    return applyTemplate(value, vars) as T;
  }

  if (Array.isArray(value)) {
    return value.map((item) => resolveTemplateValues(item, vars)) as T;
  }

  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>).map(([key, nested]) => [
      key,
      resolveTemplateValues(nested, vars),
    ]);
    return Object.fromEntries(entries) as T;
  }

  return value;
}

function getDefaultActivationBlock(): ActivationStepBlockConfig {
  const activation = subscriptionPageConfig.blocks
    .flatMap((block) => block.type === "steps" ? block.steps : [])
    .map((step) => step.block)
    .find((block): block is ActivationStepBlockConfig => block.type === "activation");
  if (activation) return activation;

  return {
    type: "activation",
    formLabel: "Ключ активации",
    formPlaceholder: "Введите ключ активации",
    submitLabel: "Активировать",
    submitLoadingLabel: "Активация...",
    activateErrorFallback: "Не удалось активировать подписку",
    addButtonLabel: "Добавить подписку в Happ",
    addButtonDisabledLabel: "Сначала активируйте ключ",
    manualLinkLabel: "Ссылка подписки:",
    copyLabel: "Копировать",
    copiedLabel: "Скопировано",
  };
}

function hydrateBlocks(blocks: PageBlock[]): PageBlock[] {
  const defaultActivationBlock = getDefaultActivationBlock();

  return blocks.map((block) => {
    if (block.type !== "steps") return block;

    return {
      ...block,
      steps: block.steps.map((step) => {
        if (step.block.type !== "activation") return step;
        return {
          ...step,
          block: {
            ...defaultActivationBlock,
            ...step.block,
          },
        };
      }),
    };
  });
}

function normalizeTheme(theme: Partial<SubscriptionTheme> & Record<string, string | undefined>): SubscriptionTheme {
  return {
    ...subscriptionPageConfig.theme,
    ...theme,
    buttonPrimaryBackgroundHover:
      theme.buttonPrimaryBackgroundHover ?? theme.buttonPrimaryHover ?? subscriptionPageConfig.theme.buttonPrimaryBackgroundHover,
    buttonSecondaryBackgroundHover:
      theme.buttonSecondaryBackgroundHover ??
      theme.buttonSecondaryHover ??
      subscriptionPageConfig.theme.buttonSecondaryBackgroundHover,
    buttonSubscribeBackgroundHover:
      theme.buttonSubscribeBackgroundHover ??
      theme.buttonSubscribeHover ??
      subscriptionPageConfig.theme.buttonSubscribeBackgroundHover,
  };
}

export function normalizeRemoteConfig(raw: unknown): SubscriptionPageConfig {
  if (!raw || typeof raw !== "object") {
    return subscriptionPageConfig;
  }

  const candidate = raw as Partial<SubscriptionPageConfig>;
  const blocks = Array.isArray(candidate.blocks) && candidate.blocks.length > 0
    ? (candidate.blocks as PageBlock[])
    : subscriptionPageConfig.blocks;

  return {
    locale: candidate.locale || subscriptionPageConfig.locale,
    templateVars: {
      ...subscriptionPageConfig.templateVars,
      ...(candidate.templateVars ?? {}),
    },
    theme: normalizeTheme((candidate.theme ?? {}) as Partial<SubscriptionTheme> & Record<string, string | undefined>),
    blocks: hydrateBlocks(blocks),
  };
}
