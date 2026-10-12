package main

import (
	"encoding/json"
	"html"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type publicPageContracts struct {
	Config   model.SubscriptionPageConfig `json:"config"`
	Expected struct {
		Brand, Subtitle, Subhead, Section, Copyright, ImportURL, GoPrimaryHover string
	} `json:"expected"`
	Admission []struct {
		Name, Raw  string
		GoAccepted bool
	} `json:"admission"`
	Links []struct {
		Name, Href string
		GoAllowed  bool
	} `json:"links"`
	Themes   []struct{ Name, Value, GoValue string } `json:"themes"`
	Defaults struct {
		GoBrand, Locale                    string
		BlockTypes, StepTypes, DownloadIDs []string
	} `json:"defaults"`
	Template struct{ Source, Go string } `json:"template"`
}

func loadPublicPageContracts(t *testing.T) publicPageContracts {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/subscription-page/contracts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures publicPageContracts
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestPublicPageSharedSupportedBlocks(t *testing.T) {
	f := loadPublicPageContracts(t)
	document := renderSubscriptionPageHTML(f.Config, model.PanelSettings{}, "Fixture title", "/favicon.ico", "", "https://example.test/sub/fixture", f.Expected.ImportURL)
	for _, text := range []string{f.Expected.Brand, f.Expected.Subtitle, f.Expected.Subhead, f.Expected.Section, f.Expected.Copyright, "Text-only step", "Import fixture", "Fixture URL"} {
		if !strings.Contains(document, html.EscapeString(text)) {
			t.Errorf("missing escaped semantic text %q", text)
		}
	}
	for _, fragment := range []string{`href="https://example.test/client?a=1&amp;b=2"`, `target="_blank" rel="noreferrer"`, `href="/help?q=one&amp;lang=ru"`, `href="/admin/login"`, `href="` + f.Expected.ImportURL + `"`, `--btn-primary-hover: ` + f.Expected.GoPrimaryHover} {
		if !strings.Contains(document, fragment) {
			t.Errorf("missing contract %q", fragment)
		}
	}
	if strings.Contains(document, `<script>fixture</script>`) {
		t.Fatal("text escaped into executable markup")
	}
	if !strings.Contains(document, `const copiedLabel = "Copied fixture"`) || !strings.Contains(document, `const copyLabel = "Copy fixture"`) {
		t.Fatal("copy script lost configured labels")
	}
}

func TestPublicPageSharedAdmission(t *testing.T) {
	for _, f := range loadPublicPageContracts(t).Admission {
		t.Run(f.Name, func(t *testing.T) {
			_, raw, err := normalizeSubscriptionPageConfig(f.Raw)
			if (err == nil) != f.GoAccepted {
				t.Fatalf("accepted=%v, error=%v", err == nil, err)
			}
			if err == nil && raw != f.Raw {
				t.Fatal("configured JSON bytes changed")
			}
		})
	}
	_, raw, err := normalizeSubscriptionPageConfig(" \r\n\t")
	if err != nil || raw != defaultSubscriptionPageConfigJSON {
		t.Fatal("blank config lost default admission")
	}
}

func TestPublicPageSharedURLPolicy(t *testing.T) {
	for _, f := range loadPublicPageContracts(t).Links {
		t.Run(f.Name, func(t *testing.T) {
			var content strings.Builder
			renderLinkButtons(&content, []model.SubscriptionPageButton{{Label: "Fixture link", Href: f.Href}}, nil)
			if strings.Contains(content.String(), `<a `) != f.GoAllowed {
				t.Fatalf("unexpected link admission: %s", content.String())
			}
			if f.GoAllowed && !strings.Contains(content.String(), `href="`+html.EscapeString(f.Href)+`"`) {
				t.Fatal("href changed or failed attribute escaping")
			}
		})
	}
	if safePageURL("happ://add/fixture", nil, true, false) != "happ://add/fixture" {
		t.Fatal("activation import scheme rejected")
	}
	if safePageURL("data:image/png;base64,AA==", nil, false, true) == "" {
		t.Fatal("allowed raster image rejected")
	}
	if safePageURL("data:image/svg+xml;base64,AA==", nil, false, true) != "" {
		t.Fatal("unsafe SVG data admitted")
	}
}

func TestPublicPageSharedThemePolicy(t *testing.T) {
	for _, f := range loadPublicPageContracts(t).Themes {
		t.Run(f.Name, func(t *testing.T) {
			if got := cssVar(map[string]string{"badgeText": f.Value}, "badgeText", "#1f3a8a"); got != f.GoValue {
				t.Fatalf("got %q, want %q", got, f.GoValue)
			}
		})
	}
}

func publicPageDefaultLayout(cfg model.SubscriptionPageConfig) map[string][]string {
	layout := map[string][]string{}
	for _, block := range cfg.Blocks {
		layout["blocks"] = append(layout["blocks"], block.Type)
		for _, step := range block.Steps {
			layout["steps"] = append(layout["steps"], step.Block.Type)
			for _, button := range step.Block.Buttons {
				layout["downloads"] = append(layout["downloads"], button.ID)
			}
		}
	}
	return layout
}

func TestPublicPageSharedDefaults(t *testing.T) {
	f := loadPublicPageContracts(t)
	cfg := defaultSubscriptionPageConfig()
	if cfg.Locale != f.Defaults.Locale || cfg.TemplateVars["brandName"] != f.Defaults.GoBrand {
		t.Fatal("localized defaults changed")
	}
	want := map[string][]string{"blocks": f.Defaults.BlockTypes, "steps": f.Defaults.StepTypes, "downloads": f.Defaults.DownloadIDs}
	if !reflect.DeepEqual(publicPageDefaultLayout(cfg), want) {
		t.Fatal("supported default layout changed")
	}
}

func TestPublicPageSharedTemplates(t *testing.T) {
	f := loadPublicPageContracts(t)
	if applyTemplate(f.Template.Source, map[string]string{"brandName": "Fixture"}) != f.Template.Go {
		t.Fatal("template compatibility changed")
	}
	vars := mergeTemplateVars(f.Config, model.PanelSettings{PanelTitle: " Panel "}, "plain", "import")
	want := map[string]string{"brandName": "Panel", "greeting": "Здравствуйте", "subscriptionUrl": "plain", "importSubscriptionUrl": "import", "adminLoginPath": "/admin/login"}
	if !reflect.DeepEqual(vars, want) {
		t.Fatal("runtime template variables changed")
	}
}
