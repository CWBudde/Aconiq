package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPICommandWritesSpec(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	outPath := filepath.Join(projectDir, "openapi.v1.json")

	mustRunCLI(t, "--project", projectDir, "openapi", "--out", outPath)

	payload, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read openapi output: %v", err)
	}

	var doc map[string]any

	err = json.Unmarshal(payload, &doc)
	if err != nil {
		t.Fatalf("decode openapi output: %v", err)
	}

	if got := doc["openapi"]; got != "3.1.0" {
		t.Fatalf("unexpected openapi version: %#v", got)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths object in openapi document")
	}

	for _, required := range []string{
		"/api/v1/artifacts/{id}/content",
		"/api/v1/health",
		"/api/v1/import/osm",
		"/api/v1/import/terrain",
		"/api/v1/runs",
		"/api/v1/runs/{id}/log",
		"/api/v1/project/status",
		"/api/v1/standards",
		"/api/v1/events",
		"/api/v1/openapi.json",
	} {
		if _, exists := paths[required]; !exists {
			t.Fatalf("expected %s in openapi paths", required)
		}
	}
}

func TestOpenAPICommandEmbedsServerURL(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	outPath := filepath.Join(projectDir, "openapi.v1.json")
	serverURL := "http://127.0.0.1:9999"

	mustRunCLI(t, "--project", projectDir, "openapi", "--out", outPath, "--server-url", serverURL)

	payload, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read openapi output: %v", err)
	}

	var doc map[string]any

	err = json.Unmarshal(payload, &doc)
	if err != nil {
		t.Fatalf("decode openapi output: %v", err)
	}

	servers, ok := doc["servers"].([]any)
	if !ok || len(servers) == 0 {
		t.Fatalf("expected servers in openapi document")
	}

	firstServer, ok := servers[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected server object: %#v", servers[0])
	}

	if firstServer["url"] != serverURL {
		t.Fatalf("unexpected server url: %#v", firstServer["url"])
	}
}
