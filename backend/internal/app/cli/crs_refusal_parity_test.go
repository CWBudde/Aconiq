package cli

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/api/httpv1"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// parityHandler is an API handler over an empty project, which is all the
// transform endpoint needs: it projects the batch in the request body and never
// reads the stored model.
func parityHandler(t *testing.T) http.Handler {
	t.Helper()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, err := store.Init("CRS Refusal Parity", "EPSG:25832"); err != nil {
		t.Fatalf("init project: %v", err)
	}

	return httpv1.NewHandler(store, nil)
}

// apiRefusalMessage is what POST /api/v1/transform tells a client about a site
// it will not project.
func apiRefusalMessage(t *testing.T, handler http.Handler, body string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform", strings.NewReader(body))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpv1.ClientHeaderName, "crs-refusal-parity-test")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/transform %s: status %d, want 400: %s", body, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope %s: %v", rec.Body.String(), err)
	}

	return envelope.Error.Message
}

// kernelRefusalMessage is what browser mode is told about the same site.
func kernelRefusalMessage(t *testing.T, body string) string {
	t.Helper()

	_, err := wasmkernel.Transform([]byte(body))
	if err == nil {
		t.Fatalf("the kernel projected %s; it must refuse the site", body)
	}

	return err.Error()
}

// cliRefusalMessage is the message inside `aconiq run`'s own error envelope.
//
// The envelope is the CLI's — its Kind drives the exit code and its Op names
// the stage — but the sentence it carries must not be, so the message is read
// back off the AppError rather than off Error(), which prefixes the operation.
func cliRefusalMessage(t *testing.T, projectCRS string, lon, lat float64) string {
	t.Helper()

	model := modelgeojson.Model{
		ProjectCRS: projectCRS,
		Features: []modelgeojson.Feature{{
			ID:           "src-1",
			Kind:         modelgeojson.FeatureKindSource,
			GeometryType: modelgeojson.GeometryTypePoint,
			Coordinates:  []any{lon, lat},
		}},
	}

	_, _, err := resolveComputeModel(model, projectCRS)
	if err == nil {
		t.Fatalf("resolveComputeModel projected a site at %v, %v; it must refuse it", lon, lat)
	}

	var appErr *domainerrors.AppError
	if !stderrors.As(err, &appErr) {
		t.Fatalf("resolveComputeModel returned %T, want a *domainerrors.AppError", err)
	}

	if appErr.Kind != domainerrors.KindUserInput {
		t.Errorf("kind = %q, want %q; the exit code depends on it", appErr.Kind, domainerrors.KindUserInput)
	}

	if appErr.Op != "cli.resolveComputeModel" {
		t.Errorf("op = %q, want cli.resolveComputeModel", appErr.Op)
	}

	return appErr.Msg
}

// All three surfaces that can refuse a site refuse it in the same sentence.
//
// httpv1.TestTransformRefusesAnUnprojectableSiteInTheProjectorsWords pins the
// two surfaces that share crstransform.Transform. `aconiq run` is the third and
// does not go through it: it resolves the zone itself and wraps the refusal in
// the CLI's own error envelope, so it once carried a second copy of the
// sentence that nothing compared. This test is what stops that copy coming
// back — the words now come from crstransform.GeographicRefusal for all three.
//
// The CLI is asked with a lower-case project CRS on purpose. It names the CRS
// as geo.ParseCRS canonicalises it, which is what the other two interpolate; an
// identifier echoed back the way the user typed it would break the parity for
// every project whose CRS is not already upper-case.
func TestGeographicRefusalReadsTheSameOnAllThreeSurfaces(t *testing.T) {
	t.Parallel()

	handler := parityHandler(t)

	testCases := []struct {
		name string
		lon  float64
		lat  float64
	}{
		{name: "southern hemisphere", lon: 9.0, lat: -51.0},
		{name: "outside zones 31 to 34", lon: -120.0, lat: 37.0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(
				`{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[%v,%v]}`,
				testCase.lon, testCase.lat,
			)

			api := apiRefusalMessage(t, handler, body)
			kernel := kernelRefusalMessage(t, body)
			cli := cliRefusalMessage(t, "epsg:4326", testCase.lon, testCase.lat)

			if api != kernel || api != cli {
				t.Errorf("the three surfaces refuse the same site in different words:\n api:    %q\n kernel: %q\n cli:    %q",
					api, kernel, cli)
			}
		})
	}
}
