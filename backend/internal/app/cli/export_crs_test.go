package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	unreadable := filepath.Join(t.TempDir(), "corrupt.json")

	err := os.WriteFile(unreadable, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatalf("write corrupt provenance: %v", err)
	}

	cases := []struct {
		name    string
		path    string
		want    string
		wantErr bool
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
		{name: "no provenance path", path: "", want: ""},
		// The two below must not answer "" — that is the legacy-run sentinel,
		// and reusing it here would label a geographic run's UTM results
		// EPSG:4326 on the strength of a file nothing could read.
		{name: "a path that is not there", path: filepath.Join(t.TempDir(), "missing.json"), wantErr: true},
		{name: "a manifest that does not decode", path: unreadable, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := computeCRSFromProvenance(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("computeCRSFromProvenance = %q with no error, want an error", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("computeCRSFromProvenance: %v", err)
			}

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

	ctx := mustFormatExportContext(
		t, t.TempDir(), "EPSG:4326", "EPSG:25832",
		copiedRunResults{},
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
		ctx := mustFormatExportContext(
			t, t.TempDir(), "EPSG:25832", resultsCRS,
			copiedRunResults{},
		)

		if ctx.resultsCRS != "EPSG:25832" || ctx.resultsEPSG != 25832 {
			t.Fatalf("with resultsCRS %q the results were labelled %s/%d, want the project's EPSG:25832/25832",
				resultsCRS, ctx.resultsCRS, ctx.resultsEPSG)
		}
	}
}

// newFormatExportContext used to read both CRS strings with
// `_, _ = fmt.Sscanf(crs, "EPSG:%d", &code)` — the error discarded twice over.
// A project CRS that is a typo therefore became EPSG code 0, which a
// GeoPackage spells "Undefined geographic SRS", and the export succeeded with
// every geometry and every metadata row claiming a CRS the model is not in.
//
// The reason is held on the context rather than raised in the constructor,
// which is what the rest of this file does: a format that never reads an EPSG
// code must not fail over one it does not use.
func TestExportRefusesACRSItCannotParse(t *testing.T) {
	t.Parallel()

	const badCRS = "EPSG:two-five-eight-three-two"

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(`{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"kind": "receiver", "height_m": 4},
				"geometry": {"type": "Point", "coordinates": [100, 200]}
			}
		]
	}`), 0o600)
	if err != nil {
		t.Fatalf("write the model: %v", err)
	}

	ctx := newFormatExportContext(dir, badCRS, badCRS, copiedRunResults{}, 5.0, modelPath)

	err = ctx.exportGeoPackage(map[string][]string{})
	if err == nil {
		t.Fatal("a CRS that is not a CRS exported as srs_id 0 without complaint")
	}

	if !strings.Contains(err.Error(), "two-five-eight-three-two") {
		t.Fatalf("the refusal does not name the CRS it could not read: %v", err)
	}
}

// The other half of that contract. An absent CRS is not a typo: it means no
// projection was declared, it still resolves to EPSG code 0, and it must not
// start refusing exports that work today.
func TestExportStillAcceptsAnAbsentCRS(t *testing.T) {
	t.Parallel()

	ctx := newFormatExportContext(t.TempDir(), "", "", copiedRunResults{}, 5.0, "")

	if ctx.projectCRSErr != nil || ctx.resultsCRSErr != nil {
		t.Fatalf("an absent CRS was recorded as unreadable: %v / %v", ctx.projectCRSErr, ctx.resultsCRSErr)
	}

	if ctx.epsgCode != 0 || ctx.resultsEPSG != 0 {
		t.Fatalf("epsgCode = %d, resultsEPSG = %d, want 0 and 0", ctx.epsgCode, ctx.resultsEPSG)
	}
}

// The two CRS label different files and no format reads both, so an
// unreadable one must cost only the formats that carry it.
//
// They can diverge in practice: the results CRS is the metric one a
// geographic project was projected into before computing, and it is read from
// a run's metadata rather than from the manifest. A single shared error field
// made an unreadable project CRS refuse the contours, which are labelled with
// the results CRS and never look at the project's.
func TestExportRefusesOnlyTheFormatsThatCarryTheUnreadableCRS(t *testing.T) {
	t.Parallel()

	const badCRS = "EPSG:not-a-number"

	t.Run("a bad project CRS leaves the contours alone", func(t *testing.T) {
		t.Parallel()

		ctx := newFormatExportContext(t.TempDir(), badCRS, "EPSG:25832", copiedRunResults{}, 5.0, "")

		if ctx.projectCRSErr == nil {
			t.Fatal("the unreadable project CRS was not recorded")
		}

		if ctx.resultsCRSErr != nil {
			t.Fatalf("the valid results CRS was recorded as unreadable: %v", ctx.resultsCRSErr)
		}

		if ctx.resultsEPSG != 25832 {
			t.Fatalf("resultsEPSG = %d, want 25832 — a bad project CRS must not stop the results CRS being read", ctx.resultsEPSG)
		}

		// No raster, so this returns before it would need the results CRS;
		// what matters is that it does not refuse over the project's.
		err := ctx.exportContourGeoPackage(map[string][]string{})
		if err != nil {
			t.Fatalf("the contour export refused over a CRS it does not carry: %v", err)
		}
	})

	t.Run("a bad results CRS leaves the model GeoPackage alone", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modelPath := filepath.Join(dir, "model.geojson")

		err := os.WriteFile(modelPath, []byte(`{
			"type": "FeatureCollection",
			"features": [
				{
					"type": "Feature",
					"properties": {"kind": "receiver", "height_m": 4},
					"geometry": {"type": "Point", "coordinates": [100, 200]}
				}
			]
		}`), 0o600)
		if err != nil {
			t.Fatalf("write the model: %v", err)
		}

		ctx := newFormatExportContext(dir, "EPSG:25832", badCRS, copiedRunResults{}, 5.0, modelPath)

		if ctx.resultsCRSErr == nil {
			t.Fatal("the unreadable results CRS was not recorded")
		}

		if ctx.projectCRSErr != nil {
			t.Fatalf("the valid project CRS was recorded as unreadable: %v", ctx.projectCRSErr)
		}

		// No receiver table, so the only thing this writes is model.gpkg,
		// which is labelled with the project CRS.
		err = ctx.exportGeoPackage(map[string][]string{})
		if err != nil {
			t.Fatalf("the model GeoPackage refused over a CRS it does not carry: %v", err)
		}
	})
}
