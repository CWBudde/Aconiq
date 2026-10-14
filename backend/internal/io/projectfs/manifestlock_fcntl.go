//go:build aix

package projectfs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive POSIX record lock on the whole of file, waiting
// for it. AIX has no flock, so this is fcntl(F_SETLKW). A record lock belongs
// to the process, not the descriptor, so it does not exclude a second
// descriptor in this process — LockManifest's in-process mutex does that —
// and the kernel drops it when the process dies.
func lockFile(file *os.File) error {
	lock := unix.Flock_t{Type: unix.F_WRLCK, Whence: 0, Start: 0, Len: 0}

	for {
		err := unix.FcntlFlock(file.Fd(), unix.F_SETLKW, &lock)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			return fmt.Errorf("fcntl lock: %w", err)
		}

		return nil
	}
}

func unlockFile(file *os.File) error {
	lock := unix.Flock_t{Type: unix.F_UNLCK, Whence: 0, Start: 0, Len: 0}

	err := unix.FcntlFlock(file.Fd(), unix.F_SETLK, &lock)
	if err != nil {
		return fmt.Errorf("fcntl unlock: %w", err)
	}

	return nil
}
