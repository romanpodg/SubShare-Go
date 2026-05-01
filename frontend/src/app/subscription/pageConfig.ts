export type PageBlock = HeroBlockConfig | StepsBlockConfig | FooterBlockConfig;

export interface SubscriptionTheme {
  pageBackground: string;
  cardBackground: string;
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  border: string;
  shadow: string;
  heroBorder: string;
  inputBackground: string;
  inputBorder: string;
  inputText: string;
  successBackground: string;
  successText: string;
  errorBackground: string;
  errorText: string;
  badgeBackground: string;
  badgeText: string;
  badgeBorder: string;
  buttonPrimaryBackground: string;
  buttonPrimaryBackgroundHover: string;
  buttonPrimaryText: string;
  buttonSecondaryBackground: string;
  buttonSecondaryBackgroundHover: string;
  buttonSecondaryBorder: string;
  buttonSecondaryBorderHover: string;
  buttonSecondaryText: string;
  buttonSubscribeBackground: string;
  buttonSubscribeBackgroundHover: string;
  buttonSubscribeText: string;
  buttonSubscribeShadow: string;
  buttonRecommendedBackground: string;
  buttonRecommendedBackgroundHover: string;
  buttonRecommendedBorder: string;
  buttonRecommendedBorderHover: string;
  buttonRecommendedText: string;
  buttonRecommendedShadow: string;
  buttonRecommendedNeutralBackground: string;
  buttonRecommendedNeutralBackgroundHover: string;
  buttonRecommendedNeutralBorder: string;
  buttonRecommendedNeutralBorderHover: string;
  buttonRecommendedNeutralText: string;
  buttonRecommendedWindowsBackground: string;
  buttonRecommendedWindowsBackgroundHover: string;
  buttonRecommendedWindowsBorder: string;
  buttonRecommendedWindowsBorderHover: string;
  buttonRecommendedWindowsText: string;
  buttonRecommendedAndroidBackground: string;
  buttonRecommendedAndroidBackgroundHover: string;
  buttonRecommendedAndroidBorder: string;
  buttonRecommendedAndroidBorderHover: string;
  buttonRecommendedAndroidText: string;
  buttonRecommendedLinuxBackground: string;
  buttonRecommendedLinuxBackgroundHover: string;
  buttonRecommendedLinuxBorder: string;
  buttonRecommendedLinuxBorderHover: string;
  buttonRecommendedLinuxText: string;
  buttonRecommendedAppleBackground: string;
  buttonRecommendedAppleBackgroundHover: string;
  buttonRecommendedAppleBorder: string;
  buttonRecommendedAppleBorderHover: string;
  buttonRecommendedAppleText: string;
  buttonDisabledBackground: string;
  buttonDisabledBorder: string;
  buttonDisabledText: string;
  codeBackground: string;
  codeText: string;
  codeLinkText: string;
  stepNumberBackground: string;
  stepNumberText: string;
  stepCardBackground: string;
  stepCardBorder: string;
  languageBadgeBackground: string;
  languageBadgeText: string;
  fontFamily: string;
}

export interface LogoConfig {
  src: string;
  alt: string;
}

export interface HeroBlockConfig {
  type: "hero";
  id: string;
  brandTitle: string;
  brandSubtitle: string;
  subhead: string;
  logo: LogoConfig;
  statusBadge?: string;
}

export interface PlatformIcon {
  type: "image" | "download";
  src?: string;
  alt?: string;
}

export type ButtonVariant = "primary" | "secondary" | "subscribe";

export interface LinkButtonConfig {
  id: string;
  label: string;
  href: string;
  variant: ButtonVariant;
  icon?: PlatformIcon;
  external?: boolean;
}

export interface ActivationStepBlockConfig {
  type: "activation";
  formLabel: string;
  formPlaceholder: string;
  submitLabel: string;
  submitLoadingLabel: string;
  activateErrorFallback: string;
  addButtonLabel: string;
  addButtonDisabledLabel: string;
  manualLinkLabel: string;
  copyLabel: string;
  copiedLabel: string;
}

export interface LinkButtonsStepBlockConfig {
  type: "linkButtons";
  buttons: LinkButtonConfig[];
}

export interface TextStepBlockConfig {
  type: "text";
}

export type StepBodyBlockConfig =
  | LinkButtonsStepBlockConfig
  | ActivationStepBlockConfig
  | TextStepBlockConfig;

export interface StepConfig {
  id: string;
  title: string;
  description: string;
  block: StepBodyBlockConfig;
}

export interface StepsBlockConfig {
  type: "steps";
  id: string;
  title: string;
  steps: StepConfig[];
}

export interface FooterLinkConfig {
  label: string;
  href: string;
}

export interface FooterBlockConfig {
  type: "footer";
  id: string;
  copyright: string;
  languageBadge: string;
  footerLink?: FooterLinkConfig;
}

export interface SubscriptionPageConfig {
  locale: string;
  templateVars: Record<string, string>;
  theme: SubscriptionTheme;
  blocks: PageBlock[];
}

export const subscriptionPageConfig: SubscriptionPageConfig = {
  locale: "ru",
  templateVars: {
    brandName: "FoxtCloud",
  },
  theme: {
    pageBackground: "#f7f9fc",
    cardBackground: "#ffffff",
    textPrimary: "#0f172a",
    textSecondary: "#334155",
    textMuted: "#5b6e8c",
    border: "#eef2f6",
    shadow: "0 12px 30px rgba(0, 0, 0, 0.05), 0 4px 8px rgba(0, 0, 0, 0.02)",
    heroBorder: "#eef2f6",
    inputBackground: "#ffffff",
    inputBorder: "#dce3ec",
    inputText: "#1a1e2b",
    successBackground: "#ecfdf5",
    successText: "#166534",
    errorBackground: "#fef2f2",
    errorText: "#b91c1c",
    badgeBackground: "#eef2ff",
    badgeText: "#1f3a8a",
    badgeBorder: "#dbe4ff",
    buttonPrimaryBackground: "#0f172a",
    buttonPrimaryBackgroundHover: "#1e293b",
    buttonPrimaryText: "#ffffff",
    buttonSecondaryBackground: "#ffffff",
    buttonSecondaryBackgroundHover: "#f8fafd",
    buttonSecondaryBorder: "#dce3ec",
    buttonSecondaryBorderHover: "#cbd5e1",
    buttonSecondaryText: "#1f2a44",
    buttonSubscribeBackground: "#facc15",
    buttonSubscribeBackgroundHover: "#fde047",
    buttonSubscribeText: "#0f172a",
    buttonSubscribeShadow: "0 4px 8px rgba(250, 204, 21, 0.2)",
    buttonRecommendedBackground: "#ffffff",
    buttonRecommendedBackgroundHover: "#edf2f7",
    buttonRecommendedBorder: "#ffffff",
    buttonRecommendedBorderHover: "#edf2f7",
    buttonRecommendedText: "#0f172a",
    buttonRecommendedShadow: "0 6px 16px rgba(2, 6, 23, 0.18)",
    buttonRecommendedNeutralBackground: "#ffffff",
    buttonRecommendedNeutralBackgroundHover: "#edf2f7",
    buttonRecommendedNeutralBorder: "#ffffff",
    buttonRecommendedNeutralBorderHover: "#edf2f7",
    buttonRecommendedNeutralText: "#0f172a",
    buttonRecommendedWindowsBackground: "#e7f2ff",
    buttonRecommendedWindowsBackgroundHover: "#dcecff",
    buttonRecommendedWindowsBorder: "#b8d9ff",
    buttonRecommendedWindowsBorderHover: "#a8ccff",
    buttonRecommendedWindowsText: "#0f3d73",
    buttonRecommendedAndroidBackground: "#e9f8ef",
    buttonRecommendedAndroidBackgroundHover: "#ddf3e5",
    buttonRecommendedAndroidBorder: "#bee6cc",
    buttonRecommendedAndroidBorderHover: "#acdcbf",
    buttonRecommendedAndroidText: "#14532d",
    buttonRecommendedLinuxBackground: "#fff2e4",
    buttonRecommendedLinuxBackgroundHover: "#ffe8ce",
    buttonRecommendedLinuxBorder: "#ffd6ad",
    buttonRecommendedLinuxBorderHover: "#ffc891",
    buttonRecommendedLinuxText: "#8a3d0f",
    buttonRecommendedAppleBackground: "#f1f4f7",
    buttonRecommendedAppleBackgroundHover: "#e7ecf2",
    buttonRecommendedAppleBorder: "#d7dee6",
    buttonRecommendedAppleBorderHover: "#cad3dd",
    buttonRecommendedAppleText: "#273244",
    buttonDisabledBackground: "#f3f4f6",
    buttonDisabledBorder: "#e5e7eb",
    buttonDisabledText: "#9ca3af",
    codeBackground: "#fafcff",
    codeText: "#5b6e8c",
    codeLinkText: "#2563eb",
    stepNumberBackground: "#f1f4f9",
    stepNumberText: "#2c3e66",
    stepCardBackground: "#ffffff",
    stepCardBorder: "#eef2f8",
    languageBadgeBackground: "#f1f5f9",
    languageBadgeText: "#334155",
    fontFamily: "'Inter', system-ui, -apple-system, 'Segoe UI', Roboto, Helvetica, sans-serif",
  },
  blocks: [
    {
      type: "hero",
      id: "hero",
      brandTitle: "{brandName}",
      brandSubtitle: "быстрый · приватный · надёжный",
      subhead:
        "Ничего настраивать не требуется. Исключение на российские сервисы будет установлено автоматически",
      logo: {
        src: "/subscription/logo.jpg",
        alt: "{brandName} logo",
      },
      statusBadge: "Подписка готова к подключению",
    },
    {
      type: "steps",
      id: "subscription",
      title: "Как подключиться к {brandName}",
      steps: [
        {
          id: "install-app",
          title: "Установите приложение Happ",
          description:
            "Скачайте клиент из официального магазина или напрямую APK. Happ отлично подходит для работы с {brandName}.",
          block: {
            type: "linkButtons",
            buttons: [
              {
                id: "google-play",
                label: "Скачать из Google Play",
                href: "https://play.google.com/store/apps/details?id=com.happproxy",
                variant: "primary",
                external: true,
                icon: {
                  type: "image",
                  src: "/subscription/googleplay.svg",
                  alt: "Google Play",
                },
              },
              {
                id: "app-store-ru",
                label: "Скачать из App Store (RU)",
                href: "https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973",
                variant: "secondary",
                external: true,
                icon: {
                  type: "image",
                  src: "/subscription/appstore.svg",
                  alt: "App Store",
                },
              },
              {
                id: "app-store-global",
                label: "Скачать из App Store (Global)",
                href: "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215",
                variant: "secondary",
                external: true,
                icon: {
                  type: "image",
                  src: "/subscription/appstore.svg",
                  alt: "App Store",
                },
              },
              {
                id: "android-apk",
                label: "Скачать APK (Android)",
                href: "https://github.com/Happ-proxy/happ-android/releases/latest/download/Happ.apk",
                variant: "secondary",
                external: true,
                icon: {
                  type: "download",
                },
              },
              {
                id: "windows",
                label: "Скачать для Windows",
                href: "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/setup-Happ.x64.exe",
                variant: "secondary",
                external: true,
                icon: {
                  type: "image",
                  src: "/subscription/windows.svg",
                  alt: "Windows",
                },
              },
              {
                id: "macos",
                label: "Скачать для macOS",
                href: "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.macOS.universal.dmg",
                variant: "secondary",
                external: true,
                icon: {
                  type: "image",
                  src: "/subscription/apple.svg",
                  alt: "macOS",
                },
              },
              {
                id: "linux",
                label: "Скачать для Linux",
                href: "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.deb",
                variant: "secondary",
                external: true,
                icon: {
                  type: "image",
                  src: "/subscription/linux.svg",
                  alt: "Linux",
                },
              },
            ],
          },
        },
        {
          id: "activate-subscription",
          title: "Добавьте подписку {brandName}",
          description:
            "Введите ваш ключ активации. После этого сможете добавить подписку в Happ автоматически или скопировать ссылку вручную.",
          block: {
            type: "activation",
            formLabel: "Ключ активации",
            formPlaceholder: "Введите ключ активации",
            submitLabel: "Активировать",
            submitLoadingLabel: "Активация...",
            activateErrorFallback: "Не удалось активировать подписку",
            addButtonLabel: "🦊 Добавить подписку в Happ",
            addButtonDisabledLabel: "Сначала активируйте ключ",
            manualLinkLabel: "🔗 Ссылка подписки:",
            copyLabel: "📋 Копировать",
            copiedLabel: "✅ Скопировано",
          },
        },
        {
          id: "connect",
          title: "Подключитесь и работайте",
          description:
            "Выберите сервер из подписки в приложении и нажмите «Подключиться». Ваш трафик теперь под защитой.",
          block: {
            type: "text",
          },
        },
      ],
    },
    {
      type: "footer",
      id: "footer",
      copyright: "© {brandName} — ваша приватность в сети",
      languageBadge: "🇷🇺 Русский",
      footerLink: {
        label: "Панель управления",
        href: "/admin/login",
      },
    },
  ],
};
