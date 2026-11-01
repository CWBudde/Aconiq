package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/api/httpv1"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// deleteRunInput names the run twice: once to delete it, and once as
// confirm, the value the agent read from list_runs and repeats on purpose.
type deleteRunInput struct {
	RunID   string `json:"run_id"  jsonschema:"The run's id, as list_runs returned it."`
	Confirm string `json:"confirm" jsonschema:"The run id again. A delete is refused unless confirm repeats run_id."`
}

// importTerrainInput names a GeoTIFF inside the project. The file's bytes
// never travel in the arguments: a DTM runs to tens of megabytes.
type importTerrainInput struct {
	Path    string `json:"path"    jsonschema:"The GeoTIFF (.tif or .tiff) to import, relative to the project root; it cannot leave the project. Its elevations must be in the project CRS."`
	Confirm string `json:"confirm" jsonschema:"The hash of the terrain being replaced, as project_status returned it, or none when the project has no terrain yet."`
}

// addDestructiveTools registers delete_run and import_terrain, with the
// low-level AddTool for the reason save_model uses it: a call whose arguments
// the schema refuses is still audited.
func (t tools) addDestructiveTools(server *mcp.Server) error {
	deleteSchema, deleteResolved, err := inputSchema[deleteRunInput](nil, "run_id")
	if err != nil {
		return err
	}

	terrainSchema, terrainResolved, err := inputSchema[importTerrainInput](nil, "path")
	if err != nil {
		return err
	}

	destructive := &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)}

	server.AddTool(&mcp.Tool{
		Name: toolDeleteRun,
		Description: "Delete a finished run: its row, its artifact references and its .noise/runs/<id>/ directory. Export bundles are kept. " +
			"confirm must repeat run_id. A run still being written is refused with run_not_finished, and one holding export bundles in its own directory with export_inside_run.",
		InputSchema: deleteSchema,
		Annotations: destructive,
	}, t.audited(toolDeleteRun, func(ctx context.Context, arguments json.RawMessage) served {
		return t.serveDeleteRun(ctx, deleteResolved, arguments)
	}))

	server.AddTool(&mcp.Tool{
		Name: toolImportTerrain,
		Description: "Replace the project's terrain (DTM) with a GeoTIFF that is already inside the project, named by its path relative to the project root. " +
			"confirm binds the import to the terrain you read: the terrain hash project_status returned, or \"none\" when it reports no terrain. " +
			"A terrain that changed since is refused with hash_mismatch, not overwritten. Whether the file is a GeoTIFF the loader accepts, and within the 50 MB upload cap, is the API's to judge.",
		InputSchema: terrainSchema,
		Annotations: destructive,
	}, t.audited(toolImportTerrain, func(ctx context.Context, arguments json.RawMessage) served {
		return t.serveImportTerrain(ctx, terrainResolved, arguments)
	}))

	return nil
}

func (t tools) serveDeleteRun(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	in, err := decodeArguments[deleteRunInput](schema, arguments)
	if err != nil {
		return invalidArguments(toolDeleteRun, err, "Pass run_id, the id list_runs returned, and confirm, the same id again.")
	}

	touched := affected{runID: in.RunID}

	switch in.Confirm {
	case "":
		return refused(errorCodeConfirmRequired, "delete_run removes a run and its results, and needs confirm",
			"Pass the run id again as confirm.", touched)
	case in.RunID:
	default:
		return refused(errorCodeConfirmMismatch, fmt.Sprintf("confirm %q does not name run %q", in.Confirm, in.RunID),
			"confirm must repeat run_id exactly; nothing was deleted.", touched)
	}

	segment, ok := pathSegment(in.RunID)
	if !ok {
		return served{result: runNotFound(in.RunID).result(), outcome: errorCodeNotFound, touched: touched}
	}

	resp, err := t.api.delete(ctx, "/api/v1/runs/"+segment)
	if err != nil {
		return internalError(err)
	}

	outcome := auditOutcomeOK
	if !resp.ok() {
		outcome = errorCode(resp)
	}

	return served{result: resp.toolResult(), outcome: outcome, touched: touched}
}

func (t tools) serveImportTerrain(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	in, err := decodeArguments[importTerrainInput](schema, arguments)
	if err != nil {
		return invalidArguments(toolImportTerrain, err, "Pass path, a GeoTIFF relative to the project root, and confirm; nothing else.")
	}

	// Checked before the file is opened: an unconditional replace is refused
	// for what it is, whatever the path names.
	if in.Confirm == "" {
		return refused(errorCodeConfirmRequired, "import_terrain replaces the project's terrain and needs confirm",
			"Pass the terrain hash project_status returned, or \"none\" if it reports no terrain.", affected{})
	}

	file, refusal := openProjectFile(t.projectRoot, in.Path)
	if refusal != nil {
		return served{result: refusal.result(), outcome: refusal.Code}
	}

	defer func() { _ = file.Close() }()

	digest := sha256.New()

	resp, err := t.uploadTerrain(ctx, file, filepath.Base(in.Path), in.Confirm, digest)
	if err != nil {
		return internalError(err)
	}

	if !resp.ok() {
		return served{result: resp.toolResult(), outcome: errorCode(resp)}
	}

	// The API accepted the whole file, so the bytes digested are the bytes it
	// stored: the hash project_status now reports.
	return served{
		result:  resp.toolResult(),
		outcome: auditOutcomeOK,
		touched: affected{terrainHash: hex.EncodeToString(digest.Sum(nil))},
	}
}

// refused is a refusal this layer makes before any request, audited under
// its own code.
func refused(code, message, hint string, touched affected) served {
	return served{result: toolError{Code: code, Message: message, Hint: hint}.result(), outcome: code, touched: touched}
}

// openProjectFile opens rel inside root and nowhere else. A path that is not
// local — absolute, or climbing out with .. — is refused before anything is
// opened; os.Root then refuses a symlink that leads out of the project.
func openProjectFile(root, rel string) (*os.File, *toolError) {
	invalid := func(reason string) *toolError {
		return &toolError{
			Code:    errorCodeInvalidPath,
			Message: fmt.Sprintf("path %q %s", rel, reason),
			Hint:    "Name a file inside the project by its path relative to the project root, such as dtm/terrain.tif.",
		}
	}

	if !filepath.IsLocal(rel) {
		return nil, invalid("is not a path inside the project")
	}

	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, &toolError{Code: errorCodeInternal, Message: "open project root: " + err.Error()}
	}

	defer func() { _ = dir.Close() }()

	file, err := dir.Open(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &toolError{
			Code:    errorCodeNotFound,
			Message: fmt.Sprintf("no file at %q in the project", rel),
			Hint:    "The path is relative to the project root.",
		}
	}

	if err != nil {
		return nil, invalid("cannot be opened inside the project: " + err.Error())
	}

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()

		return nil, invalid("is not a regular file")
	}

	return file, nil
}

// uploadTerrain streams the file into the API's multipart terrain upload,
// with confirm as expected_hash, and digests what it sends. It reads at most
// one byte past the API's cap: that is enough for the API to refuse an
// oversized file, and no file is read whole on the way to a refusal.
func (t tools) uploadTerrain(ctx context.Context, file io.Reader, name, expected string, digest hash.Hash) (apiResponse, error) {
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	written := make(chan error, 1)

	go func() {
		err := writeTerrainForm(form, io.TeeReader(io.LimitReader(file, httpv1.MaxTerrainUploadBytes+1), digest), name, expected)
		_ = writer.CloseWithError(err)

		written <- err
	}()

	resp, err := t.api.postStream(ctx, "/api/v1/import/terrain", form.FormDataContentType(), reader)

	// The handler may stop reading early, on a refusal; closing the reader
	// releases the writer either way.
	_ = reader.Close()
	writeErr := <-written

	if err != nil {
		return apiResponse{}, err
	}

	// A write that failed for any reason but the handler hanging up has sent
	// a truncated upload; an answer of success to it would be wrong.
	if writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) && resp.ok() {
		return apiResponse{}, fmt.Errorf("stream terrain upload: %w", writeErr)
	}

	return resp, nil
}

func writeTerrainForm(form *multipart.Writer, data io.Reader, name, expected string) error {
	err := form.WriteField("expected_hash", expected)
	if err != nil {
		return fmt.Errorf("write expected_hash: %w", err)
	}

	part, err := form.CreateFormFile("file", name)
	if err != nil {
		return fmt.Errorf("create file part: %w", err)
	}

	_, err = io.Copy(part, data)
	if err != nil {
		return fmt.Errorf("copy terrain file: %w", err)
	}

	err = form.Close()
	if err != nil {
		return fmt.Errorf("close multipart form: %w", err)
	}

	return nil
}
