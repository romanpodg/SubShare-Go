package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderersPreserveVMessTrojanAndXrayOutbounds(t *testing.T) {
	vmessPayload, err := json.Marshal(map[string]any{
		"v": "2", "ps": "vmess", "add": "vmess.example", "port": "443",
		"id": "22222222-2222-2222-2222-222222222222", "aid": "0",
		"net": "ws", "path": "/socket", "host": "cdn.example", "tls": "tls", "sni": "vmess.example",
	})
	requireFixturef(t, err == nil, "fixture Marshal: %v", err)

	vmess := "vmess://" + base64.StdEncoding.EncodeToString(vmessPayload)
	trojan := "trojan://secret@trojan.example:443?security=tls&sni=trojan.example&type=grpc&serviceName=edge#trojan"
	xray := `{"outbounds":[{"protocol":"vless","tag":"xray-vless","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"33333333-3333-3333-3333-333333333333","encryption":"none"}]}]},"streamSettings":{"network":"grpc","security":"reality","realitySettings":{"serverName":"xray.example","publicKey":"public","shortId":"abcd"}}}]}`
	raw := strings.Join([]string{vmess, trojan, xray}, "\n")

	for name, render := range map[string]func(string) (string, error){
		"mihomo":   renderMihomoSubscription,
		"sing-box": renderSingBoxSubscription,
	} {
		t.Run(name, func(t *testing.T) {
			rendered, err := render(raw)
			requireFixturef(t, err == nil, "render: %v", err)

			for _, expected := range []string{"vmess", "trojan", "xray-vless"} {
				requireFixturef(t, strings.Contains(rendered, expected), "%s disappeared from output: %s", expected, rendered)

			}
		})
	}
}

func TestStrictJSONRejectsUnknownTrailingAndOversizedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown", `{"mode":"all","surprise":true}`},
		{"trailing", `{"mode":"all"} {"mode":"selected"}`},
		{"oversized", `{"mode":"` + strings.Repeat("x", 1024) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request model.UpdateKeyAssignmentRequest
			err := httpapi.ReadJSONWithLimit(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString(test.body)), &request, 128)
			requireFixture(t, err != nil, "invalid JSON body was accepted")

		})
	}
}

func TestPublicPageEscapesScriptCSSAndUnsafeURLs(t *testing.T) {
	cfg := defaultSubscriptionPageConfig()
	cfg.Theme["pageBackground"] = `red;} </style><script>alert(1)</script>`
	cfg.Blocks[0].Logo.Src = `javascript:alert(1)`
	cfg.Blocks[1].Steps[0].Block.Buttons[0].Href = `javascript:alert(2)`
	htmlDocument := renderSubscriptionPageHTML(
		cfg,
		model.PanelSettings{PanelTitle: `</script><script>alert(3)</script>`},
		`</title><script>alert(4)</script>`,
		`javascript:alert(5)`,
		`javascript:alert(6)`,
		`https://example.test/sub/</script><script>alert(7)</script>`,
		`happ://add/</script><script>alert(8)</script>`,
	)
	requireFixturef(t, !(strings.Contains(htmlDocument, "<script>alert(")), "unsafe content reached page: %s", htmlDocument)
	requireFixturef(t, !(strings.Contains(htmlDocument, "javascript:")), "unsafe content reached page: %s", htmlDocument)

	requireFixturef(t, strings.Contains(htmlDocument, `\u003c/script\u003e`), "inline JSON was not safely encoded: %s", htmlDocument)

}
