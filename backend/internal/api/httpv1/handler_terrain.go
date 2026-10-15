package httpv1

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

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

	// Checked before the upload is read: a malformed precondition is refused
	// for what it is, not after a 50 MB read it could never apply to.
	expected := r.FormValue(expectedHashField)
	if !validExpectedHash(expected) {
		writeInvalidExpectedHash(w)
		return
	}

	data, ok := readTerrainUpload(w, r)
	if !ok {
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

	err = h.storeTerrainArtifact(data, expected)
	if writeHashMismatch(w, err) {
		return
	}

	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, model.Info())
}

// readTerrainUpload reads the multipart `file` part of a parsed form, refusing
// a missing part, a name that is not a GeoTIFF's and a payload over the cap.
// It answers the request itself when it refuses, and reports whether it did
// not.
func readTerrainUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "missing 'file' field in multipart form",
		})

		return nil, false
	}

	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".tif" && ext != ".tiff" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "file must have .tif or .tiff extension",
		})

		return nil, false
	}

	if header.Size > maxTerrainUploadBytes {
		writeTooLarge(w, maxTerrainUploadBytes)
		return nil, false
	}

	// The handler's body cap already bounds the part, but a part is not the
	// body: bounding the read itself keeps the guarantee local to the
	// allocation it protects rather than to a MaxBytesReader in the caller.
	data, err := io.ReadAll(io.LimitReader(file, maxTerrainUploadBytes+1))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "failed to read uploaded file: " + err.Error(),
		})

		return nil, false
	}

	if len(data) > maxTerrainUploadBytes {
		writeTooLarge(w, maxTerrainUploadBytes)
		return nil, false
	}

	return data, true
}

// storeTerrainArtifact persists a validated DTM through the store's one terrain
// write, which `aconiq import --terrain` shares. The in-process lock is the
// handler's half; the store takes the cross-process one, under which it
// checks expected and renames.
func (h Handler) storeTerrainArtifact(data []byte, expected string) error {
	defer h.lockManifest()()

	proj := project.Project{}

	err := h.store.SaveTerrain(&proj, data, expected)
	if err != nil {
		return fmt.Errorf("store terrain: %w", err)
	}

	return nil
}
