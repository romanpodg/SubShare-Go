package keymanagement

import (
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func set(value string) *model.TriStatePatch[string] {
	return &model.TriStatePatch[string]{Set: true, Operation: model.TriStateSet, Value: value}
}

func TestBuildURIFromStructuredCreateEscapesReservedCharacters(t *testing.T) {
	uri, err := BuildURIFromStructuredCreate("hysteria2", "fallback", &model.StructuredProfilePatch{
		Server:      set("edge.example"),
		Port:        set("8443"),
		DisplayName: set("Edge #1 @home"),
		Hysteria2: &model.Hysteria2StructuredPatch{
			Authentication:      set("p@ss#word?x"),
			SNI:                 set("sni.example"),
			ObfuscationType:     set("salamander"),
			ObfuscationPassword: set("ob&fs"),
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	parsed, err := profiles.Parse(uri)
	if err != nil {
		t.Fatalf("built uri does not parse: %v (%s)", err, uri)
	}
	data, ok := parsed.Data.(profiles.Hysteria2Data)
	if !ok {
		t.Fatalf("data = %T", parsed.Data)
	}
	if parsed.Server != "edge.example" || parsed.Port.Expression != "8443" || parsed.DisplayName != "Edge #1 @home" {
		t.Fatalf("endpoint lost: %v", parsed)
	}
	if data.Authentication.Reveal() != "p@ss#word?x" || data.SNI != "sni.example" || data.ObfuscationPassword.Reveal() != "ob&fs" {
		t.Fatal("credentials or query values were corrupted by the builder")
	}
}

func TestBuildURIFromStructuredCreateShadowsocksPlugin(t *testing.T) {
	uri, err := BuildURIFromStructuredCreate("shadowsocks", "SS", &model.StructuredProfilePatch{
		Server: set("ss.example"),
		Shadowsocks: &model.ShadowsocksStructuredPatch{
			Method:        set("2022-blake3-aes-128-gcm"),
			Password:      set("MTIzNDU2Nzg5MDEyMzQ1Ng=="),
			PluginName:    set("v2ray-plugin"),
			PluginOptions: set("mode=websocket;host=x"),
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	parsed, err := profiles.Parse(uri)
	if err != nil {
		t.Fatalf("built uri does not parse: %v (%s)", err, uri)
	}
	data := parsed.Data.(profiles.ShadowsocksData)
	if data.Method != "2022-blake3-aes-128-gcm" || data.Password.Reveal() != "MTIzNDU2Nzg5MDEyMzQ1Ng==" || data.Plugin == nil || data.Plugin.Name != "v2ray-plugin" {
		t.Fatalf("shadowsocks fields lost: %+v", data)
	}
}
