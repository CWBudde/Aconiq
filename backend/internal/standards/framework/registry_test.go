package framework

import (
	"strings"
	"testing"
)

// descriptorWithID builds a minimal valid descriptor under the given ID.
func descriptorWithID(id string) StandardDescriptor {
	d := descriptorWithTier(EvidenceTierNormative)
	d.ID = id

	return d
}

// NewRegistry is the only place a descriptor is validated before it becomes
// runnable, so an unlabelled or malformed module has to be refused here rather
// than surface as a run that emits authoritative-looking dB(A).
func TestNewRegistryRefusesAnInvalidDescriptor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		descriptor  StandardDescriptor
		wantInError string
	}{
		{
			name:        "no evidence tier",
			descriptor:  descriptorWithTier(""),
			wantInError: "evidence_tier",
		},
		{
			name: "no id",
			descriptor: func() StandardDescriptor {
				d := descriptorWithID("")

				return d
			}(),
			wantInError: "id is required",
		},
		{
			name: "unknown context",
			descriptor: func() StandardDescriptor {
				d := descriptorWithID("ctx")
				d.Context = "assessment"

				return d
			}(),
			wantInError: "context must be",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			registry, err := NewRegistry(descriptorWithID("valid"), testCase.descriptor)
			if err == nil {
				t.Fatalf("expected a registration error, got a registry of %d standards", len(registry.List()))
			}

			if !strings.Contains(err.Error(), testCase.wantInError) {
				t.Fatalf("error %q does not mention %q", err, testCase.wantInError)
			}

			// The whole registration fails; a partially built registry would
			// let the valid neighbours run while the broken module is silently
			// absent.
			if len(registry.List()) != 0 {
				t.Fatalf("expected the zero Registry on failure, got %d standards", len(registry.List()))
			}
		})
	}
}

// Two modules claiming the same ID is a build-time mistake that would otherwise
// resolve to whichever one happened to be registered last.
func TestNewRegistryRefusesADuplicateStandardID(t *testing.T) {
	t.Parallel()

	first := descriptorWithID("rls19-road")
	second := descriptorWithID("rls19-road")
	second.Description = "a second module under the same name"

	_, err := NewRegistry(first, second)
	if err == nil {
		t.Fatal("expected an error for a duplicated standard ID")
	}

	if !strings.Contains(err.Error(), "registered more than once") {
		t.Fatalf("error %q does not report the duplicate", err)
	}
}

// The ID is trimmed on the way in, so " x " and "x" are the same standard and
// must collide rather than both register.
func TestNewRegistryTreatsSurroundingSpaceAsTheSameID(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(descriptorWithID("iso9613"), descriptorWithID("  iso9613  "))
	if err == nil {
		t.Fatal("expected an error: the padded ID names the same standard")
	}
}

// List backs `aconiq status`, the CLI help and GET /api/v1/standards. Map
// iteration order is deliberately not allowed to reach any of them.
func TestRegistryListIsSortedAndIndependentOfRegistrationOrder(t *testing.T) {
	t.Parallel()

	forward, err := NewRegistry(
		descriptorWithID("schall03"),
		descriptorWithID("iso9613"),
		descriptorWithID("rls19-road"),
		descriptorWithID("beb-exposure"),
	)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	backward, err := NewRegistry(
		descriptorWithID("beb-exposure"),
		descriptorWithID("rls19-road"),
		descriptorWithID("iso9613"),
		descriptorWithID("schall03"),
	)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	want := []string{"beb-exposure", "iso9613", "rls19-road", "schall03"}

	for label, registry := range map[string]Registry{"forward": forward, "backward": backward} {
		listed := registry.List()
		if len(listed) != len(want) {
			t.Fatalf("%s: listed %d standards, want %d", label, len(listed), len(want))
		}

		for i, descriptor := range listed {
			if descriptor.ID != want[i] {
				t.Fatalf("%s: List()[%d].ID = %q, want %q", label, i, descriptor.ID, want[i])
			}
		}
	}

	// Repeated calls must agree too: List() rebuilds its slice from a map each
	// time.
	for range 8 {
		first := forward.List()
		for i, descriptor := range first {
			if descriptor.ID != want[i] {
				t.Fatalf("repeated List() drifted at %d: %q", i, descriptor.ID)
			}
		}
	}
}

// An empty registry lists nothing rather than panicking on its nil map.
func TestZeroRegistryListsNothingAndResolvesNothing(t *testing.T) {
	t.Parallel()

	var registry Registry

	if got := registry.List(); len(got) != 0 {
		t.Fatalf("zero Registry listed %d standards", len(got))
	}

	_, err := registry.Resolve("rls19-road", "", "")
	if err == nil {
		t.Fatal("expected an error resolving against an empty registry")
	}
}

func TestRegistryResolve(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(descriptorWithID("rls19-road"), descriptorWithID("schall03"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	cases := []struct {
		name        string
		standardID  string
		version     string
		profile     string
		wantInError string
	}{
		{name: "empty id", standardID: "", wantInError: "standard id is required"},
		{name: "blank id", standardID: "   ", wantInError: "standard id is required"},
		{name: "unknown standard", standardID: "cnossos-road", wantInError: `unknown standard "cnossos-road"`},
		{name: "unknown version", standardID: "rls19-road", version: "v2", wantInError: `does not provide version "v2"`},
		{name: "unknown profile", standardID: "rls19-road", profile: "detailed", wantInError: `does not provide profile "detailed"`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := registry.Resolve(testCase.standardID, testCase.version, testCase.profile)
			if err == nil {
				t.Fatalf("expected an error, resolved %#v", resolved)
			}

			if !strings.Contains(err.Error(), testCase.wantInError) {
				t.Fatalf("error %q does not contain %q", err, testCase.wantInError)
			}

			if resolved.StandardID != "" || resolved.Version != "" || resolved.Profile != "" {
				t.Fatalf("expected the zero ResolvedProfile alongside the error, got %#v", resolved)
			}
		})
	}
}

// A standard ID typed with surrounding space — which is what a shell here-doc
// or a JSON request body can carry — must resolve to the same module.
func TestRegistryResolveTrimsTheStandardID(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(descriptorWithID("iso9613"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	resolved, err := registry.Resolve("  iso9613\t", "", "")
	if err != nil {
		t.Fatalf("resolve padded standard ID: %v", err)
	}

	if resolved.StandardID != "iso9613" {
		t.Fatalf("StandardID = %q, want %q", resolved.StandardID, "iso9613")
	}
}

// Resolving carries the identity a run has to record in provenance.json.
func TestRegistryResolveCarriesIdentityAndTier(t *testing.T) {
	t.Parallel()

	descriptor := descriptorWithID("beb-exposure")
	descriptor.EvidenceTier = EvidenceTierPreview
	descriptor.Description = "exposure aggregation over preview-grade levels"

	registry, err := NewRegistry(descriptor)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	resolved, err := registry.Resolve("beb-exposure", "", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if resolved.StandardID != "beb-exposure" {
		t.Fatalf("StandardID = %q", resolved.StandardID)
	}

	if resolved.StandardDescription != descriptor.Description {
		t.Fatalf("StandardDescription = %q, want %q", resolved.StandardDescription, descriptor.Description)
	}

	if resolved.Context != StandardContextPlanning {
		t.Fatalf("Context = %q, want %q", resolved.Context, StandardContextPlanning)
	}

	if resolved.EvidenceTier != EvidenceTierPreview {
		t.Fatalf("EvidenceTier = %q, want %q", resolved.EvidenceTier, EvidenceTierPreview)
	}

	if resolved.Version != "v1" || resolved.Profile != "default" {
		t.Fatalf("resolved %q/%q, want v1/default", resolved.Version, resolved.Profile)
	}
}
