//go:build windows

package projectfs

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive LockFileEx lock on the first byte of file,
// waiting for it. Windows releases it when the handle closes or the process
// exits.
func lockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)

	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped)
	if err != nil {
		return fmt.Errorf("LockFileEx: %w", err)
	}

	return nil
}

func unlockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)

	err := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
	if err != nil {
		return fmt.Errorf("UnlockFileEx: %w", err)
	}

	return nil
}
