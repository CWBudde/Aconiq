package standards

import (
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/standards/framework"
)

// expectedEvidenceTiers is the authoritative tier assignment from PLAN.md
// ("Standards evidence tiers"). It is exhaustive on purpose: registering a new
// module without a deliberate tier decision must fail this test.
var expectedEvidenceTiers = map[string]framework.EvidenceTier{
	"beb-exposure":     framework.EvidenceTierPreview,
	"bub-industry":     framework.EvidenceTierScaffold,
	"bub-rail":         framework.EvidenceTierScaffold,
	"bub-road":         framework.EvidenceTierScaffold,
	"buf-aircraft":     framework.EvidenceTierScaffold,
	"cnossos-aircraft": framework.EvidenceTierScaffold,
	"cnossos-industry": framework.EvidenceTierScaffold,
	"cnossos-rail":     framework.EvidenceTierScaffold,
	"cnossos-road":     framework.EvidenceTierScaffold,
	"dummy-freefield":  framework.EvidenceTierTestFixture,
	"iso9613":          framework.EvidenceTierNormative,
	"rls19-road":       framework.EvidenceTierNormative,
	"schall03":         framework.EvidenceTierNormative,
}

// removedConformanceClaims are the exact description openings that used to
// assert an implementation the module does not have. None of them may come back.
var removedConformanceClaims = []string{
	"CNOSSOS-EU road preview module",
	"CNOSSOS-EU rail preview module",
	"CNOSSOS-EU industry preview module",
	"CNOSSOS-EU aircraft preview module",
	"BUB road mapping baseline",
	"BUB rail mapping baseline",
	"BUB industry mapping baseline",
	"BUF aircraft mapping baseline",
}

func TestRegistryDeclaresExpectedEvidenceTiers(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	descriptors := registry.List()
	if len(descriptors) != len(expectedEvidenceTiers) {
		t.Fatalf("expected %d registered standards, got %d", len(expectedEvidenceTiers), len(descriptors))
	}

	seen := make(map[string]struct{}, len(descriptors))

	for _, descriptor := range descriptors {
		expected, known := expectedEvidenceTiers[descriptor.ID]
		if !known {
			t.Fatalf("standard %q is registered but has no declared evidence tier", descriptor.ID)
		}

		if descriptor.EvidenceTier != expected {
			t.Fatalf("standard %q: expected evidence tier %q, got %q", descriptor.ID, expected, descriptor.EvidenceTier)
		}

		seen[descriptor.ID] = struct{}{}
	}

	for id := range expectedEvidenceTiers {
		if _, ok := seen[id]; !ok {
			t.Fatalf("standard %q is expected but not registered", id)
		}
	}
}

func TestRegistryResolvePropagatesEvidenceTier(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	for id, expected := range expectedEvidenceTiers {
		resolved, err := registry.Resolve(id, "", "")
		if err != nil {
			t.Fatalf("resolve %q: %v", id, err)
		}

		if resolved.EvidenceTier != expected {
			t.Fatalf("standard %q: expected resolved tier %q, got %q", id, expected, resolved.EvidenceTier)
		}

		if resolved.EvidenceTier.Headline() == "" {
			t.Fatalf("standard %q: evidence tier %q has no headline", id, resolved.EvidenceTier)
		}
	}
}

func TestDescriptionsDoNotOverclaim(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	for _, descriptor := range registry.List() {
		if strings.TrimSpace(descriptor.Description) == "" {
			t.Fatalf("standard %q has an empty description", descriptor.ID)
		}

		if descriptor.EvidenceTier != framework.EvidenceTierScaffold {
			continue
		}

		if !descriptor.EvidenceTier.RequiresExperimentalOptIn() {
			t.Fatalf("standard %q: scaffold tier must require an experimental opt-in", descriptor.ID)
		}

		if !strings.HasPrefix(descriptor.Description, "Scaffold ") {
			t.Fatalf("standard %q: scaffold description must say so first: %q", descriptor.ID, descriptor.Description)
		}

		for _, claim := range removedConformanceClaims {
			if strings.Contains(descriptor.Description, claim) {
				t.Fatalf("standard %q description reasserts %q: %q", descriptor.ID, claim, descriptor.Description)
			}
		}
	}
}

func TestAircraftDescriptionsDiscloseMissingCnossosMethod(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	// CNOSSOS-EU (Directive 2015/996 Annex II) covers road, rail and industry
	// only. Both aircraft ids are kept for manifest compatibility, so the
	// description is where that has to be said.
	for _, id := range []string{"cnossos-aircraft", "buf-aircraft"} {
		resolved, err := registry.Resolve(id, "", "")
		if err != nil {
			t.Fatalf("resolve %q: %v", id, err)
		}

		if !strings.Contains(resolved.StandardDescription, "defines no aircraft method") {
			t.Fatalf("standard %q must disclose that CNOSSOS-EU defines no aircraft method: %q", id, resolved.StandardDescription)
		}
	}
}

// unitSuffixes maps a parameter-name suffix to the unit symbol a parameter
// carrying it must declare. It is ordered longest-first so that
// air_absorption_db_per_km resolves to dB/km rather than to 1/km or dB.
//
// The table is deliberately not exhaustive over units: a parameter may carry a
// unit without advertising it in its name — traffic_day_pkw is vehicles per
// hour. What it pins is the reverse direction, that a name promising a unit
// delivers it, so a new *_db parameter cannot ship unitless.
var unitSuffixes = []struct {
	suffix string
	unit   string
}{
	{"_db_per_km", "dB/km"},
	{"_trains_per_hour", "1/h"},
	{"_per_hour", "1/h"},
	{"_per_km", "1/km"},
	{"_percent", "%"},
	{"_kph", "km/h"},
	{"_vph", "1/h"},
	{"_deg", "°"},
	{"_db", "dB"},
	{"_m", "m"},
	{"_c", "°C"},
}

// unitFromName returns the unit a parameter name promises, and whether it
// promises one at all.
func unitFromName(name string) (string, bool) {
	for _, candidate := range unitSuffixes {
		if strings.HasSuffix(name, candidate.suffix) {
			return candidate.unit, true
		}
	}

	return "", false
}

func TestParameterUnitsMatchNameSuffixes(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	descriptors := registry.List()
	if len(descriptors) == 0 {
		t.Fatal("standards registry is empty")
	}

	checked := 0

	for _, descriptor := range descriptors {
		for _, version := range descriptor.Versions {
			for _, profile := range version.Profiles {
				for _, parameter := range profile.ParameterSchema.Parameters {
					expected, implied := unitFromName(parameter.Name)
					if !implied {
						continue
					}

					checked++

					if parameter.Unit != expected {
						t.Errorf(
							"standard %q version %q profile %q: parameter %q must declare unit %q, got %q",
							descriptor.ID, version.Name, profile.Name, parameter.Name, expected, parameter.Unit,
						)
					}
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("no parameter name implied a unit; the suffix table has stopped matching the schemas")
	}
}

// TestParameterUnitsSurviveProfileResolution pins the clone path. A unit a
// module declares but Resolve drops would reach no consumer, because the CLI and
// the HTTP API both read a resolved profile rather than the raw descriptor.
func TestParameterUnitsSurviveProfileResolution(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	for id := range expectedEvidenceTiers {
		resolved, err := registry.Resolve(id, "", "")
		if err != nil {
			t.Fatalf("resolve %q: %v", id, err)
		}

		for _, parameter := range resolved.RunParameterSchema.Parameters {
			expected, implied := unitFromName(parameter.Name)
			if implied && parameter.Unit != expected {
				t.Errorf("standard %q: resolved parameter %q lost its unit: want %q, got %q", id, parameter.Name, expected, parameter.Unit)
			}
		}
	}
}
