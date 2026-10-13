package projectfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// manifestLockFileName is the file the cross-process manifest lock is taken
// on. It sits beside project.json rather than being project.json, because Save
// replaces the manifest by rename: a lock held on the old inode would guard a
// file nobody opens any more.
const manifestLockFileName = "project.json.lock"

// LockManifest serialises one read-modify-write of `.noise/project.json`
// against every other one — other goroutines and other processes alike — and
// returns the release.
//
// # Why a cross-process lock
//
// Every writer loads the manifest, changes the value and saves it back.
// atomicfile makes each save whole, but it cannot make two read-modify-writes
// see each other: the later save drops the other's change. Two `aconiq run`
// processes on one project lost a run that way in 19 of 40 measured attempts,
// and `aconiq serve` starts one child per run, so concurrent runs through the
// API hit the same race. No in-process mutex reaches another process.
//
// # How
//
// An advisory lock on manifestLockFileName: flock(2) on the Unix systems that
// have it, LockFileEx on Windows. Both are released by the kernel when the
// process dies, so a killed `aconiq run` cannot leave the project locked. On
// a platform with neither — js/wasm, which has no project folder to share —
// only the in-process mutex below applies.
//
// The in-process mutex is taken first and always. flock already excludes two
// descriptors in one process, but the mutex is what holds on the fallback
// platforms, and it keeps goroutines from parking OS threads in the syscall.
//
// The lock is not reentrant. The Store methods that rewrite the manifest —
// CreateRun, DeleteRun, FailOpenRun, FailInterruptedRuns — take it
// themselves, so a caller must not hold it around them; a caller that loads
// and saves the manifest itself takes it around both.
//
// An uninitialised project has no `.noise/` to put the lock file in, and is
// answered with the error Load gives for it.
func (s Store) LockManifest() (func(), error) {
	mu := manifestMutexFor(s.manifestPath())
	mu.Lock()

	path := filepath.Join(s.controlDir(), manifestLockFileName)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		mu.Unlock()

		if errors.Is(err, os.ErrNotExist) {
			_, loadErr := s.Load()
			if loadErr != nil {
				return nil, loadErr
			}
		}

		return nil, domainerrors.New(domainerrors.KindInternal, "projectfs.LockManifest", "open manifest lock file", err)
	}

	err = lockFile(file)
	if err != nil {
		_ = file.Close()

		mu.Unlock()

		return nil, domainerrors.New(domainerrors.KindInternal, "projectfs.LockManifest", "lock manifest", err)
	}

	return func() {
		_ = unlockFile(file)
		_ = file.Close()

		mu.Unlock()
	}, nil
}

// manifestMutexes holds one mutex per manifest path, so two Store values over
// one project exclude each other in-process exactly as two processes do.
var manifestMutexes sync.Map

func manifestMutexFor(path string) *sync.Mutex {
	mu, _ := manifestMutexes.LoadOrStore(path, &sync.Mutex{})

	m, ok := mu.(*sync.Mutex)
	if !ok {
		panic(fmt.Sprintf("projectfs: manifest mutex for %s has type %T", path, mu))
	}

	return m
}
