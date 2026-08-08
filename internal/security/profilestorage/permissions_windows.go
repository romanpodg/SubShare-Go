//go:build windows

package profilestorage

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

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
	return nil
}
