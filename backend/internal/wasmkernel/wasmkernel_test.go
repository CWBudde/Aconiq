package wasmkernel_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/standards/descriptorjson"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// The kernel must not advertise a standard it has no entry point for. That is
// the browser-mode form of the evidence-tier rule: a run page offering
// cnossos-road because the registry happened to be linked in would be offering
// a computation this binary cannot perform.
func TestEveryStandardHasItsEntryPoint(t *testing.T) {
	t.Parallel()

	source := wasmEntryPointSource(t)

	standards := wasmkernel.Standards()
	if len(standards) == 0 {
		t.Fatal("the kernel advertises no standards at all")
	}

	for _, standard := range standards {
		if standard.Entry == "" {
			t.Errorf("standard %q declares no entry point", standard.Descriptor.ID)

			continue
		}

		want := `aconiq.Set("` + standard.Entry + `"`
		if !strings.Contains(source, want) {
			t.Errorf("standard %q names entry point %q, which cmd/wasm/main.go does not register",
				standard.Descriptor.ID, standard.Entry)
		}
	}
}

// wasmEntryPointSource reads cmd/wasm/main.go as text.
//
// Grepping a source file is a blunt assertion, and it is the only one available
// here: that file is `//go:build js && wasm`, so a host `go test` cannot link
// it, let alone call it. What it buys is that a Go function and the JavaScript
// name it is reached by cannot part company without a test failing.
func wasmEntryPointSource(t *testing.T) string {
	t.Helper()

	source, err := os.ReadFile(filepath.Join("..", "..", "cmd", "wasm", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/wasm/main.go: %v", err)
	}

	return string(source)
}

// The entry points that are not standards need the same guard. wasmkernel
// exports the logic, cmd/wasm registers the name, and only this file holds the
// two together — a Contours that nothing calls `contours` is a TypeError in the
// browser and nothing at all in CI.
func TestNonStandardEntryPointsAreRegistered(t *testing.T) {
	t.Parallel()

	source := wasmEntryPointSource(t)

	for _, entry := range []string{"transform", "maskFootprints", "contours", "loadTerrain", "clearTerrain"} {
		if !strings.Contains(source, `aconiq.Set("`+entry+`"`) {
			t.Errorf("cmd/wasm/main.go registers no %q entry point", entry)
		}
	}
}

// Registering the entry point is only half of it: the descriptor has to be one
// the framework would accept, or the browser is offered a standard the server
// would refuse to register.
func TestAdvertisedDescriptorsAreValid(t *testing.T) {
	t.Parallel()

	for _, descriptor := range wasmkernel.Descriptors() {
		if err := descriptor.Validate(); err != nil {
			t.Errorf("descriptor %q: %v", descriptor.ID, err)
		}

		if descriptor.EvidenceTier == "" {
			t.Errorf("descriptor %q declares no evidence tier", descriptor.ID)
		}
	}
}

func TestSupportsOnlyWhatIsAdvertised(t *testing.T) {
	t.Parallel()

	if !wasmkernel.Supports("rls19-road") {
		t.Error("the kernel does not claim rls19-road, which it registers an entry point for")
	}

	for _, id := range []string{"cnossos-road", "schall03", "iso9613", "dummy-freefield", ""} {
		if wasmkernel.Supports(id) {
			t.Errorf("the kernel claims %q, which it has no entry point for", id)
		}
	}
}

// StandardsJSON is what the browser parses, so its shape is the contract.
func TestStandardsJSONCarriesTheDescriptorContract(t *testing.T) {
	t.Parallel()

	raw, err := wasmkernel.StandardsJSON()
	if err != nil {
		t.Fatalf("StandardsJSON: %v", err)
	}

	var standards []descriptorjson.Standard
	if err := json.Unmarshal(raw, &standards); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(standards) != 1 {
		t.Fatalf("published %d standards, want 1", len(standards))
	}

	road := standards[0]

	if road.ID != "rls19-road" {
		t.Errorf("id = %q, want rls19-road", road.ID)
	}

	if road.EvidenceTier != "normative" {
		t.Errorf("evidence_tier = %q, want normative", road.EvidenceTier)
	}

	if road.Context != "planning" {
		t.Errorf("context = %q, want planning", road.Context)
	}

	if road.DefaultVersion != "2019" {
		t.Errorf("default_version = %q, want 2019", road.DefaultVersion)
	}

	if len(road.Versions) != 1 || len(road.Versions[0].Profiles) != 1 {
		t.Fatalf("expected exactly one version with one profile, got %d/%v",
			len(road.Versions), road.Versions)
	}

	profile := road.Versions[0].Profiles[0]

	// The Parkplatz half of the module: a browser offered "line" only cannot
	// select an area source it can compute.
	if len(profile.SupportedSourceTypes) != 2 {
		t.Errorf("supported_source_types = %v, want line and area", profile.SupportedSourceTypes)
	}

	surface, ok := parameterByName(profile.Parameters, "surface_type")
	if !ok {
		t.Fatal("surface_type is not published")
	}

	// Seventeen selectable surfaces, each with its own Tabelle 4a row. A subset
	// here is a surface a browser-mode run cannot select.
	if len(surface.Enum) != 17 {
		t.Errorf("surface_type offers %d surfaces, want 17", len(surface.Enum))
	}

	if surface.Unit != "" {
		t.Errorf("surface_type carries unit %q; an enum has none", surface.Unit)
	}
}

// The evidence tier has to survive the crossing. It is the whole of Priority 4:
// the browser used to declare it in a second place, where it could drift from
// the module's own claim.
func TestStandardsJSONMatchesTheModuleDescriptor(t *testing.T) {
	t.Parallel()

	raw, err := wasmkernel.StandardsJSON()
	if err != nil {
		t.Fatalf("StandardsJSON: %v", err)
	}

	want, err := json.Marshal(descriptorjson.FromDescriptors(wasmkernel.Descriptors()))
	if err != nil {
		t.Fatalf("marshal descriptors: %v", err)
	}

	if string(raw) != string(want) {
		t.Error("StandardsJSON does not encode its own descriptor list")
	}
}

func parameterByName(params []descriptorjson.Parameter, name string) (descriptorjson.Parameter, bool) {
	for _, param := range params {
		if param.Name == name {
			return param, true
		}
	}

	return descriptorjson.Parameter{}, false
}
