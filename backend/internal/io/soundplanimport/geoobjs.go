package soundplanimport

import (
	"fmt"
	"os"
)

// Object type codes in GeoObjs.geo :O& headers.
const (
	objTypeBuilding = 0x03ec // closed polygon (Gebäude)
	objTypeReceiver = 0x0028 // immission point (Immissionsort)
)

// Building represents a building footprint extracted from GeoObjs.geo.
type Building struct {
	Footprint []Point3D // closed polygon (first == last)
}

// Point3D is a 3D coordinate.
type Point3D struct {
	X float64
	Y float64
	Z float64
}

// ReceiverPoint represents an immission point (receiver) from GeoObjs.geo.
type ReceiverPoint struct {
	X float64
	Y float64
	Z float64
}

// GeoObjects holds all parsed geometry objects from a GeoObjs.geo file.
type GeoObjects struct {
	Buildings []Building
	Receivers []ReceiverPoint
}

// ParseGeoObjsFile reads a SoundPlan GeoObjs.geo binary file and extracts
// building footprints and receiver points.
func ParseGeoObjsFile(path string) (*GeoObjects, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read geoobjs: %w", err)
	}

	return parseGeoObjsData(data), nil
}

// objsParser holds mutable state while scanning object groups.
type objsParser struct {
	data      []byte
	result    GeoObjects
	groupType uint32
	points    []Point3D
}

func parseGeoObjsData(data []byte) *GeoObjects {
	p := &objsParser{data: data}
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

	// Flush last group.
	p.flushGroup()

	return &p.result
}

// handleObjectGroup parses an :O& record header and starts a new group.
// Layout: :O& + 3 padding + u32(typeCode) + 28 bytes bbox/flags.
func (p *objsParser) handleObjectGroup(i int) int {
	p.flushGroup()

	hdrEnd := i + 3 + 3 + 4 + 28 // marker + pad + type + rest
	if hdrEnd > len(p.data) {
		return i + 3
	}

	p.groupType = readU32(p.data, i+6)
	p.points = p.points[:0]

	return hdrEnd
}

func (p *objsParser) handlePoint(i int) int {
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

func (p *objsParser) flushGroup() {
	if len(p.points) == 0 {
		return
	}

	switch p.groupType {
	case objTypeBuilding:
		pts := make([]Point3D, len(p.points))
		copy(pts, p.points)

		p.result.Buildings = append(p.result.Buildings, Building{Footprint: pts})

	case objTypeReceiver:
		if len(p.points) >= 1 {
			p.result.Receivers = append(p.result.Receivers, ReceiverPoint(p.points[0]))
		}
	}

	p.points = p.points[:0]
}

func readU32(data []byte, off int) uint32 {
	_ = data[off+3]

	return uint32(data[off]) | uint32(data[off+1])<<8 | uint32(data[off+2])<<16 | uint32(data[off+3])<<24
}
