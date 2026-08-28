package cli

import (
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
	"github.com/aconiq/backend/internal/standards/framework"
)

// tierRunArgs carries the CLI flags a run needs purely because of the evidence
// tier its standard sits at. Scaffold-tier standards refuse to run until the
// operator acknowledges that their levels are invented, so every test that
// drives one end to end has to opt in.
var tierRunArgs = map[framework.EvidenceTier][]string{
	framework.EvidenceTierScaffold: {"--experimental"},
}

// registryRunFixtures names the model each registered standard needs in order
// to complete a run. Every registered ID must appear here — a standard that
// declares no fixture fails the test rather than being skipped.
var registryRunFixtures = map[string][]string{
	"beb-exposure":     {"phase16", "beb_model.geojson"},
	"bub-industry":     {"phase12", "industry_model.geojson"},
	"bub-rail":         {"phase11", "rail_model.geojson"},
	"bub-road":         {"phase14", "bub_road_model.geojson"},
	"buf-aircraft":     {"phase15", "aircraft_model.geojson"},
	"cnossos-aircraft": {"phase13", "aircraft_model.geojson"},
	"cnossos-industry": {"phase12", "industry_model.geojson"},
	"cnossos-rail":     {"phase11", "rail_model.geojson"},
	"cnossos-road":     {"phase10", "road_model.geojson"},
	"dummy-freefield":  {"phase8", "model.geojson"},
	"iso9613":          {"phase19", "iso9613_industry_model.geojson"},
	"rls19-road":       {"phase17", "rls19_road_model.geojson"},
	// schall03 defaults to schall03_engine=auto, which refuses to run without
	// normative track data, so this fixture must be the one that carries it.
	"schall03": {"phase18", "schall03_normative_model.geojson"},
}

// The registry must not offer what the executor cannot run. Registering a
// standard without wiring it into the run pipeline has to fail here, not at a
// user's first run: this is what keeps the pipeline's default branch — the one
// that reports "registered but not wired in run pipeline yet" — unreachable.
func TestEveryRegisteredStandardCompletesARun(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new standards registry: %v", err)
	}

	descriptors := registry.List()
	if len(descriptors) == 0 {
		t.Fatal("standards registry is empty")
	}

	for _, descriptor := range descriptors {
		t.Run(descriptor.ID, func(t *testing.T) {
			t.Parallel()

			fixture, ok := registryRunFixtures[descriptor.ID]
			if !ok {
				t.Fatalf("standard %q is registered but declares no run fixture; add one rather than skipping it", descriptor.ID)
			}

			projectDir := t.TempDir()

			mustRunCLI(t, "--project", projectDir, "init", "--name", "RegistryCoverage", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, fixture...))

			args := []string{"--project", projectDir, "run", "--standard", descriptor.ID}
			args = append(args, tierRunArgs[descriptor.EvidenceTier]...)

			mustRunCLI(t, args...)

			store, err := projectfs.New(projectDir)
			if err != nil {
				t.Fatalf("new project store: %v", err)
			}

			proj, err := store.Load()
			if err != nil {
				t.Fatalf("load project: %v", err)
			}

			if len(proj.Runs) != 1 {
				t.Fatalf("expected exactly one run, got %d", len(proj.Runs))
			}

			run := proj.Runs[0]
			if run.Standard.ID != descriptor.ID {
				t.Fatalf("expected run standard %q, got %q", descriptor.ID, run.Standard.ID)
			}

			if run.Status != project.RunStatusCompleted {
				t.Fatalf("expected completed run status, got %q", run.Status)
			}
		})
	}
}

// The fixture table above must not drift away from the registry in the other
// direction either: a standard that is deregistered should take its fixture
// with it.
func TestRegistryRunFixturesNameOnlyRegisteredStandards(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new standards registry: %v", err)
	}

	registered := make(map[string]struct{}, len(registry.List()))
	for _, descriptor := range registry.List() {
		registered[descriptor.ID] = struct{}{}
	}

	for standardID := range registryRunFixtures {
		if _, ok := registered[standardID]; !ok {
			t.Errorf("run fixture declared for unregistered standard %q", standardID)
		}
	}
}
