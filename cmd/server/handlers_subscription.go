package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

// --- Subscription API ---

func (a *App) apiActivateSubscription(w http.ResponseWriter, r *http.Request) {
	var req model.ActivateRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	activationCode := strings.TrimSpace(req.ActivationCode)
	if activationCode == "" || strings.Contains(activationCode, "/") {
		httpapi.WriteError(w, r, http.StatusBadRequest, "Введите корректный ключ активации")
		return
	}

	subscriptionID, code, reason, err := a.redeemActivationCode(activationCode)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to activate subscription")
		return
	}
	if code != http.StatusOK {
		if strings.TrimSpace(reason) == "" {
			reason = "Не удалось активировать подписку"
		}
		httpapi.WriteError(w, r, code, reason)
		return
	}

	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	if strings.TrimSpace(a.happCryptoAPIURL) != "" {
		if encryptedURL, err := a.encryptSubscriptionURL(subscriptionURL); err == nil && strings.TrimSpace(encryptedURL) != "" {
			subscriptionURL = encryptedURL
		} else if err != nil {
			log.Printf("apiActivateSubscription: failed to encrypt url via configured Happ API: %v", err)
		}
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"subscription_url": subscriptionURL,
		"message":          "Ключ активирован. Ссылка готова — скопируйте и вставьте её в VPN-клиент",
	})
}

// --- Subscription delivery (unchanged --- serves plaintext for VPN clients) ---

func (a *App) handleSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}

	prepared, denial, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		log.Printf("prepare subscription delivery: %v", err)
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if denial.denied() {
		writeSubscriptionDenial(w, denial)
		a.incrementSubscriptionMetric(denial.Status, "denied")
		return
	}
	rule := prepared.Rule
	if (rule != nil && rule.ResponseType == "browser") || (rule == nil && isBrowserSubscriptionRequest(r)) {
		a.renderSubscriptionBrowserPage(w, r, subscriptionID)
		a.incrementSubscriptionMetric("browser", "success")
		return
	}

	responseType := ""
	if rule != nil {
		responseType = rule.ResponseType
	}
	generated, ok := a.writeGeneratedOrDeny(w, r, prepared, responseType)
	if !ok {
		return
	}
	settings := prepared.Settings
	body := generated.Body
	if rule != nil {
		if template, loadErr := a.loadTemplate(rule.TemplateID); loadErr == nil && template != nil && template.Enabled {
			body = applyTemplateContent(template.Content, body, settings.Title)
		}
		applyRuleHeaders(w, rule.Headers)
	}
	if err := delivery.ValidateStructuredBody(responseType, body); err != nil {
		http.Error(w, "failed to render subscription format", http.StatusUnprocessableEntity)
		a.incrementSubscriptionMetric(responseType, "render_failed")
		return
	}
	body, responseType = a.encodeSubscriptionBody(w.Header(), responseType, body, settings)
	_, _ = w.Write([]byte(body))
	a.incrementSubscriptionMetric(responseType, "success")
}

// encodeSubscriptionBody sets the content headers for the response type and
// returns the final body together with the effective response type used for
// metrics. An empty response type falls back to the installation defaults.
func (a *App) encodeSubscriptionBody(header http.Header, responseType, body string, settings model.SubscriptionSettings) (string, string) {
	switch responseType {
	case "base64":
		body = delivery.EncodeBase64(body)
		header.Set("Content-Type", "text/plain; charset=utf-8")
	case "plain":
		header.Set("Content-Type", "text/plain; charset=utf-8")
	case "mihomo":
		header.Set("Content-Type", "application/yaml; charset=utf-8")
		header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, sanitizeSubscriptionFilenamePart(settings.Title)))
	case "sing-box", "xray-json":
		header.Set("Content-Type", "application/json; charset=utf-8")
		header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, sanitizeSubscriptionFilenamePart(settings.Title)))
	default:
		if a.subscriptionBodyEncoding == "base64" && settings.SubscriptionFormat != model.SubscriptionFormatXrayJSON {
			body = base64.StdEncoding.EncodeToString([]byte(body))
			responseType = "base64"
		} else if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
			responseType = "xray-json"
		} else {
			responseType = "plain"
		}
	}
	return body, responseType
}

// writeGeneratedOrDeny renders the subscription for a prepared request and
// writes the common headers. It returns false after writing a denial or
// error response.
func (a *App) writeGeneratedOrDeny(w http.ResponseWriter, r *http.Request, prepared subscriptionDeliveryContext, responseType string) (delivery.Generated, bool) {
	generated, denial, err := a.generateSubscription(prepared, responseType)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return generated, false
	}
	delivery.ApplyExclusionHeaders(w.Header(), generated.Exclusions)
	if denial.denied() {
		if denial.Code == http.StatusUnprocessableEntity && denial.Reason == delivery.ReasonAllExcluded {
			httpapi.WriteJSON(w, denial.Code, delivery.FailurePayload(generated))
			a.incrementSubscriptionMetric(generated.OutputFormat, "all_excluded")
			return generated, false
		}
		status := remarkStatusFromReason(denial.Reason)
		writeSubscriptionDenial(w, deny(denial.Code, status, a.subscriptionRemark(denial.Reason)))
		a.incrementSubscriptionMetric(status, "denied")
		return generated, false
	}
	a.applySubscriptionResponseHeaders(w, r, prepared.Settings, prepared.SubscriptionID)
	a.applyGlobalDeliveryHeaders(w)
	applySubscriptionExpirationMetadata(w.Header(), prepared)
	return generated, true
}

func sanitizeSubscriptionFilenamePart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "subscription"
	}

	var builder strings.Builder
	for _, ch := range raw {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			builder.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
		case ch == '-', ch == '_', ch == '.':
			builder.WriteRune(ch)
		default:
			builder.WriteRune('_')
		}
	}

	name := strings.Trim(strings.TrimSpace(builder.String()), "._")
	if name == "" {
		return "subscription"
	}
	return name
}

func buildSubscriptionAttachmentFilename(settings model.SubscriptionSettings) string {
	base := sanitizeSubscriptionFilenamePart(settings.Title)
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		return base + ".json"
	}
	return base + ".txt"
}

func (a *App) applySubscriptionResponseHeaders(
	w http.ResponseWriter,
	r *http.Request,
	settings model.SubscriptionSettings,
	subscriptionID string,
) {
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}

	filename := buildSubscriptionAttachmentFilename(settings)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if title := strings.TrimSpace(settings.Title); title != "" {
		w.Header().Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	}
	if settings.RefreshHours > 0 {
		w.Header().Set("profile-update-interval", strconv.Itoa(settings.RefreshHours))
	}
	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	profileURL := strings.TrimSpace(settings.InfoURL)
	if profileURL == "" {
		profileURL = subscriptionURL
	}
	w.Header().Set("profile-web-page-url", profileURL)
	w.Header().Set("Subscription-Status", "active")
	if providerID := strings.TrimSpace(settings.ProviderID); providerID != "" {
		w.Header().Set("providerid", providerID)
	}
	if extraURL := strings.TrimSpace(settings.ExtraURL); extraURL != "" {
		w.Header().Set("support-url", extraURL)
	}
	if extraStatus := strings.TrimSpace(settings.ExtraStatus); extraStatus != "" {
		w.Header().Set("announce", "base64:"+base64.StdEncoding.EncodeToString([]byte(extraStatus)))
	}
}

func subscriptionUserinfoWithExpire(current string, expire *int64) string {
	parts := strings.Split(current, ";")
	result := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, _, _ := strings.Cut(part, "=")
		if strings.EqualFold(strings.TrimSpace(key), "expire") {
			continue
		}
		result = append(result, part)
	}
	if expire != nil {
		result = append(result, "expire="+strconv.FormatInt(*expire, 10))
	}
	return strings.Join(result, "; ")
}

func applySubscriptionExpirationMetadata(header http.Header, ctx subscriptionDeliveryContext) {
	var expire *int64
	if ctx.Settings.ShowSubscriptionExpiration && ctx.ExpiresAt.Valid {
		value := ctx.ExpiresAt.Time.UTC().Unix()
		expire = &value
	}
	userinfo := subscriptionUserinfoWithExpire(header.Get("Subscription-Userinfo"), expire)
	if userinfo == "" {
		header.Del("Subscription-Userinfo")
	} else {
		header.Set("Subscription-Userinfo", userinfo)
	}
}

func (a *App) handleSubscriptionSubBody(w http.ResponseWriter, r *http.Request) {
	a.serveSubscriptionBody(w, r, true)
}

func (a *App) handleSubscriptionSubBodyPlain(w http.ResponseWriter, r *http.Request) {
	a.serveSubscriptionBody(w, r, false)
}

// serveSubscriptionBody is the /subbody endpoint: the plain rendering, either
// base64-wrapped or raw.
func (a *App) serveSubscriptionBody(w http.ResponseWriter, r *http.Request, wrapBase64 bool) {
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}
	prepared, denial, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if denial.denied() {
		writeSubscriptionDenial(w, denial)
		a.incrementSubscriptionMetric(denial.Status, "denied")
		return
	}
	generated, ok := a.writeGeneratedOrDeny(w, r, prepared, "plain")
	if !ok {
		return
	}
	if wrapBase64 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(delivery.EncodeBase64(generated.Body)))
		a.incrementSubscriptionMetric("subbody-base64", "success")
		return
	}
	if prepared.Settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	_, _ = w.Write([]byte(generated.Body))
	a.incrementSubscriptionMetric("subbody-plain", "success")
}

func isBrowserSubscriptionRequest(r *http.Request) bool {
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if !strings.Contains(accept, "text/html") {
		return false
	}

	secFetchDest := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Dest")))
	if secFetchDest != "" && secFetchDest != "document" {
		return false
	}

	return true
}

func (a *App) renderSubscriptionBrowserPage(w http.ResponseWriter, r *http.Request, subscriptionID string) {
	allowed, _, code, reason, err := a.subscriptionAccessAllowed(subscriptionID)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, reason, code)
		return
	}

	panelSettings, err := a.getPanelSettings()
	if err != nil {
		panelSettings = model.PanelSettings{
			PanelTitle:            "SubShare",
			PageTitleSubscription: "VPN-подписка — SubShare",
		}
	}

	pageTitle := strings.TrimSpace(panelSettings.PageTitleSubscription)
	if pageTitle == "" {
		pageTitle = "VPN-подписка — SubShare"
	}

	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	importSubscriptionURL := subscriptionURL
	if encryptedURL, err := a.encryptSubscriptionURL(subscriptionURL); err == nil && strings.TrimSpace(encryptedURL) != "" {
		importSubscriptionURL = encryptedURL
	} else if err != nil {
		log.Printf("renderSubscriptionBrowserPage: failed to encrypt url via happ api: %v", err)
	}

	cfg, _, err := normalizeSubscriptionPageConfig(panelSettings.SubscriptionPageConfig)
	if err != nil {
		log.Printf("renderSubscriptionBrowserPage: invalid subscription page config, using default: %v", err)
		cfg = defaultSubscriptionPageConfig()
	}

	htmlDoc := renderSubscriptionPageHTML(
		cfg,
		panelSettings,
		pageTitle,
		strings.TrimSpace(panelSettings.FaviconDataURL),
		strings.TrimSpace(panelSettings.LogoDataURL),
		subscriptionURL,
		importSubscriptionURL,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Subscription-Status", "active")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data: https: http:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	_, _ = w.Write([]byte(htmlDoc))
}

// --- Data export API ---

func (a *App) apiExportUsers(w http.ResponseWriter, r *http.Request) {
	applySensitiveResponseHeaders(w)
	users, err := a.listUsers()
	if err != nil {
		log.Printf("apiExportUsers: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to export users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="users.json"`)
	httpapi.WriteJSON(w, http.StatusOK, users)
}

func (a *App) apiGetSubscriptionInfo(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("subscription_id")
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}
	prepared, denial, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription info", http.StatusInternalServerError)
		return
	}
	if denial.denied() {
		writeSubscriptionDenial(w, denial)
		a.incrementSubscriptionMetric(denial.Status, "denied")
		return
	}
	w.Header().Set("Subscription-Status", "active")
	if providerID := strings.TrimSpace(prepared.Settings.ProviderID); providerID != "" {
		w.Header().Set("providerid", providerID)
	}

	var user struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}

	var expiresAt sql.NullTime
	err = a.db.QueryRow(`
SELECT name, status, expires_at
FROM users
WHERE subscription_id = ?
`, subscriptionID).Scan(&user.Name, &user.Status, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("apiGetSubscriptionInfo: %v", err)
		http.Error(w, "failed to load subscription info", http.StatusInternalServerError)
		return
	}

	if expiresAt.Valid {
		user.ExpiresAt = expiresAt.Time.Format(time.RFC3339)
	}

	httpapi.WriteJSON(w, http.StatusOK, user)
	a.incrementSubscriptionMetric("info", "success")
}
