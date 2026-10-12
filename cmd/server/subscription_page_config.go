package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

const defaultSubscriptionPageConfigJSON = `{
  "locale": "ru",
  "templateVars": {
    "brandName": "XRAY VPN"
  },
  "theme": {
    "pageBackground": "#f7f9fc",
    "cardBackground": "#ffffff",
    "textPrimary": "#0f172a",
    "textSecondary": "#334155",
    "textMuted": "#5b6e8c",
    "border": "#eef2f6",
    "shadow": "0 12px 30px rgba(0,0,0,0.05), 0 4px 8px rgba(0,0,0,0.02)",
    "heroBorder": "#eef2f6",
    "badgeBackground": "#eef2ff",
    "badgeText": "#1f3a8a",
    "badgeBorder": "#dbe4ff",
    "buttonPrimaryBackground": "#0f172a",
    "buttonPrimaryHover": "#1e293b",
    "buttonPrimaryText": "#ffffff",
    "buttonSecondaryBackground": "#ffffff",
    "buttonSecondaryHover": "#f8fafd",
    "buttonSecondaryBorder": "#dce3ec",
    "buttonSecondaryBorderHover": "#cbd5e1",
    "buttonSecondaryText": "#1f2a44",
    "buttonSubscribeBackground": "#facc15",
    "buttonSubscribeHover": "#fde047",
    "buttonSubscribeText": "#0f172a",
    "buttonSubscribeShadow": "0 4px 8px rgba(250, 204, 21, 0.2)",
    "buttonRecommendedBackground": "#ffffff",
    "buttonRecommendedBackgroundHover": "#edf2f7",
    "buttonRecommendedBorder": "#ffffff",
    "buttonRecommendedBorderHover": "#edf2f7",
    "buttonRecommendedText": "#0f172a",
    "buttonRecommendedShadow": "0 6px 16px rgba(2, 6, 23, 0.18)",
    "buttonRecommendedNeutralBackground": "#ffffff",
    "buttonRecommendedNeutralBackgroundHover": "#edf2f7",
    "buttonRecommendedNeutralBorder": "#ffffff",
    "buttonRecommendedNeutralBorderHover": "#edf2f7",
    "buttonRecommendedNeutralText": "#0f172a",
    "buttonRecommendedWindowsBackground": "#e7f2ff",
    "buttonRecommendedWindowsBackgroundHover": "#dcecff",
    "buttonRecommendedWindowsBorder": "#b8d9ff",
    "buttonRecommendedWindowsBorderHover": "#a8ccff",
    "buttonRecommendedWindowsText": "#0f3d73",
    "buttonRecommendedAndroidBackground": "#e9f8ef",
    "buttonRecommendedAndroidBackgroundHover": "#ddf3e5",
    "buttonRecommendedAndroidBorder": "#bee6cc",
    "buttonRecommendedAndroidBorderHover": "#acdcbf",
    "buttonRecommendedAndroidText": "#14532d",
    "buttonRecommendedLinuxBackground": "#fff2e4",
    "buttonRecommendedLinuxBackgroundHover": "#ffe8ce",
    "buttonRecommendedLinuxBorder": "#ffd6ad",
    "buttonRecommendedLinuxBorderHover": "#ffc891",
    "buttonRecommendedLinuxText": "#8a3d0f",
    "buttonRecommendedAppleBackground": "#f1f4f7",
    "buttonRecommendedAppleBackgroundHover": "#e7ecf2",
    "buttonRecommendedAppleBorder": "#d7dee6",
    "buttonRecommendedAppleBorderHover": "#cad3dd",
    "buttonRecommendedAppleText": "#273244",
    "stepCardBackground": "#ffffff",
    "stepCardBorder": "#eef2f8",
    "stepNumberBackground": "#f1f4f9",
    "stepNumberText": "#2c3e66",
    "codeBackground": "#fafcff",
    "codeText": "#5b6e8c",
    "codeLinkText": "#2563eb",
    "languageBadgeBackground": "#f1f5f9",
    "languageBadgeText": "#334155",
    "fontFamily": "'Inter', system-ui, -apple-system, 'Segoe UI', Roboto, Helvetica, sans-serif"
  },
  "blocks": [
    {
      "type": "hero",
      "id": "hero",
      "brandTitle": "{brandName}",
      "brandSubtitle": "быстрый · приватный · надёжный",
      "subhead": "Ничего настраивать не требуется. Просто добавьте подписку и пользуйтесь интернетом без ограничений.",
      "logo": {
        "src": "/subscription/logo.jpg",
        "alt": "{brandName} logo"
      },
      "statusBadge": "Сервис работает исправно"
    },
    {
      "type": "steps",
      "id": "subscription",
      "title": "Как подключиться к {brandName}",
      "steps": [
        {
          "id": "install-app",
          "title": "Установите приложение Happ",
          "description": "Скачайте клиент из официального магазина или напрямую APK. Happ — идеальный выбор для работы с {brandName}.",
          "block": {
            "type": "linkButtons",
            "buttons": [
              {
                "id": "google-play",
                "label": "Скачать из Google Play",
                "href": "https://play.google.com/store/apps/details?id=com.happproxy",
                "variant": "primary",
                "external": true,
                "icon": {
                  "type": "image",
                  "src": "/subscription/googleplay.svg",
                  "alt": "Google Play"
                }
              },
              {
                "id": "app-store-ru",
                "label": "Скачать из App Store (RU)",
                "href": "https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973",
                "variant": "secondary",
                "external": true,
                "icon": {
                  "type": "image",
                  "src": "/subscription/appstore.svg",
                  "alt": "App Store"
                }
              },
              {
                "id": "app-store-global",
                "label": "Скачать из App Store (Global)",
                "href": "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215",
                "variant": "secondary",
                "external": true,
                "icon": {
                  "type": "image",
                  "src": "/subscription/appstore.svg",
                  "alt": "App Store"
                }
              },
              {
                "id": "android-apk",
                "label": "Скачать APK (Android)",
                "href": "https://github.com/Happ-proxy/happ-android/releases/latest/download/Happ.apk",
                "variant": "secondary",
                "external": true,
                "icon": {
                  "type": "download"
                }
              },
              {
                "id": "windows",
                "label": "Скачать для Windows",
                "href": "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/setup-Happ.x64.exe",
                "variant": "secondary",
                "external": true,
                "icon": {
                  "type": "image",
                  "src": "/subscription/windows.svg",
                  "alt": "Windows"
                }
              },
              {
                "id": "macos",
                "label": "Скачать для macOS",
                "href": "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.macOS.universal.dmg",
                "variant": "secondary",
                "external": true,
                "icon": {
                  "type": "image",
                  "src": "/subscription/apple.svg",
                  "alt": "macOS"
                }
              },
              {
                "id": "linux",
                "label": "Скачать для Linux",
                "href": "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.deb",
                "variant": "secondary",
                "external": true,
                "icon": {
                  "type": "image",
                  "src": "/subscription/linux.svg",
                  "alt": "Linux"
                }
              }
            ]
          }
        },
        {
          "id": "add-subscription",
          "title": "Добавьте подписку {brandName}",
          "description": "Нажмите на кнопку ниже и подписка с конфигурацией маршрутизации добавится автоматически.",
          "block": {
            "type": "activation",
            "addButtonLabel": "Добавить подписку в Happ",
            "manualLinkLabel": "🔗 Ссылка подписки:",
            "copyLabel": "📋 Копировать",
            "copiedLabel": "✅ Скопировано"
          }
        },
        {
          "id": "connect",
          "title": "Подключитесь и работайте",
          "description": "Выберите сервер из подписки в приложении и нажмите «Подключиться». Ваш трафик теперь под защитой и любые блокировки вам не страшны.",
          "block": {
            "type": "text"
          }
        }
      ]
    },
    {
      "type": "footer",
      "id": "footer",
      "copyright": "© {brandName}",
      "languageBadge": "🇷🇺 Русский",
      "footerLink": {
        "label": "Панель управления",
        "href": "/admin/login"
      }
    }
  ]
}`

func defaultSubscriptionPageConfig() model.SubscriptionPageConfig {
	var cfg model.SubscriptionPageConfig
	_ = json.Unmarshal([]byte(defaultSubscriptionPageConfigJSON), &cfg)
	return cfg
}

func normalizeSubscriptionPageConfig(raw string) (model.SubscriptionPageConfig, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		cfg := defaultSubscriptionPageConfig()
		return cfg, defaultSubscriptionPageConfigJSON, nil
	}

	var cfg model.SubscriptionPageConfig
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return model.SubscriptionPageConfig{}, "", err
	}
	if len(cfg.Blocks) == 0 {
		return model.SubscriptionPageConfig{}, "", fmt.Errorf("blocks must not be empty")
	}

	return cfg, trimmed, nil
}

func mergeTemplateVars(cfg model.SubscriptionPageConfig, panelSettings model.PanelSettings, subscriptionURL, importSubscriptionURL string) map[string]string {
	vars := map[string]string{}
	for key, value := range cfg.TemplateVars {
		vars[key] = value
	}

	brandName := strings.TrimSpace(panelSettings.PanelTitle)
	if brandName == "" {
		brandName = strings.TrimSpace(vars["brandName"])
	}
	if brandName == "" {
		brandName = "SubShare"
	}

	vars["brandName"] = brandName
	vars["subscriptionUrl"] = subscriptionURL
	vars["importSubscriptionUrl"] = importSubscriptionURL
	vars["adminLoginPath"] = "/admin/login"
	return vars
}

func applyTemplate(input string, vars map[string]string) string {
	output := input
	for key, value := range vars {
		output = strings.ReplaceAll(output, "{"+key+"}", value)
	}
	return output
}

// The template order is a compatibility contract. Defaults come from the same
// config returned to administrators; this private map is read only.
var subscriptionPageThemeFallbacks = defaultSubscriptionPageConfig().Theme

var subscriptionPageThemeKeys = []string{
	"pageBackground",
	"cardBackground",
	"textPrimary",
	"textSecondary",
	"textMuted",
	"border",
	"shadow",
	"heroBorder",
	"badgeBackground",
	"badgeText",
	"badgeBorder",
	"buttonPrimaryBackground",
	"buttonPrimaryHover",
	"buttonPrimaryText",
	"buttonSecondaryBackground",
	"buttonSecondaryHover",
	"buttonSecondaryBorder",
	"buttonSecondaryBorderHover",
	"buttonSecondaryText",
	"buttonSubscribeBackground",
	"buttonSubscribeHover",
	"buttonSubscribeText",
	"buttonSubscribeShadow",
	"stepCardBackground",
	"stepCardBorder",
	"stepNumberBackground",
	"stepNumberText",
	"codeBackground",
	"codeText",
	"codeLinkText",
	"languageBadgeBackground",
	"languageBadgeText",
	"fontFamily",
}

// subscriptionPageThemeArgs keeps the template slots and sanitization policy.
func subscriptionPageThemeArgs(theme map[string]string) []any {
	args := make([]any, 0, len(subscriptionPageThemeKeys))
	for _, key := range subscriptionPageThemeKeys {
		args = append(args, cssVar(theme, key, subscriptionPageThemeFallbacks[key]))
	}
	return args
}
