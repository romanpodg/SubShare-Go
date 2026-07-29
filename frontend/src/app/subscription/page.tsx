"use client";

import { CSSProperties, FormEvent, useEffect, useMemo, useState } from "react";
import { usePanelSettings } from "@/context/PanelSettingsContext";
import { copyToClipboard } from "@/lib/clipboard";
import { subscription, subscriptionPageConfig as subscriptionPageConfigApi } from "@/lib/api";
import { SubscriptionBlocks } from "./SubscriptionBlocks";
import {
  subscriptionPageConfig,
  type ActivationStepBlockConfig,
  type PageBlock,
  type SubscriptionPageConfig,
  type SubscriptionTheme,
} from "./pageConfig";
import styles from "./subscription-page.module.css";

function applyTemplate(source: string, vars: Record<string, string>): string {
  return source.replace(/\{([a-zA-Z0-9_]+)\}/g, (_, key: string) => vars[key] ?? "");
}

function resolveTemplateValues<T>(value: T, vars: Record<string, string>): T {
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
  for (const block of subscriptionPageConfig.blocks) {
    if (block.type !== "steps") continue;
    for (const step of block.steps) {
      if (step.block.type === "activation") {
        return step.block;
      }
    }
  }

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

function normalizeRemoteConfig(raw: unknown): SubscriptionPageConfig {
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

function buildDarkTheme(theme: SubscriptionTheme): SubscriptionTheme {
  const customOr = (key: keyof SubscriptionTheme, fallback: string) =>
    theme[key] !== subscriptionPageConfig.theme[key] ? theme[key] : fallback;

  return {
    ...theme,
    pageBackground: "#050505",
    cardBackground: "#070808",
    textPrimary: "#f2f3ef",
    textSecondary: "#c7cbc7",
    textMuted: "#7f8580",
    border: "rgba(255,255,255,0.1)",
    shadow: "none",
    heroBorder: "rgba(255,255,255,0.1)",
    inputBackground: "#0d0f11",
    inputBorder: "rgba(255,255,255,0.12)",
    inputText: "#f2f3ef",
    successBackground: "rgba(183,255,42,0.055)",
    successText: "#b7ff2a",
    errorBackground: "rgba(255,91,103,0.055)",
    errorText: "#ff7a83",
    badgeBackground: customOr("badgeBackground", "#090a0b"),
    badgeText: customOr("badgeText", "#9ca19d"),
    badgeBorder: customOr("badgeBorder", "rgba(255,255,255,0.1)"),
    buttonPrimaryBackground: customOr("buttonPrimaryBackground", "#b7ff2a"),
    buttonPrimaryBackgroundHover: customOr("buttonPrimaryBackgroundHover", "#c8ff5c"),
    buttonPrimaryText: customOr("buttonPrimaryText", "#081000"),
    buttonSecondaryBackground: customOr("buttonSecondaryBackground", "#0d0f11"),
    buttonSecondaryBackgroundHover: customOr("buttonSecondaryBackgroundHover", "#131619"),
    buttonSecondaryBorder: customOr("buttonSecondaryBorder", "rgba(255,255,255,0.12)"),
    buttonSecondaryBorderHover: customOr("buttonSecondaryBorderHover", "rgba(255,255,255,0.25)"),
    buttonSecondaryText: customOr("buttonSecondaryText", "#c7cbc7"),
    buttonSubscribeBackground: customOr("buttonSubscribeBackground", "#b7ff2a"),
    buttonSubscribeBackgroundHover: customOr("buttonSubscribeBackgroundHover", "#c8ff5c"),
    buttonSubscribeText: customOr("buttonSubscribeText", "#081000"),
    buttonSubscribeShadow: "none",
    buttonRecommendedBackground: customOr("buttonRecommendedBackground", "#b7ff2a"),
    buttonRecommendedBackgroundHover: customOr("buttonRecommendedBackgroundHover", "#c8ff5c"),
    buttonRecommendedBorder: customOr("buttonRecommendedBorder", "#b7ff2a"),
    buttonRecommendedBorderHover: customOr("buttonRecommendedBorderHover", "#c8ff5c"),
    buttonRecommendedText: customOr("buttonRecommendedText", "#081000"),
    buttonRecommendedShadow: "none",
    buttonDisabledBackground: "#101210",
    buttonDisabledBorder: "rgba(255,255,255,0.08)",
    buttonDisabledText: "#555b56",
    codeBackground: "#070808",
    codeText: "#747a75",
    codeLinkText: customOr("codeLinkText", "#b7ff2a"),
    stepNumberBackground: "rgba(183,255,42,0.055)",
    stepNumberText: "#b7ff2a",
    stepCardBackground: "#090a0b",
    stepCardBorder: "rgba(255,255,255,0.1)",
    languageBadgeBackground: "#0d0f11",
    languageBadgeText: "#8b918c",
    fontFamily: "var(--font-sans)",
  };
}

function buildThemeVars(config: SubscriptionPageConfig): CSSProperties {
  const theme = buildDarkTheme(config.theme);
  return {
    "--sub-page-bg": theme.pageBackground,
    "--sub-card-bg": theme.cardBackground,
    "--sub-text-primary": theme.textPrimary,
    "--sub-text-secondary": theme.textSecondary,
    "--sub-text-muted": theme.textMuted,
    "--sub-border": theme.border,
    "--sub-shadow": theme.shadow,
    "--sub-hero-border": theme.heroBorder,
    "--sub-input-bg": theme.inputBackground,
    "--sub-input-border": theme.inputBorder,
    "--sub-input-text": theme.inputText,
    "--sub-success-bg": theme.successBackground,
    "--sub-success-text": theme.successText,
    "--sub-error-bg": theme.errorBackground,
    "--sub-error-text": theme.errorText,
    "--sub-badge-bg": theme.badgeBackground,
    "--sub-badge-text": theme.badgeText,
    "--sub-badge-border": theme.badgeBorder,
    "--sub-btn-primary-bg": theme.buttonPrimaryBackground,
    "--sub-btn-primary-hover": theme.buttonPrimaryBackgroundHover,
    "--sub-btn-primary-text": theme.buttonPrimaryText,
    "--sub-btn-secondary-bg": theme.buttonSecondaryBackground,
    "--sub-btn-secondary-hover": theme.buttonSecondaryBackgroundHover,
    "--sub-btn-secondary-border": theme.buttonSecondaryBorder,
    "--sub-btn-secondary-border-hover": theme.buttonSecondaryBorderHover,
    "--sub-btn-secondary-text": theme.buttonSecondaryText,
    "--sub-btn-subscribe-bg": theme.buttonSubscribeBackground,
    "--sub-btn-subscribe-hover": theme.buttonSubscribeBackgroundHover,
    "--sub-btn-subscribe-text": theme.buttonSubscribeText,
    "--sub-btn-subscribe-shadow": theme.buttonSubscribeShadow,
    "--sub-btn-rec-bg": theme.buttonRecommendedBackground,
    "--sub-btn-rec-hover-bg": theme.buttonRecommendedBackgroundHover,
    "--sub-btn-rec-border": theme.buttonRecommendedBorder,
    "--sub-btn-rec-hover-border": theme.buttonRecommendedBorderHover,
    "--sub-btn-rec-text": theme.buttonRecommendedText,
    "--sub-btn-rec-shadow": theme.buttonRecommendedShadow,
    "--sub-btn-rec-neutral-bg": theme.buttonRecommendedNeutralBackground,
    "--sub-btn-rec-neutral-hover-bg": theme.buttonRecommendedNeutralBackgroundHover,
    "--sub-btn-rec-neutral-border": theme.buttonRecommendedNeutralBorder,
    "--sub-btn-rec-neutral-hover-border": theme.buttonRecommendedNeutralBorderHover,
    "--sub-btn-rec-neutral-text": theme.buttonRecommendedNeutralText,
    "--sub-btn-rec-windows-bg": theme.buttonRecommendedWindowsBackground,
    "--sub-btn-rec-windows-hover-bg": theme.buttonRecommendedWindowsBackgroundHover,
    "--sub-btn-rec-windows-border": theme.buttonRecommendedWindowsBorder,
    "--sub-btn-rec-windows-hover-border": theme.buttonRecommendedWindowsBorderHover,
    "--sub-btn-rec-windows-text": theme.buttonRecommendedWindowsText,
    "--sub-btn-rec-android-bg": theme.buttonRecommendedAndroidBackground,
    "--sub-btn-rec-android-hover-bg": theme.buttonRecommendedAndroidBackgroundHover,
    "--sub-btn-rec-android-border": theme.buttonRecommendedAndroidBorder,
    "--sub-btn-rec-android-hover-border": theme.buttonRecommendedAndroidBorderHover,
    "--sub-btn-rec-android-text": theme.buttonRecommendedAndroidText,
    "--sub-btn-rec-linux-bg": theme.buttonRecommendedLinuxBackground,
    "--sub-btn-rec-linux-hover-bg": theme.buttonRecommendedLinuxBackgroundHover,
    "--sub-btn-rec-linux-border": theme.buttonRecommendedLinuxBorder,
    "--sub-btn-rec-linux-hover-border": theme.buttonRecommendedLinuxBorderHover,
    "--sub-btn-rec-linux-text": theme.buttonRecommendedLinuxText,
    "--sub-btn-rec-apple-bg": theme.buttonRecommendedAppleBackground,
    "--sub-btn-rec-apple-hover-bg": theme.buttonRecommendedAppleBackgroundHover,
    "--sub-btn-rec-apple-border": theme.buttonRecommendedAppleBorder,
    "--sub-btn-rec-apple-hover-border": theme.buttonRecommendedAppleBorderHover,
    "--sub-btn-rec-apple-text": theme.buttonRecommendedAppleText,
    "--sub-btn-disabled-bg": theme.buttonDisabledBackground,
    "--sub-btn-disabled-border": theme.buttonDisabledBorder,
    "--sub-btn-disabled-text": theme.buttonDisabledText,
    "--sub-code-bg": theme.codeBackground,
    "--sub-code-text": theme.codeText,
    "--sub-code-link-text": theme.codeLinkText,
    "--sub-step-number-bg": theme.stepNumberBackground,
    "--sub-step-number-text": theme.stepNumberText,
    "--sub-step-card-bg": theme.stepCardBackground,
    "--sub-step-card-border": theme.stepCardBorder,
    "--sub-lang-bg": theme.languageBadgeBackground,
    "--sub-lang-text": theme.languageBadgeText,
    "--sub-font-family": theme.fontFamily,
  } as CSSProperties;
}

export default function SubscriptionPage() {
  const [pageConfig, setPageConfig] = useState<SubscriptionPageConfig>(subscriptionPageConfig);
  const [code, setCode] = useState("");
  const [subscriptionUrl, setSubscriptionUrl] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const { settings } = usePanelSettings();

  useEffect(() => {
    document.title = settings.pageTitles.subscription;
  }, [settings.pageTitles.subscription]);

  useEffect(() => {
    let mounted = true;

    subscriptionPageConfigApi
      .getPublic()
      .then((response) => {
        if (!mounted || !response.config_json) return;
        setPageConfig(normalizeRemoteConfig(JSON.parse(response.config_json)));
      })
      .catch(() => {
        if (mounted) {
          setPageConfig(subscriptionPageConfig);
        }
      });

    return () => {
      mounted = false;
    };
  }, []);

  const templateVars = useMemo(() => {
    const brandName = settings.panelTitle.trim() || pageConfig.templateVars.brandName;
    return {
      ...pageConfig.templateVars,
      brandName,
    };
  }, [pageConfig.templateVars, settings.panelTitle]);

  const blocks = useMemo(
    () => resolveTemplateValues(pageConfig.blocks, templateVars),
    [pageConfig.blocks, templateVars]
  );

  const activationErrorFallback = useMemo(() => {
    for (const block of blocks) {
      if (block.type !== "steps") continue;
      for (const step of block.steps) {
        if (step.block.type === "activation") {
          return step.block.activateErrorFallback;
        }
      }
    }
    return "Не удалось активировать подписку";
  }, [blocks]);

  const themeVars = useMemo(() => buildThemeVars(pageConfig), [pageConfig]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    setMessage("");
    setSubscriptionUrl("");
    setLoading(true);

    try {
      const data = await subscription.activate(code);
      setSubscriptionUrl(data.subscription_url);
      setMessage(data.message);
    } catch (requestError: unknown) {
      setError(requestError instanceof Error ? requestError.message : activationErrorFallback);
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    try {
      const copiedSuccess = await copyToClipboard(subscriptionUrl);
      if (!copiedSuccess) {
        if (typeof window !== "undefined") {
          window.prompt("Скопируйте ссылку вручную:", subscriptionUrl);
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
          return;
        }

        setError("Не удалось скопировать ссылку");
        return;
      }

      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      if (typeof window !== "undefined") {
        window.prompt("Скопируйте ссылку вручную:", subscriptionUrl);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
        return;
      }

      setError("Не удалось скопировать ссылку");
    }
  };

  return (
    <div className={`${styles.page} ${styles.pageDark}`} style={themeVars}>
      <div className={styles.container}>
        <div className={styles.card}>
          <SubscriptionBlocks
            blocks={blocks}
            panelLogoDataUrl={settings.logoDataUrl || undefined}
            themeMode="dark"
            activation={{
              code,
              loading,
              error,
              message,
              subscriptionUrl,
              copied,
              onCodeChange: setCode,
              onSubmit: handleSubmit,
              onCopy: handleCopy,
            }}
          />
        </div>
      </div>
    </div>
  );
}
