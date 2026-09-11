//go:build windows

package profilestorage

import "testing"

func TestFirstBroadAllowTrustee(t *testing.T) {
	for name, testCase := range map[string]struct {
		descriptor string
		wantFound  bool
		wantName   string
	}{
		"owner system and administrators only": {
			descriptor: `O:BAG:SYD:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;FA;;;S-1-5-21-1-2-3-1001)`,
			wantFound:  false,
		},
		"inherited builtin users read": {
			descriptor: `O:BAG:SYD:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1200a9;;;BU)`,
			wantFound:  true,
			wantName:   `BUILTIN\Users`,
		},
		"everyone by abbreviation": {
			descriptor: `O:BAG:SYD:(A;;FA;;;WD)`,
			wantFound:  true,
			wantName:   "Everyone",
		},
		"everyone by raw sid": {
			descriptor: `O:BAG:SYD:(A;;FA;;;S-1-1-0)`,
			wantFound:  true,
			wantName:   "Everyone",
		},
		"deny ace for a broad group is not a grant": {
			descriptor: `O:BAG:SYD:(D;;FA;;;WD)(A;;FA;;;SY)`,
			wantFound:  false,
		},
		"broad trustee only in the sacl is ignored": {
			descriptor: `O:BAG:SYD:(A;;FA;;;SY)S:(AU;SA;FA;;;WD)`,
			wantFound:  false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, found := firstBroadAllowTrustee(testCase.descriptor)
			if found != testCase.wantFound {
				t.Fatalf("found = %v, want %v (trustee %q)", found, testCase.wantFound, got)
			}
			if found && got != testCase.wantName {
				t.Fatalf("trustee = %q, want %q", got, testCase.wantName)
			}
		})
	}
}
