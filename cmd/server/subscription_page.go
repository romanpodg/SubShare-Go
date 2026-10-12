package main

import (
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func htmlText(input string, vars map[string]string) string {
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
	if scheme != "http" && scheme != "https" && (!allowHapp || scheme != "happ") {
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

// defaultIfBlank returns fallback when value is empty or whitespace.
func defaultIfBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func renderHeroBlock(content *strings.Builder, block model.SubscriptionPageBlock, vars map[string]string, panelLogoURL string) {
	logoSrc := strings.TrimSpace(panelLogoURL)
	if logoSrc == "" {
		logoSrc = applyTemplate(block.Logo.Src, vars)
	}
	logoAlt := defaultIfBlank(block.Logo.Alt, "logo")

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
}

func renderStepsBlock(content *strings.Builder, block model.SubscriptionPageBlock, vars map[string]string, importSubscriptionURL string) {
	sectionID := defaultIfBlank(strings.TrimSpace(block.ID), "subscription")
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
			renderLinkButtons(content, step.Block.Buttons, vars)
		case "activation":
			renderActivationBlock(content, step.Block, vars, importSubscriptionURL)
		}

		content.WriteString(`</div></div>`)
	}
	content.WriteString(`</div></section>`)
}

func renderLinkButtons(content *strings.Builder, buttons []model.SubscriptionPageButton, vars map[string]string) {
	if len(buttons) == 0 {
		return
	}
	content.WriteString(`<div class="buttons">`)
	for _, btn := range buttons {
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

func renderActivationBlock(content *strings.Builder, block model.SubscriptionPageStepBlock, vars map[string]string, importSubscriptionURL string) {
	addLabel := defaultIfBlank(block.AddButtonLabel, "🦊 Добавить подписку в Happ")
	manualLabel := defaultIfBlank(block.ManualLinkLabel, "🔗 Ссылка подписки:")
	copyLabel := defaultIfBlank(block.CopyLabel, "📋 Копировать")

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

func renderFooterBlock(content *strings.Builder, block model.SubscriptionPageBlock, vars map[string]string) {
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

func faviconLinkTag(faviconURL string, vars map[string]string) string {
	if strings.TrimSpace(faviconURL) == "" {
		return ""
	}
	safeFaviconURL := safePageURL(faviconURL, vars, false, true)
	if safeFaviconURL == "" {
		return ""
	}
	return `<link rel="icon" href="` + safeFaviconURL + `">`
}

// activationCopyLabels returns the copy-button labels used by the page script,
// taking the last activation block's overrides when present.
func activationCopyLabels(cfg model.SubscriptionPageConfig, vars map[string]string) (string, string) {
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
	return copiedLabel, copyLabel
}

const subscriptionPageTemplate = `<!DOCTYPE html>
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
</html>`

func renderSubscriptionPageHTML(cfg model.SubscriptionPageConfig, panelSettings model.PanelSettings, pageTitle, faviconURL, panelLogoURL, subscriptionURL, importSubscriptionURL string) string {
	vars := mergeTemplateVars(cfg, panelSettings, subscriptionURL, importSubscriptionURL)
	theme := cfg.Theme

	var content strings.Builder
	for _, block := range cfg.Blocks {
		switch block.Type {
		case "hero":
			renderHeroBlock(&content, block, vars, panelLogoURL)
		case "steps":
			renderStepsBlock(&content, block, vars, importSubscriptionURL)
		case "footer":
			renderFooterBlock(&content, block, vars)
		}
	}

	faviconTag := faviconLinkTag(faviconURL, vars)
	copiedLabel, copyLabel := activationCopyLabels(cfg, vars)

	args := []any{html.EscapeString(pageTitle), faviconTag}
	args = append(args, subscriptionPageThemeArgs(theme)...)
	args = append(args,
		content.String(),
		inlineJSON(subscriptionURL),
		inlineJSON(importSubscriptionURL),
		inlineJSON(copiedLabel),
		inlineJSON(copyLabel),
	)
	return fmt.Sprintf(subscriptionPageTemplate, args...)
}

func (a *App) apiGetSubscriptionPageConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")

	settings, err := a.getPanelSettings()
	if err != nil {
		log.Printf("apiGetSubscriptionPageConfig: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load subscription page config")
		return
	}

	_, pretty, err := normalizeSubscriptionPageConfig(settings.SubscriptionPageConfig)
	if err != nil {
		log.Printf("apiGetSubscriptionPageConfig: invalid stored config: %v", err)
		pretty = defaultSubscriptionPageConfigJSON
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"config_json":         pretty,
		"default_config_json": defaultSubscriptionPageConfigJSON,
	})
}

func (a *App) apiUpdateSubscriptionPageConfig(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSubscriptionPageConfigRequest
	if err := httpapi.ReadJSONWithLimit(w, r, &req, 1<<20); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	_, pretty, err := normalizeSubscriptionPageConfig(req.ConfigJSON)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "config_json must be a valid JSON config")
		return
	}

	if err := a.updateSubscriptionPageConfig(pretty); err != nil {
		log.Printf("apiUpdateSubscriptionPageConfig: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to save subscription page config")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"config_json": pretty})
}
