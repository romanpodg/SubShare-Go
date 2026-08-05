package keymanagement

import "github.com/romanpodg/SubShare-Go/internal/model"

const (
	generationReasonUnsupportedProtocol = "EXCLUSION_UNSUPPORTED_PROTOCOL"
	generationReasonClientVersion       = "EXCLUSION_CLIENT_VERSION_REQUIRED"
	generationReasonPlugin              = "EXCLUSION_UNSUPPORTED_PLUGIN"
	generationReasonCompatibility       = "EXCLUSION_COMPATIBILITY_ONLY"
	generationReasonInvalidStored       = "EXCLUSION_INVALID_STORED_PROFILE"
	generationReasonUnsafeControl       = "EXCLUSION_UNSAFE_CONTROL_CHARS"
	generationReasonUnrepresentable     = "EXCLUSION_UNREPRESENTABLE_PARAMETERS"
	generationReasonAmbiguous           = "EXCLUSION_AMBIGUOUS_PARAMETERS"
	generationReasonSerialization       = "EXCLUSION_SERIALIZATION_FAILED"
	generationReasonAllExcluded         = "EXCLUSION_ALL_PROFILES_EXCLUDED"
)

func (s *Service) EditorSchema() model.KeyEditorSchemaResponse {
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
		"uuid":     {FieldType: "password", Required: true, CanClear: false},
		"password": {FieldType: "password", Required: true, CanClear: false},
		"sni":      {FieldType: "text", Required: false, CanClear: true},
		"alpn":     {FieldType: "string_list", Required: false, CanClear: true, DefaultValue: []string{"h3"}},
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

	return model.KeyEditorSchemaResponse{
		Protocols:            protocols,
		ExclusionReasonCodes: reasonCatalog,
	}
}
