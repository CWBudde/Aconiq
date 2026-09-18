package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A unit map that does not match the channel list is the state the field was
// introduced to remove, so the container has to refuse it rather than write a
// file whose values are half labelled. The error names the offending channel
// because the writer is a standards module with a dozen indicators: "units do
// not match" would leave the author to find which one.
func TestReceiverTableRefusesUnitsThatDoNotMatchTheIndicators(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		units    map[string]string
		wantName string
	}{
		"missing unit for a declared indicator": {
			units:    map[string]string{"Lden": UnitDecibel},
			wantName: "Lnight",
		},
		"empty unit for a declared indicator": {
			units:    map[string]string{"Lden": UnitDecibel, "Lnight": ""},
			wantName: "Lnight",
		},
		// What a renamed indicator leaves behind: the unit outlives the
		// channel it belonged to, and nothing else would notice.
		"unit for an indicator that is not declared": {
			units:    map[string]string{"Lden": UnitDecibel, "Lnight": UnitDecibel, "Lday": UnitDecibel},
			wantName: "Lday",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			table := ReceiverTable{
				IndicatorOrder: []string{"Lden", "Lnight"},
				Units:          testCase.units,
			}

			err := table.Validate()
			if err == nil {
				t.Fatalf("expected %s to be refused", name)
			}

			if !strings.Contains(err.Error(), testCase.wantName) {
				t.Fatalf("error %q does not name the offending indicator %q", err, testCase.wantName)
			}
		})
	}
}

func TestNewRasterRefusesUnitsThatDoNotMatchTheBands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		bandNames []string
		bands     int
		units     map[string]string
		wantName  string
	}{
		"missing unit for a declared band": {
			bandNames: []string{"Lden", "Lnight"},
			bands:     2,
			units:     map[string]string{"Lden": UnitDecibel},
			wantName:  "Lnight",
		},
		"empty unit for a declared band": {
			bandNames: []string{"Lden", "Lnight"},
			bands:     2,
			units:     map[string]string{"Lden": UnitDecibel, "Lnight": ""},
			wantName:  "Lnight",
		},
		"unit for a band that is not declared": {
			bandNames: []string{"Lden"},
			bands:     1,
			units:     map[string]string{"Lden": UnitDecibel, "Lnight": UnitDecibel},
			wantName:  "Lnight",
		},
		// An unnamed raster has no key to hang a unit on, so a unit here
		// labels nothing and could only mislead whoever reads the sidecar.
		"unit on a raster that names no bands": {
			bands:    1,
			units:    map[string]string{"Lden": UnitDecibel},
			wantName: "Lden",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRaster(RasterMetadata{
				Width: 2, Height: 2, Bands: testCase.bands, NoData: -9999,
				Units: testCase.units, BandNames: testCase.bandNames,
			})
			if err == nil {
				t.Fatalf("expected %s to be refused", name)
			}

			if !strings.Contains(err.Error(), testCase.wantName) {
				t.Fatalf("error %q does not name the offending band %q", err, testCase.wantName)
			}
		})
	}
}

// Runs written before the unit was per channel are still on disk and must open
// without being migrated: their scalar was true of every indicator, so
// expanding it across the list loses nothing.
//
// Through the loader and not through json.Unmarshal, because the loader is
// where the expansion lives: a legacy spelling is a property of a file, and a
// method on ReceiverTable would be promoted into anything embedding it.
func TestReceiverTableReadsALegacyScalarUnit(t *testing.T) {
	t.Parallel()

	table := loadTableJSON(t, `{
		"indicator_order": ["Lden", "Lnight"],
		"unit": "dB",
		"records": []
	}`)

	for _, indicator := range table.IndicatorOrder {
		if table.Units[indicator] != "dB" {
			t.Fatalf("unit for %s = %q, want dB", indicator, table.Units[indicator])
		}
	}

	if len(table.Units) != 2 {
		t.Fatalf("units = %v, want one entry per indicator", table.Units)
	}
}

// loadTableJSON writes a receiver table document and reads it back the way
// every caller does.
func loadTableJSON(t *testing.T, document string) ReceiverTable {
	t.Helper()

	path := filepath.Join(t.TempDir(), "receivers.json")

	err := os.WriteFile(path, []byte(document), 0o600)
	if err != nil {
		t.Fatalf("write table document: %v", err)
	}

	table, err := LoadReceiverTableJSON(path)
	if err != nil {
		t.Fatalf("load table document: %v", err)
	}

	return table
}

// A document carrying both is a per-indicator table written by a tool that
// also filled the old key for older readers. The per-indicator map is the
// richer of the two, so the scalar must not flatten it back.
func TestReceiverTableKeepsPerIndicatorUnitsOverALegacyScalar(t *testing.T) {
	t.Parallel()

	table := loadTableJSON(t, `{
		"indicator_order": ["Lden", "dwellings"],
		"unit": "mixed",
		"units": {"Lden": "dB", "dwellings": "count"},
		"records": []
	}`)

	if table.Units["Lden"] != UnitDecibel || table.Units["dwellings"] != UnitCount {
		t.Fatalf("units = %v, want the per-indicator map the document carries", table.Units)
	}
}

func TestRasterMetadataReadsALegacyScalarUnit(t *testing.T) {
	t.Parallel()

	var meta RasterMetadata

	err := json.Unmarshal([]byte(`{
		"width": 2, "height": 2, "bands": 2, "nodata": -9999,
		"unit": "dB(A)",
		"band_names": ["LrDay", "LrNight"]
	}`), &meta)
	if err != nil {
		t.Fatalf("decode legacy sidecar: %v", err)
	}

	for _, band := range meta.BandNames {
		if meta.Units[band] != "dB(A)" {
			t.Fatalf("unit for %s = %q, want dB(A)", band, meta.Units[band])
		}
	}

	if len(meta.Units) != 2 {
		t.Fatalf("units = %v, want one entry per band", meta.Units)
	}
}

func TestRasterMetadataKeepsPerBandUnitsOverALegacyScalar(t *testing.T) {
	t.Parallel()

	var meta RasterMetadata

	err := json.Unmarshal([]byte(`{
		"width": 2, "height": 2, "bands": 2, "nodata": -9999,
		"unit": "mixed",
		"units": {"Lden": "dB", "dwellings": "count"},
		"band_names": ["Lden", "dwellings"]
	}`), &meta)
	if err != nil {
		t.Fatalf("decode sidecar: %v", err)
	}

	if meta.Units["Lden"] != UnitDecibel || meta.Units["dwellings"] != UnitCount {
		t.Fatalf("units = %v, want the per-band map the document carries", meta.Units)
	}
}

// The one legacy sidecar the expansion cannot serve: with no band names there
// is no key to file the scalar under, and inventing one would be a label the
// raster never carried. It comes back unitless, which NewRaster accepts.
func TestRasterMetadataDropsALegacyScalarUnitWhenNoBandIsNamed(t *testing.T) {
	t.Parallel()

	var meta RasterMetadata

	err := json.Unmarshal([]byte(`{
		"width": 2, "height": 2, "bands": 1, "nodata": -9999,
		"unit": "dB"
	}`), &meta)
	if err != nil {
		t.Fatalf("decode legacy sidecar: %v", err)
	}

	if len(meta.Units) != 0 {
		t.Fatalf("units = %v, want none on a sidecar that names no band", meta.Units)
	}

	_, err = NewRaster(meta)
	if err != nil {
		t.Fatalf("a legacy unnamed raster must still open: %v", err)
	}
}

func TestUniformUnitsDeclaresTheUnitForEveryChannel(t *testing.T) {
	t.Parallel()

	units := UniformUnits([]string{"Lden", "Lnight"}, UnitDecibel)

	want := map[string]string{"Lden": UnitDecibel, "Lnight": UnitDecibel}
	if len(units) != len(want) {
		t.Fatalf("units = %v, want %v", units, want)
	}

	for name, unit := range want {
		if units[name] != unit {
			t.Fatalf("unit for %s = %q, want %q", name, units[name], unit)
		}
	}

	// No channels is not an error here — it is a container whose channel list
	// its own Validate() rejects, and that is where the complaint belongs.
	if len(UniformUnits(nil, UnitDecibel)) != 0 {
		t.Fatal("no channels must yield no units")
	}
}

// Metadata() promises a value nobody can write back through, and a shallow
// struct copy would hand out the container's own map.
func TestCopyUnitsIsIndependentOfTheOriginal(t *testing.T) {
	t.Parallel()

	original := map[string]string{"Lden": UnitDecibel}

	copied := CopyUnits(original)
	copied["Lden"] = UnitCount
	copied["Lnight"] = UnitDecibel

	if original["Lden"] != UnitDecibel {
		t.Fatalf("original unit = %q, want it untouched by the copy", original["Lden"])
	}

	if _, exists := original["Lnight"]; exists {
		t.Fatal("a key added to the copy reached the original")
	}

	// A container that declares no units keeps declaring none: an empty map
	// where there was nothing would be a different value in the sidecar.
	if CopyUnits(nil) != nil {
		t.Fatal("copying no units must yield no units")
	}
}
