package terrain

import "testing"

// flatPatch stands in for a DTM: it covers one rectangle in its own CRS and
// records the coordinates it was last asked about, so a test can tell whether
// the query was transformed on the way in.
type flatPatch struct {
	minX, minY, maxX, maxY float64
	elevation              float64
	lastX, lastY           float64
}

func (p *flatPatch) ElevationAt(x, y float64) (float64, bool) {
	p.lastX, p.lastY = x, y

	if x < p.minX || x > p.maxX || y < p.minY || y > p.maxY {
		return 0, false
	}

	return p.elevation, true
}

func (p *flatPatch) Bounds() [4]float64 {
	return [4]float64{p.minX, p.minY, p.maxX, p.maxY}
}

func (p *flatPatch) Info() Info {
	return Info{Bounds: p.Bounds(), PixelSize: [2]float64{0.001, 0.001}, GridSize: [2]int{10, 10}}
}

func hamburgPatch() *flatPatch {
	return &flatPatch{minX: 9.99, minY: 53.54, maxX: 10.01, maxY: 53.56, elevation: 31.5}
}

// A terrain already in the CRS the caller computes in is handed back as it is —
// not wrapped in a transform between a CRS and itself, which would cost two
// projections on every one of a run's millions of lookups and could only ever
// return what the grid already returns.
func TestInComputeCRSDoesNotWrapAModelThatIsAlreadyThere(t *testing.T) {
	t.Parallel()

	inner := hamburgPatch()

	got, err := InComputeCRS(inner, "EPSG:25832", "EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	if got != Model(inner) {
		t.Fatalf("the terrain was wrapped although it is already in the compute CRS: %T", got)
	}
}

// The CRS identifiers are compared as geo canonicalises them, not as strings:
// "epsg:25832" and "EPSG:25832" name one CRS, and a caller that spells it
// differently must not pay for a transform.
func TestInComputeCRSComparesCanonicalIdentifiers(t *testing.T) {
	t.Parallel()

	inner := hamburgPatch()

	got, err := InComputeCRS(inner, "epsg:25832", "EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	if got != Model(inner) {
		t.Fatalf("two spellings of one CRS produced a transform: %T", got)
	}
}

func TestInComputeCRSPassesNilThrough(t *testing.T) {
	t.Parallel()

	got, err := InComputeCRS(nil, "EPSG:4326", "EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	if got != nil {
		t.Fatalf("a caller with no terrain gained one: %T", got)
	}
}

func TestInComputeCRSRefusesACRSItCannotParse(t *testing.T) {
	t.Parallel()

	for name, crs := range map[string][2]string{
		"terrain": {"not-a-crs", "EPSG:25832"},
		"compute": {"EPSG:4326", "not-a-crs"},
		"empty":   {"EPSG:4326", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := InComputeCRS(hamburgPatch(), crs[0], crs[1]); err == nil {
				t.Fatalf("InComputeCRS(%q, %q) was accepted", crs[0], crs[1])
			}
		})
	}
}

// Bounds and Info describe the stored raster and stay in its own CRS:
// projecting a rectangle does not produce a rectangle, so any [4]float64
// answer would be too large or too small somewhere.
func TestInComputeCRSReportsTheRastersOwnExtent(t *testing.T) {
	t.Parallel()

	inner := hamburgPatch()

	wrapped, err := InComputeCRS(inner, "EPSG:4326", "EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	if wrapped.Bounds() != inner.Bounds() {
		t.Errorf("Bounds = %v, want the raster's own %v", wrapped.Bounds(), inner.Bounds())
	}

	if wrapped.Info().GridSize != inner.Info().GridSize {
		t.Errorf("Info lost the grid size: %v", wrapped.Info())
	}
}
