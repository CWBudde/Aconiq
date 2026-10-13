//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package projectfs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockFile takes an exclusive flock on file, waiting for it. flock is per open
// file description, so it excludes another descriptor in this process as
// well as another process, and the kernel drops it when the process dies.
func lockFile(file *os.File) error {
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
		if errors.Is(err, syscall.EINTR) {
			continue
		}

		if err != nil {
			return fmt.Errorf("flock: %w", err)
		}

		return nil
	}
}

func unlockFile(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if err != nil {
		return fmt.Errorf("flock unlock: %w", err)
	}

	return nil
}
