package mcpserver_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/mcpserver"
)

// testGeoTIFF is a 2×2 float32 GeoTIFF, uncompressed, with a 10 m pixel
// scale and a tie point at (100, 200): the least the terrain loader accepts.
// extra trailing bytes change its hash and nothing it says.
func testGeoTIFF(t *testing.T, extra int) []byte {
	t.Helper()

	const (
		width, height = 2, 2
		pixelOffset   = 8
		numTags       = 9
	)

	order := binary.LittleEndian
	pixelDataSize := width * height * 4
	ifdOffset := pixelOffset + pixelDataSize
	scaleOffset := ifdOffset + 2 + numTags*12 + 4
	tiepointOffset := scaleOffset + 24

	buf := make([]byte, tiepointOffset+48+extra)
	copy(buf, "II")
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	for i, v := range []float32{10, 20, 30, 40} {
		order.PutUint32(buf[pixelOffset+i*4:], math.Float32bits(v))
	}

	pos := ifdOffset
	order.PutUint16(buf[pos:], numTags)
	pos += 2

	for _, tag := range [][4]uint32{
		{256, 3, 1, width},
		{257, 3, 1, height},
		{258, 3, 1, 32},
		{259, 3, 1, 1},
		{273, 4, 1, pixelOffset},
		{279, 4, 1, uint32(pixelDataSize)},
		{339, 3, 1, 3},
		{33550, 12, 3, uint32(scaleOffset)},
		{33922, 12, 6, uint32(tiepointOffset)},
	} {
		order.PutUint16(buf[pos:], uint16(tag[0]))
		order.PutUint16(buf[pos+2:], uint16(tag[1]))
		order.PutUint32(buf[pos+4:], tag[2])
		order.PutUint32(buf[pos+8:], tag[3])
		pos += 12
	}

	for i, v := range []float64{10, 10, 0} {
		order.PutUint64(buf[scaleOffset+i*8:], math.Float64bits(v))
	}

	for i, v := range []float64{0, 0, 0, 100, 200, 0} {
		order.PutUint64(buf[tiepointOffset+i*8:], math.Float64bits(v))
	}

	return buf
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// writeProjectFile writes data at rel under the project root.
func (f fixture) writeProjectFile(t *testing.T, rel string, data []byte) {
	t.Helper()

	path := filepath.Join(f.store.Root(), rel)

	err := os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// terrainHash is the hash project_status reports for the terrain, or "" when
// it reports none.
func (f fixture) terrainHash(t *testing.T) string {
	t.Helper()

	var status struct {
		Terrain *struct {
			Hash string `json:"hash"`
		} `json:"terrain"`
	}

	text := resultText(t, f.call(t, "project_status", map[string]any{}))

	err := json.Unmarshal([]byte(text), &status)
	if err != nil {
		t.Fatalf("project_status: %v: %s", err, text)
	}

	if status.Terrain == nil {
		return ""
	}

	return status.Terrain.Hash
}

func (f fixture) seedRunWithStatus(t *testing.T, status string) project.Run {
	t.Helper()

	run, _, err := f.store.CreateRun(projectfs.CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
		Status:   status,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return run
}

func (f fixture) runIDs(t *testing.T) []string {
	t.Helper()

	proj, err := f.store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	ids := make([]string, 0, len(proj.Runs))
	for _, run := range proj.Runs {
		ids = append(ids, run.ID)
	}

	return ids
}

// The destructive tools are listed only under --allow-writes, annotated
// destructive, as save_model is.
func TestDestructiveToolsFollowTheWriteGate(t *testing.T) {
	t.Parallel()

	readOnly := listTools(t, newFixture(t, false).session)
	writable := listTools(t, newFixture(t, true).session)

	for _, name := range []string{"delete_run", "import_terrain"} {
		if _, ok := readOnly[name]; ok {
			t.Fatalf("read-only server lists %s", name)
		}

		tool, ok := writable[name]
		if !ok {
			t.Fatalf("writable server does not list %s", name)
		}

		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint ||
			tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Fatalf("%s is not annotated destructive: %+v", name, tool.Annotations)
		}
	}
}

// delete_run deletes nothing unless confirm repeats the run id; each call is
// audited with the run it named.
func TestDeleteRunNeedsTheRunIDAsConfirm(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	run := f.seedRunWithStatus(t, project.RunStatusCompleted)
	other := f.seedRunWithStatus(t, project.RunStatusCompleted)

	for confirm, code := range map[string]string{"": "confirm_required", other.ID: "confirm_mismatch"} {
		apiErr := apiErrorOf(t, f.call(t, "delete_run", map[string]any{"run_id": run.ID, "confirm": confirm}))
		if apiErr["code"] != code {
			t.Fatalf("confirm %q: envelope = %v, want %s", confirm, apiErr, code)
		}
	}

	if ids := f.runIDs(t); len(ids) != 2 {
		t.Fatalf("a refused delete removed a run: %v", ids)
	}

	result := f.call(t, "delete_run", map[string]any{"run_id": run.ID, "confirm": run.ID})
	if result.IsError {
		t.Fatalf("delete_run: %s", resultText(t, result))
	}

	var deleted struct {
		RunID string `json:"run_id"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &deleted)
	if err != nil || deleted.RunID != run.ID {
		t.Fatalf("delete_run answered %s (%v)", resultText(t, result), err)
	}

	if ids := f.runIDs(t); len(ids) != 1 || ids[0] != other.ID {
		t.Fatalf("runs after delete: %v", ids)
	}

	lines := auditLines(t, f.store)
	if len(lines) != 3 {
		t.Fatalf("audit = %v", lines)
	}

	outcomes := map[string]bool{}

	for _, line := range lines {
		if line["tool"] != "delete_run" || line["run_id"] != run.ID {
			t.Fatalf("audit line = %v", line)
		}

		outcome, _ := line["outcome"].(string)
		outcomes[outcome] = true
	}

	for _, want := range []string{"confirm_required", "confirm_mismatch", "ok"} {
		if !outcomes[want] {
			t.Fatalf("audit outcomes %v lack %s", outcomes, want)
		}
	}
}

// What the API refuses reaches the agent in its words, and the audit line
// records the code.
func TestDeleteRunPassesTheAPIRefusalsThrough(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	running := f.seedRunWithStatus(t, project.RunStatusRunning)

	for runID, code := range map[string]string{running.ID: "run_not_finished", "run-missing": "not_found"} {
		apiErr := apiErrorOf(t, f.call(t, "delete_run", map[string]any{"run_id": runID, "confirm": runID}))
		if apiErr["code"] != code {
			t.Fatalf("%s: envelope = %v, want %s", runID, apiErr, code)
		}
	}

	if ids := f.runIDs(t); len(ids) != 1 {
		t.Fatalf("runs after refused deletes: %v", ids)
	}

	lines := auditLines(t, f.store)
	if len(lines) != 2 {
		t.Fatalf("audit = %v", lines)
	}

	for _, line := range lines {
		if line["outcome"] == "ok" {
			t.Fatalf("a refused delete was audited ok: %v", line)
		}
	}
}

// import_terrain reads a GeoTIFF from the project and replaces the terrain
// only when confirm names the terrain being replaced; the audit line records
// the terrain the import left.
func TestImportTerrainIsBoundToTheTerrainItReplaces(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	original := testGeoTIFF(t, 0)
	replacement := testGeoTIFF(t, 4)

	f.writeProjectFile(t, "dtm/original.tif", original)
	f.writeProjectFile(t, "dtm/replacement.tiff", replacement)

	apiErr := apiErrorOf(t, f.call(t, "import_terrain", map[string]any{"path": "dtm/original.tif", "confirm": ""}))
	if apiErr["code"] != "confirm_required" {
		t.Fatalf("empty confirm: envelope = %v", apiErr)
	}

	if hash := f.terrainHash(t); hash != "" {
		t.Fatalf("a refused import left terrain %s", hash)
	}

	result := f.call(t, "import_terrain", map[string]any{"path": "dtm/original.tif", "confirm": "none"})
	if result.IsError {
		t.Fatalf("first import: %s", resultText(t, result))
	}

	if hash := f.terrainHash(t); hash != sha256Hex(original) {
		t.Fatalf("terrain hash = %s, want %s", hash, sha256Hex(original))
	}

	// A terrain that changed since the agent read "none" is not overwritten.
	apiErr = apiErrorOf(t, f.call(t, "import_terrain", map[string]any{"path": "dtm/replacement.tiff", "confirm": "none"}))
	if apiErr["code"] != "hash_mismatch" {
		t.Fatalf("stale confirm: envelope = %v", apiErr)
	}

	result = f.call(t, "import_terrain", map[string]any{"path": "dtm/replacement.tiff", "confirm": sha256Hex(original)})
	if result.IsError {
		t.Fatalf("replace: %s", resultText(t, result))
	}

	if hash := f.terrainHash(t); hash != sha256Hex(replacement) {
		t.Fatalf("terrain hash after replace = %s", hash)
	}

	lines := auditLines(t, f.store)
	if len(lines) != 4 {
		t.Fatalf("audit = %v", lines)
	}

	want := []struct{ outcome, terrainHash string }{
		{"confirm_required", ""},
		{"ok", sha256Hex(original)},
		{"hash_mismatch", ""},
		{"ok", sha256Hex(replacement)},
	}

	for i, line := range lines {
		hash, _ := line["terrain_hash"].(string)
		if line["tool"] != "import_terrain" || line["outcome"] != want[i].outcome || hash != want[i].terrainHash {
			t.Fatalf("audit line %d = %v, want %+v", i, line, want[i])
		}
	}
}

// The path never leaves the project: an absolute path, a climb out of it and
// a symlink pointing out of it are refused before the API sees the call, and
// a missing file is not found.
func TestImportTerrainPathStaysInsideTheProject(t *testing.T) {
	t.Parallel()

	var apiCalls atomic.Int64

	f := newFixtureWrapped(t, true, func(api http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls.Add(1)
			api.ServeHTTP(w, r)
		})
	})

	outside := filepath.Join(t.TempDir(), "outside.tif")

	err := os.WriteFile(outside, testGeoTIFF(t, 0), 0o600)
	if err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	err = os.Symlink(outside, filepath.Join(f.store.Root(), "link.tif"))
	if err != nil {
		t.Fatalf("symlink: %v", err)
	}

	cases := map[string]string{
		outside:                      "invalid_path",
		"../outside.tif":             "invalid_path",
		"dtm/../../outside.tif":      "invalid_path",
		".noise/../../x/outside.tif": "invalid_path",
		"link.tif":                   "invalid_path",
		"dtm/missing.tif":            "not_found",
	}

	for path, code := range cases {
		apiErr := apiErrorOf(t, f.call(t, "import_terrain", map[string]any{"path": path, "confirm": "none"}))
		if apiErr["code"] != code {
			t.Fatalf("path %q: envelope = %v, want %s", path, apiErr, code)
		}
	}

	if n := apiCalls.Load(); n != 0 {
		t.Fatalf("refused paths reached the API %d times", n)
	}

	lines := auditLines(t, f.store)
	if len(lines) != len(cases) {
		t.Fatalf("%d refused imports left %d audit lines: %v", len(cases), len(lines), lines)
	}

	for _, line := range lines {
		if line["tool"] != "import_terrain" || line["outcome"] == "ok" {
			t.Fatalf("audit line = %v", line)
		}
	}
}

// What a file is stays the API's to judge: a name that is not a GeoTIFF's,
// bytes that are not one, and a file over the upload cap are refused in the
// API's words.
func TestImportTerrainLeavesTheFileToTheAPI(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	f.writeProjectFile(t, "dtm.png", testGeoTIFF(t, 0))
	f.writeProjectFile(t, "garbage.tif", []byte("not a tiff"))

	// Sparse, so the test does not write 50 MB.
	large, err := os.Create(filepath.Join(f.store.Root(), "large.tif"))
	if err != nil {
		t.Fatalf("create large file: %v", err)
	}

	err = large.Truncate(50<<20 + 1)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	_ = large.Close()

	for path, code := range map[string]string{
		"dtm.png":     "bad_request",
		"garbage.tif": "bad_request",
		"large.tif":   "request_too_large",
	} {
		result := f.call(t, "import_terrain", map[string]any{"path": path, "confirm": "none"})
		if apiErr := apiErrorOf(t, result); apiErr["code"] != code {
			t.Fatalf("%s: envelope = %v, want %s", path, apiErr, code)
		}
	}

	if hash := f.terrainHash(t); hash != "" {
		t.Fatalf("a refused import left terrain %s", hash)
	}
}

// No run is deleted and no terrain replaced unaudited.
func TestDestructiveToolsNeedTheAuditLog(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	run := f.seedRunWithStatus(t, project.RunStatusCompleted)
	f.writeProjectFile(t, "dtm.tif", testGeoTIFF(t, 0))

	err := os.MkdirAll(f.store.MCPAuditLogPath(), 0o750)
	if err != nil {
		t.Fatalf("block audit log: %v", err)
	}

	for tool, args := range map[string]map[string]any{
		"delete_run":     {"run_id": run.ID, "confirm": run.ID},
		"import_terrain": {"path": "dtm.tif", "confirm": "none"},
	} {
		if apiErr := apiErrorOf(t, f.call(t, tool, args)); apiErr["code"] != "audit_unavailable" {
			t.Fatalf("%s: envelope = %v", tool, apiErr)
		}
	}

	if ids := f.runIDs(t); len(ids) != 1 {
		t.Fatalf("runs after refused delete: %v", ids)
	}

	_, err = os.Stat(f.store.TerrainPath())
	if !os.IsNotExist(err) {
		t.Fatalf("a refused import wrote the terrain (%v)", err)
	}
}

// A writable server reads terrain files from the project it serves, so it
// has to be told where that is.
func TestWritesNeedTheProjectRoot(t *testing.T) {
	t.Parallel()

	_, err := mcpserver.New(http.NotFoundHandler(), mcpserver.Options{
		AllowWrites: true,
		AuditPath:   filepath.Join(t.TempDir(), "audit.jsonl"),
	})
	if err == nil || !strings.Contains(err.Error(), "project root") {
		t.Fatalf("New without a project root: %v", err)
	}
}
