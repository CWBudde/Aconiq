//go:build !(linux || darwin || freebsd || openbsd || netbsd || dragonfly || windows)

package projectfs

import "os"

// lockFile is a no-op where the platform offers no advisory file lock this
// package can reach without a new dependency — js/wasm above all, which has
// no project folder another process could share. LockManifest's in-process
// mutex is the whole of the lock there.
func lockFile(*os.File) error { return nil }

func unlockFile(*os.File) error { return nil }
