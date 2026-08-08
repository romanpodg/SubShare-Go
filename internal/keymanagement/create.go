package keymanagement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func generateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func BuildURIFromStructuredCreate(proto, label string, patch *model.StructuredProfilePatch) (string, error) {
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

func (s *Service) CreateLocal(ctx context.Context, params CreateLocalParams) (*model.KeyProfileDetailResponse, error) {
	label := strings.TrimSpace(params.Label)
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	templateText := strings.TrimSpace(params.TemplateText)

	if !statusOK || !kindOK || label == "" {
		return nil, ErrInvalidInput
	}
	if len(label) > 255 {
		return nil, ErrLabelTooLong
	}

	creationMode := strings.ToLower(strings.TrimSpace(params.CreationMode))
	if creationMode == "" {
		if params.RawURI != "" {
			creationMode = "raw"
		} else {
			creationMode = "structured"
		}
	}
	if creationMode != "raw" && creationMode != "structured" {
		return nil, ErrInvalidCreationMode
	}
	if creationMode == "raw" && params.Structured != nil {
		return nil, ErrMutuallyExclusiveMode
	}
	if creationMode == "structured" && strings.TrimSpace(params.RawURI) != "" {
		return nil, ErrMutuallyExclusiveMode
	}

	builtURI := ""
	if kind == model.KeyKindReal {
		if creationMode == "raw" {
			builtURI = strings.TrimSpace(params.RawURI)
			if builtURI == "" {
				return nil, ErrRawURIRequired
			}
			if _, parseErr := profiles.Parse(builtURI); parseErr != nil {
				return nil, fmt.Errorf("%w: %s", ErrInvalidProfileURI, parseErr.Error())
			}
		} else {
			proto := strings.ToLower(strings.TrimSpace(params.Protocol))
			if proto == "tuic_v4" {
				return nil, ErrTUICv4StructuredForbidden
			}
			if params.Structured == nil {
				return nil, ErrStructuredPayloadRequired
			}
			var err error
			builtURI, err = BuildURIFromStructuredCreate(proto, label, params.Structured)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if creationMode == "structured" && params.Structured != nil {
			return nil, ErrInformationalStructuredForbidden
		}
		if templateText == "" {
			templateText = label
		}
		token, err := generateToken(12)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key token: %w", err)
		}
		builtURI = "info://" + token
	}

	parsedProfile, _ := profiles.Parse(builtURI)
	storedProtocol := "legacy"
	if parsedProfile != nil {
		storedProtocol = string(parsedProfile.Protocol)
	}

	createdKey, decryptedURI, err := s.profileRepo.CreateLocal(ctx, profilepersistence.CreateProfileParams{
		Label:        label,
		Status:       status,
		Kind:         kind,
		Category:     params.Category,
		CategoryID:   params.CategoryID,
		TemplateText: templateText,
		Protocol:     storedProtocol,
		BuiltURI:     builtURI,
	})
	if err != nil {
		return nil, err
	}

	detail := BuildKeyProfileDetailResponse(*createdKey, decryptedURI, s.capabilityResolver)
	return &detail, nil
}
