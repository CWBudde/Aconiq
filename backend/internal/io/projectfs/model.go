package projectfs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/jsonio"
)

// The files a model import writes under .noise/model/. `aconiq run` reads the
// normalized model from here by default, so the names are part of the project
// format, not an implementation detail of whichever importer wrote them.
const (
	modelDirName            = "model"
	modelNormalizedFileName = "model.normalized.geojson"
	modelDumpFileName       = "model.dump.json"
	modelValidationFileName = "validation-report.json"
	modelDirMode            = 0o750
)

// ModelArtifactPaths names the three model artifacts as absolute paths.
type ModelArtifactPaths struct {
	// Normalized is the canonical GeoJSON the engine consumes.
	Normalized string
	// Dump is the compact debug projection of the same model.
	Dump string
	// Validation is the validation report the import ran before persisting.
	Validation string
}

// ModelArtifactPaths returns where this store keeps the model artifacts.
func (s Store) ModelArtifactPaths() ModelArtifactPaths {
	dir := s.modelDir()

	return ModelArtifactPaths{
		Normalized: filepath.Join(dir, modelNormalizedFileName),
		Dump:       filepath.Join(dir, modelDumpFileName),
		Validation: filepath.Join(dir, modelValidationFileName),
	}
}

// RelativePath renders an absolute path inside the project as the
// project-relative, forward-slash form the manifest and every user-facing
// output use. A path outside the root still comes back relative, climbing out
// with ".." segments; only a path filepath.Rel cannot relate to the root at all
// (a relative input, or another volume on Windows) is returned slash-normalised
// but otherwise unchanged.
func (s Store) RelativePath(path string) string {
	rel, err := filepath.Rel(s.root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}

// ReadModel returns the normalized model exactly as it sits on disk.
//
// The path is derived from the store root — ModelArtifactPaths, the same
// derivation SaveModel writes through — and never from a manifest artifact ref.
// A manifest is editable data: a ref could name any path on the machine, so
// resolving one here would make the read only as contained as the file it
// reads. Deriving the path structurally means there is nothing to validate.
func (s Store) ReadModel() ([]byte, error) {
	raw, err := os.ReadFile(s.ModelArtifactPaths().Normalized)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domainerrors.New(domainerrors.KindNotFound, "projectfs.ReadModel", "no model has been saved", err)
		}

		return nil, domainerrors.New(domainerrors.KindInternal, "projectfs.ReadModel", "read normalized model", err)
	}

	return raw, nil
}

// ReadModelWithHash returns the normalized model together with the SHA-256 of
// exactly the bytes it returned.
//
// Reading the file and hashing the path are two reads, and a save landing
// between them hands the caller one model's bytes under another model's
// receipt. A client that stores that pair believes it holds the saved model
// when it holds the previous one — and the whole point of the receipt is that
// the client never checks. Hashing what was read cannot drift.
//
// The bytes are materialised, unlike hashFile's streaming of untrusted imports:
// this file is one SaveModel already accepted whole.
func (s Store) ReadModelWithHash() ([]byte, string, error) {
	raw, err := s.ReadModel()
	if err != nil {
		return nil, "", err
	}

	sum := sha256.Sum256(raw)

	return raw, hex.EncodeToString(sum[:]), nil
}

// ModelHash returns the SHA-256 of the normalized model file as bare lowercase
// hex — the spelling hashInputs writes into provenance.json, so a caller can
// compare the two without normalising either.
//
// It is a receipt of what is on disk, computed on the server side. A client
// stores the value it was handed and compares strings later; it never recomputes
// one, because a re-serialised model is not the same bytes.
func (s Store) ModelHash() (string, error) {
	sum, err := hashFile(s.ModelArtifactPaths().Normalized)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", domainerrors.New(domainerrors.KindNotFound, "projectfs.ModelHash", "no model has been saved", err)
		}

		return "", domainerrors.New(domainerrors.KindInternal, "projectfs.ModelHash", "hash normalized model", err)
	}

	return sum, nil
}

// SaveModel replaces the project model: it writes the normalized GeoJSON, the
// dump and the validation report to ModelArtifactPaths, upserts the three model
// artifact refs into proj and saves the manifest.
//
// It is the one persistence path for a model, shared by `aconiq import` and by
// `POST /api/v1/model`, so the two cannot drift in file names, artifact IDs or
// manifest handling. Callers validate before calling: a report with errors is
// persisted as given, because refusing it is the caller's decision to surface,
// not the store's.
//
// extraRefs are upserted into the same manifest save. An importer that writes
// a report of its own beside the model - the SoundPLAN one does - needs its
// ref to land with the three model refs rather than in a second
// read-modify-write. It is variadic because the three callers that register
// nothing extra should not have to say so.
//
// # The manifest it saves is re-read, not the caller's
//
// proj is the copy the caller loaded at its start, and `aconiq import` loads it
// before an Overpass, LGLN or SoundPLAN fetch that can take minutes. Saving
// that copy would put back whatever another process changed meanwhile — a run
// `aconiq serve` completed would read `running` again, without its artifact
// refs, and be swept to failed. So the refs are upserted into the manifest as
// it is now, re-read under LockManifest, and *proj is replaced with what was
// saved. Nothing else the caller changed on proj is saved: the model is the
// only thing this function owns. The lock is taken here, so a caller must not
// hold it.
func (s Store) SaveModel(
	proj *project.Project,
	model modelgeojson.Model,
	report modelgeojson.ValidationReport,
	extraRefs ...project.ArtifactRef,
) error {
	_, err := s.SaveModelIf(proj, model, report, "", extraRefs...)

	return err
}

// SaveModelIf is SaveModel with a precondition: the model on disk must be the
// one expected names, or nothing is written.
//
// expected is the current model's hash (ModelHash), ExpectNone for "no model
// may exist yet", or empty for no precondition, which is SaveModel. A refusal
// is a *HashMismatchError naming the hash that is there.
//
// It returns the saved model's receipt: the SHA-256 of the normalized bytes it
// committed, which is what ModelHash reads back off the file. It is computed
// from those bytes rather than re-read after the save, because by then the lock
// is released and another writer may already have replaced the file — a
// receipt for their model would let the client overwrite it under the belief
// that it was guarding its own.
//
// An uninitialised folder is refused before anything is written: creating
// .noise/model would give LockManifest a directory to lock in, and the files
// would land before the manifest load failed.
//
// # Why the files are staged
//
// The check has to be atomic with the write, or two clients holding the same
// hash both pass it and the second silently overwrites the first. So the
// compare happens under LockManifest — the lock `aconiq run` children and
// every other writer of the project take — and the files are only renamed
// into place after it. Writing them before the lock would put a refused model
// on disk before the refusal: the check would hold the manifest and lose the
// model. The encoding and the writes still happen
// outside the lock, as staged files beside their destinations; the lock is
// held for a hash and three renames.
//
// The normalized file is renamed last. It is the file the hash describes, so a
// reader who sees the new hash finds the new dump and report already beside it.
func (s Store) SaveModelIf(
	proj *project.Project,
	model modelgeojson.Model,
	report modelgeojson.ValidationReport,
	expected string,
	extraRefs ...project.ArtifactRef,
) (string, error) {
	paths := s.ModelArtifactPaths()

	_, err := s.Load()
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(s.modelDir(), modelDirMode)
	if err != nil {
		return "", domainerrors.New(domainerrors.KindInternal, "projectfs.SaveModel", "create model directory", err)
	}

	staged, receipt, err := stageModelFiles(paths, model, report)

	defer func() {
		for _, file := range staged {
			file.Discard()
		}
	}()

	if err != nil {
		return "", err
	}

	unlock, err := s.LockManifest()
	if err != nil {
		return "", err
	}
	defer unlock()

	// Loaded before anything is committed, so a manifest that cannot be read
	// fails the save with the model files untouched.
	current, err := s.Load()
	if err != nil {
		return "", err
	}

	err = checkExpectedHash("model", paths.Normalized, expected)
	if err != nil {
		return "", err
	}

	for _, file := range staged {
		err = file.Commit()
		if err != nil {
			return "", domainerrors.New(domainerrors.KindInternal, "projectfs.SaveModel", "replace model file", err)
		}
	}

	now := time.Now().UTC()
	refs := []project.ArtifactRef{
		{ID: project.ArtifactIDModelNormalized, Kind: project.ArtifactKindModelNormalizedGeoJSON, Path: s.RelativePath(paths.Normalized), CreatedAt: now},
		{ID: project.ArtifactIDModelDump, Kind: project.ArtifactKindModelDumpJSON, Path: s.RelativePath(paths.Dump), CreatedAt: now},
		{ID: project.ArtifactIDModelValidation, Kind: project.ArtifactKindModelValidationReport, Path: s.RelativePath(paths.Validation), CreatedAt: now},
	}

	for _, ref := range extraRefs {
		if ref.CreatedAt.IsZero() {
			ref.CreatedAt = now
		}

		refs = append(refs, ref)
	}

	for _, ref := range refs {
		current.Artifacts = upsertArtifactRef(current.Artifacts, ref)
	}

	err = s.Save(current)
	if err != nil {
		return "", err
	}

	*proj = current

	return receipt, nil
}

// stageModelFiles writes the dump, the validation report and the normalized
// model beside their destinations, in the order SaveModelIf commits them, and
// returns the receipt of the normalized bytes. On failure it returns what it
// staged so far, for the caller to discard.
func stageModelFiles(
	paths ModelArtifactPaths,
	model modelgeojson.Model,
	report modelgeojson.ValidationReport,
) ([]*atomicfile.Staged, string, error) {
	staged := make([]*atomicfile.Staged, 0, 3)

	var receipt string

	for _, file := range []struct {
		path  string
		value any
	}{
		{paths.Dump, model.ToDump()},
		{paths.Validation, report},
		{paths.Normalized, model.ToFeatureCollection()},
	} {
		next, data, err := stageJSONFile(file.path, file.value)
		if err != nil {
			return staged, "", err
		}

		staged = append(staged, next)

		if file.path == paths.Normalized {
			sum := sha256.Sum256(data)
			receipt = hex.EncodeToString(sum[:])
		}
	}

	return staged, receipt, nil
}

// ExpectNone is the expected hash that holds only while there is no file at
// all: the precondition of a first save.
const ExpectNone = "none"

// HashMismatchError is a compare-and-write refused because the file on disk
// is not the one the caller expected. Nothing was written.
type HashMismatchError struct {
	// Subject names what was being replaced: "model" or "terrain".
	Subject string
	// Expected is the hash the caller sent.
	Expected string
	// Current is the hash on disk, or ExpectNone when there is no file.
	Current string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("the %s changed: expected %s, found %s", e.Subject, e.Expected, e.Current)
}

// checkExpectedHash holds a compare-and-write's precondition against the file
// at path. The caller holds LockManifest, so the answer stays true until its
// rename.
func checkExpectedHash(subject, path, expected string) error {
	if expected == "" {
		return nil
	}

	current, err := hashFile(path)
	if errors.Is(err, os.ErrNotExist) {
		current = ExpectNone
	} else if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.checkExpectedHash", "hash current "+subject, err)
	}

	if current == expected {
		return nil
	}

	return &HashMismatchError{Subject: subject, Expected: expected, Current: current}
}

// stageJSONFile is writeJSONFile without the rename: the bytes are written
// beside path and committed later, by the caller, under its lock. It returns
// the bytes too, so the caller can hash exactly what it will commit.
func stageJSONFile(path string, v any) (*atomicfile.Staged, []byte, error) {
	data, err := jsonio.Marshal(v)
	if err != nil {
		return nil, nil, domainerrors.New(domainerrors.KindInternal, "projectfs.writeJSONFile", "encode json", err)
	}

	staged, err := atomicfile.Stage(path, data)
	if err != nil {
		return nil, nil, domainerrors.New(domainerrors.KindInternal, "projectfs.writeJSONFile", "replace "+filepath.Base(path), err)
	}

	return staged, data, nil
}

func (s Store) modelDir() string {
	return filepath.Join(s.controlDir(), modelDirName)
}

// upsertArtifactRef drops any ref with the same ID and appends the new one, so
// the manifest lists artifacts in the order they were last written. This is the
// ordering `aconiq import` has always produced; the API inherits it rather than
// introducing a second one.
func upsertArtifactRef(artifacts []project.ArtifactRef, ref project.ArtifactRef) []project.ArtifactRef {
	out := make([]project.ArtifactRef, 0, len(artifacts)+1)

	for _, existing := range artifacts {
		if existing.ID != ref.ID {
			out = append(out, existing)
		}
	}

	return append(out, ref)
}

// KeepImportRecord puts the saved model's import record back on a model read
// from its normalized GeoJSON. That file carries the features only; the
// source, import CRS, transform and time of the import live in the dump beside
// it, and SaveModelIf rewrites the dump from the model it is given. Re-keying
// a receiver or adding a window is not an import, so the record must survive it. A project
// without a dump has no record to keep; one whose dump does not read is
// refused rather than overwritten.
func (s Store) KeepImportRecord(model *modelgeojson.Model) error {
	raw, err := os.ReadFile(s.ModelArtifactPaths().Dump)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.KeepImportRecord", "read model dump", err)
	}

	var dump modelgeojson.ModelDump

	err = json.Unmarshal(raw, &dump)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.KeepImportRecord", "decode model dump", err)
	}

	model.ImportCRS = dump.ImportCRS
	model.TransformApplied = dump.TransformApplied
	model.ImportedAt = dump.ImportedAt
	model.SourcePath = dump.SourcePath

	return nil
}
