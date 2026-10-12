package main

import (
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

var externalTestFingerprintKey = []byte("0123456789abcdef0123456789abcdef")

const (
	externalTestVLESS  = "vless://11111111-1111-4111-8111-111111111111@vless.example:443?security=tls#VLESS"
	externalTestVMess  = "vmess://eyJ2IjoiMiIsInBzIjoiVk1lc3MiLCJhZGQiOiJ2bWVzcy5leGFtcGxlIiwicG9ydCI6IjQ0MyIsImlkIjoiMjIyMjIyMjItMjIyMi00MjIyLTgyMjItMjIyMjIyMjIyMjIyIn0="
	externalTestTrojan = "trojan://trojan-password@trojan.example:443#Trojan"
	externalTestSS     = "ss://YWVzLTI1Ni1nY206c2hhZG93LXBhc3N3b3Jk@ss.example:8388?plugin=local%3Bpsk%3Dplugin-secret#SS"
	externalTestHY2    = "hysteria2://hysteria-auth@hy.example:443,5000-5010?sni=tls.example&obfs=salamander&obfs-password=obfs-secret#HY2"
	externalTestTUIC   = "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?sni=tls.example&congestion_control=bbr#TUIC"
	externalTestTUICV4 = "tuic://legacy-token-value@tuic-v4.example:443?sni=tls.example#TUIC-v4"
	externalTestXray   = `{"outbounds":[{"protocol":"vless","tag":"Xray","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"44444444-4444-4444-8444-444444444444"}]}]}}]}`
)

const (
	deliverySSClean     = "ss://YWVzLTI1Ni1nY206c2hhZG93LXBhc3N3b3Jk@ss.example:8388#SS"
	deliverySSPlugin    = "ss://YWVzLTI1Ni1nY206c2hhZG93LXBhc3N3b3Jk@ss.example:8388?plugin=obfs-local%3Bobfs%3Dtls%3Bobfs-host%3Dcdn.example#SS-plugin"
	deliverySSXrayAlias = "ss://Y2hhY2hhMjAtcG9seTEzMDU6cGFzc3dvcmQ=@ss-alias.example:8388#SS-alias"
	deliveryHY2         = "hysteria2://hysteria-auth@hy.example:443,5000-5010?sni=tls.example&pinSHA256=abababababababababababababababababababababababababababababababab&obfs=salamander&obfs-password=obfs-secret#HY2"
	deliveryTUICMihomo  = "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?sni=tls.example&alpn=h3%2Chq-29&skip-cert-verify=true&congestion-controller=bbr&udp-relay-mode=quic&reduce-rtt=true&heartbeat-interval=10s&request-timeout=8s&fast-open=true&max-open-streams=20&max-udp-relay-packet-size=1500#TUIC"
	deliveryTUICSingBox = "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?server_name=tls.example&alpn=h3%2Chq-29&allow_insecure=true&congestion_control=bbr&udp_over_stream=true&zero_rtt_handshake=true&heartbeat=10s#TUIC"
)

func parseExternalTestBody(t *testing.T, body string) sources.ParseResult {
	t.Helper()
	parsed, err := sources.ParseBody(body, [][]byte{externalTestFingerprintKey})
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	return parsed
}
