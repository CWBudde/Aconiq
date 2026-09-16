package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
)

func writeProvenanceWithMetadata(t *testing.T, metadata map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "provenance.json")

	payload, err := json.Marshal(project.ProvenanceManifest{Metadata: metadata})
	if err != nil {
		t.Fatalf("marshal provenance: %v", err)
	}

	err = os.WriteFile(path, payload, 0o600)
	if err != nil {
		t.Fatalf("write provenance: %v", err)
	}

	return path
}

func TestComputeCRSFromProvenance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "a run that recorded one",
			path: writeProvenanceWithMetadata(t, map[string]string{provenanceComputeCRSKey: "EPSG:25832"}),
			want: "EPSG:25832",
		},
		{
			name: "a run written before the key existed",
			path: writeProvenanceWithMetadata(t, map[string]string{"evidence_tier": "normative"}),
			want: "",
		},
		{"no provenance path", "", ""},
		{"a path that is not there", filepath.Join(t.TempDir(), "missing.json"), ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := computeCRSFromProvenance(tc.path)
			if got != tc.want {
				t.Fatalf("computeCRSFromProvenance = %q, want %q", got, tc.want)
			}
		})
	}
}

// The GIS formats carry the CRS the results are actually in. A bundle from a
// run that had to project a geographic project CRS holds a receiver table, a
// raster and contours in metres; labelling them EPSG:4326 would put them off
// the coast of Africa in any GIS that believed the label.
func TestFormatExportContextLabelsResultsWithTheComputeCRS(t *testing.T) {
	t.Parallel()

	ctx := newFormatExportContext(
		t.TempDir(), "EPSG:4326", "EPSG:25832",
		copiedRunResults{}, 5.0, "",
	)

	if ctx.resultsCRS != "EPSG:25832" || ctx.resultsEPSG != 25832 {
		t.Fatalf("results labelled %s/%d, want EPSG:25832/25832", ctx.resultsCRS, ctx.resultsEPSG)
	}

	// The model GeoJSON in the bundle is the project's stored model and did
	// not move, so it keeps the project's own label.
	if ctx.projectCRS != "EPSG:4326" || ctx.epsgCode != 4326 {
		t.Fatalf("model labelled %s/%d, want EPSG:4326/4326", ctx.projectCRS, ctx.epsgCode)
	}
}

func TestFormatExportContextFallsBackToTheProjectCRS(t *testing.T) {
	t.Parallel()

	for _, resultsCRS := range []string{"", "   "} {
		ctx := newFormatExportContext(
			t.TempDir(), "EPSG:25832", resultsCRS,
			copiedRunResults{}, 5.0, "",
		)

		if ctx.resultsCRS != "EPSG:25832" || ctx.resultsEPSG != 25832 {
			t.Fatalf("with resultsCRS %q the results were labelled %s/%d, want the project's EPSG:25832/25832",
				resultsCRS, ctx.resultsCRS, ctx.resultsEPSG)
		}
	}
}
