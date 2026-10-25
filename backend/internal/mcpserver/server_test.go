package mcpserver_test

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/api/httpv1"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/mcpserver"
	"github.com/aconiq/backend/internal/standards"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// validModel is a source and a receiver, enough for the v1 input schema.
const validModel = `{
  "type": "FeatureCollection",
  "features": [
    {"type": "Feature", "id": "src-1", "properties": {"kind": "source", "source_type": "point"},
     "geometry": {"type": "Point", "coordinates": [500000, 5700000]}},
    {"type": "Feature", "id": "rcv-1", "properties": {"kind": "receiver", "height_m": 4},
     "geometry": {"type": "Point", "coordinates": [500050, 5700050]}}
  ]
}`

// invalidModel breaks the schema: a receiver needs height_m > 0.
const invalidModel = `{
  "type": "FeatureCollection",
  "features": [
    {"type": "Feature", "id": "rcv-bad", "properties": {"kind": "receiver", "height_m": 0},
     "geometry": {"type": "Point", "coordinates": [500050, 5700050]}}
  ]
}`

var fixedNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

type fixture struct {
	store   projectfs.Store
	handler http.Handler
	session *mcp.ClientSession
}

// newFixture starts the server over an in-memory transport against the handler
// `aconiq serve` mounts, on a fresh project, and connects a client to it.
func newFixture(t *testing.T, allowWrites bool) fixture {
	t.Helper()

	return newFixtureWrapped(t, allowWrites, nil)
}

// testPhotoTransmission is the decision a writable fixture is started
// under, so it offers propose_windows as a server started with it does.
const testPhotoTransmission = "engagement-test-decision-1"

// newFixtureWrapped is newFixture with the server's handler wrapped by wrap,
// for a test that must answer one route itself; nil wraps nothing. A
// writable fixture is started with a photo transmission decision.
func newFixtureWrapped(t *testing.T, allowWrites bool, wrap func(http.Handler) http.Handler) fixture {
	t.Helper()

	photoTransmission := ""
	if allowWrites {
		photoTransmission = testPhotoTransmission
	}

	return newFixtureWith(t, mcpserver.Options{AllowWrites: allowWrites, PhotoTransmission: photoTransmission}, wrap)
}

// newFixtureWith starts a server with opts, its version, audit path and
// clock filled in.
func newFixtureWith(t *testing.T, opts mcpserver.Options, wrap func(http.Handler) http.Handler) fixture {
	t.Helper()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("MCP", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	handler := httpv1.NewServeHandler(store, func() time.Time { return fixedNow }, registry, httpv1.ServeOptions{})

	served := http.Handler(handler)
	if wrap != nil {
		served = wrap(handler)
	}

	opts.Version = "test"
	opts.AuditPath = store.MCPAuditLogPath()
	opts.Clock = func() time.Time { return fixedNow }

	server, err := mcpserver.New(served, opts)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() { _ = session.Close() })

	return fixture{store: store, handler: handler, session: session}
}

func (f fixture) call(t *testing.T, name string, args any) *mcp.CallToolResult {
	t.Helper()

	result, err := f.session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}

	return result
}

func saveArgs(model string, confirm string) map[string]any {
	return map[string]any{"model": json.RawMessage(model), "confirm": confirm}
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("first content is %T, want text", result.Content[0])
	}

	return text.Text
}

// apiErrorOf decodes the API error envelope a failed tool call carries.
func apiErrorOf(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	if !result.IsError {
		t.Fatalf("expected a tool error, got %s", resultText(t, result))
	}

	var envelope struct {
		Error map[string]any `json:"error"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &envelope)
	if err != nil || envelope.Error == nil {
		t.Fatalf("tool error is not the API envelope: %q (%v)", resultText(t, result), err)
	}

	return envelope.Error
}

func (f fixture) modelHash(t *testing.T) string {
	t.Helper()

	result := f.call(t, "get_model", map[string]any{})
	if result.IsError {
		t.Fatalf("get_model: %s", resultText(t, result))
	}

	var body struct {
		Hash string `json:"hash"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &body)
	if err != nil || body.Hash == "" {
		t.Fatalf("get_model answered no hash: %s (%v)", resultText(t, result), err)
	}

	return body.Hash
}

// modelFiles snapshots what a refused save must leave alone.
func (f fixture) modelFiles(t *testing.T) map[string][]byte {
	t.Helper()

	files := map[string][]byte{}

	for _, path := range []string{f.store.ManifestPath(), f.store.ModelArtifactPaths().Normalized} {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("read %s: %v", path, err)
		}

		files[path] = data
	}

	return files
}

func (f fixture) assertUnchanged(t *testing.T, before map[string][]byte) {
	t.Helper()

	for path, data := range f.modelFiles(t) {
		if !bytes.Equal(data, before[path]) {
			t.Fatalf("a refused save changed %s", filepath.Base(path))
		}
	}
}

func auditLines(t *testing.T, store projectfs.Store) []map[string]any {
	t.Helper()

	data, err := os.ReadFile(store.MCPAuditLogPath())
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}

	var lines []map[string]any

	for line := range strings.Lines(string(data)) {
		var entry map[string]any

		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Fatalf("audit line is not JSON: %q (%v)", line, err)
		}

		lines = append(lines, entry)
	}

	return lines
}

func listTools(t *testing.T, session *mcp.ClientSession) map[string]*mcp.Tool {
	t.Helper()

	listed, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	tools := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		tools[tool.Name] = tool
	}

	return tools
}

// A read-only server lists no tool it would refuse, so an agent cannot plan
// around one; the write class appears only under --allow-writes.
func TestToolsListedFollowTheWriteGate(t *testing.T) {
	t.Parallel()

	readOnly := listTools(t, newFixture(t, false).session)

	if names := slices.Sorted(maps.Keys(readOnly)); !slices.Equal(names, []string{
		"estimate_run", "get_contours", "get_model", "get_run", "get_run_log", "get_run_results",
		"list_runs", "list_standards", "project_status", "transform",
	}) {
		t.Fatalf("read-only server lists %v", names)
	}

	for name, tool := range readOnly {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s is not annotated read-only", name)
		}
	}

	writable := listTools(t, newFixture(t, true).session)

	if names := slices.Sorted(maps.Keys(writable)); !slices.Equal(names, []string{
		"cancel_run", "estimate_run", "get_contours", "get_model", "get_run", "get_run_log", "get_run_results",
		"list_runs", "list_standards", "project_status", "propose_windows", "save_model", "start_run", "transform",
	}) {
		t.Fatalf("writable server lists %v", names)
	}

	save := writable["save_model"]
	if save.Annotations == nil || save.Annotations.ReadOnlyHint ||
		save.Annotations.DestructiveHint == nil || !*save.Annotations.DestructiveHint {
		t.Fatalf("save_model is not annotated destructive: %+v", save.Annotations)
	}
}

func TestReadOnlyServerRefusesSaveModel(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	before := f.modelFiles(t)

	result, err := f.session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "save_model",
		Arguments: saveArgs(validModel, "none"),
	})
	if err == nil && !result.IsError {
		t.Fatal("a read-only server saved the model")
	}

	f.assertUnchanged(t, before)
}

// The tool answers with the API's own bytes, not a re-encoding of them.
func TestProjectStatusIsTheAPIResponseVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	result := f.call(t, "project_status", map[string]any{})
	if result.IsError {
		t.Fatalf("project_status: %s", resultText(t, result))
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/project/status", nil)
	req.Host = "127.0.0.1"

	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	want, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}

	if rec.Code != http.StatusOK || resultText(t, result) != string(want) {
		t.Fatalf("project_status differs from the API (%d):\n got %s\nwant %s", rec.Code, resultText(t, result), want)
	}
}

// An API refusal reaches the agent as the envelope the API wrote, code and
// hint included, rather than a summary of it.
func TestToolErrorCarriesTheAPIEnvelope(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	apiErr := apiErrorOf(t, f.call(t, "get_model", map[string]any{}))
	if apiErr["code"] != "model_not_found" || apiErr["hint"] == nil {
		t.Fatalf("envelope = %v", apiErr)
	}
}

func TestGetModelAnswersInTheRequestedCRS(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	result := f.call(t, "save_model", saveArgs(validModel, "none"))
	if result.IsError {
		t.Fatalf("seed save: %s", resultText(t, result))
	}

	result = f.call(t, "get_model", map[string]any{"crs": "EPSG:4326"})
	if result.IsError {
		t.Fatalf("get_model: %s", resultText(t, result))
	}

	var body struct {
		CRS        string `json:"crs"`
		ProjectCRS string `json:"project_crs"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &body)
	if err != nil || body.CRS != "EPSG:4326" || body.ProjectCRS != "EPSG:25832" {
		t.Fatalf("get_model crs = %+v (%v)", body, err)
	}
}

func TestSaveModelRefusesAnEmptyConfirmAndWritesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	before := f.modelFiles(t)

	apiErr := apiErrorOf(t, f.call(t, "save_model", saveArgs(validModel, "")))
	if apiErr["code"] != "confirm_required" || apiErr["hint"] == nil {
		t.Fatalf("envelope = %v", apiErr)
	}

	f.assertUnchanged(t, before)

	lines := auditLines(t, f.store)
	if len(lines) != 1 || lines[0]["outcome"] != "confirm_required" {
		t.Fatalf("audit = %v", lines)
	}
}

// The schema requires confirm, so an agent sees it is not optional, and types
// the model as an object rather than the bytes the Go field holds.
func TestSaveModelSchemaRequiresConfirmAndModel(t *testing.T) {
	t.Parallel()

	save := listTools(t, newFixture(t, true).session)["save_model"]

	schema, err := json.Marshal(save.InputSchema)
	if err != nil {
		t.Fatalf("encode schema: %v", err)
	}

	var decoded struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}

	err = json.Unmarshal(schema, &decoded)
	if err != nil {
		t.Fatalf("decode schema: %v", err)
	}

	slices.Sort(decoded.Required)

	if !slices.Equal(decoded.Required, []string{"confirm", "model"}) {
		t.Fatalf("required = %v", decoded.Required)
	}

	if !strings.Contains(string(decoded.Properties["model"]), `"object"`) {
		t.Fatalf("model is not an object in the schema: %s", decoded.Properties["model"])
	}
}

// The whole compare-and-write cycle as an agent runs it: "none" on a fresh
// project, a stale confirm refused with the current hash, and the hash
// get_model returned accepted.
func TestSaveModelBindsConfirmToTheModelBeingReplaced(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	result := f.call(t, "save_model", saveArgs(validModel, "none"))
	if result.IsError {
		t.Fatalf("save with none: %s", resultText(t, result))
	}

	first := f.modelHash(t)
	before := f.modelFiles(t)

	apiErr := apiErrorOf(t, f.call(t, "save_model", saveArgs(validModel, strings.Repeat("0", 64))))
	details, _ := apiErr["details"].(map[string]any)

	if apiErr["code"] != "hash_mismatch" || details["current_hash"] != first {
		t.Fatalf("stale confirm: envelope = %v", apiErr)
	}

	apiErr = apiErrorOf(t, f.call(t, "save_model", saveArgs(validModel, "none")))
	if apiErr["code"] != "hash_mismatch" {
		t.Fatalf("none against an existing model: envelope = %v", apiErr)
	}

	f.assertUnchanged(t, before)

	result = f.call(t, "save_model", saveArgs(validModel, first))
	if result.IsError {
		t.Fatalf("save with the current hash: %s", resultText(t, result))
	}

	lines := auditLines(t, f.store)

	outcomes := make([]any, 0, len(lines))
	for _, line := range lines {
		outcomes = append(outcomes, line["outcome"])
	}

	if !slices.Equal(outcomes, []any{"ok", "hash_mismatch", "hash_mismatch", "ok"}) {
		t.Fatalf("audit outcomes = %v", outcomes)
	}

	if lines[0]["model_hash"] != first {
		t.Fatalf("audit model_hash = %v, want %s", lines[0]["model_hash"], first)
	}

	if _, ok := lines[1]["model_hash"]; ok {
		t.Fatalf("a refused save recorded a model hash: %v", lines[1])
	}
}

// Validation issues are returned as the API reported them, and nothing is
// coerced into validity.
func TestSaveModelReturnsValidationIssuesAndLeavesTheModel(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	result := f.call(t, "save_model", saveArgs(validModel, "none"))
	if result.IsError {
		t.Fatalf("seed save: %s", resultText(t, result))
	}

	before := f.modelFiles(t)

	result = f.call(t, "save_model", saveArgs(invalidModel, f.modelHash(t)))
	apiErr := apiErrorOf(t, result)

	if apiErr["code"] == "" || !strings.Contains(resultText(t, result), "rcv-bad") {
		t.Fatalf("validation error does not name the feature: %s", resultText(t, result))
	}

	f.assertUnchanged(t, before)
}

// The audit log records that a write happened and how it ended, never what
// was written: model payloads can carry customer geometry.
func TestAuditLogHoldsDigestsNotPayloads(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	f.call(t, "project_status", map[string]any{})
	f.call(t, "get_model", map[string]any{})

	if lines := auditLines(t, f.store); len(lines) != 0 {
		t.Fatalf("read tools were audited: %v", lines)
	}

	f.call(t, "save_model", saveArgs(validModel, "none"))

	raw, err := os.ReadFile(f.store.MCPAuditLogPath())
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}

	if strings.Contains(string(raw), "5700050") || strings.Contains(string(raw), "rcv-1") {
		t.Fatalf("audit log carries the payload: %s", raw)
	}

	lines := auditLines(t, f.store)
	if len(lines) != 1 {
		t.Fatalf("audit = %v", lines)
	}

	entry := lines[0]

	if entry["time"] != "2026-10-20T12:00:00Z" || entry["tool"] != "save_model" {
		t.Fatalf("audit entry = %v", entry)
	}

	if digest, _ := entry["args_sha256"].(string); len(digest) != 64 {
		t.Fatalf("args_sha256 = %v", entry["args_sha256"])
	}
}

// The digest is over the canonical arguments, so the same call spelled with
// other whitespace or key order audits the same.
func TestAuditDigestIgnoresArgumentSpelling(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	f.call(t, "save_model", saveArgs(validModel, ""))
	f.call(t, "save_model", map[string]any{"confirm": "", "model": json.RawMessage(strings.Join(strings.Fields(validModel), ""))})

	lines := auditLines(t, f.store)
	if len(lines) != 2 || lines[0]["args_sha256"] != lines[1]["args_sha256"] {
		t.Fatalf("audit digests differ: %v", lines)
	}
}

// A call the input schema refuses is still a destructive attempt: it is
// refused in the API's envelope and audited like any other, not dropped
// before the audit log sees it.
func TestSaveModelAuditsArgumentsTheSchemaRefuses(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	before := f.modelFiles(t)

	for _, args := range []map[string]any{
		{"model": json.RawMessage(validModel)},
		{"model": "not an object", "confirm": "none"},
		{"model": json.RawMessage(validModel), "confirm": "none", "force": true},
	} {
		apiErr := apiErrorOf(t, f.call(t, "save_model", args))
		if apiErr["code"] != "invalid_arguments" {
			t.Fatalf("arguments %v: envelope = %v", args, apiErr)
		}
	}

	f.assertUnchanged(t, before)

	lines := auditLines(t, f.store)
	if len(lines) != 3 {
		t.Fatalf("audit = %v", lines)
	}

	for _, line := range lines {
		if line["outcome"] != "invalid_arguments" || line["tool"] != "save_model" {
			t.Fatalf("audit entry = %v", line)
		}
	}
}

// The audit log is opened before the save is attempted, so a project whose
// log cannot be written refuses the write instead of making it unaudited.
func TestSaveModelRefusesWhenTheAuditLogCannotBeOpened(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	logs := filepath.Dir(f.store.MCPAuditLogPath())

	err := os.RemoveAll(logs)
	if err != nil {
		t.Fatalf("remove logs: %v", err)
	}

	err = os.WriteFile(logs, []byte("a file where the log directory belongs"), 0o600)
	if err != nil {
		t.Fatalf("block logs: %v", err)
	}

	before := f.modelFiles(t)

	apiErr := apiErrorOf(t, f.call(t, "save_model", saveArgs(validModel, "none")))
	if apiErr["code"] != "audit_unavailable" {
		t.Fatalf("envelope = %v", apiErr)
	}

	f.assertUnchanged(t, before)
}
