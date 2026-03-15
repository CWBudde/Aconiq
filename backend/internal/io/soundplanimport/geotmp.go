package soundplanimport

import (
	"fmt"
	"os"
)

// Object type codes for terrain data in GeoTmp.geo.
const (
	objTypeElevPoint = 0x040b // single elevation point (Höhenpunkt)
	objTypeContour   = 0x040a // elevation contour line (Höhenlinie)
	objTypeTerrain   = 0x046e // additional terrain break line (Böschung etc.)
)

// ElevationPoint holds a single terrain elevation sample.
type ElevationPoint struct {
	X float64
	Y float64
	Z float64
}

// ContourLine holds an elevation contour polyline.
type ContourLine struct {
	Points []Point3D
}

// TerrainData holds all terrain geometry extracted from GeoTmp.geo.
type TerrainData struct {
	ElevationPoints []ElevationPoint
	ContourLines    []ContourLine
}

// ParseGeoTmpFile reads a SoundPlan GeoTmp.geo binary file and extracts
// terrain elevation points and contour lines.
func ParseGeoTmpFile(path string) (*TerrainData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read geotmp: %w", err)
	}

	return parseGeoTmpData(data), nil
}

// tmpParser holds state during terrain file parsing.
type tmpParser struct {
	data      []byte
	result    TerrainData
	groupType uint32
	points    []Point3D
}

func parseGeoTmpData(data []byte) *TerrainData {
	p := &tmpParser{
		data:   data,
		result: TerrainData{ElevationPoints: make([]ElevationPoint, 0, 1024)},
	}
	i := 0

	for i < len(data)-6 {
		if data[i] != ':' {
			i++

			continue
		}

		switch {
		case data[i+1] == 'O' && data[i+2] == '&':
			i = p.handleObjectGroup(i)
		case data[i+1] == 'G' && data[i+2] == ' ':
			i = p.handlePoint(i)
		default:
			i++
		}
	}

	p.flushGroup()

	return &p.result
}

func (p *tmpParser) handleObjectGroup(i int) int {
	p.flushGroup()

	hdrEnd := i + 3 + 3 + 4 + 28
	if hdrEnd > len(p.data) {
		return i + 3
	}

	p.groupType = readU32(p.data, i+6)
	p.points = p.points[:0]

	return hdrEnd
}

func (p *tmpParser) handlePoint(i int) int {
	recEnd := i + 6 + 32
	if recEnd > len(p.data) {
		return i + 3
	}

	off := i + 6

	p.points = append(p.points, Point3D{
		X: readF64(p.data, off),
		Y: readF64(p.data, off+8),
		Z: readF64(p.data, off+16),
	})

	return recEnd
}

func (p *tmpParser) flushGroup() {
	if len(p.points) == 0 {
		return
	}

	switch p.groupType {
	case objTypeElevPoint:
		// Single-point groups: each group has exactly 1 elevation point.
		p.result.ElevationPoints = append(p.result.ElevationPoints, ElevationPoint(p.points[0]))

	case objTypeContour, objTypeTerrain:
		pts := make([]Point3D, len(p.points))
		copy(pts, p.points)

		p.result.ContourLines = append(p.result.ContourLines, ContourLine{Points: pts})
	}

	p.points = p.points[:0]
}
