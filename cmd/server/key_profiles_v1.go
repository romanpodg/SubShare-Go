package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func sanitizeCheckError(raw string) model.SanitizedCheckError {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return model.SanitizedCheckError{Code: "none", Message: ""}
	}
	switch {
	case strings.Contains(trimmed, "connection refused") || strings.Contains(trimmed, "refused"):
		return model.SanitizedCheckError{Code: "connection_refused", Message: "Connection refused by target server"}
	case strings.Contains(trimmed, "timeout") || strings.Contains(trimmed, "timed out") || strings.Contains(trimmed, "deadline"):
		return model.SanitizedCheckError{Code: "timeout", Message: "Connection timed out"}
	case strings.Contains(trimmed, "tls") || strings.Contains(trimmed, "handshake") || strings.Contains(trimmed, "certificate"):
		return model.SanitizedCheckError{Code: "tls_handshake_failed", Message: "TLS handshake failed"}
	case strings.Contains(trimmed, "dns") || strings.Contains(trimmed, "lookup") || strings.Contains(trimmed, "host"):
		return model.SanitizedCheckError{Code: "dns_lookup_failed", Message: "DNS resolution failed"}
	default:
		return model.SanitizedCheckError{Code: "health_check_failed", Message: "Health check failed"}
	}
}

func applyNoStoreHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Pragma", "no-cache")
}

func applyStrictNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (a *App) buildSafeStructuredProfile(parsed *profiles.Profile) *model.SafeStructuredProfile {
	if parsed == nil {
		return nil
	}
	safe := &model.SafeStructuredProfile{
		Server:      parsed.Server,
		Port:        parsed.Port.Expression,
		PortKind:    string(parsed.Port.Kind),
		DisplayName: parsed.DisplayName,
	}

	switch data := parsed.Data.(type) {
	case profiles.ShadowsocksData:
		safe.Shadowsocks = &model.SafeShadowsocksDetail{
			Method:               data.Method,
			PasswordPresent:      data.Password.IsSet(),
			PluginName:           "",
			PluginOptionsPresent: data.Plugin != nil && data.Plugin.Options.IsSet(),
			UserInfoStyle:        string(data.UserInfoStyle),
		}
		if data.Plugin != nil {
			safe.Shadowsocks.PluginName = data.Plugin.Name
		}
	case profiles.Hysteria2Data:
		safe.Hysteria2 = &model.SafeHysteria2Detail{
			AuthenticationPresent:      data.Authentication.IsSet(),
			SNI:                        data.SNI,
			Insecure:                   data.Insecure,
			CertificateSHA256:          data.CertificateSHA256,
			ObfuscationType:            data.ObfuscationType,
			ObfuscationPasswordPresent: data.ObfuscationPassword.IsSet(),
		}
	case profiles.TUICData:
		obsDTO := make([]model.TUICFieldObservationDTO, 0, len(data.FieldObservations))
		for _, obs := range data.FieldObservations {
			obsDTO = append(obsDTO, model.TUICFieldObservationDTO{
				Field:      obs.Field,
				FieldClass: string(obs.FieldClass),
				Provenance: string(obs.SourceProvenance),
			})
		}
		safe.TUIC = &model.SafeTUICDetail{
			Generation:                  data.Generation,
			UUIDPresent:                 data.UUID.IsSet(),
			PasswordPresent:             data.Password.IsSet(),
			TokenPresent:                data.Token.IsSet(),
			SNI:                         data.SNI,
			ALPN:                        append([]string(nil), data.ALPN...),
			SkipCertificateVerification: data.SkipCertificateVerification,
			DisableSNI:                  data.DisableSNI,
			CongestionController:        data.CongestionController,
			UDPRelayMode:                data.UDPRelayMode,
			UDPOverStream:               data.UDPOverStream,
			ZeroRTT:                     data.ZeroRTT,
			Heartbeat:                   data.Heartbeat,
			FieldObservations:           obsDTO,
		}
	}

	return safe
}

func (a *App) buildCapabilitiesMap(parsed *profiles.Profile) map[string]map[string]any {
	caps := make(map[string]map[string]any)
	if parsed == nil {
		return caps
	}
	matrix := subscriptionCapabilityMatrix()
	for _, item := range matrix {
		if strings.EqualFold(item.Protocol, string(parsed.Protocol)) {
			for fmtName, outCap := range item.Outputs {
				payload := map[string]any{
					"status": string(outCap.Status),
				}
				if outCap.ReasonCode != "" {
					payload["reason_code"] = outCap.ReasonCode
				}
				if outCap.TargetVersion != "" {
					payload["target_version"] = outCap.TargetVersion
				}
				caps[fmtName] = payload
			}

			// Apply TUIC v4 overrides
			if parsed.Protocol == profiles.ProtocolTUIC && parsed.Data != nil {
				if tuicData, ok := parsed.Data.(profiles.TUICData); ok && tuicData.Generation == 4 {
					caps["mihomo"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["sing-box"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["xray-json"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["plain"] = map[string]any{"status": string(capabilityCompatibility), "reason_code": "raw_delivery_only"}
					caps["base64"] = map[string]any{"status": string(capabilityCompatibility), "reason_code": "raw_delivery_only"}
				}
			}
			break
		}
	}
	if len(caps) == 0 {
		caps["plain"] = map[string]any{"status": "supported"}
		caps["base64"] = map[string]any{"status": "supported"}
	}
	return caps
}

func (a *App) buildKeyProfileDetailResponse(key model.VLESSKey, decryptedURI string) model.KeyProfileDetailResponse {
	var categoryID *int64
	if key.CategoryID > 0 {
		categoryID = &key.CategoryID
	}
	var extSourceID *int64
	ownership := model.OwnershipLocal
	if key.ExternalSourceID > 0 {
		extSourceID = &key.ExternalSourceID
		ownership = model.OwnershipExternalSource
	}

	warnings := key.ProfileWarnings
	if warnings == nil {
		warnings = []string{}
	}

	var parsed *profiles.Profile
	if key.Kind == model.KeyKindReal && decryptedURI != "" {
		if p, err := profiles.Parse(decryptedURI); err == nil {
			parsed = p
		}
	}

	safeStructured := a.buildSafeStructuredProfile(parsed)
	unknownParams := make([]model.UnknownQueryParamDTO, 0)
	if parsed != nil {
		for _, qp := range parsed.UnknownQueryParameters {
			unknownParams = append(unknownParams, model.UnknownQueryParamDTO{
				Key:      qp.Key,
				HasValue: qp.HasValue,
			})
		}
	}

	caps := a.buildCapabilitiesMap(parsed)
	sanitizedErr := sanitizeCheckError(key.CheckError)

	return model.KeyProfileDetailResponse{
		ID:                     key.ID,
		Label:                  key.Label,
		CategoryID:             categoryID,
		Category:               key.Category,
		Kind:                   key.Kind,
		Status:                 key.Status,
		CheckStatus:            key.CheckStatus,
		CheckError:             sanitizedErr,
		LastLatencyMS:          key.LastLatencyMS,
		LastCheckedAt:          key.LastCheckedAtText,
		TemplateText:           key.TemplateText,
		Ownership:              ownership,
		ExternalSourceID:       extSourceID,
		ExternalSourceName:     key.ExternalSourceName,
		Protocol:               key.Protocol,
		ProfileSchemaVersion:   key.ProfileSchemaVersion,
		ProfileCompatibility:   key.ProfileCompatibility,
		ProfileWarnings:        warnings,
		ProfileRevision:        key.ProfileRevision,
		CreatedAt:              key.CreatedAt,
		UpdatedAt:              key.UpdatedAt,
		SafeStructured:         safeStructured,
		UnknownQueryParameters: unknownParams,
		Capabilities:           caps,
	}
}

func (a *App) fetchKeyByID(id int64) (*model.VLESSKey, string, error) {
	var key model.VLESSKey
	var encURL sql.NullString
	var category sql.NullString
	var kind sql.NullString
	var templateText sql.NullString
	var status sql.NullString
	var checkStatus sql.NullString
	var checkError sql.NullString
	var lastCheckedAt sql.NullTime
	var latency sql.NullInt64
	var categoryID sql.NullInt64
	var externalSourceID sql.NullInt64
	var externalSourceName sql.NullString
	var warningsJSON string
	var revision sql.NullInt64
	var updatedAt sql.NullString

	err := a.db.QueryRow(`
		SELECT k.id, k.label, s.encrypted_url, k.category_id, COALESCE(kc.name, k.category),
		       k.key_kind, k.template_text, k.status, k.check_status, k.check_error,
		       k.last_checked_at, k.last_latency_ms, k.created_at, k.external_source_id,
		       COALESCE(es.name, ''), k.protocol, k.profile_schema_version,
		       k.profile_compatibility, k.profile_warnings_json,
		       COALESCE(k.profile_revision, 1), COALESCE(k.updated_at, k.created_at)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		LEFT JOIN key_categories kc ON kc.id = k.category_id
		LEFT JOIN external_subscription_sources es ON es.id = k.external_source_id
		WHERE k.id = ?
	`, id).Scan(
		&key.ID, &key.Label, &encURL, &categoryID, &category, &kind, &templateText, &status,
		&checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt, &externalSourceID,
		&externalSourceName, &key.Protocol, &key.ProfileSchemaVersion, &key.ProfileCompatibility,
		&warningsJSON, &revision, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", sql.ErrNoRows
	}
	if err != nil {
		return nil, "", err
	}

	decryptedURI := ""
	if encURL.Valid && encURL.String != "" {
		if dec, err := profilestorage.Decrypt(encURL.String, a.profileKeyring, key.ID); err == nil {
			decryptedURI = dec.Reveal()
		}
	}
	if err := json.Unmarshal([]byte(warningsJSON), &key.ProfileWarnings); err != nil || key.ProfileWarnings == nil {
		key.ProfileWarnings = []string{}
	}
	if categoryID.Valid {
		key.CategoryID = categoryID.Int64
	}
	key.Category = strings.TrimSpace(category.String)
	key.Kind, _ = model.NormalizeKeyKind(kind.String)
	if key.Kind == "" {
		key.Kind = model.KeyKindReal
	}
	key.TemplateText = strings.TrimSpace(templateText.String)
	key.Status, _ = model.NormalizeKeyStatus(status.String)
	if key.Status == "" {
		key.Status = model.KeyStatusActive
	}
	key.CheckStatus = model.NormalizeCheckStatus(checkStatus.String)
	key.ProfileRevision = revision.Int64
	if key.ProfileRevision < 1 {
		key.ProfileRevision = 1
	}
	if updatedAt.Valid && updatedAt.String != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
			key.UpdatedAt = t
		} else if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			key.UpdatedAt = t
		} else {
			key.UpdatedAt = key.CreatedAt
		}
	} else {
		key.UpdatedAt = key.CreatedAt
	}
	if externalSourceID.Valid {
		key.ExternalSourceID = externalSourceID.Int64
	}
	key.ExternalSourceName = strings.TrimSpace(externalSourceName.String)

	return &key, decryptedURI, nil
}

func (a *App) apiV1GetKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	key, decryptedURI, err := a.fetchKeyByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "key_not_found", "key not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "key_load_failed", "failed to load key")
		return
	}

	detail := a.buildKeyProfileDetailResponse(*key, decryptedURI)
	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (a *App) apiV1RevealKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyStrictNoCacheHeaders(w)

	var req model.KeySecretRevealRequest
	if err := readJSON(r, &req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	if req.Target != "raw" && req.Target != "structured-secrets" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_target", "target must be raw or structured-secrets")
		return
	}

	key, decryptedURI, err := a.fetchKeyByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "key_not_found", "key not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "key_load_failed", "failed to load key")
		return
	}

	if key.ProfileRevision != req.ProfileRevision {
		writeV1Error(w, r, http.StatusConflict, "profile_revision_conflict", "profile revision conflict")
		return
	}

	a.recordAuditEvent(r, "key.reveal", "key", strconv.FormatInt(id, 10), map[string]any{
		"target": req.Target,
	})

	if req.Target == "raw" {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": model.KeyRawSecretResponse{
				KeyID:                    id,
				ConfirmedProfileRevision: key.ProfileRevision,
				Target:                   "raw",
				RawURI:                   decryptedURI,
			},
		})
		return
	}

	// structured-secrets
	secrets := model.StructuredSecretsMap{}
	if p, parseErr := profiles.Parse(decryptedURI); parseErr == nil && p != nil {
		switch data := p.Data.(type) {
		case profiles.ShadowsocksData:
			secrets.Password = data.Password.Reveal()
			if data.Plugin != nil {
				secrets.PluginOptions = data.Plugin.Options.Reveal()
			}
		case profiles.Hysteria2Data:
			secrets.Authentication = data.Authentication.Reveal()
			secrets.ObfuscationPassword = data.ObfuscationPassword.Reveal()
		case profiles.TUICData:
			secrets.UUID = data.UUID.Reveal()
			secrets.Password = data.Password.Reveal()
			secrets.Token = data.Token.Reveal()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": model.KeyStructuredSecretsResponse{
			KeyID:                    id,
			ConfirmedProfileRevision: key.ProfileRevision,
			Target:                   "structured-secrets",
			Secrets:                  secrets,
		},
	})
}

func (a *App) apiV1CloneKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	var req model.KeyCloneRequest
	if err := readJSON(r, &req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	activeID, activeKey, err := a.profileKeyring.GetActiveEncryptionKey()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "encryption_unavailable", "encryption key unavailable")
		return
	}
	_, bikKey, err := a.profileKeyring.GetActiveBlindIndexKey()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "blind_index_unavailable", "blind index key unavailable")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to start transaction")
		return
	}
	defer tx.Rollback()

	var sourceLabel, sourceCategory, sourceKind, sourceStatus, sourceProtocol string
	var sourceTemplateText, sourceWarnings, sourceEncURL sql.NullString
	var sourceCategoryID sql.NullInt64
	var sourceSchemaVer int
	var sourceRevision int64
	err = tx.QueryRow(`
		SELECT k.label, k.category, k.key_kind, k.template_text, k.status, k.protocol,
		       k.profile_schema_version, k.profile_warnings_json, s.encrypted_url,
		       k.category_id, COALESCE(k.profile_revision, 1)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.id = ?
	`, id).Scan(
		&sourceLabel, &sourceCategory, &sourceKind, &sourceTemplateText, &sourceStatus,
		&sourceProtocol, &sourceSchemaVer, &sourceWarnings, &sourceEncURL, &sourceCategoryID, &sourceRevision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "key_not_found", "source key not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to load source key")
		return
	}

	if sourceRevision != req.ExpectedProfileRevision {
		writeV1Error(w, r, http.StatusConflict, "profile_revision_conflict", "profile revision conflict")
		return
	}

	decryptedURI := ""
	if sourceEncURL.Valid && sourceEncURL.String != "" {
		if dec, decErr := profilestorage.Decrypt(sourceEncURL.String, a.profileKeyring, id); decErr == nil {
			decryptedURI = dec.Reveal()
		}
	}

	newLabel := strings.TrimSpace(req.NewLabel)
	if newLabel == "" {
		newLabel = sourceLabel + " (Копия)"
	}
	if len(newLabel) > 255 {
		writeV1Error(w, r, http.StatusBadRequest, "label_too_long", "label is too long")
		return
	}

	var nextSortOrder int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to prepare key order")
		return
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, decryptedURI)
	res, err := tx.Exec(`
		INSERT INTO vless_keys(
			label, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, external_source_id, external_key_ref,
			protocol, profile_schema_version, profile_compatibility, profile_warnings_json,
			profile_revision, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?, NULL, NULL, ?, ?, 'full', ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, newLabel, blindIndex, nullInt64Value(sourceCategoryID.Int64), sourceCategory, sourceStatus, sourceKind, nullStringValue(sourceTemplateText.String), nextSortOrder, sourceProtocol, sourceSchemaVer, nullStringValue(sourceWarnings.String))
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to insert cloned key")
		return
	}

	newID, err := res.LastInsertId()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to get new key ID")
		return
	}

	env, err := profilestorage.Encrypt([]byte(decryptedURI), activeID, activeKey, newID)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to encrypt cloned key")
		return
	}

	if _, err := tx.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, newID, env); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to save secret")
		return
	}

	if err := tx.Commit(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to commit clone")
		return
	}

	a.recordAuditEvent(r, "key.clone", "key", strconv.FormatInt(id, 10), map[string]any{
		"source_key_id": id,
		"new_key_id":    newID,
	})

	clonedKey, clonedURI, fetchErr := a.fetchKeyByID(newID)
	if fetchErr != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "failed to load cloned key")
		return
	}

	detail := a.buildKeyProfileDetailResponse(*clonedKey, clonedURI)
	writeJSON(w, http.StatusCreated, map[string]any{"data": detail})
}

func (a *App) apiV1GetKeyEditorSchema(w http.ResponseWriter, r *http.Request) {
	applyNoStoreHeaders(w)

	ssFields := map[string]model.ProtocolEditorFieldSchema{
		"method": {
			FieldType: "select", Required: true, CanClear: false,
			Options: []map[string]string{
				{"value": "aes-128-gcm", "label": "AES-128-GCM"},
				{"value": "aes-256-gcm", "label": "AES-256-GCM"},
				{"value": "chacha20-ietf-poly1305", "label": "ChaCha20-IETF-Poly1305"},
				{"value": "2022-blake3-aes-128-gcm", "label": "Shadowsocks 2022 (Blake3-AES-128-GCM)"},
				{"value": "2022-blake3-aes-256-gcm", "label": "Shadowsocks 2022 (Blake3-AES-256-GCM)"},
				{"value": "2022-blake3-chacha20-poly1305", "label": "Shadowsocks 2022 (Blake3-ChaCha20-Poly1305)"},
			},
			DefaultValue: "2022-blake3-aes-128-gcm",
		},
		"password": {FieldType: "password", Required: true, CanClear: false},
		"plugin_name": {
			FieldType: "text", Required: false, CanClear: true,
			Options: []map[string]string{
				{"value": "v2ray-plugin", "label": "v2ray-plugin"},
				{"value": "obfs-local", "label": "obfs-local"},
			},
		},
		"plugin_options": {FieldType: "text", Required: false, CanClear: true},
	}

	hy2Fields := map[string]model.ProtocolEditorFieldSchema{
		"authentication": {FieldType: "password", Required: true, CanClear: false},
		"sni":            {FieldType: "text", Required: false, CanClear: true},
		"insecure": {
			FieldType: "boolean", Required: false, CanClear: false, DefaultValue: false,
			WarningRules: []model.DeclarativeWarningRule{
				{Field: "insecure", Operator: "is_true", WarningCode: "WARNING_INSECURE_TLS", WarningMessage: "Отключение проверки сертификата TLS небезопасно"},
			},
		},
		"certificate_sha256": {FieldType: "text", Required: false, CanClear: true},
		"obfuscation_type": {
			FieldType: "select", Required: false, CanClear: true,
			Options: []map[string]string{
				{"value": "", "label": "Без маскировки"},
				{"value": "salamander", "label": "Salamander"},
				{"value": "gecko", "label": "Gecko (требует совместимый клиент)"},
			},
		},
		"obfuscation_password": {FieldType: "password", Required: false, CanClear: true},
	}

	tuicFields := map[string]model.ProtocolEditorFieldSchema{
		"uuid":            {FieldType: "password", Required: true, CanClear: false},
		"password":        {FieldType: "password", Required: true, CanClear: false},
		"sni":             {FieldType: "text", Required: false, CanClear: true},
		"alpn":            {FieldType: "string_list", Required: false, CanClear: true, DefaultValue: []string{"h3"}},
		"skip_cert_verify": {
			FieldType: "boolean", Required: false, CanClear: false, DefaultValue: false,
			WarningRules: []model.DeclarativeWarningRule{
				{Field: "skip_cert_verify", Operator: "is_true", WarningCode: "WARNING_INSECURE_TLS", WarningMessage: "Отключение проверки сертификата TLS небезопасно"},
			},
		},
		"congestion_controller": {
			FieldType: "select", Required: false, CanClear: true,
			Options: []map[string]string{
				{"value": "bbr", "label": "BBR"},
				{"value": "cubic", "label": "CUBIC"},
				{"value": "new_reno", "label": "New Reno"},
			},
			DefaultValue: "bbr",
		},
		"udp_relay_mode": {
			FieldType: "select", Required: false, CanClear: true,
			Options: []map[string]string{
				{"value": "native", "label": "Native"},
				{"value": "quic", "label": "QUIC"},
			},
			DefaultValue: "native",
		},
		"udp_over_stream": {FieldType: "boolean", Required: false, CanClear: false, DefaultValue: false},
		"zero_rtt":        {FieldType: "boolean", Required: false, CanClear: false, DefaultValue: false},
		"heartbeat":       {FieldType: "text", Required: false, CanClear: true, DefaultValue: "10s"},
	}

	protocols := []model.ProtocolSchemaDTO{
		{Protocol: "shadowsocks", Label: "Shadowsocks", SupportedCreations: []string{"raw", "structured"}, Fields: ssFields},
		{Protocol: "hysteria2", Label: "Hysteria 2", SupportedCreations: []string{"raw", "structured"}, Fields: hy2Fields},
		{Protocol: "tuic", Label: "TUIC v5", SupportedCreations: []string{"raw", "structured"}, Fields: tuicFields},
	}

	reasonCatalog := map[string]string{
		generationReasonUnsupportedProtocol: "Протокол не поддерживается выбранным форматом выгрузки",
		generationReasonClientVersion:       "Требуется более новая версия клиента",
		generationReasonPlugin:              "Плагин профиля не поддерживается генератором",
		generationReasonCompatibility:       "Профиль доступен только в совместимом исходном формате",
		generationReasonInvalidStored:       "Некорректная исходная конфигурация профиля",
		generationReasonUnsafeControl:       "Содержит непечатаемые символы",
		generationReasonUnrepresentable:     "Некоторые параметры не поддаются преобразованию",
		generationReasonAmbiguous:           "Профиль содержит неоднозначные параметры",
		generationReasonSerialization:       "Ошибка сборки результирующего файла",
		generationReasonAllExcluded:         "Все профили исключены из выгрузки",
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": model.KeyEditorSchemaResponse{
			Protocols:            protocols,
			ExclusionReasonCodes: reasonCatalog,
		},
	})
}



func (a *App) apiV1CreateKeyProfile(w http.ResponseWriter, r *http.Request) {
	applyNoStoreHeaders(w)

	var req model.CreateKeyProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid or unallowed JSON request body")
		return
	}

	label := strings.TrimSpace(req.Label)
	status, statusOK := model.NormalizeKeyStatus(req.Status)
	kind, kindOK := model.NormalizeKeyKind(req.Kind)
	category := normalizeKeyCategory(req.Category)
	templateText := strings.TrimSpace(req.TemplateText)

	if !statusOK || !kindOK || label == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_input", "label, valid status and kind are required")
		return
	}
	if len(label) > 255 {
		writeV1Error(w, r, http.StatusBadRequest, "label_too_long", "label is too long (max 255 chars)")
		return
	}

	// Validate creation mode mutual exclusion
	creationMode := strings.ToLower(strings.TrimSpace(req.CreationMode))
	if creationMode == "" {
		if req.RawURI != "" {
			creationMode = "raw"
		} else {
			creationMode = "structured"
		}
	}
	if creationMode != "raw" && creationMode != "structured" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_creation_mode", "creation_mode must be raw or structured")
		return
	}
	if creationMode == "raw" && req.Structured != nil {
		writeV1Error(w, r, http.StatusBadRequest, "mutually_exclusive_mode", "cannot specify structured payload in raw creation_mode")
		return
	}
	if creationMode == "structured" && strings.TrimSpace(req.RawURI) != "" {
		writeV1Error(w, r, http.StatusBadRequest, "mutually_exclusive_mode", "cannot specify raw_uri in structured creation_mode")
		return
	}

	builtURI := ""
	if kind == model.KeyKindReal {
		if creationMode == "raw" {
			builtURI = strings.TrimSpace(req.RawURI)
			if builtURI == "" {
				writeV1Error(w, r, http.StatusBadRequest, "raw_uri_required", "raw_uri is required for real key in raw mode")
				return
			}
			p, parseErr := profiles.Parse(builtURI)
			if parseErr != nil {
				writeV1Error(w, r, http.StatusBadRequest, "invalid_profile_uri", parseErr.Error())
				return
			}
			_ = p
		} else {
			// Structured creation
			proto := strings.ToLower(strings.TrimSpace(req.Protocol))
			if proto == "tuic_v4" {
				writeV1Error(w, r, http.StatusBadRequest, "tuic_v4_structured_forbidden", "TUIC v4 is compatibility-only; structured creation is unsupported")
				return
			}
			if req.Structured == nil {
				writeV1Error(w, r, http.StatusBadRequest, "structured_payload_required", "structured payload is required")
				return
			}
			builtURI, err := a.buildURIFromStructuredCreate(proto, label, req.Structured)
			if err != nil {
				writeV1Error(w, r, http.StatusBadRequest, "structured_create_failed", err.Error())
				return
			}
			_ = builtURI
		}
	} else {
		// Informational key
		if creationMode == "structured" && req.Structured != nil {
			writeV1Error(w, r, http.StatusBadRequest, "informational_structured_forbidden", "informational keys do not support structured profile creation")
			return
		}
		if templateText == "" {
			templateText = label
		}
		token, err := generateToken(12)
		if err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "token_generation_failed", "failed to generate key token")
			return
		}
		builtURI = "info://" + token
	}

	if err := a.upsertKeyCategory(category); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "category_save_failed", "failed to save key category")
		return
	}
	var categoryIDVal any
	if req.CategoryID != nil && *req.CategoryID > 0 {
		categoryIDVal = *req.CategoryID
	} else if catID, err := a.keyCategoryID(category); err == nil {
		categoryIDVal = catID
	}

	parsedProfile, _ := profiles.Parse(builtURI)
	storedProtocol := "legacy"
	if parsedProfile != nil {
		storedProtocol = string(parsedProfile.Protocol)
	}

	activeID, activeKey, err := a.profileKeyring.GetActiveEncryptionKey()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "encryption_unavailable", "encryption key unavailable")
		return
	}
	_, bikKey, err := a.profileKeyring.GetActiveBlindIndexKey()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "blind_index_unavailable", "blind index key unavailable")
		return
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, builtURI)

	tx, err := a.db.Begin()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to start transaction")
		return
	}
	defer tx.Rollback()

	var nextSort int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSort); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to calculate sort order")
		return
	}

	res, err := tx.Exec(`
		INSERT INTO vless_keys(
			label, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, protocol, profile_schema_version,
			profile_compatibility, profile_warnings_json, profile_revision,
			created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?, ?, 1, 'full', '[]', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, label, blindIndex, categoryIDVal, category, status, kind, nullStringValue(templateText), nextSort, storedProtocol)
	if err != nil {
		writeV1Error(w, r, http.StatusConflict, "create_failed", "failed to create key (maybe duplicate)")
		return
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to get key ID")
		return
	}

	env, err := profilestorage.Encrypt([]byte(builtURI), activeID, activeKey, keyID)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to encrypt secret")
		return
	}

	if _, err := tx.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to save secret")
		return
	}

	_, _ = tx.Exec(`
		INSERT INTO user_keys(user_id, key_id)
		SELECT id, ? FROM users WHERE key_assignment_mode = 'all'
	`, keyID)

	if err := tx.Commit(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to commit key creation")
		return
	}

	a.recordAuditEvent(r, "key.create", "key", strconv.FormatInt(keyID, 10), map[string]any{"label": label})

	newKey, newURI, fetchErr := a.fetchKeyByID(keyID)
	if fetchErr != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "create_failed", "failed to load created key")
		return
	}

	detail := a.buildKeyProfileDetailResponse(*newKey, newURI)
	writeJSON(w, http.StatusCreated, map[string]any{"data": detail})
}

func (a *App) buildURIFromStructuredCreate(proto, label string, patch *model.StructuredProfilePatch) (string, error) {
	if patch == nil {
		return "", fmt.Errorf("missing structured patch payload")
	}
	server := ""
	if patch.Server != nil && patch.Server.Set && patch.Server.Operation == model.TriStateSet {
		server = patch.Server.Value
	}
	port := "443"
	if patch.Port != nil && patch.Port.Set && patch.Port.Operation == model.TriStateSet {
		port = patch.Port.Value
	}
	displayName := label
	if patch.DisplayName != nil && patch.DisplayName.Set && patch.DisplayName.Operation == model.TriStateSet {
		displayName = patch.DisplayName.Value
	}

	switch proto {
	case "shadowsocks":
		if patch.Shadowsocks == nil {
			return "", fmt.Errorf("missing shadowsocks patch payload")
		}
		method := "2022-blake3-aes-128-gcm"
		if patch.Shadowsocks.Method != nil && patch.Shadowsocks.Method.Set && patch.Shadowsocks.Method.Operation == model.TriStateSet {
			method = patch.Shadowsocks.Method.Value
		}
		pass := ""
		if patch.Shadowsocks.Password != nil && patch.Shadowsocks.Password.Set && patch.Shadowsocks.Password.Operation == model.TriStateSet {
			pass = patch.Shadowsocks.Password.Value
		}
		plugin := ""
		if patch.Shadowsocks.PluginName != nil && patch.Shadowsocks.PluginName.Set && patch.Shadowsocks.PluginName.Operation == model.TriStateSet {
			plugin = patch.Shadowsocks.PluginName.Value
		}
		pluginOpts := ""
		if patch.Shadowsocks.PluginOptions != nil && patch.Shadowsocks.PluginOptions.Set && patch.Shadowsocks.PluginOptions.Operation == model.TriStateSet {
			pluginOpts = patch.Shadowsocks.PluginOptions.Value
		}
		uri := fmt.Sprintf("ss://%s:%s@%s:%s", method, pass, server, port)
		if plugin != "" {
			opts := plugin
			if pluginOpts != "" {
				opts += ";" + pluginOpts
			}
			uri += "?plugin=" + opts
		}
		if displayName != "" {
			uri += "#" + displayName
		}
		return uri, nil
	case "hysteria2", "hy2":
		if patch.Hysteria2 == nil {
			return "", fmt.Errorf("missing hysteria2 patch payload")
		}
		auth := ""
		if patch.Hysteria2.Authentication != nil && patch.Hysteria2.Authentication.Set && patch.Hysteria2.Authentication.Operation == model.TriStateSet {
			auth = patch.Hysteria2.Authentication.Value
		}
		uri := fmt.Sprintf("hysteria2://%s@%s:%s", auth, server, port)
		queryParams := make([]string, 0)
		if patch.Hysteria2.SNI != nil && patch.Hysteria2.SNI.Set && patch.Hysteria2.SNI.Operation == model.TriStateSet && patch.Hysteria2.SNI.Value != "" {
			queryParams = append(queryParams, "sni="+patch.Hysteria2.SNI.Value)
		}
		if patch.Hysteria2.Insecure != nil && patch.Hysteria2.Insecure.Set && patch.Hysteria2.Insecure.Operation == model.TriStateSet && patch.Hysteria2.Insecure.Value {
			queryParams = append(queryParams, "insecure=1")
		}
		if patch.Hysteria2.CertificateSHA256 != nil && patch.Hysteria2.CertificateSHA256.Set && patch.Hysteria2.CertificateSHA256.Operation == model.TriStateSet && patch.Hysteria2.CertificateSHA256.Value != "" {
			queryParams = append(queryParams, "pinSHA256="+patch.Hysteria2.CertificateSHA256.Value)
		}
		if patch.Hysteria2.ObfuscationType != nil && patch.Hysteria2.ObfuscationType.Set && patch.Hysteria2.ObfuscationType.Operation == model.TriStateSet && patch.Hysteria2.ObfuscationType.Value != "" {
			queryParams = append(queryParams, "obfs="+patch.Hysteria2.ObfuscationType.Value)
		}
		if patch.Hysteria2.ObfuscationPassword != nil && patch.Hysteria2.ObfuscationPassword.Set && patch.Hysteria2.ObfuscationPassword.Operation == model.TriStateSet && patch.Hysteria2.ObfuscationPassword.Value != "" {
			queryParams = append(queryParams, "obfs-password="+patch.Hysteria2.ObfuscationPassword.Value)
		}
		if len(queryParams) > 0 {
			uri += "?" + strings.Join(queryParams, "&")
		}
		if displayName != "" {
			uri += "#" + displayName
		}
		return uri, nil
	case "tuic":
		if patch.TUIC == nil {
			return "", fmt.Errorf("missing tuic patch payload")
		}
		uuidVal := ""
		if patch.TUIC.UUID != nil && patch.TUIC.UUID.Set && patch.TUIC.UUID.Operation == model.TriStateSet {
			uuidVal = patch.TUIC.UUID.Value
		}
		passVal := ""
		if patch.TUIC.Password != nil && patch.TUIC.Password.Set && patch.TUIC.Password.Operation == model.TriStateSet {
			passVal = patch.TUIC.Password.Value
		}
		uri := fmt.Sprintf("tuic://%s:%s@%s:%s", uuidVal, passVal, server, port)
		queryParams := make([]string, 0)
		if patch.TUIC.SNI != nil && patch.TUIC.SNI.Set && patch.TUIC.SNI.Operation == model.TriStateSet && patch.TUIC.SNI.Value != "" {
			queryParams = append(queryParams, "sni="+patch.TUIC.SNI.Value)
		}
		if patch.TUIC.ALPN != nil && patch.TUIC.ALPN.Set && patch.TUIC.ALPN.Operation == model.TriStateSet && len(patch.TUIC.ALPN.Value) > 0 {
			queryParams = append(queryParams, "alpn="+strings.Join(patch.TUIC.ALPN.Value, ","))
		}
		if patch.TUIC.SkipCertVerify != nil && patch.TUIC.SkipCertVerify.Set && patch.TUIC.SkipCertVerify.Operation == model.TriStateSet && patch.TUIC.SkipCertVerify.Value {
			queryParams = append(queryParams, "skip_cert_verify=1")
		}
		if patch.TUIC.CongestionController != nil && patch.TUIC.CongestionController.Set && patch.TUIC.CongestionController.Operation == model.TriStateSet && patch.TUIC.CongestionController.Value != "" {
			queryParams = append(queryParams, "congestion_control="+patch.TUIC.CongestionController.Value)
		}
		if patch.TUIC.UDPRelayMode != nil && patch.TUIC.UDPRelayMode.Set && patch.TUIC.UDPRelayMode.Operation == model.TriStateSet && patch.TUIC.UDPRelayMode.Value != "" {
			queryParams = append(queryParams, "udp_relay_mode="+patch.TUIC.UDPRelayMode.Value)
		}
		if patch.TUIC.UDPOverStream != nil && patch.TUIC.UDPOverStream.Set && patch.TUIC.UDPOverStream.Operation == model.TriStateSet && patch.TUIC.UDPOverStream.Value {
			queryParams = append(queryParams, "udp_over_stream=1")
		}
		if patch.TUIC.ZeroRTT != nil && patch.TUIC.ZeroRTT.Set && patch.TUIC.ZeroRTT.Operation == model.TriStateSet && patch.TUIC.ZeroRTT.Value {
			queryParams = append(queryParams, "zero_rtt=1")
		}
		if patch.TUIC.Heartbeat != nil && patch.TUIC.Heartbeat.Set && patch.TUIC.Heartbeat.Operation == model.TriStateSet && patch.TUIC.Heartbeat.Value != "" {
			queryParams = append(queryParams, "heartbeat="+patch.TUIC.Heartbeat.Value)
		}
		if len(queryParams) > 0 {
			uri += "?" + strings.Join(queryParams, "&")
		}
		if displayName != "" {
			uri += "#" + displayName
		}
		return uri, nil
	default:
		return "", fmt.Errorf("unsupported protocol for structured creation: %s", proto)
	}
}

func (a *App) apiV1UpdateKeyProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	var req model.UpdateKeyProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid or unallowed JSON request body")
		return
	}

	key, decryptedURI, fetchErr := a.fetchKeyByID(id)
	if errors.Is(fetchErr, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "key_not_found", "key not found")
		return
	}
	if fetchErr != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "key_load_failed", "failed to load key")
		return
	}

	// Reject profile material changes for source-owned keys
	if key.ExternalSourceID > 0 {
		writeV1Error(w, r, http.StatusForbidden, "source_owned_read_only", "profile material for source-owned keys is read-only")
		return
	}

	// Verify revision
	if key.ProfileRevision != req.ProfileRevision {
		writeV1Error(w, r, http.StatusConflict, "profile_revision_conflict", "profile revision conflict")
		return
	}

	label := strings.TrimSpace(req.Label)
	status, statusOK := model.NormalizeKeyStatus(req.Status)
	kind, kindOK := model.NormalizeKeyKind(req.Kind)
	category := normalizeKeyCategory(req.Category)
	templateText := strings.TrimSpace(req.TemplateText)

	if !statusOK || !kindOK || label == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_input", "label, status and kind are required")
		return
	}
	if len(label) > 255 {
		writeV1Error(w, r, http.StatusBadRequest, "label_too_long", "label is too long (max 255 chars)")
		return
	}

	patchMode := strings.ToLower(strings.TrimSpace(req.PatchMode))
	if patchMode == "" {
		if req.RawURI != "" {
			patchMode = "raw"
		} else {
			patchMode = "structured"
		}
	}
	if patchMode != "raw" && patchMode != "structured" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_patch_mode", "patch_mode must be raw or structured")
		return
	}
	if patchMode == "raw" && req.StructuredPatch != nil {
		writeV1Error(w, r, http.StatusBadRequest, "mutually_exclusive_mode", "cannot specify structured_patch in raw patch_mode")
		return
	}
	if patchMode == "structured" && strings.TrimSpace(req.RawURI) != "" {
		writeV1Error(w, r, http.StatusBadRequest, "mutually_exclusive_mode", "cannot specify raw_uri in structured patch_mode")
		return
	}

	newURI := decryptedURI
	if kind == model.KeyKindReal {
		if patchMode == "raw" {
			newURI = strings.TrimSpace(req.RawURI)
			if newURI == "" {
				writeV1Error(w, r, http.StatusBadRequest, "raw_uri_required", "raw_uri is required for real key in raw patch_mode")
				return
			}
			p, parseErr := profiles.Parse(newURI)
			if parseErr != nil {
				writeV1Error(w, r, http.StatusBadRequest, "invalid_profile_uri", parseErr.Error())
				return
			}
			_ = p
		} else {
			// Structured patch mode
			if req.StructuredPatch == nil {
				writeV1Error(w, r, http.StatusBadRequest, "structured_patch_required", "structured_patch is required")
				return
			}
			updatedURI, err := a.applyStructuredPatchToURI(key.Protocol, decryptedURI, label, req.StructuredPatch)
			if err != nil {
				writeV1Error(w, r, http.StatusBadRequest, "patch_application_failed", err.Error())
				return
			}
			newURI = updatedURI
		}
	}

	storedProtocol := "legacy"
	if parsed, err := profiles.Parse(newURI); err == nil && parsed != nil {
		storedProtocol = string(parsed.Protocol)
	}

	if err := a.upsertKeyCategory(category); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "category_save_failed", "failed to save key category")
		return
	}
	var categoryIDVal any
	if req.CategoryID != nil && *req.CategoryID > 0 {
		categoryIDVal = *req.CategoryID
	} else if catID, err := a.keyCategoryID(category); err == nil {
		categoryIDVal = catID
	}

	activeID, activeKey, err := a.profileKeyring.GetActiveEncryptionKey()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "encryption_unavailable", "encryption key unavailable")
		return
	}
	_, bikKey, err := a.profileKeyring.GetActiveBlindIndexKey()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "blind_index_unavailable", "blind index key unavailable")
		return
	}
	blindIndex := profilestorage.ComputeBlindIndex(bikKey, newURI)
	env, err := profilestorage.Encrypt([]byte(newURI), activeID, activeKey, id)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "encrypt_failed", "failed to encrypt secret")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "update_failed", "failed to start transaction")
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		UPDATE vless_keys
		SET label = ?, url_blind_index = ?, category_id = ?, category = ?, status = ?,
		    key_kind = ?, template_text = ?, protocol = ?, profile_revision = profile_revision + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND profile_revision = ?
	`, label, blindIndex, categoryIDVal, category, status, kind, nullStringValue(templateText), storedProtocol, id, req.ProfileRevision)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "update_failed", "failed to update key")
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeV1Error(w, r, http.StatusConflict, "profile_revision_conflict", "profile revision conflict")
		return
	}

	if _, err := tx.Exec(`
		INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)
		ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url
	`, id, env); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "update_failed", "failed to update secret")
		return
	}

	if err := tx.Commit(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "update_failed", "failed to commit key update")
		return
	}

	a.recordAuditEvent(r, "key.update", "key", strconv.FormatInt(id, 10), map[string]any{"label": label})

	updatedKey, updatedURI, fetchErr := a.fetchKeyByID(id)
	if fetchErr != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "update_failed", "failed to load updated key")
		return
	}

	detail := a.buildKeyProfileDetailResponse(*updatedKey, updatedURI)
	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (a *App) applyStructuredPatchToURI(protocol, currentURI, label string, patch *model.StructuredProfilePatch) (string, error) {
	parsed, err := profiles.Parse(currentURI)
	if err != nil {
		// Fallback create URI from scratch if currentURI empty or unparseable
		return a.buildURIFromStructuredCreate(protocol, label, patch)
	}

	if patch.Server != nil && patch.Server.Set && patch.Server.Operation == model.TriStateSet {
		parsed.Server = patch.Server.Value
	}
	if patch.Port != nil && patch.Port.Set && patch.Port.Operation == model.TriStateSet {
		parsed.Port = profiles.PortSpec{Expression: patch.Port.Value, Kind: profiles.PortSingle, Explicit: true}
	}
	if patch.DisplayName != nil && patch.DisplayName.Set && patch.DisplayName.Operation == model.TriStateSet {
		parsed.DisplayName = patch.DisplayName.Value
	}

	switch data := parsed.Data.(type) {
	case profiles.ShadowsocksData:
		if patch.Shadowsocks != nil {
			if patch.Shadowsocks.Method != nil && patch.Shadowsocks.Method.Set && patch.Shadowsocks.Method.Operation == model.TriStateSet {
				data.Method = patch.Shadowsocks.Method.Value
			}
			if patch.Shadowsocks.Password != nil && patch.Shadowsocks.Password.Set {
				if patch.Shadowsocks.Password.Operation == model.TriStateClear {
					return "", fmt.Errorf("password cannot be cleared for real shadowsocks profile")
				}
				data.Password = profiles.NewSensitiveValue(patch.Shadowsocks.Password.Value)
			}
			if patch.Shadowsocks.PluginName != nil && patch.Shadowsocks.PluginName.Set {
				if patch.Shadowsocks.PluginName.Operation == model.TriStateClear {
					data.Plugin = nil
					filteredQP := make([]profiles.QueryParameter, 0, len(parsed.QueryParameters))
					for _, qp := range parsed.QueryParameters {
						if strings.ToLower(qp.Key) != "plugin" {
							filteredQP = append(filteredQP, qp)
						}
					}
					parsed.QueryParameters = filteredQP
				} else if patch.Shadowsocks.PluginName.Value != "" {
					if data.Plugin == nil {
						data.Plugin = &profiles.ShadowsocksPlugin{Name: patch.Shadowsocks.PluginName.Value}
					} else {
						data.Plugin.Name = patch.Shadowsocks.PluginName.Value
					}
				}
			}
			if patch.Shadowsocks.PluginOptions != nil && patch.Shadowsocks.PluginOptions.Set {
				if patch.Shadowsocks.PluginOptions.Operation == model.TriStateClear {
					if data.Plugin != nil {
						data.Plugin.Options = profiles.NewSensitiveValue("")
					}
				} else if data.Plugin != nil {
					data.Plugin.Options = profiles.NewSensitiveValue(patch.Shadowsocks.PluginOptions.Value)
				}
			}
		}
		parsed.Data = data
	case profiles.Hysteria2Data:
		if patch.Hysteria2 != nil {
			if patch.Hysteria2.Authentication != nil && patch.Hysteria2.Authentication.Set {
				if patch.Hysteria2.Authentication.Operation == model.TriStateClear {
					return "", fmt.Errorf("authentication cannot be cleared for real hysteria2 profile")
				}
				data.Authentication = profiles.NewSensitiveValue(patch.Hysteria2.Authentication.Value)
			}
			if patch.Hysteria2.SNI != nil && patch.Hysteria2.SNI.Set {
				if patch.Hysteria2.SNI.Operation == model.TriStateClear {
					data.SNI = ""
				} else {
					data.SNI = patch.Hysteria2.SNI.Value
				}
			}
			if patch.Hysteria2.Insecure != nil && patch.Hysteria2.Insecure.Set {
				data.Insecure = patch.Hysteria2.Insecure.Value
			}
			if patch.Hysteria2.CertificateSHA256 != nil && patch.Hysteria2.CertificateSHA256.Set {
				if patch.Hysteria2.CertificateSHA256.Operation == model.TriStateClear {
					data.CertificateSHA256 = ""
				} else {
					data.CertificateSHA256 = patch.Hysteria2.CertificateSHA256.Value
				}
			}
			if patch.Hysteria2.ObfuscationType != nil && patch.Hysteria2.ObfuscationType.Set {
				if patch.Hysteria2.ObfuscationType.Operation == model.TriStateClear {
					data.ObfuscationType = ""
				} else {
					data.ObfuscationType = patch.Hysteria2.ObfuscationType.Value
				}
			}
			if patch.Hysteria2.ObfuscationPassword != nil && patch.Hysteria2.ObfuscationPassword.Set {
				if patch.Hysteria2.ObfuscationPassword.Operation == model.TriStateClear {
					data.ObfuscationPassword = profiles.NewSensitiveValue("")
				} else {
					data.ObfuscationPassword = profiles.NewSensitiveValue(patch.Hysteria2.ObfuscationPassword.Value)
				}
			}
		}
		parsed.Data = data
	case profiles.TUICData:
		if patch.TUIC != nil {
			if patch.TUIC.UUID != nil && patch.TUIC.UUID.Set {
				if patch.TUIC.UUID.Operation == model.TriStateClear {
					return "", fmt.Errorf("uuid cannot be cleared for real tuic profile")
				}
				data.UUID = profiles.NewSensitiveValue(patch.TUIC.UUID.Value)
			}
			if patch.TUIC.Password != nil && patch.TUIC.Password.Set {
				if patch.TUIC.Password.Operation == model.TriStateClear {
					return "", fmt.Errorf("password cannot be cleared for real tuic profile")
				}
				data.Password = profiles.NewSensitiveValue(patch.TUIC.Password.Value)
			}
			if patch.TUIC.SNI != nil && patch.TUIC.SNI.Set {
				if patch.TUIC.SNI.Operation == model.TriStateClear {
					data.SNI = ""
				} else {
					data.SNI = patch.TUIC.SNI.Value
				}
			}
			if patch.TUIC.ALPN != nil && patch.TUIC.ALPN.Set {
				if patch.TUIC.ALPN.Operation == model.TriStateClear {
					data.ALPN = []string{}
				} else {
					data.ALPN = patch.TUIC.ALPN.Value
				}
			}
			if patch.TUIC.SkipCertVerify != nil && patch.TUIC.SkipCertVerify.Set {
				data.SkipCertificateVerification = patch.TUIC.SkipCertVerify.Value
			}
			if patch.TUIC.CongestionController != nil && patch.TUIC.CongestionController.Set {
				if patch.TUIC.CongestionController.Operation == model.TriStateClear {
					data.CongestionController = ""
				} else {
					data.CongestionController = patch.TUIC.CongestionController.Value
				}
			}
			if patch.TUIC.UDPRelayMode != nil && patch.TUIC.UDPRelayMode.Set {
				if patch.TUIC.UDPRelayMode.Operation == model.TriStateClear {
					data.UDPRelayMode = ""
				} else {
					data.UDPRelayMode = patch.TUIC.UDPRelayMode.Value
				}
			}
			if patch.TUIC.UDPOverStream != nil && patch.TUIC.UDPOverStream.Set {
				data.UDPOverStream = patch.TUIC.UDPOverStream.Value
			}
			if patch.TUIC.ZeroRTT != nil && patch.TUIC.ZeroRTT.Set {
				data.ZeroRTT = patch.TUIC.ZeroRTT.Value
			}
			if patch.TUIC.Heartbeat != nil && patch.TUIC.Heartbeat.Set {
				if patch.TUIC.Heartbeat.Operation == model.TriStateClear {
					data.Heartbeat = ""
				} else {
					data.Heartbeat = patch.TUIC.Heartbeat.Value
				}
			}
		}
		parsed.Data = data
	}

	res, err := profiles.Serialize(parsed, profiles.CanonicalSerialization)
	if err != nil {
		return "", err
	}
	return res.URI.Reveal(), nil
}
