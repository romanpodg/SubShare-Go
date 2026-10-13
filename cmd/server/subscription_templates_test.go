package main

import (
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"testing"
)

func TestFinalStructuredTemplateValidationRejectsUnsafeTransforms(t *testing.T) {
	tests := []struct {
		name   string
		format string
		body   string
	}{
		{"YAML scalar coercion", "mihomo", "proxies:\n  - name: true\n    type: ss\n"},
		{"YAML duplicate key", "mihomo", "proxies:\n  - name: first\n    name: second\n    type: ss\n"},
		{"YAML malformed indentation", "mihomo", "proxies:\n - name: edge\n   type: ss\n    server: example.test\n"},
		{"YAML removed required structure", "mihomo", "proxy-groups: []\n"},
		{"sing-box JSON type replacement", "sing-box", `{"outbounds":"removed"}`},
		{"sing-box JSON empty required structure", "sing-box", `{"outbounds":[]}`},
		{"sing-box JSON invalid outbound type", "sing-box", `{"outbounds":[{"type":7}]}`},
		{"Xray JSON object replaces array", "xray-json", `{"outbounds":[{"protocol":"vless"}]}`},
		{"Xray JSON removed required structure", "xray-json", `[{"routing":{}}]`},
		{"Xray JSON invalid protocol type", "xray-json", `[{"outbounds":[{"protocol":false}]}]`},
		{"unsafe NUL", "sing-box", "{\"outbounds\":[{\"type\":\"direct\"}]}\x00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			{
				err := delivery.ValidateStructuredBody(test.format, test.body)
				requireFixture(t, err != nil, "unsafe or structurally invalid templated output was accepted")
				requireFixture(t, err.Error() == delivery.ReasonSerialization, "unsafe or structurally invalid templated output was accepted")
			}

		})
	}

	valid := map[string]string{
		"mihomo":    `{"proxies":[{"name":"edge","type":"ss"}]}`,
		"sing-box":  `{"outbounds":[{"tag":"edge","type":"shadowsocks"}]}`,
		"xray-json": `[{"outbounds":[{"tag":"edge","protocol":"shadowsocks"}]}]`,
	}
	for format, body := range valid {
		final := applyTemplateContent("{{subscription}}", body, "ignored")
		{
			err := delivery.ValidateStructuredBody(format, final)
			requireFixturef(t, err == nil, "valid final %s template was rejected", format)
		}

	}
}

func TestStructuredTemplateRequiresSubscriptionPlaceholder(t *testing.T) {
	for _, format := range []string{"mihomo", "sing-box", "xray-json"} {
		_, err := validateTemplateInput(templateInput{Name: "safe", Format: format, Content: `{"replaced":true}`})
		requireFixturef(t, err != nil, "%s template without subscription placeholder was accepted", format)

	}
}
