package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseCalcArea_PointCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	area, err := ParseCalcAreaFile(filepath.Join(dir, "CalcArea.geo"))
	if err != nil {
		t.Fatalf("ParseCalcAreaFile: %v", err)
	}

	// Closed rectangle: 4 corners + closing point = 5.
	if len(area.Points) != 5 {
		t.Errorf("got %d points, want 5", len(area.Points))
	}
}

func TestParseCalcArea_Closed(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	area, err := ParseCalcAreaFile(filepath.Join(dir, "CalcArea.geo"))
	if err != nil {
		t.Fatalf("ParseCalcAreaFile: %v", err)
	}

	first := area.Points[0]
	last := area.Points[len(area.Points)-1]

	if math.Abs(first.X-last.X) > 0.01 || math.Abs(first.Y-last.Y) > 0.01 {
		t.Errorf("polygon not closed: first=(%.2f,%.2f) last=(%.2f,%.2f)", first.X, first.Y, last.X, last.Y)
	}
}

func TestParseCalcArea_BoundsPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	area, err := ParseCalcAreaFile(filepath.Join(dir, "CalcArea.geo"))
	if err != nil {
		t.Fatalf("ParseCalcAreaFile: %v", err)
	}

	// From hex analysis: x range ~7405-7831, y range ~6659-7023.
	var minX, maxX, minY, maxY float64

	for i, pt := range area.Points {
		if i == 0 {
			minX, maxX = pt.X, pt.X
			minY, maxY = pt.Y, pt.Y

			continue
		}

		if pt.X < minX {
			minX = pt.X
		}

		if pt.X > maxX {
			maxX = pt.X
		}

		if pt.Y < minY {
			minY = pt.Y
		}

		if pt.Y > maxY {
			maxY = pt.Y
		}
	}

	width := maxX - minX
	height := maxY - minY

	if width < 300 || width > 600 {
		t.Errorf("width=%.1f, want 300-600", width)
	}

	if height < 250 || height > 500 {
		t.Errorf("height=%.1f, want 250-500", height)
	}
}
