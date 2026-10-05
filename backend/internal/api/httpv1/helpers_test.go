package httpv1

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
	"github.com/aconiq/backend/internal/standards/framework"
)

// mustCreateCompletedRun seeds one finished run, carrying a standard context so
// that a payload comparison covers the members that are easiest to forget.
func mustCreateCompletedRun(t *testing.T, store projectfs.Store) project.Run {
	t.Helper()

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		ScenarioID: "default",
		Standard: project.StandardRef{
			Context: framework.StandardContextPlanning,
			ID:      "rls19-road",
			Version: "2019",
			Profile: "default",
		},
		Status: project.RunStatusCompleted,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return run
}

// anySliceToStrings flattens a decoded JSON array into its string members.
func anySliceToStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	texts := make([]string, 0, len(items))

	for _, item := range items {
		text, isText := item.(string)
		if !isText {
			continue
		}

		texts = append(texts, text)
	}

	return texts
}

func decodeResponse(t *testing.T, payload []byte, out any) {
	t.Helper()

	err := json.Unmarshal(payload, out)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func runGateFixture(t *testing.T) (projectfs.Store, framework.Registry) {
	t.Helper()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Run Gate Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	return store, registry
}

// newAPIRequest builds a request that satisfies the transport-level controls in
// security.go, so a test that is about an endpoint's own behaviour does not have
// to restate them. Tests that are about the controls build their requests with
// httptest.NewRequest directly, in security_test.go.
func newAPIRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = testHost

	if !isSafeMethod(method) {
		req.Header.Set(ClientHeaderName, "handler-test")
	}

	return req
}
