package profileconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

type sharedXrayProjection struct {
	Protocol   string `json:"protocol"`
	Server     string `json:"server"`
	Port       string `json:"port"`
	Identifier string `json:"identifier"`
	Network    string `json:"network"`
	Security   string `json:"security"`
	SNI        string `json:"sni"`
}

type sharedXrayFixture struct {
	Name     string               `json:"name"`
	Raw      string               `json:"raw"`
	Expected sharedXrayProjection `json:"expected"`
}

func readSharedXrayFixtures(t *testing.T) []sharedXrayFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "configuration", "xray-shared.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []sharedXrayFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 63 {
		t.Fatal("shared corpus must cover protocol, network and security combinations")
	}
	return fixtures
}

func TestSharedFrontendXrayProjection(t *testing.T) {
	for _, fixture := range readSharedXrayFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			assertSharedXrayDraft(t, fixture)
			assertSharedXrayTarget(t, fixture)
		})
	}
}

func assertSharedXrayDraft(t *testing.T, fixture sharedXrayFixture) {
	t.Helper()
	drafts, err := ParseXrayJSONDrafts(fixture.Raw)
	if err != nil || len(drafts) != 2 {
		t.Fatalf("expected two supported drafts: count=%d err=%v", len(drafts), err)
	}
	first := drafts[0]
	got := sharedXrayProjection{first.Protocol, first.Server, strconv.Itoa(first.Port), first.Identifier, first.Network, first.Security, first.SNI}
	if !reflect.DeepEqual(got, fixture.Expected) {
		t.Fatal("first supported draft differs from the shared frontend projection")
	}
}

func assertSharedXrayTarget(t *testing.T, fixture sharedXrayFixture) {
	t.Helper()
	host, port, err := ParseXrayJSONTarget(fixture.Raw)
	if err != nil {
		t.Fatalf("shared target parse: %v", err)
	}
	if host != fixture.Expected.Server || port != fixture.Expected.Port {
		t.Fatal("shared target differs from the frontend connection")
	}
}
