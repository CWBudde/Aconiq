package httpv1

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware_LocalhostAllowed(t *testing.T) {
	origins := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:5173", true},
		{"http://localhost:8080", true},
		{"http://127.0.0.1:5173", true},
		{"http://127.0.0.1:8080", true},
		{"https://evil.example.com", false},
		{"https://notlocalhost.com", false},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := corsMiddleware(nil)(inner)

	for _, tc := range origins {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			req.Header.Set("Origin", tc.origin)

			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin") != ""
			if got != tc.want {
				t.Errorf("origin %q: got CORS header present=%v, want %v", tc.origin, got, tc.want)
			}

			if tc.want && rec.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Errorf("origin %q: ACAO header = %q, want %q",
					tc.origin, rec.Header().Get("Access-Control-Allow-Origin"), tc.origin)
			}
		})
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not reach here for OPTIONS preflight.
		w.WriteHeader(http.StatusTeapot)
	})
	mw := corsMiddleware(nil)(inner)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight: got %d, want %d", rec.Code, http.StatusNoContent)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("preflight: missing ACAO header")
	}
}

func TestCORSMiddleware_ExtraOrigins(t *testing.T) {
	extra := []string{"https://myapp.example.com"}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := corsMiddleware(extra)(inner)

	for _, origin := range []string{"https://myapp.example.com", "https://other.example.com"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.Header.Set("Origin", origin)

		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)

		present := rec.Header().Get("Access-Control-Allow-Origin") != ""

		want := origin == "https://myapp.example.com"
		if present != want {
			t.Errorf("origin %q: CORS header present=%v, want %v", origin, present, want)
		}
	}
}

func TestCORSMiddleware_NoOriginHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := corsMiddleware(nil)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("no Origin header: should not produce CORS headers")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("no Origin header: got %d, want %d", rec.Code, http.StatusOK)
	}
}
