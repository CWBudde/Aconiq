package cli

import (
	"regexp"
	"testing"

	"github.com/aconiq/backend/internal/standards"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/schall03"
)

var hexDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// TestBuildRunStandardDataCoversEveryRegisteredStandard is the gate that stops a
// new standards module from shipping without declaring what coefficient data it
// carries. Only dummy-freefield is allowed to carry none: it computes from its
// run parameters alone.
func TestBuildRunStandardDataCoversEveryRegisteredStandard(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	digests := make(map[string]string)

	for _, descriptor := range registry.List() {
		resolved, err := registry.Resolve(descriptor.ID, "", "")
		if err != nil {
			t.Fatalf("resolve %s: %v", descriptor.ID, err)
		}

		ref, err := buildRunStandardData(resolved)
		if err != nil {
			t.Fatalf("build standard data for %s: %v", descriptor.ID, err)
		}

		if descriptor.ID == "dummy-freefield" {
			if !ref.IsZero() {
				t.Fatalf("dummy-freefield carries no coefficient data but reported %#v", ref)
			}

			continue
		}

		if ref.IsZero() {
			t.Fatalf("standard %s reports no standard data; declare its tables in a StandardData() function", descriptor.ID)
		}

		if ref.Algorithm != framework.StandardDataDigestAlgorithm {
			t.Errorf("standard %s digest algorithm = %q", descriptor.ID, ref.Algorithm)
		}

		if !hexDigestPattern.MatchString(ref.Digest) {
			t.Errorf("standard %s digest %q is not a sha256 hex digest", descriptor.ID, ref.Digest)
		}

		if ref.EvidenceTier != string(descriptor.EvidenceTier) {
			t.Errorf("standard %s digest tier = %q, descriptor tier = %q", descriptor.ID, ref.EvidenceTier, descriptor.EvidenceTier)
		}

		if len(ref.Tables) == 0 {
			t.Errorf("standard %s declares a digest but no tables", descriptor.ID)
		}

		for i, table := range ref.Tables {
			if table.Name == "" || !hexDigestPattern.MatchString(table.Digest) {
				t.Errorf("standard %s table %d is malformed: %#v", descriptor.ID, i, table)
			}

			if i > 0 && ref.Tables[i-1].Name >= table.Name {
				t.Errorf("standard %s tables are not sorted by name: %#v", descriptor.ID, ref.Tables)
			}
		}

		digests[descriptor.ID] = ref.Digest
	}

	// bub-rail and bub-industry are aliases over the cnossos packages and share
	// their coefficients, so they legitimately share a table set; the standard
	// ID is part of the hash, so their digests still differ.
	seen := make(map[string]string, len(digests))

	for id, digest := range digests {
		if other, clash := seen[digest]; clash {
			t.Errorf("standards %s and %s share a digest", other, id)
		}

		seen[digest] = id
	}
}

// TestBuildRunStandardDataFoldsInTheEvidenceTier pins the reason the tier is an
// input to the hash rather than a neighbouring field: re-tiering a module
// changes what its numbers mean, so two runs across that change must not look
// interchangeable.
func TestBuildRunStandardDataFoldsInTheEvidenceTier(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	resolved, err := registry.Resolve(schall03.StandardID, "", "")
	if err != nil {
		t.Fatalf("resolve schall03: %v", err)
	}

	normative, err := buildRunStandardData(resolved)
	if err != nil {
		t.Fatalf("build standard data: %v", err)
	}

	resolved.EvidenceTier = framework.EvidenceTierPreview

	preview, err := buildRunStandardData(resolved)
	if err != nil {
		t.Fatalf("build standard data: %v", err)
	}

	if normative.Digest == preview.Digest {
		t.Fatal("the evidence tier does not participate in the standard data digest")
	}

	if preview.EvidenceTier != string(framework.EvidenceTierPreview) {
		t.Fatalf("unexpected recorded tier: %q", preview.EvidenceTier)
	}
}
