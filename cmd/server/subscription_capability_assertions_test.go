package main

import (
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/delivery"
)

func findDeliveryCapability(t *testing.T, matrix []delivery.ProtocolCapability, protocol, generation, output string) delivery.OutputCapability {
	t.Helper()
	for _, capability := range matrix {
		if capability.Protocol == protocol && capability.Generation == generation {
			result, ok := capability.Outputs[output]
			if !ok {
				t.Fatalf("%s/%s is missing %s", protocol, generation, output)
			}
			return result
		}
	}
	t.Fatalf("missing capability %s/%s", protocol, generation)
	return delivery.OutputCapability{}
}
func assertOfficialDeliveryCapability(t *testing.T, capability delivery.ProtocolCapability, output string, claim delivery.OutputCapability) {
	t.Helper()
	if claim.SyntaxValidation != "official_binary" {
		return
	}
	if claim.RuntimeInteroperability != "not_tested" {
		t.Fatalf("official syntax validation overstated runtime interoperability for %s/%s/%s", capability.Protocol, capability.Generation, output)
	}
	versions := map[string]struct{ target, minimum string }{
		"mihomo":    {delivery.TargetMihomoVersion, delivery.TargetMihomoVersion},
		"sing-box":  {delivery.TargetSingBoxVersion, delivery.TargetSingBoxVersion},
		"xray-json": {delivery.TargetXrayCurrentVersion, delivery.TargetXrayMinimumVersion},
	}
	if want, pinned := versions[output]; pinned && (claim.TargetVersion != want.target || claim.MinimumVersion != want.minimum) {
		t.Fatalf("%s/%s/%s capability versions=%q/%q want=%q/%q", capability.Protocol, capability.Generation, output, claim.TargetVersion, claim.MinimumVersion, want.target, want.minimum)
	}
}
