package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/spf13/cobra"
)

// runProfiles is the measurement hook behind `aconiq run --cpuprofile` and
// `--memprofile`: a thin wrapper over runtime/pprof, so that a slow real scene
// can be profiled through the binary a user actually runs rather than through
// a benchmark that only approximates it.
//
// The CPU profile spans the whole command and the heap profile is taken once,
// at the end, after a forced GC so that it shows what the run still holds
// rather than garbage it has not yet collected. Both are written whether the
// run succeeds, fails or is cancelled: a run stopped by Ctrl-C returns through
// RunE like any other failure (see interruptContext), and stop runs on that
// path. A second Ctrl-C ends the process outright and takes the profile with
// it, which is the price of that escape hatch.
//
// Each profile is streamed into a temporary file beside its destination and
// renamed over it on stop, the streaming form of what internal/atomicfile
// does for buffers. Opening the temporary files before the run starts is
// deliberate: a profile path in a directory that does not exist is refused in
// the first millisecond, not after a half-hour run whose profile then has
// nowhere to go. Nothing is written into the project: there is no default
// path, and a relative one resolves against the working directory, as
// `go test -cpuprofile` does.
type runProfiles struct {
	cpuPath string
	cpuTmp  *os.File
	memPath string
	memTmp  *os.File
}

// startRunProfiles opens both temporary files and starts the CPU profile. An
// empty path disables that profile; with both empty it is a no-op.
func startRunProfiles(cpuPath, memPath string) (*runProfiles, error) {
	profiles := &runProfiles{cpuPath: cpuPath, memPath: memPath}

	if cpuPath != "" && memPath != "" && filepath.Clean(cpuPath) == filepath.Clean(memPath) {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.run", "--cpuprofile and --memprofile name the same file", nil)
	}

	var err error

	if memPath != "" {
		profiles.memTmp, err = createProfileTemp(memPath)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.run", "cannot write --memprofile", err)
		}
	}

	if cpuPath != "" {
		profiles.cpuTmp, err = createProfileTemp(cpuPath)
		if err != nil {
			profiles.discard()

			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.run", "cannot write --cpuprofile", err)
		}

		err = pprof.StartCPUProfile(profiles.cpuTmp)
		if err != nil {
			profiles.discard()

			return nil, domainerrors.New(domainerrors.KindInternal, "cli.run", "cannot start the CPU profile", err)
		}
	}

	return profiles, nil
}

// stop ends the CPU profile, takes the heap profile, and moves both into
// place. It returns the paths it wrote, so the caller can say where they are.
func (p *runProfiles) stop() ([]string, error) {
	var (
		written []string
		errs    []error
	)

	if p.cpuTmp != nil {
		pprof.StopCPUProfile()

		err := commitProfile(p.cpuTmp, p.cpuPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("write --cpuprofile: %w", err))
		} else {
			written = append(written, p.cpuPath)
		}
	}

	if p.memTmp != nil {
		err := writeHeapProfile(p.memTmp, p.memPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("write --memprofile: %w", err))
		} else {
			written = append(written, p.memPath)
		}
	}

	if len(errs) > 0 {
		return written, domainerrors.New(domainerrors.KindInternal, "cli.run", "profile not written", errors.Join(errs...))
	}

	return written, nil
}

// discard closes and removes whatever temporary files were opened, for a
// start that fails half-way.
func (p *runProfiles) discard() {
	for _, tmp := range []*os.File{p.cpuTmp, p.memTmp} {
		if tmp != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}
}

func createProfileTemp(path string) (*os.File, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary file for %s: %w", filepath.Base(path), err)
	}

	return tmp, nil
}

// writeHeapProfile collects first, so the profile's in-use figures describe
// what the run still holds rather than garbage it had not yet collected.
func writeHeapProfile(tmp *os.File, path string) error {
	runtime.GC()

	err := pprof.Lookup("heap").WriteTo(tmp, 0)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("write temporary %s: %w", filepath.Base(path), err)
	}

	return commitProfile(tmp, path)
}

func commitProfile(tmp *os.File, path string) error {
	err := tmp.Close()
	if err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("close temporary %s: %w", filepath.Base(path), err)
	}

	err = os.Rename(tmp.Name(), path)
	if err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("rename temporary %s: %w", filepath.Base(path), err)
	}

	return nil
}

// finishRunProfiles stops the profiles after the run has returned, however it
// returned, and logs where they went. The run's own error wins: it is what
// decides the exit code and what run.log already recorded, so a profile that
// could not be written is joined to it rather than put in its place.
func finishRunProfiles(cmd *cobra.Command, profiles *runProfiles, runErr error) error {
	written, err := profiles.stop()

	if state, ok := stateFromCommand(cmd); ok {
		for _, path := range written {
			state.Logger.Info("profile written", slog.String("path", path))
		}
	}

	if err == nil {
		return runErr
	}

	if runErr == nil {
		return err
	}

	return errors.Join(runErr, err)
}
