package soundplanimport

import (
	"fmt"
	"os"
)

// Wall type code in GeoWand.geo :O& headers.
const objTypeWall = 0x03eb // noise barrier (Lärmschutzwand)

// NoiseBarrier represents a noise barrier polyline from GeoWand.geo.
type NoiseBarrier struct {
	Points []BarrierPoint
}

// BarrierPoint holds a single point along a noise barrier.
type BarrierPoint struct {
	X      float64 // easting
	Y      float64 // northing
	ZTop   float64 // elevation of barrier top [m above datum]
	Height float64 // barrier height above ground [m]
}

// ParseGeoWandFile reads a SoundPlan GeoWand.geo binary file and extracts
// noise barrier geometry.
func ParseGeoWandFile(path string) ([]NoiseBarrier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read geowand: %w", err)
	}

	return parseGeoWandData(data), nil
}

// wandParser holds state during geo wand file parsing.
type wandParser struct {
	data     []byte
	barriers []NoiseBarrier
	points   []BarrierPoint
	inWall   bool
}

func parseGeoWandData(data []byte) []NoiseBarrier {
	p := &wandParser{data: data}
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

	p.flushBarrier()

	return p.barriers
}

func (p *wandParser) handleObjectGroup(i int) int {
	p.flushBarrier()

	hdrEnd := i + 3 + 3 + 4 + 28
	if hdrEnd > len(p.data) {
		return i + 3
	}

	tc := readU32(p.data, i+6)
	p.inWall = tc == objTypeWall
	p.points = p.points[:0]

	return hdrEnd
}

func (p *wandParser) handlePoint(i int) int {
	recEnd := i + 6 + 32
	if recEnd > len(p.data) || !p.inWall {
		return i + 3
	}

	off := i + 6

	p.points = append(p.points, BarrierPoint{
		X:      readF64(p.data, off),
		Y:      readF64(p.data, off+8),
		ZTop:   readF64(p.data, off+16),
		Height: readF64(p.data, off+24),
	})

	return recEnd
}

func (p *wandParser) flushBarrier() {
	if len(p.points) == 0 {
		return
	}

	pts := make([]BarrierPoint, len(p.points))
	copy(pts, p.points)

	p.barriers = append(p.barriers, NoiseBarrier{Points: pts})
	p.points = p.points[:0]
}
