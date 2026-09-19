//go:build windows

package profilestorage

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// broadTrustees are well-known SIDs standing for "more than the owner and the
// machine". An allow ACE naming any of them means the keyring is readable by
// accounts that must never see key material — most commonly BUILTIN\Users,
// inherited from a parent directory that was never locked down. This is the
// Windows counterpart of the Unix build's refusal of any group/other bit.
var broadTrustees = map[string]string{
	"WD":           "Everyone",
	"S-1-1-0":      "Everyone",
	"BU":           "BUILTIN\\Users",
	"S-1-5-32-545": "BUILTIN\\Users",
	"AU":           "Authenticated Users",
	"S-1-5-11":     "Authenticated Users",
	"IU":           "Interactive",
	"S-1-5-4":      "Interactive",
	"BG":           "BUILTIN\\Guests",
	"S-1-5-32-546": "BUILTIN\\Guests",
	"AN":           "Anonymous",
	"S-1-5-7":      "Anonymous",
}

func checkFilePermissions(filePath string, _ os.FileInfo) error {
	sd, err := windows.GetNamedSecurityInfo(
		filePath,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read windows DACL: %w", err)
	}
	if sd == nil {
		return fmt.Errorf("null security descriptor")
	}
	descriptor := sd.String()
	if !strings.Contains(descriptor, "D:") {
		return fmt.Errorf("could not read the file's access control list")
	}
	// A null DACL is not an empty one: it grants full control to everyone.
	if strings.Contains(descriptor, "D:NO_ACCESS_CONTROL") {
		return fmt.Errorf("file has a null DACL, which grants full control to everyone")
	}
	if trustee, found := firstBroadAllowTrustee(descriptor); found {
		return fmt.Errorf(
			"DACL grants access to %s (must be limited to the owner, SYSTEM, and Administrators)",
			trustee,
		)
	}
	return nil
}

// firstBroadAllowTrustee scans the DACL section of an SDDL descriptor and
// returns the first broad trustee that is granted access. SDDL ACEs have the
// form (type;flags;rights;object_guid;inherit_object_guid;trustee), so the
// trustee is field 5 and the ACE type is field 0.
func firstBroadAllowTrustee(descriptor string) (string, bool) {
	start := strings.Index(descriptor, "D:")
	if start < 0 {
		return "", false
	}
	section := descriptor[start+2:]
	// The SACL section, when present, always follows the DACL.
	if end := strings.Index(section, "S:"); end >= 0 {
		section = section[:end]
	}

	for {
		open := strings.IndexByte(section, '(')
		if open < 0 {
			return "", false
		}
		shut := strings.IndexByte(section[open:], ')')
		if shut < 0 {
			return "", false
		}
		ace := section[open+1 : open+shut]
		section = section[open+shut:]

		fields := strings.Split(ace, ";")
		if len(fields) < 6 {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(fields[0])) {
		case "A", "OA": // ACCESS_ALLOWED and ACCESS_ALLOWED_OBJECT
		default:
			continue
		}
		trustee := strings.ToUpper(strings.TrimSpace(fields[5]))
		if name, broad := broadTrustees[trustee]; broad {
			return name, true
		}
	}
}
