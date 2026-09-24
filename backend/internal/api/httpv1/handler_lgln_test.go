package httpv1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/io/lglnimport"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// newLGLNHandler serves the API with an LGLN client pointed at a fake STAC
// server. None of these tests gets as far as fetching a tile.
func newLGLNHandler(t *testing.T, stac http.HandlerFunc) http.Handler {
	t.Helper()

	server := httptest.NewTLSServer(stac)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("LGLN Import Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	return newHandlerWithOptions(store, handlerOptions{
		lgln: lglnimport.NewClient(
			lglnimport.WithSTACURL(server.URL),
			lglnimport.WithHTTPClient(server.Client()),
			lglnimport.WithAllowedHosts(u.Host),
		),
	})
}

// stacPage answers every search with one page listing the given item IDs.
func stacPage(ids ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			features = append(features, map[string]any{
				"id":         id,
				"type":       "Feature",
				"properties": map[string]any{"letzte_aenderung": "2024-06-12"},
				"assets": map[string]any{
					"lod2-gml": map[string]any{"href": "https://" + r.Host + "/" + id + ".gml"},
				},
			})
		}

		w.Header().Set("Content-Type", "application/geo+json")
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "FeatureCollection", "features": features})
	}
}

func postLGLN(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodPost, "/api/v1/import/lgln", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

const hannoverLGLNBody = `{"south":52.372,"west":9.735,"north":52.378,"east":9.745}`

func TestImportLGLNEmptyAreaAnswersEmptyCollection(t *testing.T) {
	t.Parallel()

	rec := postLGLN(t, newLGLNHandler(t, stacPage()), hannoverLGLNBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp importLGLNResponse
	decodeResponse(t, rec.Body.Bytes(), &resp)

	if resp.Type != "FeatureCollection" || resp.Features == nil || len(resp.Features) != 0 {
		t.Errorf("response = %+v", resp)
	}

	if resp.Tiles == nil || resp.Skipped == nil {
		t.Errorf("tiles and skipped must be present, even when empty: %s", rec.Body.String())
	}
}

func TestImportLGLNRefusals(t *testing.T) {
	t.Parallel()

	tooMany := make([]string, 0, lglnimport.MaxTiles+1)
	for i := range lglnimport.MaxTiles + 1 {
		tooMany = append(tooMany, fmt.Sprintf("LoD2_32_%d_5803_1_ni", 550+i))
	}

	cases := []struct {
		name     string
		stac     http.HandlerFunc
		body     string
		wantCode int
		wantErr  string
	}{
		{
			name:     "south not below north",
			stac:     stacPage(),
			body:     `{"south":52.4,"west":9.735,"north":52.3,"east":9.745}`,
			wantCode: http.StatusBadRequest,
			wantErr:  errorCodeBadRequest,
		},
		{
			name:     "outside Lower Saxony",
			stac:     stacPage(),
			body:     `{"south":48.1,"west":11.5,"north":48.2,"east":11.6}`,
			wantCode: http.StatusBadRequest,
			wantErr:  errorCodeBadRequest,
		},
		{
			name:     "too many tiles",
			stac:     stacPage(tooMany...),
			body:     hannoverLGLNBody,
			wantCode: http.StatusBadRequest,
			wantErr:  errorCodeLGLNTooManyTiles,
		},
		{
			name: "service failing",
			stac: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			body:     hannoverLGLNBody,
			wantCode: http.StatusBadGateway,
			wantErr:  errorCodeLGLNUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := postLGLN(t, newLGLNHandler(t, tc.stac), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}

			var resp errorResponse
			decodeResponse(t, rec.Body.Bytes(), &resp)

			if resp.Error.Code != tc.wantErr {
				t.Errorf("code = %q, want %q", resp.Error.Code, tc.wantErr)
			}
		})
	}
}

func TestImportLGLNRejectsGet(t *testing.T) {
	t.Parallel()

	req := newAPIRequest(http.MethodGet, "/api/v1/import/lgln", nil)
	rec := httptest.NewRecorder()
	newLGLNHandler(t, stacPage()).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
