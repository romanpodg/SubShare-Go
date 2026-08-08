//go:build !windows

package profilestorage

import (
	"fmt"
	"os"
)

func checkFilePermissions(_ string, info os.FileInfo) error {
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		return fmt.Errorf("file mode %04o is too permissive (must be 0600 or stricter)", perm)
	}
	return nil
}
