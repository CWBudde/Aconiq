package standards

import "testing"

func TestRegistryResolvesDummyFreefield(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	resolved, err := registry.Resolve("dummy-freefield", "v0", "default")
	if err != nil {
		t.Fatalf("resolve standard: %v", err)
	}

	if resolved.StandardID != "dummy-freefield" {
		t.Fatalf("unexpected standard id: %s", resolved.StandardID)
	}
	if len(resolved.SupportedIndicators) == 0 {
		t.Fatalf("expected indicators in resolved descriptor")
	}

	cnossos, err := registry.Resolve("cnossos-road", "", "")
	if err != nil {
		t.Fatalf("resolve cnossos-road: %v", err)
	}
	if cnossos.StandardID != "cnossos-road" {
		t.Fatalf("unexpected cnossos standard id: %s", cnossos.StandardID)
	}
	if cnossos.Version == "" || cnossos.Profile == "" {
		t.Fatalf("expected default cnossos version/profile")
	}

	rail, err := registry.Resolve("cnossos-rail", "", "")
	if err != nil {
		t.Fatalf("resolve cnossos-rail: %v", err)
	}
	if rail.StandardID != "cnossos-rail" {
		t.Fatalf("unexpected rail standard id: %s", rail.StandardID)
	}
}
