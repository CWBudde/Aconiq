// Package terrain provides elevation queries over a terrain surface loaded
// from a GeoTIFF digital terrain model (DTM).
package terrain

import (
	"fmt"
	"math"
)

// Model provides elevation queries over a terrain surface.
type Model interface {
	// ElevationAt returns the bilinear-interpolated elevation at (x, y) in the
	// terrain's native CRS. The second return value is false when the point
	// falls outside the terrain bounds.
	ElevationAt(x, y float64) (float64, bool)

	// Bounds returns [minX, minY, maxX, maxY] in the terrain's native CRS.
	Bounds() [4]float64
}

// Load reads a GeoTIFF elevation raster and returns a Model for querying elevations.
// Supports single-band float32, float64, and int16 GeoTIFF files with strip
// or tile layout, uncompressed or deflate-compressed.
func Load(path string) (Model, error) {
	grid, err := readGeoTIFF(path)
	if err != nil {
		return nil, fmt.Errorf("terrain: load %q: %w", path, err)
	}

	return grid, nil
}

// gridModel is an in-memory elevation grid with bilinear interpolation.
type gridModel struct {
	data       []float64
	width      int
	height     int
	originX    float64 // X coordinate of upper-left pixel center
	originY    float64 // Y coordinate of upper-left pixel center
	pixelSizeX float64 // positive; X increases rightward
	pixelSizeY float64 // positive; Y decreases downward
	noData     float64
	hasNoData  bool
}

func (g *gridModel) Bounds() [4]float64 {
	minX := g.originX - g.pixelSizeX/2
	maxX := g.originX + float64(g.width-1)*g.pixelSizeX + g.pixelSizeX/2
	maxY := g.originY + g.pixelSizeY/2
	minY := g.originY - float64(g.height-1)*g.pixelSizeY - g.pixelSizeY/2

	return [4]float64{minX, minY, maxX, maxY}
}

func (g *gridModel) ElevationAt(x, y float64) (float64, bool) {
	// Convert world coordinates to continuous pixel coordinates.
	px := (x - g.originX) / g.pixelSizeX
	py := (g.originY - y) / g.pixelSizeY

	// Check bounds with a half-pixel margin (data covers pixel centers).
	if px < -0.5 || px > float64(g.width)-0.5 || py < -0.5 || py > float64(g.height)-0.5 {
		return 0, false
	}

	// Bilinear interpolation between four surrounding pixel centers.
	x0 := int(math.Floor(px))
	y0 := int(math.Floor(py))
	x1 := x0 + 1
	y1 := y0 + 1

	// Clamp to valid pixel range.
	x0 = clampInt(x0, g.width-1)
	x1 = clampInt(x1, g.width-1)
	y0 = clampInt(y0, g.height-1)
	y1 = clampInt(y1, g.height-1)

	fx := px - math.Floor(px)
	fy := py - math.Floor(py)

	v00 := g.at(x0, y0)
	v10 := g.at(x1, y0)
	v01 := g.at(x0, y1)
	v11 := g.at(x1, y1)

	// Skip nodata cells.
	if g.hasNoData && (g.isNoData(v00) || g.isNoData(v10) || g.isNoData(v01) || g.isNoData(v11)) {
		return 0, false
	}

	v := v00*(1-fx)*(1-fy) + v10*fx*(1-fy) + v01*(1-fx)*fy + v11*fx*fy

	return v, true
}

func (g *gridModel) at(x, y int) float64 {
	return g.data[y*g.width+x]
}

func (g *gridModel) isNoData(v float64) bool {
	return v == g.noData || math.IsNaN(v)
}

func clampInt(v, hi int) int {
	if v < 0 {
		return 0
	}

	if v > hi {
		return hi
	}

	return v
}
