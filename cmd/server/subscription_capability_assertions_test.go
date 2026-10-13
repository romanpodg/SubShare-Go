package main

import (
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/delivery"
)

type deliveryCapabilitySelector struct{ Protocol, Generation, Output string }

func findDeliveryCapability(t *testing.T, matrix []delivery.ProtocolCapability, selector deliveryCapabilitySelector) delivery.OutputCapability {
	t.Helper()
	for _, capability := range matrix {
		if capability.Protocol == selector.Protocol && capability.Generation == selector.Generation {
			result, ok := capability.Outputs[selector.Output]
			requireFixturef(t, ok, "%s/%s is missing %s", selector.Protocol, selector.Generation, selector.Output)

			return result
		}
	}
	t.Fatalf("missing capability %s/%s", selector.Protocol, selector.Generation)
	return delivery.OutputCapability{}
}
func assertOfficialDeliveryCapability(t *testing.T, capability delivery.ProtocolCapability, output string, claim delivery.OutputCapability) {
	t.Helper()
	if claim.SyntaxValidation != "official_binary" {
		return
	}
	requireFixturef(t, claim.RuntimeInteroperability == "not_tested", "official syntax validation overstated runtime interoperability for %s/%s/%s", capability.Protocol, capability.Generation, output)

	versions := map[string]struct{ target, minimum string }{
		"mihomo":    {delivery.TargetMihomoVersion, delivery.TargetMihomoVersion},
		"sing-box":  {delivery.TargetSingBoxVersion, delivery.TargetSingBoxVersion},
		"xray-json": {delivery.TargetXrayCurrentVersion, delivery.TargetXrayMinimumVersion},
	}
	want, pinned := versions[output]
	if !pinned {
		return
	}
	requireFixturef(t, claim.TargetVersion == want.target, "%s/%s/%s target version=%q want=%q", capability.Protocol, capability.Generation, output, claim.TargetVersion, want.target)
	requireFixturef(t, claim.MinimumVersion == want.minimum, "%s/%s/%s minimum version=%q want=%q", capability.Protocol, capability.Generation, output, claim.MinimumVersion, want.minimum)

}
