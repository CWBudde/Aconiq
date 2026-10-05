package httpv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/io/projectfs"
	overpass "github.com/cwbudde/go-overpass"
)

func TestOverpassAPIErrorNamesTheUpstreamStatus(t *testing.T) {
	t.Parallel()

	// Unit-tested rather than driven through the endpoint on purpose: reaching
	// the 502 branch would need an httptest server standing in for Overpass, and
	// validateOverpassEndpoint admits neither 127.0.0.1 nor http. Faking that
	// would mean weakening the allowlist to test an error message.
	cases := []struct {
		name     string
		err      error
		status   any
		wantHint string
	}{
		{
			name:     "a refusal names the status and says the server refused",
			err:      fmt.Errorf("overpass query: %w", &overpass.ServerError{StatusCode: http.StatusNotAcceptable}),
			status:   float64(http.StatusNotAcceptable),
			wantHint: "blocked",
		},
		{
			name:     "a rate limit tells the reader to wait and shrink the box",
			err:      fmt.Errorf("overpass query: %w", &overpass.ServerError{StatusCode: http.StatusTooManyRequests}),
			status:   float64(http.StatusTooManyRequests),
			wantHint: "smaller bounding box",
		},
		{
			name:     "a gateway timeout reads as the rate limit does",
			err:      fmt.Errorf("overpass query: %w", &overpass.ServerError{StatusCode: http.StatusGatewayTimeout}),
			status:   float64(http.StatusGatewayTimeout),
			wantHint: "smaller bounding box",
		},
		{
			name:     "a server fault is the server's, not the request's",
			err:      fmt.Errorf("overpass query: %w", &overpass.ServerError{StatusCode: http.StatusInternalServerError}),
			status:   float64(http.StatusInternalServerError),
			wantHint: "fault of its own",
		},
		{
			name:     "a transport failure carries no status at all",
			err:      errors.New("dial tcp: no route to host"),
			status:   nil,
			wantHint: "could not be reached",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			apiErr := overpassAPIError(tc.err)

			if apiErr.Code != errorCodeUpstreamError {
				t.Errorf("expected code %q, got %q", errorCodeUpstreamError, apiErr.Code)
			}

			// Round-tripped through JSON, because the contract is what the client
			// receives and a number becomes a float64 on the way.
			encoded, err := json.Marshal(errorResponse{Error: apiErr})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var decoded struct {
				Error struct {
					Message string         `json:"message"`
					Details map[string]any `json:"details"`
					Hint    string         `json:"hint"`
				} `json:"error"`
			}

			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got := decoded.Error.Details["upstream_status"]; got != tc.status {
				t.Errorf("expected upstream_status %v, got %v", tc.status, got)
			}

			if !strings.Contains(decoded.Error.Hint, tc.wantHint) {
				t.Errorf("hint %q does not mention %q", decoded.Error.Hint, tc.wantHint)
			}

			// The three causes must not read alike — that sameness is the defect.
			if tc.status != nil && !strings.Contains(decoded.Error.Message, strconv.Itoa(int(tc.status.(float64)))) {
				t.Errorf("message %q does not name the status", decoded.Error.Message)
			}
		})
	}
}

func TestImportOSMEndpointRejectsBadMethod(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/import/osm", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "method_not_allowed" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsBadBBoxSouthGeNorth(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`{"south":52.5,"west":13.3,"north":52.0,"east":13.5}`)
	req := newAPIRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsBadBBoxWestGeEast(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`{"south":52.0,"west":13.5,"north":52.5,"east":13.3}`)
	req := newAPIRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsOutOfRangeCoordinates(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`{"south":-91.0,"west":13.3,"north":52.5,"east":13.5}`)
	req := newAPIRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`not-json`)
	req := newAPIRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}
