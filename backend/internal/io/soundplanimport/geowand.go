package soundplanimport

import (
	"fmt"
	"os"
)

// Wall type code in GeoWand.geo :O& headers.
const objTypeWall = 0x03eb // noise barrier (Lärmschutzwand)

// NoiseBarrier represents a noise barrier polyline from GeoWand.geo.
type NoiseBarrier struct {
	Points                []BarrierPoint
	HasAcousticProperties bool
	AbsorptionSideADB     float64
	AbsorptionSideBDB     float64
	MaterialCode          int64
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
	data              []byte
	barriers          []NoiseBarrier
	points            []BarrierPoint
	inWall            bool
	hasAcousticProps  bool
	absorptionSideADB float64
	absorptionSideBDB float64
	materialCode      int64
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
		case data[i+1] == 'D' && data[i+2] == '!':
			i = p.handleBarrierAcoustics(i)
		case data[i+1] == 'D':
			i = p.skipDataRecord(i)
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
	p.hasAcousticProps = false
	p.absorptionSideADB = 0
	p.absorptionSideBDB = 0
	p.materialCode = 0

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

func (p *wandParser) handleBarrierAcoustics(i int) int {
	payload, recEnd, ok := p.readDataRecordPayload(i)
	if !ok {
		return i + 3
	}

	if !p.inWall || len(payload) < 24 {
		return recEnd
	}

	p.hasAcousticProps = true
	p.absorptionSideADB = readF64(payload, 0)
	p.absorptionSideBDB = readF64(payload, 8)
	p.materialCode = readI64(payload, 16)

	return recEnd
}

func (p *wandParser) skipDataRecord(i int) int {
	_, recEnd, ok := p.readDataRecordPayload(i)
	if !ok {
		return i + 3
	}

	return recEnd
}

func (p *wandParser) readDataRecordPayload(i int) ([]byte, int, bool) {
	recEnd := i + 14
	if recEnd > len(p.data) {
		return nil, i + 3, false
	}

	payloadLen := int(readU32(p.data, i+10))
	recEnd += payloadLen
	if recEnd > len(p.data) {
		return nil, i + 3, false
	}

	return p.data[i+14 : recEnd], recEnd, true
}

func (p *wandParser) flushBarrier() {
	if len(p.points) == 0 {
		return
	}

	pts := make([]BarrierPoint, len(p.points))
	copy(pts, p.points)

	p.barriers = append(p.barriers, NoiseBarrier{
		Points:                pts,
		HasAcousticProperties: p.hasAcousticProps,
		AbsorptionSideADB:     p.absorptionSideADB,
		AbsorptionSideBDB:     p.absorptionSideBDB,
		MaterialCode:          p.materialCode,
	})
	p.points = p.points[:0]
}

func readI64(data []byte, off int) int64 {
	_ = data[off+7]

	return int64(uint64(data[off]) |
		uint64(data[off+1])<<8 |
		uint64(data[off+2])<<16 |
		uint64(data[off+3])<<24 |
		uint64(data[off+4])<<32 |
		uint64(data[off+5])<<40 |
		uint64(data[off+6])<<48 |
		uint64(data[off+7])<<56)
}
