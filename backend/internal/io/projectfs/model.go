package projectfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
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
func (s Store) SaveModel(proj *project.Project, model modelgeojson.Model, report modelgeojson.ValidationReport) error {
	paths := s.ModelArtifactPaths()

	err := os.MkdirAll(s.modelDir(), modelDirMode)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.SaveModel", "create model directory", err)
	}

	for _, file := range []struct {
		path  string
		value any
	}{
		{paths.Normalized, model.ToFeatureCollection()},
		{paths.Dump, model.ToDump()},
		{paths.Validation, report},
	} {
		err = writeJSONFile(file.path, file.value)
		if err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	for _, ref := range []project.ArtifactRef{
		{ID: project.ArtifactIDModelNormalized, Kind: project.ArtifactKindModelNormalizedGeoJSON, Path: s.RelativePath(paths.Normalized), CreatedAt: now},
		{ID: project.ArtifactIDModelDump, Kind: project.ArtifactKindModelDumpJSON, Path: s.RelativePath(paths.Dump), CreatedAt: now},
		{ID: project.ArtifactIDModelValidation, Kind: project.ArtifactKindModelValidationReport, Path: s.RelativePath(paths.Validation), CreatedAt: now},
	} {
		proj.Artifacts = upsertArtifactRef(proj.Artifacts, ref)
	}

	return s.Save(*proj)
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
