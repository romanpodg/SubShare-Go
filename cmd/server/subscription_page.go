package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
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

func htmlText(input string, vars map[string]string) string {
	return html.EscapeString(applyTemplate(input, vars))
}

func htmlAttr(input string, vars map[string]string) string {
	return html.EscapeString(applyTemplate(input, vars))
}

func cssVar(theme map[string]string, key string, fallback string) string {
	value := strings.TrimSpace(theme[key])
	if value == "" {
		return fallback
	}
	if len(value) > 256 || strings.ContainsAny(value, "{};<>\\\r\n") {
		return fallback
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "url(") || strings.Contains(lower, "expression(") || strings.Contains(lower, "@import") {
		return fallback
	}
	return value
}

var safeDataImagePattern = regexp.MustCompile(`(?i)^data:image/(?:png|jpeg|gif|webp);base64,[a-z0-9+/=\s]+$`)

func safePageURL(input string, vars map[string]string, allowHapp bool, allowDataImage bool) string {
	value := strings.TrimSpace(applyTemplate(input, vars))
	if value == "" || strings.ContainsAny(value, "\r\n\t\\") {
		return ""
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return html.EscapeString(value)
	}
	if allowDataImage && safeDataImagePattern.MatchString(value) {
		return html.EscapeString(value)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && !(allowHapp && scheme == "happ") {
		return ""
	}
	return html.EscapeString(value)
}

func inlineJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func buildButtonIconHTML(icon model.SubscriptionPageIcon, vars map[string]string) string {
	if icon.Type == "download" {
		return `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="stroke: currentColor; stroke-width:1.6" aria-hidden="true"><path d="M12 3v12m0 0-3-3m3 3 3-3M5 17h14" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round"/><rect x="4" y="19" width="16" height="2" fill="currentColor" stroke="none"/></svg>`
	}
	if icon.Type == "image" && strings.TrimSpace(icon.Src) != "" {
		if src := safePageURL(icon.Src, vars, false, true); src != "" {
			return `<img src="` + src + `" alt="` + htmlText(icon.Alt, vars) + `">`
		}
	}
	return ""
}

func buttonClass(variant string) string {
	switch strings.TrimSpace(variant) {
	case "primary":
		return "button primary"
	case "subscribe":
		return "button subscribe"
	default:
		return "button secondary"
	}
}

func renderSubscriptionPageHTML(cfg model.SubscriptionPageConfig, panelSettings model.PanelSettings, pageTitle, faviconURL, panelLogoURL, subscriptionURL, importSubscriptionURL string) string {
	vars := mergeTemplateVars(cfg, panelSettings, subscriptionURL, importSubscriptionURL)
	theme := cfg.Theme

	var content strings.Builder
	for _, block := range cfg.Blocks {
		switch block.Type {
		case "hero":
			logoSrc := strings.TrimSpace(panelLogoURL)
			if logoSrc == "" {
				logoSrc = applyTemplate(block.Logo.Src, vars)
			}
			logoAlt := block.Logo.Alt
			if strings.TrimSpace(logoAlt) == "" {
				logoAlt = "logo"
			}

			content.WriteString(`<div class="hero">`)
			content.WriteString(`<div class="brand">`)
			content.WriteString(`<div class="fox-logo">`)
			if strings.TrimSpace(logoSrc) != "" {
				if src := safePageURL(logoSrc, vars, false, true); src != "" {
					content.WriteString(`<img src="` + src + `" alt="` + htmlText(logoAlt, vars) + `">`)
				}
			}
			content.WriteString(`</div>`)
			content.WriteString(`<div class="brand-text">`)
			content.WriteString(`<h1>` + htmlText(block.BrandTitle, vars) + `</h1>`)
			content.WriteString(`<p>` + htmlText(block.BrandSubtitle, vars) + `</p>`)
			content.WriteString(`</div></div>`)
			if strings.TrimSpace(block.Subhead) != "" {
				content.WriteString(`<div class="subhead">` + htmlText(block.Subhead, vars) + `</div>`)
			}
			if strings.TrimSpace(block.StatusBadge) != "" {
				content.WriteString(`<div class="status-badge"><span class="status-dot"></span>` + htmlText(block.StatusBadge, vars) + `</div>`)
			}
			content.WriteString(`</div>`)

		case "steps":
			sectionID := strings.TrimSpace(block.ID)
			if sectionID == "" {
				sectionID = "subscription"
			}
			content.WriteString(`<section class="section" id="` + html.EscapeString(sectionID) + `">`)
			content.WriteString(`<h2>` + htmlText(block.Title, vars) + `</h2>`)
			content.WriteString(`<div class="steps">`)
			for idx, step := range block.Steps {
				content.WriteString(`<div class="step">`)
				content.WriteString(`<div class="step-number">` + fmt.Sprintf("%d", idx+1) + `</div>`)
				content.WriteString(`<div class="step-content">`)
				content.WriteString(`<div class="step-title">` + htmlText(step.Title, vars) + `</div>`)
				content.WriteString(`<div class="step-text">` + htmlText(step.Description, vars) + `</div>`)

				switch step.Block.Type {
				case "linkButtons":
					if len(step.Block.Buttons) > 0 {
						content.WriteString(`<div class="buttons">`)
						for _, btn := range step.Block.Buttons {
							href := applyTemplate(btn.Href, vars)
							if strings.TrimSpace(href) == "" {
								continue
							}
							safeHref := safePageURL(href, vars, false, false)
							if safeHref == "" {
								continue
							}
							content.WriteString(`<a class="` + buttonClass(btn.Variant) + `" href="` + safeHref + `"`)
							if btn.External {
								content.WriteString(` target="_blank" rel="noreferrer"`)
							}
							content.WriteString(`>`)
							content.WriteString(buildButtonIconHTML(btn.Icon, vars))
							content.WriteString(`<span>` + htmlText(btn.Label, vars) + `</span></a>`)
						}
						content.WriteString(`</div>`)
					}
				case "activation":
					addLabel := step.Block.AddButtonLabel
					if strings.TrimSpace(addLabel) == "" {
						addLabel = "🦊 Добавить подписку в Happ"
					}
					manualLabel := step.Block.ManualLinkLabel
					if strings.TrimSpace(manualLabel) == "" {
						manualLabel = "🔗 Ссылка подписки:"
					}
					copyLabel := step.Block.CopyLabel
					if strings.TrimSpace(copyLabel) == "" {
						copyLabel = "📋 Копировать"
					}

					content.WriteString(`<div class="buttons">`)
					if safeImportURL := safePageURL(importSubscriptionURL, vars, true, false); safeImportURL != "" {
						content.WriteString(`<a id="btnAddHapp" class="button subscribe" href="` + safeImportURL + `">` + htmlText(addLabel, vars) + `</a>`)
					}
					content.WriteString(`</div>`)
					content.WriteString(`<div class="sub-link-row" id="manualLinkRow" style="display: none;">`)
					content.WriteString(`<span>` + htmlText(manualLabel, vars) + `</span>`)
					content.WriteString(`<code id="plainSubUrl" style="font-size:0.75rem; color:var(--code-link-text); word-break:break-all;"></code>`)
					content.WriteString(`<button class="copy-link" id="copySubBtn" type="button">` + htmlText(copyLabel, vars) + `</button>`)
					content.WriteString(`</div>`)
				}

				content.WriteString(`</div></div>`)
			}
			content.WriteString(`</div></section>`)

		case "footer":
			content.WriteString(`<footer class="footer">`)
			content.WriteString(`<span>` + htmlText(block.Copyright, vars) + `</span>`)
			content.WriteString(`<div class="footer-right">`)
			if block.FooterLink != nil && strings.TrimSpace(block.FooterLink.Href) != "" {
				if safeHref := safePageURL(block.FooterLink.Href, vars, false, false); safeHref != "" {
					content.WriteString(`<a class="footer-link" href="` + safeHref + `">` + htmlText(block.FooterLink.Label, vars) + `</a>`)
				}
			}
			content.WriteString(`<div class="lang">` + htmlText(block.LanguageBadge, vars) + `</div></div></footer>`)
		}
	}

	faviconTag := ""
	if strings.TrimSpace(faviconURL) != "" {
		if safeFaviconURL := safePageURL(faviconURL, vars, false, true); safeFaviconURL != "" {
			faviconTag = `<link rel="icon" href="` + safeFaviconURL + `">`
		}
	}

	copiedLabel := "✅ Скопировано"
	copyLabel := "📋 Копировать"
	for _, block := range cfg.Blocks {
		if block.Type != "steps" {
			continue
		}
		for _, step := range block.Steps {
			if step.Block.Type != "activation" {
				continue
			}
			if strings.TrimSpace(step.Block.CopiedLabel) != "" {
				copiedLabel = applyTemplate(step.Block.CopiedLabel, vars)
			}
			if strings.TrimSpace(step.Block.CopyLabel) != "" {
				copyLabel = applyTemplate(step.Block.CopyLabel, vars)
			}
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
  <title>%s</title>

  <meta name="robots" content="noindex, nofollow, noarchive, nosnippet, notranslate, noimageindex">
  <meta name="googlebot" content="noindex, nofollow, noarchive, nosnippet, notranslate, noimageindex">
  <meta name="yandex" content="noindex, nofollow, noarchive">
  <meta name="bingbot" content="noindex, nofollow">
  <meta name="slurp" content="noindex, nofollow">

  <meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate">
  <meta http-equiv="Pragma" content="no-cache">
  <meta http-equiv="Expires" content="0">
  %s

  <style>
    :root {
      --page-bg: %s;
      --card-bg: %s;
      --text-primary: %s;
      --text-secondary: %s;
      --text-muted: %s;
      --border: %s;
      --shadow: %s;
      --hero-border: %s;
      --badge-bg: %s;
      --badge-text: %s;
      --badge-border: %s;
      --btn-primary-bg: %s;
      --btn-primary-hover: %s;
      --btn-primary-text: %s;
      --btn-secondary-bg: %s;
      --btn-secondary-hover: %s;
      --btn-secondary-border: %s;
      --btn-secondary-border-hover: %s;
      --btn-secondary-text: %s;
      --btn-subscribe-bg: %s;
      --btn-subscribe-hover: %s;
      --btn-subscribe-text: %s;
      --btn-subscribe-shadow: %s;
      --step-card-bg: %s;
      --step-card-border: %s;
      --step-number-bg: %s;
      --step-number-text: %s;
      --code-bg: %s;
      --code-text: %s;
      --code-link-text: %s;
      --lang-bg: %s;
      --lang-text: %s;
      --font-family: %s;
    }

    * { margin: 0; padding: 0; box-sizing: border-box; }

    body {
      background: var(--page-bg);
      color: var(--text-primary);
      line-height: 1.5;
      padding: 2rem 1rem;
      font-family: var(--font-family);
    }

    .container { max-width: 780px; margin: 0 auto; }

    .card {
      background: var(--card-bg);
      border-radius: 32px;
      box-shadow: var(--shadow);
      overflow: hidden;
      transition: all 0.2s ease;
    }

    .hero { padding: 2rem 2rem 1.5rem 2rem; border-bottom: 1px solid var(--hero-border); }

    .brand { display: flex; align-items: center; gap: 14px; margin-bottom: 1.2rem; }

    .fox-logo {
      width: 56px;
      height: 56px;
      flex-shrink: 0;
      border-radius: 60px;
      background: #f1f4f9;
      display: flex;
      align-items: center;
      justify-content: center;
      overflow: hidden;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.02);
      transition: transform 0.2s ease;
    }

    .fox-logo img { width: 100%%; height: 100%%; object-fit: cover; display: block; }
    .fox-logo:hover { transform: scale(1.02); }

    .brand-text h1 {
      font-size: 1.8rem;
      font-weight: 600;
      letter-spacing: -0.3px;
      color: var(--text-primary);
      line-height: 0.8;
      margin-top: 6px;
    }

    .brand-text p {
      font-size: 0.85rem;
      color: var(--text-muted);
      letter-spacing: 0.3px;
      margin-top: 4px;
    }

    .subhead {
      font-size: 0.95rem;
      color: var(--text-secondary);
      margin-top: 12px;
      max-width: 85%%;
      border-left: 3px solid #facc15;
      padding-left: 14px;
    }

    .status-badge {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      background: var(--badge-bg);
      border: 1px solid var(--badge-border);
      padding: 8px 18px;
      border-radius: 40px;
      font-size: 0.85rem;
      font-weight: 500;
      color: var(--badge-text);
      width: fit-content;
      margin-top: 20px;
    }

    .status-dot { width: 8px; height: 8px; border-radius: 999px; background: #22c55e; }

    .section { padding: 1.8rem 2rem; border-top: 1px solid var(--border); }

    .section h2 {
      font-size: 1.25rem;
      font-weight: 600;
      margin-bottom: 1.3rem;
      color: #0b1e33;
      letter-spacing: -0.2px;
    }

    .steps { display: flex; flex-direction: column; gap: 18px; }

    .step {
      display: flex;
      gap: 18px;
      align-items: flex-start;
      background: var(--step-card-bg);
      border: 1px solid var(--step-card-border);
      border-radius: 24px;
      padding: 18px 20px;
      transition: box-shadow 0.1s;
    }

    .step-number {
      width: 38px;
      height: 38px;
      background: var(--step-number-bg);
      border-radius: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 700;
      color: var(--step-number-text);
      font-size: 1rem;
      flex-shrink: 0;
    }

    .step-content { flex: 1; }
    .step-title { font-weight: 600; margin-bottom: 6px; color: var(--text-primary); }
    .step-text { font-size: 0.88rem; color: #4b5e7c; margin-bottom: 12px; }

    .buttons { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 12px; }

    .button {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      padding: 10px 20px;
      border-radius: 40px;
      font-size: 0.85rem;
      font-weight: 500;
      text-decoration: none;
      transition: all 0.2s ease;
      background: #ffffff;
      border: 1px solid #e0e7ef;
      color: #1f2a44;
      cursor: pointer;
      font-family: inherit;
    }

    .button img { width: 20px; height: 20px; display: inline-block; object-fit: contain; filter: none; }

    .button.primary {
      background: var(--btn-primary-bg);
      border-color: var(--btn-primary-bg);
      color: var(--btn-primary-text);
    }

    .button.primary:hover { background: var(--btn-primary-hover); transform: translateY(-1px); }

    .button.secondary {
      background: var(--btn-secondary-bg);
      border-color: var(--btn-secondary-border);
      color: var(--btn-secondary-text);
    }

    .button.secondary:hover {
      background: var(--btn-secondary-hover);
      border-color: var(--btn-secondary-border-hover);
    }

    .button.subscribe {
      background: var(--btn-subscribe-bg);
      border-color: var(--btn-subscribe-bg);
      color: var(--btn-subscribe-text);
      font-weight: 600;
    }

    .button.subscribe:hover {
      background: var(--btn-subscribe-hover);
      transform: translateY(-1px);
      box-shadow: var(--btn-subscribe-shadow);
    }

    .sub-link-row {
      margin-top: 14px;
      background: var(--code-bg);
      border-radius: 20px;
      padding: 12px 16px;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      font-size: 0.8rem;
      border: 1px solid var(--border);
    }

    .sub-link-row span { color: var(--code-text); }

    .copy-link {
      background: none;
      border: none;
      color: #3b6eff;
      font-weight: 500;
      cursor: pointer;
      font-size: 0.8rem;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-family: inherit;
    }

    .footer {
      padding: 1.2rem 2rem;
      border-top: 1px solid var(--border);
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
      font-size: 0.75rem;
      color: #6c7f9e;
      gap: 10px;
    }

    .footer-right { display: flex; align-items: center; gap: 10px; }

    .footer-link {
      color: #3b6eff;
      text-decoration: none;
      font-weight: 500;
    }

    .footer-link:hover { text-decoration: underline; }

    .lang {
      background: var(--lang-bg);
      color: var(--lang-text);
      padding: 5px 12px;
      border-radius: 30px;
      font-size: 0.75rem;
    }

    @media (max-width: 640px) {
      body { padding: 1rem; }
      .hero { padding: 1.5rem 1.2rem; }
      .section { padding: 1.4rem 1.2rem; }
      .footer { padding: 1rem 1.2rem; }
      .step { flex-direction: column; gap: 12px; padding: 16px; }
      .step-number { width: 34px; height: 34px; }
      .buttons { flex-direction: column; }
      .button { justify-content: center; width: 100%%; }
      .brand-text h1 { font-size: 1.5rem; }
      .subhead { max-width: 100%%; }
      .fox-logo { width: 48px; height: 48px; border-radius: 18px; }
    }

    @media (max-width: 480px) {
      .hero { padding: 1.2rem; }
      .sub-link-row { flex-direction: column; align-items: flex-start; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="card">
      %s
    </div>
  </div>

  <script>
    const plainSubscriptionUrl = %s;
    const importSubscriptionUrl = %s;
    const copiedLabel = %s;
    const copyLabel = %s;

    const addBtn = document.getElementById("btnAddHapp");
    const manualRow = document.getElementById("manualLinkRow");
    const plainUrlNode = document.getElementById("plainSubUrl");
    const copyBtn = document.getElementById("copySubBtn");

    if (plainUrlNode) {
      plainUrlNode.textContent = plainSubscriptionUrl;
    }

    function copySubscriptionUrl() {
      if (!plainSubscriptionUrl) return;
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(plainSubscriptionUrl).then(() => {
          if (copyBtn) {
            copyBtn.textContent = copiedLabel;
            setTimeout(() => {
              copyBtn.textContent = copyLabel;
            }, 1800);
          }
        }).catch(() => {
          window.prompt("Скопируйте ссылку вручную:", plainSubscriptionUrl);
        });
      } else {
        window.prompt("Скопируйте ссылку вручную:", plainSubscriptionUrl);
      }
    }

    if (copyBtn) {
      copyBtn.addEventListener("click", copySubscriptionUrl);
    }

    if (addBtn) {
      addBtn.addEventListener("click", function(event) {
        event.preventDefault();
        const deepLink = importSubscriptionUrl.startsWith("happ://")
          ? importSubscriptionUrl
          : "happ://add/" + encodeURIComponent(importSubscriptionUrl);

        let appOpened = false;
        const onBlur = () => { appOpened = true; };
        window.addEventListener("blur", onBlur, { once: true });
        window.location.href = deepLink;

        setTimeout(() => {
          window.removeEventListener("blur", onBlur);
          if (appOpened) {
            return;
          }
          if (manualRow) {
            manualRow.style.display = "flex";
          }
          copySubscriptionUrl();
        }, 1500);
      });
    }
  </script>
</body>
</html>`,
		html.EscapeString(pageTitle),
		faviconTag,
		cssVar(theme, "pageBackground", "#f7f9fc"),
		cssVar(theme, "cardBackground", "#ffffff"),
		cssVar(theme, "textPrimary", "#0f172a"),
		cssVar(theme, "textSecondary", "#334155"),
		cssVar(theme, "textMuted", "#5b6e8c"),
		cssVar(theme, "border", "#eef2f6"),
		cssVar(theme, "shadow", "0 12px 30px rgba(0,0,0,0.05), 0 4px 8px rgba(0,0,0,0.02)"),
		cssVar(theme, "heroBorder", "#eef2f6"),
		cssVar(theme, "badgeBackground", "#eef2ff"),
		cssVar(theme, "badgeText", "#1f3a8a"),
		cssVar(theme, "badgeBorder", "#dbe4ff"),
		cssVar(theme, "buttonPrimaryBackground", "#0f172a"),
		cssVar(theme, "buttonPrimaryHover", "#1e293b"),
		cssVar(theme, "buttonPrimaryText", "#ffffff"),
		cssVar(theme, "buttonSecondaryBackground", "#ffffff"),
		cssVar(theme, "buttonSecondaryHover", "#f8fafd"),
		cssVar(theme, "buttonSecondaryBorder", "#dce3ec"),
		cssVar(theme, "buttonSecondaryBorderHover", "#cbd5e1"),
		cssVar(theme, "buttonSecondaryText", "#1f2a44"),
		cssVar(theme, "buttonSubscribeBackground", "#facc15"),
		cssVar(theme, "buttonSubscribeHover", "#fde047"),
		cssVar(theme, "buttonSubscribeText", "#0f172a"),
		cssVar(theme, "buttonSubscribeShadow", "0 4px 8px rgba(250, 204, 21, 0.2)"),
		cssVar(theme, "stepCardBackground", "#ffffff"),
		cssVar(theme, "stepCardBorder", "#eef2f8"),
		cssVar(theme, "stepNumberBackground", "#f1f4f9"),
		cssVar(theme, "stepNumberText", "#2c3e66"),
		cssVar(theme, "codeBackground", "#fafcff"),
		cssVar(theme, "codeText", "#5b6e8c"),
		cssVar(theme, "codeLinkText", "#2563eb"),
		cssVar(theme, "languageBadgeBackground", "#f1f5f9"),
		cssVar(theme, "languageBadgeText", "#334155"),
		cssVar(theme, "fontFamily", "'Inter', system-ui, -apple-system, 'Segoe UI', Roboto, Helvetica, sans-serif"),
		content.String(),
		inlineJSON(subscriptionURL),
		inlineJSON(importSubscriptionURL),
		inlineJSON(copiedLabel),
		inlineJSON(copyLabel),
	)
}

func (a *App) apiGetSubscriptionPageConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")

	settings, err := a.getPanelSettings()
	if err != nil {
		log.Printf("apiGetSubscriptionPageConfig: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription page config")
		return
	}

	_, pretty, err := normalizeSubscriptionPageConfig(settings.SubscriptionPageConfig)
	if err != nil {
		log.Printf("apiGetSubscriptionPageConfig: invalid stored config: %v", err)
		pretty = defaultSubscriptionPageConfigJSON
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"config_json":         pretty,
		"default_config_json": defaultSubscriptionPageConfigJSON,
	})
}

func (a *App) apiUpdateSubscriptionPageConfig(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSubscriptionPageConfigRequest
	if err := readJSONWithLimit(w, r, &req, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, pretty, err := normalizeSubscriptionPageConfig(req.ConfigJSON)
	if err != nil {
		writeError(w, http.StatusBadRequest, "config_json must be a valid JSON config")
		return
	}

	if err := a.updateSubscriptionPageConfig(pretty); err != nil {
		log.Printf("apiUpdateSubscriptionPageConfig: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save subscription page config")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"config_json": pretty})
}
