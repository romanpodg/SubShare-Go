package keymanagement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"net"
	"net/url"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func generateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// BuildURIFromStructuredCreate turns a structured create payload into a share
// URI. The URI is assembled with proper escaping and then handed to the
// protocol adapter, whose canonical serialisation is what gets stored.
func BuildURIFromStructuredCreate(proto, label string, patch *model.StructuredProfilePatch) (string, error) {
	if patch == nil {
		return "", fmt.Errorf("missing structured patch payload")
	}
	server := setString(patch.Server, "")
	port := setString(patch.Port, "443")
	displayName := setString(patch.DisplayName, label)

	var built url.URL
	query := url.Values{}
	switch proto {
	case "shadowsocks":
		if patch.Shadowsocks == nil {
			return "", fmt.Errorf("missing shadowsocks patch payload")
		}
		built.Scheme = "ss"
		built.User = url.UserPassword(setString(patch.Shadowsocks.Method, "2022-blake3-aes-128-gcm"), setString(patch.Shadowsocks.Password, ""))
		if plugin := setString(patch.Shadowsocks.PluginName, ""); plugin != "" {
			if opts := setString(patch.Shadowsocks.PluginOptions, ""); opts != "" {
				plugin += ";" + opts
			}
			query.Set("plugin", plugin)
		}
	case "hysteria2", "hy2":
		if patch.Hysteria2 == nil {
			return "", fmt.Errorf("missing hysteria2 patch payload")
		}
		built.Scheme = "hysteria2"
		built.User = url.User(setString(patch.Hysteria2.Authentication, ""))
		setIf(query, "sni", setString(patch.Hysteria2.SNI, ""))
		if setBool(patch.Hysteria2.Insecure) {
			query.Set("insecure", "1")
		}
		setIf(query, "pinSHA256", setString(patch.Hysteria2.CertificateSHA256, ""))
		setIf(query, "obfs", setString(patch.Hysteria2.ObfuscationType, ""))
		setIf(query, "obfs-password", setString(patch.Hysteria2.ObfuscationPassword, ""))
	case "tuic":
		if patch.TUIC == nil {
			return "", fmt.Errorf("missing tuic patch payload")
		}
		built.Scheme = "tuic"
		built.User = url.UserPassword(setString(patch.TUIC.UUID, ""), setString(patch.TUIC.Password, ""))
		setIf(query, "sni", setString(patch.TUIC.SNI, ""))
		if patch.TUIC.ALPN != nil && patch.TUIC.ALPN.Set && patch.TUIC.ALPN.Operation == model.TriStateSet && len(patch.TUIC.ALPN.Value) > 0 {
			query.Set("alpn", strings.Join(patch.TUIC.ALPN.Value, ","))
		}
		if setBool(patch.TUIC.SkipCertVerify) {
			query.Set("skip_cert_verify", "1")
		}
		setIf(query, "congestion_control", setString(patch.TUIC.CongestionController, ""))
		setIf(query, "udp_relay_mode", setString(patch.TUIC.UDPRelayMode, ""))
		if setBool(patch.TUIC.UDPOverStream) {
			query.Set("udp_over_stream", "1")
		}
		if setBool(patch.TUIC.ZeroRTT) {
			query.Set("zero_rtt", "1")
		}
		setIf(query, "heartbeat", setString(patch.TUIC.Heartbeat, ""))
	default:
		return "", fmt.Errorf("unsupported protocol for structured creation: %s", proto)
	}
	built.Host = net.JoinHostPort(server, port)
	built.RawQuery = query.Encode()
	built.Fragment = displayName
	raw := built.String()

	// Let the adapter validate and produce the canonical bytes; an input the
	// adapter rejects is stored as-is, exactly as before, and surfaces as a
	// legacy-compatibility profile.
	parsed, err := profiles.Parse(raw)
	if err != nil {
		return raw, nil
	}
	canonical, err := profiles.Serialize(parsed, profiles.CanonicalSerialization)
	if err != nil {
		return raw, nil
	}
	return canonical.URI.Reveal(), nil
}

func setString(field *model.TriStatePatch[string], fallback string) string {
	if field != nil && field.Set && field.Operation == model.TriStateSet {
		return field.Value
	}
	return fallback
}

func setBool(field *model.TriStatePatch[bool]) bool {
	return field != nil && field.Set && field.Operation == model.TriStateSet && field.Value
}

func setIf(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func (s *Service) CreateLocal(ctx context.Context, params CreateLocalParams) (*model.KeyProfileDetailResponse, error) {
	label := strings.TrimSpace(params.Label)
	clientDisplayName := ""
	if params.ClientDisplayName != nil {
		clientDisplayName = strings.TrimSpace(*params.ClientDisplayName)
		if clientDisplayName == "" {
			clientDisplayName = label
		}
	}
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	templateText := strings.TrimSpace(params.TemplateText)

	if !statusOK || !kindOK || label == "" {
		return nil, ErrInvalidInput
	}
	if len(label) > 255 {
		return nil, ErrLabelTooLong
	}
	if len(clientDisplayName) > 255 {
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
			if _, _, parseErr := validateStoredConfiguration(builtURI); parseErr != nil {
				return nil, invalidProfileURIError(parseErr)
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

	storedProtocol := "legacy"
	if protocol, _, validationErr := validateStoredConfiguration(builtURI); validationErr == nil {
		storedProtocol = protocol
	}

	createdKey, decryptedURI, err := s.repo.CreateLocal(ctx, CreateProfileParams{
		Label:             label,
		ClientDisplayName: clientDisplayName,
		Status:            status,
		Kind:              kind,
		Category:          params.Category,
		CategoryID:        params.CategoryID,
		TemplateText:      templateText,
		Protocol:          storedProtocol,
		BuiltURI:          builtURI,
	})
	if err != nil {
		return nil, err
	}

	detail := BuildKeyProfileDetailResponse(*createdKey, decryptedURI, s.capabilityResolver)
	return &detail, nil
}
