package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseDGMFile_VertexTable(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	dgm, err := ParseDGMFile(filepath.Join(dir, "RDGM0001.dgm"))
	if err != nil {
		t.Fatalf("ParseDGMFile: %v", err)
	}

	if dgm.SourceFile != "RDGM0001.dgm" {
		t.Fatalf("source file = %q, want RDGM0001.dgm", dgm.SourceFile)
	}

	if got := dgm.HeaderValues; got[0] != 6200 || got[3] != 3672 || got[5] != 7304 || got[6] != 39 {
		t.Fatalf("unexpected header values: %#v", got)
	}

	if len(dgm.Points) != 3672 {
		t.Fatalf("got %d DGM points, want 3672", len(dgm.Points))
	}

	first := dgm.Points[0]
	if math.Abs(first.X-7870.52) > 0.01 {
		t.Errorf("first X = %.2f, want 7870.52", first.X)
	}

	if math.Abs(first.Y-6349.22) > 0.01 {
		t.Errorf("first Y = %.2f, want 6349.22", first.Y)
	}

	if math.Abs(first.Z-222.61) > 0.02 {
		t.Errorf("first Z = %.5f, want about 222.61", first.Z)
	}
}
