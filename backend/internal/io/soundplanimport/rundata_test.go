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
