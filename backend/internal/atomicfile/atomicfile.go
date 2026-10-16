// Package atomicfile holds one way of replacing a file's contents: write a
// temporary file beside the destination, then rename it over the top.
//
// The rule is that every writer that replaces a project or export artifact
// from a finished buffer goes through it - the manifest and the other JSON
// artifacts, terrain.tif, run.log, the engine's run state and chunk caches,
// result rasters and receiver tables, reports, bundle files and the OpenAPI
// document. A reader of any of them, in this process or in an `aconiq run`
// subprocess, sees the old file or the new one, never a prefix.
//
// It exists for the reason internal/jsonio exists: the bytes were already
// shared, the mechanism around them was not. `io/projectfs` replaced a file
// through a temporary file and a rename while `app/cli` called os.WriteFile
// directly, so the same artifact was written atomically through the HTTP API
// and non-atomically through the CLI - a crash part-way through the second
// left a truncated run summary, validation report or import report where a
// reader expects whole JSON.
//
// Writers that stream do not go through it yet, because it takes a []byte and
// they hold none: the report PDF (the typst compiler writes into the file, so
// a failed compile leaves a partial PDF), the receiver-table CSV, the export
// bundle's file copies, and the three GeoPackage exports, which SQLite writes
// itself. They need a streaming variant - a temporary file handed out as an
// io.Writer and renamed on success, as io/lglnimport's tile download already
// does by hand. Writers that replace nothing are out of scope: an append
// (projectfs.appendRunLogNote), an http.ResponseWriter, and the golden and
// acceptance snapshot writers, which run only under UPDATE_GOLDEN.
//
// A rename makes the replacement atomic for readers, not durable: nothing is
// fsynced, so a power loss can still leave an empty or stale file behind.
//
// What stays with the callers is the encoding and the error taxonomy, because
// neither is shared. `app/cli` and `io/projectfs` wrap a failure as
// domainerrors.New under their own op string, which is what the CLI derives
// its exit code from, and the rest wrap it with fmt.Errorf in their own words;
// jsonio.Marshal says only that marshalling failed. This package says only
// that the write failed, and names the file by its base name alone.
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
//
// The rename replaces an existing destination on every target this project
// releases for, Windows included: os.Rename documents that contract, and on
// Windows it reaches MoveFileEx with MOVEFILE_REPLACE_EXISTING rather than the
// bare MoveFile that syscall.Rename uses and that does refuse an existing
// name. io/projectfs has replaced the manifest this way since before this
// package existed.
func WriteFile(path string, data []byte) error {
	staged, err := Stage(path, data)
	if err != nil {
		return err
	}

	return staged.Commit()
}

// Staged is a temporary file written beside its destination and not yet
// renamed over it: WriteFile split in two, for a caller that has to check a
// precondition after the bytes are on disk and before anyone can see them.
//
// The case it exists for is a compare-and-write. A writer that stages first,
// then takes its lock, checks and commits, holds the lock for a rename rather
// than for the encoding and the write; and a refused write is a Discard that
// never touched the destination. Checking under the lock while writing the
// destination outside it does not hold: the refused write has already landed.
//
// The temporary file shares the destination's directory, so Commit is a
// rename on one filesystem and atomic like WriteFile's.
type Staged struct {
	tmpPath string
	path    string
}

// Stage writes data to a uniquely named temporary file beside path and returns
// it uncommitted. The destination is not touched. The caller must Commit or
// Discard it.
func Stage(path string, data []byte) (*Staged, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary file for %s: %w", filepath.Base(path), err)
	}

	tmpPath := tmp.Name()

	_, err = tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("write temporary %s: %w", filepath.Base(path), err)
	}

	err = tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("close temporary %s: %w", filepath.Base(path), err)
	}

	return &Staged{tmpPath: tmpPath, path: path}, nil
}

// Commit renames the staged file over its destination. On failure the
// temporary file is removed and the destination keeps its old bytes. Either
// way the Staged is spent: a later Discard does nothing, so a caller may defer
// Discard for every staged file and Commit the ones it keeps.
func (s *Staged) Commit() error {
	tmpPath := s.tmpPath
	s.tmpPath = ""

	err := os.Rename(tmpPath, s.path)
	if err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("rename temporary %s: %w", filepath.Base(s.path), err)
	}

	return nil
}

// Discard removes the staged file without touching the destination. It does
// nothing on a nil Staged or one already committed or discarded — after a
// Commit the temporary name is free again, and removing it blind could delete
// another writer's file.
func (s *Staged) Discard() {
	if s == nil || s.tmpPath == "" {
		return
	}

	_ = os.Remove(s.tmpPath)
	s.tmpPath = ""
}
