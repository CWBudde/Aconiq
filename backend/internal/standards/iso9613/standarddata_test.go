package iso9613

import (
	"testing"

	"github.com/aconiq/backend/internal/standards/framework"
)

// Pinned deliberately: ISO 9613-2 Table 2 and the A-weighting values are fixed
// published data. Do not update these constants to make the test pass.
const pinnedISO9613DataDigest = "afb9ce29de5a8ee2b89823189c1b0ede218845eea37cec68300dc687b750e5bb"

var pinnedISO9613TableDigests = map[string]string{
	"iso9613/a-bewertung":                "d7cd2d09da2168762f74b31803abb1157d5ce82f0f5768fc0d8b288735b3679c",
	"iso9613/oktavband-mittenfrequenzen": "0c0bef4fbdf2aebeb13a38e51f2320387cb2b27c65452b69886de76bfa295cde",
	"iso9613/tabelle-2-luftabsorption":   "b5c3890429911ade6e4a924abe62138f1a878933d3f6392f19f5602cd47eede9",
}

func TestStandardDataDigestIsPinned(t *testing.T) {
	digest, err := StandardData().Digest(StandardID, framework.EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if digest.Digest != pinnedISO9613DataDigest {
		t.Errorf("iso9613 standard data digest = %s, pinned %s", digest.Digest, pinnedISO9613DataDigest)
	}

	seen := make(map[string]struct{}, len(digest.Tables))

	for _, table := range digest.Tables {
		seen[table.Name] = struct{}{}

		want, pinned := pinnedISO9613TableDigests[table.Name]
		if !pinned {
			t.Errorf("table %q is not pinned; add it to pinnedISO9613TableDigests", table.Name)

			continue
		}

		if table.Digest != want {
			t.Errorf("table %q digest = %s, pinned %s", table.Name, table.Digest, want)
		}
	}

	for name := range pinnedISO9613TableDigests {
		if _, ok := seen[name]; !ok {
			t.Errorf("pinned table %q is no longer carried by the module", name)
		}
	}
}
