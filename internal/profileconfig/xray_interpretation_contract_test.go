package profileconfig

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type xrayInterpretationFixture struct {
	Name, Raw string
	Go        struct {
		Target            struct{ Host, Port, Error string }
		Drafts, Projected []LinkConfigurationDraft
		DraftError        string `json:"draft_error"`
		ProjectError      string `json:"project_error"`
		Rejected          int
	}
}

func TestXrayInterpretationBoundaryContracts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/configuration/xray-interpretation.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []xrayInterpretationFixture
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			assertXrayInterpretationTarget(t, fixture)
			assertXrayInterpretationDrafts(t, fixture)
			assertXrayInterpretationProjection(t, fixture)
		})
	}
}

func assertXrayInterpretationTarget(t *testing.T, fixture xrayInterpretationFixture) {
	t.Helper()
	host, port, err := ParseXrayJSONTarget(fixture.Raw)
	xrayInterpretationEqual(t, "target host", host, fixture.Go.Target.Host)
	xrayInterpretationEqual(t, "target port", port, fixture.Go.Target.Port)
	xrayInterpretationEqual(t, "target error", xrayInterpretationError(err), fixture.Go.Target.Error)
}

func assertXrayInterpretationDrafts(t *testing.T, fixture xrayInterpretationFixture) {
	t.Helper()
	drafts, err := ParseXrayJSONDrafts(fixture.Raw)
	xrayInterpretationEqual(t, "strict drafts", drafts, fixture.Go.Drafts)
	xrayInterpretationEqual(t, "strict error", xrayInterpretationError(err), fixture.Go.DraftError)
}

func assertXrayInterpretationProjection(t *testing.T, fixture xrayInterpretationFixture) {
	t.Helper()
	drafts, rejected, err := ProjectXrayJSONDrafts(fixture.Raw)
	xrayInterpretationEqual(t, "projected drafts", drafts, fixture.Go.Projected)
	xrayInterpretationEqual(t, "rejected outbounds", rejected, fixture.Go.Rejected)
	xrayInterpretationEqual(t, "projection error", xrayInterpretationError(err), fixture.Go.ProjectError)
}

func xrayInterpretationEqual(t *testing.T, label string, actual, expected any) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s differs from contract", label)
	}
}

func xrayInterpretationError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
