package httpv1

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
)

// testHost is the Host header the handler tests present. It is loopback, so it
// passes the allowlist without the test having to know a listen address.
const testHost = "127.0.0.1:8080"

func mustStore(t *testing.T, name string) projectfs.Store {
	t.Helper()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init(name, "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	return store
}

// assertErrorCode reads the standard envelope and checks both halves of the
// contract a client switches on: the status and the code.
func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("expected %d, got %d: %s", wantStatus, rec.Code, rec.Body.String())
	}

	var response errorResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != wantCode {
		t.Fatalf("expected error code %q, got %q", wantCode, response.Error.Code)
	}
}

func TestHostAllowlistRejectsRebindingHost(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Host Allowlist"), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = "attacker.example.com"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusForbidden, errorCodeForbiddenHost)
}

func TestHostAllowlistAcceptsLoopbackAndConfiguredHost(t *testing.T) {
	t.Parallel()

	handler := newHandlerWithOptions(mustStore(t, "Host Allowlist"), handlerOptions{
		clock:        time.Now,
		allowedHosts: hostsFromListenAddr("noise.internal:8080"),
	})

	for _, host := range []string{"127.0.0.1:8080", "localhost:5173", "[::1]:8080", "noise.internal:8080"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.Host = host

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("host %q: expected 200, got %d: %s", host, rec.Code, rec.Body.String())
		}
	}
}

func TestIsAllowedHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"127.0.0.53", true},
		{"localhost", true},
		{"LOCALHOST:5173", true},
		{"[::1]:8080", true},
		{"", false},
		{"attacker.example.com", false},
		{"192.168.1.10:8080", false},
		{"localhost.attacker.example.com", false},
	}

	for _, tc := range cases {
		if got := isAllowedHost(tc.host, nil); got != tc.want {
			t.Errorf("isAllowedHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestHostsFromListenAddr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		addr string
		want []string
	}{
		{"127.0.0.1:8080", []string{"127.0.0.1"}},
		{"noise.internal:8080", []string{"noise.internal"}},
		{"[::1]:8080", []string{"::1"}},
		// A wildcard bind names no host, so nothing beyond loopback is added.
		{"0.0.0.0:8080", nil},
		{":8080", nil},
		{"", nil},
	}

	for _, tc := range cases {
		got := hostsFromListenAddr(tc.addr)
		if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
			t.Errorf("hostsFromListenAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

// A cross-origin fetch with a CORS-safelisted media type reaches the endpoint
// without a preflight, so the run executor must not be reachable through one.
func TestRunCreateRejectsSafelistedContentType(t *testing.T) {
	t.Parallel()

	executed := false
	handler := newHandlerWithOptions(mustStore(t, "Content Type"), handlerOptions{
		clock: time.Now,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			executed = true

			return nil
		},
	})

	for _, contentType := range []string{"text/plain", "application/x-www-form-urlencoded", "multipart/form-data", ""} {
		req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id":"rls19-road"}`))
		req.Header.Set("Content-Type", contentType)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assertErrorCode(t, rec, http.StatusUnsupportedMediaType, errorCodeUnsupportedMediaType)
	}

	if executed {
		t.Fatal("run executor was reached by a request that never sent JSON")
	}
}

func TestRunCreateAcceptsJSONContentTypeWithCharset(t *testing.T) {
	t.Parallel()

	reached := false
	handler := newHandlerWithOptions(mustStore(t, "Content Type"), handlerOptions{
		clock: time.Now,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			reached = true

			return nil
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id":"rls19-road"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !reached {
		t.Fatal("a JSON body with a charset parameter must be accepted")
	}
}

func TestStateChangingRequestRequiresClientHeader(t *testing.T) {
	t.Parallel()

	executed := false
	handler := newHandlerWithOptions(mustStore(t, "Client Header"), handlerOptions{
		clock: time.Now,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			executed = true

			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id":"rls19-road"}`))
	req.Host = testHost
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusForbidden, errorCodeClientHeaderRequired)

	if executed {
		t.Fatal("run executor was reached by a request that could have been a CORS simple request")
	}
}

func TestSafeMethodsDoNotNeedClientHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Client Header"), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = testHost

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPITokenIsEnforcedOnlyWhenConfigured(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "API Token")

	open := newHandlerWithOptions(store, handlerOptions{clock: time.Now})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = testHost

	rec := httptest.NewRecorder()
	open.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("without a token the API must answer as before, got %d", rec.Code)
	}

	guarded := newHandlerWithOptions(store, handlerOptions{clock: time.Now, apiToken: "s3cret"})

	for _, header := range []string{"", "Bearer wrong", "s3cret", "Basic s3cret"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.Host = testHost

		if header != "" {
			req.Header.Set("Authorization", header)
		}

		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, req)

		assertErrorCode(t, rec, http.StatusUnauthorized, errorCodeUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = testHost
	req.Header.Set("Authorization", "Bearer s3cret")

	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("a correct token must be accepted, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunCreateRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	executed := false
	handler := newHandlerWithOptions(mustStore(t, "Body Cap"), handlerOptions{
		clock: time.Now,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			executed = true

			return nil
		},
	})

	// One oversized parameter value: valid JSON, well past the endpoint's cap.
	body := `{"params":{"road_speed_kph":"` + strings.Repeat("9", maxRunCreateBodyBytes+1) + `"}}`

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusRequestEntityTooLarge, errorCodeRequestTooLarge)

	if executed {
		t.Fatal("run executor was reached by an oversized request")
	}
}

func TestImportOSMRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Body Cap"), nil)

	body := `{"south":1,"west":1,"north":2,"east":2,"overpass_endpoint":"https://overpass-api.de/` +
		strings.Repeat("a", maxImportOSMBodyBytes) + `"}`

	req := newAPIRequest(http.MethodPost, "/api/v1/import/osm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusRequestEntityTooLarge, errorCodeRequestTooLarge)
}

// zeroReader streams as many bytes as asked for without materialising them, so
// the oversized-upload test does not allocate the cap it is testing.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}

	return len(p), nil
}

func TestImportTerrainRejectsUploadOverTheCap(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Upload Cap"), nil)

	var head strings.Builder

	writer := multipart.NewWriter(&head)

	_, err := writer.CreateFormFile("file", "terrain.tif")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	var tail strings.Builder

	tailWriter := multipart.NewWriter(&tail)
	tailWriter.SetBoundary(writer.Boundary()) //nolint:errcheck // the boundary comes from the writer itself

	err = tailWriter.Close()
	if err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	body := io.MultiReader(
		strings.NewReader(head.String()),
		io.LimitReader(zeroReader{}, maxTerrainUploadBytes+1),
		strings.NewReader(tail.String()),
	)

	req := newAPIRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusRequestEntityTooLarge, errorCodeRequestTooLarge)
}

func TestImportTerrainRejectsNonMultipartBody(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Upload Cap"), nil)

	req := newAPIRequest(http.MethodPost, "/api/v1/import/terrain", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusUnsupportedMediaType, errorCodeUnsupportedMediaType)
}

// A project folder is something a user may have received from someone else, so
// the manifest inside it is untrusted: a log_path that leaves the project root
// must not become an HTTP read of whatever it points at.
func TestRunLogRefusesPathOutsideProjectRoot(t *testing.T) {
	t.Parallel()

	for _, logPath := range []string{"../../../../etc/passwd", "/etc/passwd"} {
		store := mustStore(t, "Traversal")

		proj, err := store.Load()
		if err != nil {
			t.Fatalf("load project: %v", err)
		}

		proj.Runs = append(proj.Runs, project.Run{
			ID:       "run-evil",
			Status:   project.RunStatusCompleted,
			LogPath:  logPath,
			Standard: project.StandardRef{ID: "rls19-road", Version: "2019"},
		})

		err = store.Save(proj)
		if err != nil {
			t.Fatalf("save project: %v", err)
		}

		handler := NewHandler(store, nil)
		req := newAPIRequest(http.MethodGet, "/api/v1/runs/run-evil/log", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assertErrorCode(t, rec, http.StatusForbidden, errorCodeForbiddenPath)

		if strings.Contains(rec.Body.String(), "root:") {
			t.Fatal("response leaked the contents of the traversed file")
		}
	}
}

func TestArtifactContentRefusesPathOutsideProjectRoot(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Traversal")

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
		ID:   "artifact-evil",
		Kind: "report.html",
		Path: "../../../../etc/passwd",
	})

	err = store.Save(proj)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/artifacts/artifact-evil/content", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusForbidden, errorCodeForbiddenPath)
}

// The containment must not cost the endpoints their ordinary behaviour.
func TestConfinedReadsStillServeProjectFiles(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Confined Reads")

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		Standard: project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:   project.RunStatusCompleted,
		LogLines: []string{"run started", "run finished"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/runs/"+run.ID+"/log", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response runLogResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if len(response.Lines) == 0 {
		t.Fatal("expected log lines")
	}

	artifactPath := filepath.Join(".noise", "artifacts", "summary.json")

	err = os.MkdirAll(filepath.Join(store.Root(), ".noise", "artifacts"), 0o750)
	if err != nil {
		t.Fatalf("create artifact dir: %v", err)
	}

	err = os.WriteFile(filepath.Join(store.Root(), artifactPath), []byte(`{"ok":true}`), 0o600)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
		ID:    "artifact-summary",
		Kind:  "run.summary",
		Path:  ".noise/artifacts/summary.json",
		RunID: run.ID,
	})

	err = store.Save(proj)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}

	req = newAPIRequest(http.MethodGet, "/api/v1/artifacts/artifact-summary/content", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("unexpected artifact body: %s", rec.Body.String())
	}
}

func TestImportOSMRejectsEndpointOutsideAllowlist(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "SSRF"), nil)

	endpoints := []string{
		"http://169.254.169.254/latest/meta-data/",
		"https://169.254.169.254/latest/meta-data/",
		"file:///etc/passwd",
		"http://overpass-api.de/api/interpreter",
		"https://overpass-api.de.attacker.example.com/api/interpreter",
		"https://localhost:9999/api/interpreter",
		":://not-a-url",
	}

	for _, endpoint := range endpoints {
		body := `{"south":52.5,"west":13.3,"north":52.6,"east":13.4,"overpass_endpoint":"` + endpoint + `"}`

		req := newAPIRequest(http.MethodPost, "/api/v1/import/osm", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("endpoint %q: expected 400, got %d: %s", endpoint, rec.Code, rec.Body.String())
		}

		var response errorResponse

		decodeResponse(t, rec.Body.Bytes(), &response)

		if response.Error.Code != errorCodeOverpassEndpointNotAllowed {
			t.Fatalf("endpoint %q: expected code %q, got %q",
				endpoint, errorCodeOverpassEndpointNotAllowed, response.Error.Code)
		}
	}
}

func TestValidateOverpassEndpointAcceptsKnownServers(t *testing.T) {
	t.Parallel()

	if err := validateOverpassEndpoint(""); err != nil {
		t.Fatalf("empty endpoint must be left to the importer's default: %+v", err)
	}

	for _, host := range allowedOverpassHosts {
		if err := validateOverpassEndpoint("https://" + host + "/api/interpreter"); err != nil {
			t.Fatalf("host %q must be accepted: %+v", host, err)
		}
	}
}

func TestCORSPreflightAdvertisesTheClientHeader(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	handler := NewServeHandler(mustStore(t, "CORS"), nil, registry, ServeOptions{
		ListenAddr: testHost,
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/runs", nil)
	req.Host = testHost
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", ClientHeaderName)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), ClientHeaderName) {
		t.Fatalf("preflight must advertise %s, got %q",
			ClientHeaderName, rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

// The OpenAPI document is hand-built, so the transport contract has to be
// asserted rather than assumed.
func TestOpenAPIDocumentsTheTransportContract(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	if _, ok := components["securitySchemes"].(map[string]any); !ok {
		t.Fatal("expected a securitySchemes object describing the optional bearer token")
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}

	runs, ok := paths["/api/v1/runs"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/runs path item")
	}

	assertOperationResponses(t, runs, "get", []string{"401", "403"})
	assertOperationResponses(t, runs, "post", []string{"401", "403", "413", "415"})

	post, ok := runs["post"].(map[string]any)
	if !ok {
		t.Fatal("expected post operation")
	}

	parameters, ok := post["parameters"].([]map[string]any)
	if !ok || len(parameters) == 0 {
		t.Fatal("expected the client header parameter on the mutating operation")
	}

	if parameters[0]["name"] != ClientHeaderName {
		t.Fatalf("unexpected header parameter: %#v", parameters[0])
	}
}

func assertOperationResponses(t *testing.T, item map[string]any, method string, wantCodes []string) {
	t.Helper()

	operation, ok := item[method].(map[string]any)
	if !ok {
		t.Fatalf("expected %s operation", method)
	}

	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		t.Fatalf("expected %s responses", method)
	}

	for _, code := range wantCodes {
		if _, ok := responses[code]; !ok {
			t.Fatalf("%s: expected documented %s response", method, code)
		}
	}
}
