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

	aircraft, err := registry.Resolve("cnossos-aircraft", "", "")
	if err != nil {
		t.Fatalf("resolve cnossos-aircraft: %v", err)
	}

	if aircraft.StandardID != "cnossos-aircraft" {
		t.Fatalf("unexpected aircraft standard id: %s", aircraft.StandardID)
	}

	bub, err := registry.Resolve("bub-road", "", "")
	if err != nil {
		t.Fatalf("resolve bub-road: %v", err)
	}

	if bub.StandardID != "bub-road" {
		t.Fatalf("unexpected bub standard id: %s", bub.StandardID)
	}

	if bub.Context != "mapping" {
		t.Fatalf("expected mapping context, got %q", bub.Context)
	}

	bubRail, err := registry.Resolve("bub-rail", "", "")
	if err != nil {
		t.Fatalf("resolve bub-rail: %v", err)
	}

	if bubRail.StandardID != "bub-rail" || bubRail.Context != "mapping" {
		t.Fatalf("unexpected bub rail descriptor: %#v", bubRail)
	}

	bubIndustry, err := registry.Resolve("bub-industry", "", "")
	if err != nil {
		t.Fatalf("resolve bub-industry: %v", err)
	}

	if bubIndustry.StandardID != "bub-industry" || bubIndustry.Context != "mapping" {
		t.Fatalf("unexpected bub industry descriptor: %#v", bubIndustry)
	}

	buf, err := registry.Resolve("buf-aircraft", "", "")
	if err != nil {
		t.Fatalf("resolve buf-aircraft: %v", err)
	}

	if buf.StandardID != "buf-aircraft" {
		t.Fatalf("unexpected buf standard id: %s", buf.StandardID)
	}

	if buf.Context != "mapping" {
		t.Fatalf("expected mapping context for buf-aircraft, got %q", buf.Context)
	}

	beb, err := registry.Resolve("beb-exposure", "", "")
	if err != nil {
		t.Fatalf("resolve beb-exposure: %v", err)
	}

	if beb.StandardID != "beb-exposure" {
		t.Fatalf("unexpected beb standard id: %s", beb.StandardID)
	}

	if beb.Context != "mapping" {
		t.Fatalf("expected mapping context for beb-exposure, got %q", beb.Context)
	}

	rail, err := registry.Resolve("cnossos-rail", "", "")
	if err != nil {
		t.Fatalf("resolve cnossos-rail: %v", err)
	}

	if rail.StandardID != "cnossos-rail" {
		t.Fatalf("unexpected rail standard id: %s", rail.StandardID)
	}

	industry, err := registry.Resolve("cnossos-industry", "", "")
	if err != nil {
		t.Fatalf("resolve cnossos-industry: %v", err)
	}

	if industry.StandardID != "cnossos-industry" {
		t.Fatalf("unexpected industry standard id: %s", industry.StandardID)
	}

	rls19, err := registry.Resolve("rls19-road", "", "")
	if err != nil {
		t.Fatalf("resolve rls19-road: %v", err)
	}

	if rls19.StandardID != "rls19-road" {
		t.Fatalf("unexpected rls19 standard id: %s", rls19.StandardID)
	}

	schall03, err := registry.Resolve("schall03", "", "")
	if err != nil {
		t.Fatalf("resolve schall03: %v", err)
	}

	if schall03.StandardID != "schall03" {
		t.Fatalf("unexpected schall03 standard id: %s", schall03.StandardID)
	}

	if schall03.Context != "planning" {
		t.Fatalf("expected planning context for schall03, got %q", schall03.Context)
	}
}
