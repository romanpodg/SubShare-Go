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

const STORAGE_THEME_KEY = "xray_sub_subscription_theme";

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
  return {
    ...theme,
    pageBackground: "#10151f",
    cardBackground: "#161c27",
    textPrimary: "#f3f6fb",
    textSecondary: "#cbd5e1",
    textMuted: "#b9c6d8",
    border: "#273244",
    shadow: "0 24px 70px rgba(0, 0, 0, 0.42), 0 6px 18px rgba(0, 0, 0, 0.24)",
    heroBorder: "#273244",
    inputBackground: "#101722",
    inputBorder: "#324154",
    inputText: "#f3f6fb",
    successBackground: "#163127",
    successText: "#86efac",
    errorBackground: "#391b1f",
    errorText: "#fca5a5",
    badgeBackground: "#14233c",
    badgeText: "#bed5ff",
    badgeBorder: "#23406b",
    buttonPrimaryBackground: "#f3f6fb",
    buttonPrimaryBackgroundHover: "#dbe4f0",
    buttonPrimaryText: "#0f172a",
    buttonSecondaryBackground: "#161c27",
    buttonSecondaryBackgroundHover: "#202938",
    buttonSecondaryBorder: "#334155",
    buttonSecondaryBorderHover: "#475569",
    buttonSecondaryText: "#e5edf8",
    buttonSubscribeBackground: "#ffffff",
    buttonSubscribeBackgroundHover: "#edf2f7",
    buttonSubscribeText: "#0f172a",
    buttonSubscribeShadow: "0 8px 20px rgba(2, 6, 23, 0.24)",
    buttonRecommendedBackground: "#ffffff",
    buttonRecommendedBackgroundHover: "#edf2f7",
    buttonRecommendedBorder: "#ffffff",
    buttonRecommendedBorderHover: "#edf2f7",
    buttonRecommendedText: "#0f172a",
    buttonRecommendedShadow: "0 8px 20px rgba(2, 6, 23, 0.24)",
    buttonRecommendedNeutralBackground: "#ffffff",
    buttonRecommendedNeutralBackgroundHover: "#edf2f7",
    buttonRecommendedNeutralBorder: "#ffffff",
    buttonRecommendedNeutralBorderHover: "#edf2f7",
    buttonRecommendedNeutralText: "#0f172a",
    buttonRecommendedWindowsBackground: "#ffffff",
    buttonRecommendedWindowsBackgroundHover: "#edf2f7",
    buttonRecommendedWindowsBorder: "#ffffff",
    buttonRecommendedWindowsBorderHover: "#edf2f7",
    buttonRecommendedWindowsText: "#0f172a",
    buttonRecommendedAndroidBackground: "#ffffff",
    buttonRecommendedAndroidBackgroundHover: "#edf2f7",
    buttonRecommendedAndroidBorder: "#ffffff",
    buttonRecommendedAndroidBorderHover: "#edf2f7",
    buttonRecommendedAndroidText: "#0f172a",
    buttonRecommendedLinuxBackground: "#ffffff",
    buttonRecommendedLinuxBackgroundHover: "#edf2f7",
    buttonRecommendedLinuxBorder: "#ffffff",
    buttonRecommendedLinuxBorderHover: "#edf2f7",
    buttonRecommendedLinuxText: "#0f172a",
    buttonRecommendedAppleBackground: "#ffffff",
    buttonRecommendedAppleBackgroundHover: "#edf2f7",
    buttonRecommendedAppleBorder: "#ffffff",
    buttonRecommendedAppleBorderHover: "#edf2f7",
    buttonRecommendedAppleText: "#0f172a",
    buttonDisabledBackground: "#212938",
    buttonDisabledBorder: "#313b4c",
    buttonDisabledText: "#6f7c91",
    codeBackground: "#101722",
    codeText: "#9fb0c9",
    codeLinkText: "#93c5fd",
    stepNumberBackground: "#202938",
    stepNumberText: "#f3f6fb",
    stepCardBackground: "#161c27",
    stepCardBorder: "#273244",
    languageBadgeBackground: "#202938",
    languageBadgeText: "#d6deeb",
  };
}

function buildThemeVars(config: SubscriptionPageConfig, themeMode: "light" | "dark"): CSSProperties {
  const theme = themeMode === "dark" ? buildDarkTheme(config.theme) : config.theme;
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
  const [themeMode, setThemeMode] = useState<"light" | "dark">("light");
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
    if (typeof window === "undefined") return;

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const savedTheme = window.localStorage.getItem(STORAGE_THEME_KEY);

    if (savedTheme === "light" || savedTheme === "dark") {
      setThemeMode(savedTheme);
      return;
    }

    setThemeMode(media.matches ? "dark" : "light");

    const handleChange = (event: MediaQueryListEvent) => {
      const latestSavedTheme = window.localStorage.getItem(STORAGE_THEME_KEY);
      if (latestSavedTheme === "light" || latestSavedTheme === "dark") {
        return;
      }
      setThemeMode(event.matches ? "dark" : "light");
    };

    media.addEventListener("change", handleChange);
    return () => media.removeEventListener("change", handleChange);
  }, []);

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

  const themeVars = useMemo(() => buildThemeVars(pageConfig, themeMode), [pageConfig, themeMode]);

  const handleToggleTheme = () => {
    setThemeMode((current) => {
      const next = current === "dark" ? "light" : "dark";
      if (typeof window !== "undefined") {
        window.localStorage.setItem(STORAGE_THEME_KEY, next);
      }
      return next;
    });
  };

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
    <div className={`${styles.page} ${themeMode === "dark" ? styles.pageDark : styles.pageLight}`} style={themeVars}>
      <div className={styles.container}>
        <div className={styles.card}>
          <SubscriptionBlocks
            blocks={blocks}
            panelLogoDataUrl={settings.logoDataUrl || undefined}
            themeMode={themeMode}
            onToggleTheme={handleToggleTheme}
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
