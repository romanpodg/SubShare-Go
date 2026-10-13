package keymanagement

import (
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"net/url"
	"strings"
)

func tuicConnection(patch *model.TUICStructuredPatch) (url.URL, url.Values, error) {
	var built url.URL
	query := url.Values{}
	if patch == nil {
		return url.URL{}, nil, fmt.Errorf("missing tuic patch payload")
	}
	built.Scheme = "tuic"
	built.User = url.UserPassword(setString(patch.UUID, ""), setString(patch.Password, ""))
	setIf(query, "sni", setString(patch.SNI, ""))
	if alpn := setField(patch.ALPN, []string(nil)); len(alpn) > 0 {
		query.Set("alpn", strings.Join(alpn, ","))
	}
	if setBool(patch.SkipCertVerify) {
		query.Set("skip_cert_verify", "1")
	}
	setIf(query, "congestion_control", setString(patch.CongestionController, ""))
	setIf(query, "udp_relay_mode", setString(patch.UDPRelayMode, ""))
	if setBool(patch.UDPOverStream) {
		query.Set("udp_over_stream", "1")
	}
	if setBool(patch.ZeroRTT) {
		query.Set("zero_rtt", "1")
	}
	setIf(query, "heartbeat", setString(patch.Heartbeat, ""))
	return built, query, nil
}
