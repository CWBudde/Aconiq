package soundplanimport

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestParseRunDataFiles covers the `RunData=` line: the input files SoundPLAN
// handed the kernel, quoted individually and then quoted again as a whole.
//
// It is what tells two otherwise identical result runs apart, so it is pinned
// here rather than only against the licensed fixture.
func TestParseRunDataFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "doubled quotes",
			raw:  `""GeoObjs.geo" "GeoRail.geo" "rdgm0001.dgm""`,
			want: []string{"GeoObjs.geo", "GeoRail.geo", "rdgm0001.dgm"},
		},
		{
			name: "with barrier geometry",
			raw:  `""GeoObjs.geo" "GeoWand.geo" "GeoRail.geo" "rdgm0001.dgm""`,
			want: []string{"GeoObjs.geo", "GeoWand.geo", "GeoRail.geo", "rdgm0001.dgm"},
		},
		{
			name: "single unquoted value",
			raw:  `GeoObjs.geo`,
			want: []string{"GeoObjs.geo"},
		},
		{
			name: "name containing a space",
			raw:  `""My Geometry.geo""`,
			want: []string{"My Geometry.geo"},
		},
		{name: "empty", raw: ``, want: nil},
		{name: "only quotes", raw: `""`, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := parseRunDataFiles(test.raw)
			if !slices.Equal(got, test.want) {
				t.Fatalf("parseRunDataFiles(%q) = %v, want %v", test.raw, got, test.want)
			}
		})
	}
}

// TestGridMapLayout covers the `GNM<spacing>:<height>` token in `RunCommands`.
//
// It is the second signal that tells two grid-map runs apart: the geometry list
// separates a project's grid maps into the ones computed with the noise barrier
// and the ones without, and only this height separates the pair that remains.
// Absence and nonsense both have to stay "unknown" — a zero height would look
// like a grid evaluated at ground level and would match nothing, or worse, a
// project that records no height at all.
func TestGridMapLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runCommands string
		wantOK      bool
		wantSpacing float64
		wantHeight  float64
	}{
		{name: "plain", runCommands: "GNM5:4", wantOK: true, wantSpacing: 5, wantHeight: 4},
		{name: "two metre grid", runCommands: "GNM5:2", wantOK: true, wantSpacing: 5, wantHeight: 2},
		{name: "german decimals", runCommands: "GNM2,5:1,5", wantOK: true, wantSpacing: 2.5, wantHeight: 1.5},
		{name: "dot decimals", runCommands: "GNM2.5:1.5", wantOK: true, wantSpacing: 2.5, wantHeight: 1.5},
		{name: "trailing command", runCommands: "GNM5:4 RSPS", wantOK: true, wantSpacing: 5, wantHeight: 4},
		{name: "leading command", runCommands: "RSPS GNM5:4", wantOK: true, wantSpacing: 5, wantHeight: 4},
		{name: "absent", runCommands: "RSPS0011", wantOK: false},
		{name: "empty", runCommands: "", wantOK: false},
		{name: "no height", runCommands: "GNM5", wantOK: false},
		{name: "unparseable", runCommands: "GNMx:y", wantOK: false},
		{name: "zero spacing is not a grid", runCommands: "GNM0:4", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			layout, ok := (&RunResult{RunCommands: test.runCommands}).GridMapLayout()
			if ok != test.wantOK {
				t.Fatalf("GridMapLayout(%q) ok = %v, want %v", test.runCommands, ok, test.wantOK)
			}

			if !test.wantOK {
				return
			}

			if layout.SpacingM != test.wantSpacing || layout.HeightM != test.wantHeight {
				t.Fatalf("GridMapLayout(%q) = %+v, want spacing %v height %v",
					test.runCommands, layout, test.wantSpacing, test.wantHeight)
			}
		})
	}

	// A nil run is not a run declaring zero.
	if _, ok := (*RunResult)(nil).GridMapLayout(); ok {
		t.Fatal("GridMapLayout() on a nil run reported a layout")
	}
}

// TestGeometryFileNamesUnionsBothSources checks that the two places a .res
// records its inputs are unioned rather than one being trusted: either can be
// absent, and the answer must not depend on which.
func TestGeometryFileNamesUnionsBothSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  RunResult
		want []string
	}{
		{
			name: "both agree",
			run: RunResult{
				GeoFiles:     []GeoFileRef{{Name: "GeoObjs.geo"}, {Name: "GeoWand.geo"}},
				RunDataFiles: []string{"GeoObjs.geo", "GeoWand.geo"},
			},
			want: []string{"geoobjs.geo", "geowand.geo"},
		},
		{
			name: "only GeoFiles",
			run:  RunResult{GeoFiles: []GeoFileRef{{Name: "GeoObjs.geo"}}},
			want: []string{"geoobjs.geo"},
		},
		{
			name: "only RunData, with a path",
			run:  RunResult{RunDataFiles: []string{`C:\projects\demo\GeoWand.geo`}},
			want: []string{"geowand.geo"},
		},
		{name: "nothing", run: RunResult{}, want: []string{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := test.run.GeometryFileNames()
			if !slices.Equal(got, test.want) {
				t.Fatalf("GeometryFileNames() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestParseResFileReadsRunData pins that RSPS result metadata carries both the
// [GeoFiles] section and the RunData line, and that the two agree — and that
// the barrier geometry is what distinguishes the reference project's two
// single-point runs.
func TestParseResFileReadsRunData(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RSPS0021.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if !slices.Contains(res.RunDataFiles, "GeoWand.geo") {
		t.Fatalf("RunDataFiles = %v, want it to list GeoWand.geo", res.RunDataFiles)
	}

	if !slices.Contains(res.GeometryFileNames(), "geowand.geo") {
		t.Fatalf("GeometryFileNames() = %v, want it to list geowand.geo", res.GeometryFileNames())
	}

	without, err := ParseResFile(filepath.Join(dir, "RSPS0011.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if slices.Contains(without.GeometryFileNames(), "geowand.geo") {
		t.Fatalf("RSPS0011 GeometryFileNames() = %v, want no barrier geometry", without.GeometryFileNames())
	}
}
