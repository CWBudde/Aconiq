// Package atomicfile holds one way of replacing a file's contents, and since
// this pass it is the only one two of the three JSON writers use.
//
// It exists for the reason internal/jsonio exists: the bytes were already
// shared, the mechanism around them was not. `io/projectfs` replaced a file
// through a temporary file and a rename while `app/cli` called os.WriteFile
// directly, so the same artifact was written atomically through the HTTP API
// and non-atomically through the CLI - a crash part-way through the second
// left a truncated run summary, validation report or import report where a
// reader expects whole JSON.
//
// What stays with the callers is the encoding and the error taxonomy, because
// neither is shared. `app/cli` and `io/projectfs` both wrap a failure as
// domainerrors.New under their own op string, which is what the CLI derives
// its exit code from; jsonio.Marshal says only that marshalling failed. This
// package says only that the write failed.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile replaces path with data through a temporary file in the same
// directory, so a reader sees either the old bytes or the new ones.
//
// The temporary name is unique per call, and that is the point rather than a
// detail. Both writers this was lifted from used to derive it from the
// destination, so every writer in every process shared one name: two
// concurrent writes truncated and refilled the same temp file, the first
// rename carried the second writer's bytes, and the second rename failed
// because the file it had written was already gone. The project manifest has a
// cross-process writer - the `aconiq run` subprocess - so no in-process lock
// can close that; a unique name can.
//
// The parent directory is not created. A caller that may be writing into a new
// directory calls os.MkdirAll first, which is what both do today, because the
// mode that directory should carry is theirs to decide.
//
// os.CreateTemp creates with 0o600, which is the mode every call site wrote.
func WriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", filepath.Base(path), err)
	}

	tmpPath := tmp.Name()

	_, err = tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)

		return fmt.Errorf("write temporary %s: %w", filepath.Base(path), err)
	}

	err = tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("close temporary %s: %w", filepath.Base(path), err)
	}

	err = os.Rename(tmpPath, path)
	if err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("rename temporary %s: %w", filepath.Base(path), err)
	}

	return nil
}
