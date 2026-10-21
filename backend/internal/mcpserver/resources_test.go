package mcpserver_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// apiAnswer serves one request through the handler the server wraps, as the
// reference a tool or resource must reproduce byte for byte.
func (f fixture) apiAnswer(t *testing.T, method string, path string, body []byte) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, reader)
	req.Host = "127.0.0.1"

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Aconiq-Client", "test")
	}

	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	return rec.Code, rec.Body.Bytes()
}

func (f fixture) readResource(t *testing.T, uri string) (*mcp.ReadResourceResult, error) {
	t.Helper()

	result, err := f.session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// resourceText reads a resource that must exist and returns its one text
// content.
func (f fixture) resourceText(t *testing.T, uri string) string {
	t.Helper()

	result, err := f.readResource(t, uri)
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}

	if len(result.Contents) != 1 {
		t.Fatalf("read %s: %d contents, want 1", uri, len(result.Contents))
	}

	content := result.Contents[0]
	if content.URI != uri || content.MIMEType != "application/json" {
		t.Fatalf("read %s: content uri %q, mime %q", uri, content.URI, content.MIMEType)
	}

	return content.Text
}

// seedRun creates a run row with its provenance.json, as `aconiq run` does
// before it computes.
func (f fixture) seedRun(t *testing.T) project.Run {
	t.Helper()

	run, _, err := f.store.CreateRun(projectfs.CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return run
}

func assertResourceNotFound(t *testing.T, err error) {
	t.Helper()

	// The SDK spells not-found as invalid params (SEP-2164) and tells it apart
	// by its message alone.
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams || rpcErr.Message != "Resource not found" {
		t.Fatalf("error = %v, want resource not found", err)
	}
}

// The read tools and the resources are what an agent reads before it writes,
// so a read-only server offers them as fully as a writable one.
func TestReadSurfaceIsListedWithAndWithoutWrites(t *testing.T) {
	t.Parallel()

	for _, allowWrites := range []bool{false, true} {
		f := newFixture(t, allowWrites)

		tools := listTools(t, f.session)
		for _, name := range []string{"list_standards", "transform"} {
			tool, ok := tools[name]
			if !ok {
				t.Fatalf("allow_writes=%v: %s not listed", allowWrites, name)
			}

			if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
				t.Fatalf("%s is not annotated read-only", name)
			}
		}

		resources, err := f.session.ListResources(t.Context(), nil)
		if err != nil {
			t.Fatalf("list resources: %v", err)
		}

		uris := make([]string, 0, len(resources.Resources))
		for _, resource := range resources.Resources {
			uris = append(uris, resource.URI)
		}

		if slices.Sort(uris); !slices.Equal(uris, []string{"aconiq://project", "aconiq://standards"}) {
			t.Fatalf("allow_writes=%v: resources %v", allowWrites, uris)
		}

		templates, err := f.session.ListResourceTemplates(t.Context(), nil)
		if err != nil {
			t.Fatalf("list resource templates: %v", err)
		}

		patterns := make([]string, 0, len(templates.ResourceTemplates))
		for _, template := range templates.ResourceTemplates {
			patterns = append(patterns, template.URITemplate)
		}

		if slices.Sort(patterns); !slices.Equal(patterns, []string{"aconiq://artifacts/{id}", "aconiq://runs/{id}", "aconiq://runs/{id}/provenance"}) {
			t.Fatalf("allow_writes=%v: resource templates %v", allowWrites, patterns)
		}
	}
}

// The catalogue is the descriptorjson encoding the API and the WASM kernel
// share, evidence tier and parameter units included — never a summary of it.
func TestListStandardsIsTheAPIResponseVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	result := f.call(t, "list_standards", map[string]any{})
	if result.IsError {
		t.Fatalf("list_standards: %s", resultText(t, result))
	}

	code, want := f.apiAnswer(t, http.MethodGet, "/api/v1/standards", nil)
	if code != http.StatusOK || resultText(t, result) != string(want) {
		t.Fatalf("list_standards differs from the API (%d):\n got %s\nwant %s", code, resultText(t, result), want)
	}

	var catalogue []struct {
		ID           string `json:"id"`
		EvidenceTier string `json:"evidence_tier"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &catalogue)
	if err != nil {
		t.Fatalf("decode catalogue: %v", err)
	}

	tiers := map[string]string{}
	for _, standard := range catalogue {
		tiers[standard.ID] = standard.EvidenceTier
	}

	if tiers["rls19-road"] != "normative" || tiers["cnossos-road"] != "scaffold" {
		t.Fatalf("evidence tiers = %v", tiers)
	}
}

// "auto" makes the zone decision `aconiq run` makes, because the tool is the
// API's projector and not a second one.
func TestTransformAnswersAsTheAPI(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	args := map[string]any{"source_crs": "EPSG:4326", "target_crs": "auto", "coordinates": []float64{9.73, 52.37}}

	result := f.call(t, "transform", args)
	if result.IsError {
		t.Fatalf("transform: %s", resultText(t, result))
	}

	body, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	code, want := f.apiAnswer(t, http.MethodPost, "/api/v1/transform", body)
	if code != http.StatusOK || resultText(t, result) != string(want) {
		t.Fatalf("transform differs from the API (%d):\n got %s\nwant %s", code, resultText(t, result), want)
	}

	var answer struct {
		TargetCRS string `json:"target_crs"`
	}

	err = json.Unmarshal([]byte(resultText(t, result)), &answer)
	if err != nil || answer.TargetCRS != "EPSG:25832" {
		t.Fatalf("auto zone = %+v (%v)", answer, err)
	}
}

// A site no supported zone covers is refused in the projector's own words, as
// the API and `aconiq run` refuse it.
func TestTransformRefusesAnUnprojectableSiteWithTheAPIEnvelope(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	apiErr := apiErrorOf(t, f.call(t, "transform", map[string]any{
		"source_crs": "EPSG:4326", "target_crs": "auto", "coordinates": []float64{-100, 40},
	}))
	if apiErr["code"] != "crs_not_projectable" || apiErr["hint"] == nil {
		t.Fatalf("envelope = %v", apiErr)
	}
}

func TestProjectAndStandardsResourcesAreTheAPIResponsesVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	for uri, path := range map[string]string{
		"aconiq://project":   "/api/v1/project/status",
		"aconiq://standards": "/api/v1/standards",
	} {
		code, want := f.apiAnswer(t, http.MethodGet, path, nil)
		if got := f.resourceText(t, uri); code != http.StatusOK || got != string(want) {
			t.Fatalf("%s differs from %s (%d):\n got %s\nwant %s", uri, path, code, got, want)
		}
	}
}

// Provenance is the record a run is checked against, so the resource is the
// file as written, not a re-encoding that could drop a field.
func TestRunProvenanceResourceIsTheFileVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	want, err := os.ReadFile(filepath.Join(f.store.Root(), filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	if got := f.resourceText(t, "aconiq://runs/"+run.ID+"/provenance"); got != string(want) {
		t.Fatalf("provenance resource differs from the file:\n got %s\nwant %s", got, want)
	}
}

func TestRunResourceIsTheAPIResponseVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	code, want := f.apiAnswer(t, http.MethodGet, "/api/v1/runs/"+run.ID, nil)
	if got := f.resourceText(t, "aconiq://runs/"+run.ID); code != http.StatusOK || got != string(want) {
		t.Fatalf("run resource differs from the API (%d):\n got %s\nwant %s", code, got, want)
	}
}

func TestUnknownRunResourcesAreNotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	for _, uri := range []string{"aconiq://runs/run-nope", "aconiq://runs/run-nope/provenance"} {
		_, err := f.readResource(t, uri)
		assertResourceNotFound(t, err)
	}
}

// A run id is one path segment of the API route, however it is spelled, so
// no URI can make the resource read a different route — here, the model the
// project does hold.
func TestRunIDCannotLeaveItsRoute(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	result := f.call(t, "save_model", saveArgs(validModel, "none"))
	if result.IsError {
		t.Fatalf("seed save: %s", resultText(t, result))
	}

	for _, uri := range []string{
		"aconiq://runs/..",
		"aconiq://runs/.",
		"aconiq://runs/..%2F..%2Fmodel",
		"aconiq://runs/..%2F..%2Fmodel/provenance",
		"aconiq://runs/%2E%2E/provenance",
	} {
		_, err := f.readResource(t, uri)
		assertResourceNotFound(t, err)
	}
}

// Two reads of an unchanged project are byte-identical (ADR-002,
// Deterministic outputs).
func TestResourcesAreDeterministic(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	for _, uri := range []string{
		"aconiq://project",
		"aconiq://standards",
		"aconiq://runs/" + run.ID,
		"aconiq://runs/" + run.ID + "/provenance",
	} {
		if first, second := f.resourceText(t, uri), f.resourceText(t, uri); first != second {
			t.Fatalf("%s differs between two reads:\n%s\n%s", uri, first, second)
		}
	}
}
