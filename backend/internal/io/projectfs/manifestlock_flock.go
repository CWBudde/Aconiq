//go:build unix && !aix

package projectfs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive flock on file, waiting for it. This is every
// Unix Go builds for except AIX — Linux and Android, macOS and iOS, the BSDs,
// Solaris and illumos — through x/sys/unix, which has flock on all of them.
// flock is per open file description, so it excludes another descriptor in
// this process as well as another process, and the kernel drops it when the
// process dies.
func lockFile(file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			return fmt.Errorf("flock: %w", err)
		}

		return nil
	}
}

func unlockFile(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_UN)
	if err != nil {
		return fmt.Errorf("flock unlock: %w", err)
	}

	return nil
}
