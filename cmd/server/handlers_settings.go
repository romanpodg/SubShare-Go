package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) apiGetPanelSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getPanelSettings()
	if err != nil {
		log.Printf("apiGetPanelSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load panel settings")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, settings)
}

func (a *App) apiUpdatePanelSettings(w http.ResponseWriter, r *http.Request) {
	// Panel settings may contain base64-encoded images — allow up to 16 MB.
	var req model.PanelSettings
	if err := httpapi.ReadJSONWithLimit(w, r, &req, 16<<20); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	req.PanelTitle = strings.TrimSpace(req.PanelTitle)
	if req.PanelTitle == "" {
		req.PanelTitle = "SubShare"
	}
	if req.PageTitleAdmin == "" {
		req.PageTitleAdmin = "Панель управления — SubShare"
	}
	if req.PageTitleAdminLogin == "" {
		req.PageTitleAdminLogin = "Вход — SubShare"
	}
	if req.PageTitleSubscription == "" {
		req.PageTitleSubscription = "VPN-подписка — SubShare"
	}
	if err := a.updatePanelSettings(req); err != nil {
		log.Printf("apiUpdatePanelSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to save panel settings")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, req)
}

func (a *App) apiGetSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getSubscriptionSettings()
	if err != nil {
		log.Printf("apiGetSubscriptionSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load subscription settings")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, settings)
}

// normalizeSettingsURL accepts an empty value or an absolute URL.
func normalizeSettingsURL(raw string, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New(field + " must be a valid absolute URL")
	}
	return raw, nil
}

func normalizeSettingsTimeZone(raw string) (string, error) {
	timeZone := strings.TrimSpace(raw)
	if timeZone == "" {
		timeZone = "Europe/Moscow"
	}
	if len(timeZone) > 64 {
		return "", errors.New("time_zone is too long (max 64 characters)")
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		return "", errors.New("time_zone must be a valid IANA timezone")
	}
	return timeZone, nil
}

func normalizeSettingsLanguage(raw string) (string, error) {
	language := strings.ToLower(strings.TrimSpace(raw))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
		return language, nil
	default:
		return "", errors.New("language must be one of: ru, en")
	}
}

func (a *App) apiUpdateSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSubscriptionSettingsRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "AllKeys"
	}
	if len(title) > 120 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "title is too long (max 120 characters)")
		return
	}

	refreshHours := req.RefreshHours
	if refreshHours <= 0 {
		refreshHours = 12
	}
	if refreshHours > 720 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "refresh_hours must be between 1 and 720")
		return
	}

	infoURL, err := normalizeSettingsURL(req.InfoURL, "info_url")
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	extraURL, err := normalizeSettingsURL(req.ExtraURL, "extra_url")
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	extraStatus := strings.TrimSpace(req.ExtraStatus)
	if len(extraStatus) > 255 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "extra_status is too long (max 255 characters)")
		return
	}
	subscriptionFormat, ok := model.NormalizeSubscriptionFormat(req.SubscriptionFormat)
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_format must be one of: links, xray-json")
		return
	}

	timeZone, err := normalizeSettingsTimeZone(req.TimeZone)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	language, err := normalizeSettingsLanguage(req.Language)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	providerID := strings.TrimSpace(req.ProviderID)
	if providerID != "" && !providerIDPattern.MatchString(providerID) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "provider_id must match ^[A-Za-z0-9]{8}$")
		return
	}

	happNoLimitMode := req.HappNoLimitMode
	happNoLimitModeXHTTPOnly := req.HappNoLimitModeXHTTPOnly
	happMandatoryHWID := req.HappMandatoryHWID
	happNotifyExpiration := req.HappNotifyExpiration
	happHideServerSettings := req.HappHideServerSettings
	happSubscriptionBody := req.HappSubscriptionBody
	var showSubscriptionExpiration any
	if req.ShowSubscriptionExpiration != nil {
		showSubscriptionExpiration = boolToInt(*req.ShowSubscriptionExpiration)
	}
	if len(happSubscriptionBody) > 10000 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "happ_subscription_body is too long (max 10000 characters)")
		return
	}
	if _, err := a.db.Exec(
		`UPDATE subscription_settings
		 SET title = ?, refresh_hours = ?, info_url = ?, extra_url = ?, extra_status = ?, subscription_format = ?,
		     show_subscription_expiration = COALESCE(?, show_subscription_expiration), time_zone = ?, language = ?,
		     provider_id = ?, happ_no_limit_mode = ?, happ_no_limit_mode_xhttp_only = ?, happ_mandatory_hwid = ?,
		     happ_notify_expiration = ?, happ_hide_server_settings = ?, happ_subscription_body = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		title,
		refreshHours,
		nullStringValue(infoURL),
		nullStringValue(extraURL),
		nullStringValue(extraStatus),
		subscriptionFormat,
		showSubscriptionExpiration,
		timeZone,
		language,
		nullStringValue(providerID),
		boolToInt(happNoLimitMode),
		boolToInt(happNoLimitModeXHTTPOnly),
		boolToInt(happMandatoryHWID),
		boolToInt(happNotifyExpiration),
		boolToInt(happHideServerSettings),
		nullStringValue(happSubscriptionBody),
	); err != nil {
		log.Printf("apiUpdateSubscriptionSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update subscription settings")
		return
	}

	httpapi.WriteMessage(w, "subscription settings updated")
}

func (a *App) apiGetRoutingSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getRoutingSettings()
	if err != nil {
		log.Printf("apiGetRoutingSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load routing settings")
		return
	}
	if !validRoutingDeliveryMode(settings.DeliveryMode) {
		settings.DeliveryMode = routingDeliveryModeDisabled
	}
	httpapi.WriteJSON(w, http.StatusOK, routingSettingsWithLinks(settings))
}

func (a *App) apiUpdateRoutingSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateRoutingSettingsRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	configJSON := strings.TrimSpace(req.ConfigJSON)
	current, err := a.getRoutingSettings()
	if err != nil {
		log.Printf("apiUpdateRoutingSettings load current: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update routing settings")
		return
	}
	deliveryMode := current.DeliveryMode
	if !validRoutingDeliveryMode(deliveryMode) {
		deliveryMode = routingDeliveryModeDisabled
	}
	if req.DeliveryMode != nil {
		deliveryMode = strings.TrimSpace(*req.DeliveryMode)
	}
	if !validRoutingDeliveryMode(deliveryMode) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "delivery_mode must be disabled, add, or onadd")
		return
	}
	if err := validateHappRoutingConfig(configJSON, deliveryMode != routingDeliveryModeDisabled); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	settings := model.RoutingSettings{ConfigJSON: configJSON, DeliveryMode: deliveryMode}
	if err := a.updateRoutingSettings(settings); err != nil {
		log.Printf("apiUpdateRoutingSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update routing settings")
		return
	}

	response := routingSettingsWithLinks(settings)
	response.Message = "routing settings updated"
	httpapi.WriteJSON(w, http.StatusOK, response)
}
