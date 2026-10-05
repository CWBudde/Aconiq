package httpv1

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aconiq/backend/internal/atomicfile"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/terrain"
)

func (h Handler) handleImportTerrain(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	if !requireContentType(w, r, mediaTypeMultipart) {
		return
	}

	// ParseMultipartForm's argument bounds only what is buffered in memory; the
	// remainder spills to temp files, so the request body itself is unbounded
	// without MaxBytesReader.
	r.Body = http.MaxBytesReader(w, r.Body, maxTerrainUploadBytes)

	// G120 does not model MaxBytesReader, so it reports this call whatever the
	// argument is. The body above it is bounded, the in-memory share is
	// maxTerrainMemoryBytes, and the spill-to-disk remainder cannot outlive the
	// capped body — which is what the rule is actually asking for.
	//nolint:gosec // G120: bounded by the MaxBytesReader on the line above
	err := r.ParseMultipartForm(maxTerrainMemoryBytes)
	if err != nil {
		if writeRequestTooLarge(w, err, maxTerrainUploadBytes) {
			return
		}

		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "failed to parse multipart form: " + err.Error(),
		})

		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "missing 'file' field in multipart form",
		})

		return
	}

	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".tif" && ext != ".tiff" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "file must have .tif or .tiff extension",
		})

		return
	}

	if header.Size > maxTerrainUploadBytes {
		writeTooLarge(w, maxTerrainUploadBytes)
		return
	}

	// The body cap above already bounds the part, but a part is not the body:
	// bounding the read itself keeps the guarantee local to the allocation it
	// protects rather than to a MaxBytesReader three statements away.
	data, err := io.ReadAll(io.LimitReader(file, maxTerrainUploadBytes+1))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "failed to read uploaded file: " + err.Error(),
		})

		return
	}

	if len(data) > maxTerrainUploadBytes {
		writeTooLarge(w, maxTerrainUploadBytes)
		return
	}

	model, err := terrain.LoadFromBytes(data)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "invalid GeoTIFF terrain file: " + err.Error(),
		})

		return
	}

	err = h.storeTerrainArtifact(data)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    errorCodeInternalError,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, http.StatusCreated, model.Info())
}

func (h Handler) storeTerrainArtifact(data []byte) error {
	terrainDir := filepath.Join(h.store.Root(), ".noise", "model")

	err := os.MkdirAll(terrainDir, 0o750)
	if err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	// Replaced, not rewritten: an `aconiq run` subprocess may be reading the
	// previous DTM, and it takes no lock of ours. The rename is also why the
	// write may happen outside the manifest lock - that lock serialises the
	// manifest's read-modify-write, and the artifact row names the same fixed
	// path whichever DTM is behind it.
	err = atomicfile.WriteFile(filepath.Join(terrainDir, "terrain.tif"), data)
	if err != nil {
		return fmt.Errorf("failed to write terrain file: %w", err)
	}

	defer h.lockManifest()()

	proj, err := h.store.Load()
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	proj.Artifacts = updateOrAppendArtifact(proj.Artifacts, project.ArtifactRef{
		ID:        "artifact-terrain",
		Kind:      "model.terrain_geotiff",
		Path:      ".noise/model/terrain.tif",
		CreatedAt: h.now(),
	})

	err = h.store.Save(proj)
	if err != nil {
		return fmt.Errorf("failed to save project manifest: %w", err)
	}

	return nil
}

func updateOrAppendArtifact(artifacts []project.ArtifactRef, ref project.ArtifactRef) []project.ArtifactRef {
	for i, a := range artifacts {
		if a.ID == ref.ID {
			artifacts[i] = ref
			return artifacts
		}
	}

	return append(artifacts, ref)
}
