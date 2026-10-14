//go:build !unix && !windows

package projectfs

import "os"

// lockFile is a no-op on the platforms that are neither Unix nor Windows:
// js/wasm, wasip1 and plan9. Of those only js/wasm is built from this
// repository, for the browser kernel, and it has no project folder another
// process could share. LockManifest's in-process mutex is the whole of the
// lock there.
func lockFile(*os.File) error { return nil }

func unlockFile(*os.File) error { return nil }
