package soundplanimport

import (
	"fmt"
	"math"
	"os"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Object type codes in GeoObjs.geo :O& headers.
const (
	objTypeBuilding       = 0x03ec // closed polygon (Gebäude)
	objTypeBuildingLabel  = 0x03e9 // address/name anchor point
	objTypeReceiver       = 0x0028 // immission point (Immissionsort)
	buildingAttrHeightOff = 88
)

// Building represents a building footprint extracted from GeoObjs.geo.
type Building struct {
	Footprint []Point3D // closed polygon (first == last)
	HeightM   float64
	Addresses []string
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
	data           []byte
	result         GeoObjects
	groupType      uint32
	points         []Point3D
	buildingHeight float64
	groupName      string
	addressAnchors []addressAnchor
}

type addressAnchor struct {
	Point Point3D
	Text  string
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
		case data[i+1] == 'D' && data[i+2] == '1':
			i = p.handleNameRecord(i)
		case data[i+1] == 'D' && data[i+2] == 0xa0:
			i = p.handleBuildingAttributes(i)
		case data[i+1] == 'D':
			i = p.skipDataRecord(i)
		default:
			i++
		}
	}

	// Flush last group.
	p.flushGroup()
	assignAddressAnchors(&p.result, p.addressAnchors)

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
	p.buildingHeight = 0
	p.groupName = ""

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

func (p *objsParser) handleNameRecord(i int) int {
	payload, recEnd, ok := p.readDataRecordPayload(i)
	if !ok {
		return i + 3
	}

	if len(payload) == 0 {
		return recEnd
	}

	textLen := int(payload[0])
	if textLen > len(payload)-1 {
		textLen = len(payload) - 1
	}

	p.groupName = strings.TrimSpace(decodeWindows1252(payload[1 : 1+textLen]))

	return recEnd
}

func (p *objsParser) handleBuildingAttributes(i int) int {
	payload, recEnd, ok := p.readDataRecordPayload(i)
	if !ok {
		return i + 3
	}

	if p.groupType == objTypeBuilding && len(payload) >= buildingAttrHeightOff+8 {
		p.buildingHeight = readF64(payload, buildingAttrHeightOff)
	}

	return recEnd
}

func (p *objsParser) skipDataRecord(i int) int {
	_, recEnd, ok := p.readDataRecordPayload(i)
	if !ok {
		return i + 3
	}

	return recEnd
}

func (p *objsParser) readDataRecordPayload(i int) ([]byte, int, bool) {
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

func (p *objsParser) flushGroup() {
	if len(p.points) == 0 {
		return
	}

	switch p.groupType {
	case objTypeBuilding:
		pts := make([]Point3D, len(p.points))
		copy(pts, p.points)

		p.result.Buildings = append(p.result.Buildings, Building{
			Footprint: pts,
			HeightM:   p.buildingHeight,
		})

	case objTypeReceiver:
		if len(p.points) >= 1 {
			p.result.Receivers = append(p.result.Receivers, ReceiverPoint(p.points[0]))
		}

	case objTypeBuildingLabel:
		if len(p.points) >= 1 && p.groupName != "" {
			p.addressAnchors = append(p.addressAnchors, addressAnchor{
				Point: p.points[0],
				Text:  p.groupName,
			})
		}
	}

	p.points = p.points[:0]
}

func assignAddressAnchors(result *GeoObjects, anchors []addressAnchor) {
	if len(result.Buildings) == 0 || len(anchors) == 0 {
		return
	}

	for _, anchor := range anchors {
		index := findNearestBuilding(result.Buildings, anchor.Point)
		if index < 0 {
			continue
		}

		result.Buildings[index].Addresses = appendUniqueString(result.Buildings[index].Addresses, anchor.Text)
	}
}

func findNearestBuilding(buildings []Building, point Point3D) int {
	bestInside := -1
	bestInsideDistance := math.Inf(1)
	bestOutside := -1
	bestOutsideDistance := math.Inf(1)

	for i, building := range buildings {
		distance := footprintDistanceSq(building.Footprint, point)
		if pointInFootprint(building.Footprint, point) {
			if distance < bestInsideDistance {
				bestInside = i
				bestInsideDistance = distance
			}

			continue
		}

		if distance < bestOutsideDistance {
			bestOutside = i
			bestOutsideDistance = distance
		}
	}

	if bestInside >= 0 {
		return bestInside
	}

	return bestOutside
}

func pointInFootprint(footprint []Point3D, point Point3D) bool {
	if len(footprint) < 3 {
		return false
	}

	if footprintDistanceSq(footprint, point) < 1e-9 {
		return true
	}

	inside := false
	j := len(footprint) - 1

	for i := 0; i < len(footprint); j, i = i, i+1 {
		xi := footprint[i].X
		yi := footprint[i].Y
		xj := footprint[j].X
		yj := footprint[j].Y

		intersects := (yi > point.Y) != (yj > point.Y)
		if !intersects {
			continue
		}

		crossX := (xj-xi)*(point.Y-yi)/(yj-yi) + xi
		if point.X < crossX {
			inside = !inside
		}
	}

	return inside
}

func footprintDistanceSq(footprint []Point3D, point Point3D) float64 {
	if len(footprint) == 0 {
		return math.Inf(1)
	}

	best := math.Inf(1)
	for i := 1; i < len(footprint); i++ {
		dist := pointToSegmentDistanceSq(point, footprint[i-1], footprint[i])
		if dist < best {
			best = dist
		}
	}

	if len(footprint) > 1 && (footprint[0].X != footprint[len(footprint)-1].X || footprint[0].Y != footprint[len(footprint)-1].Y) {
		dist := pointToSegmentDistanceSq(point, footprint[len(footprint)-1], footprint[0])
		if dist < best {
			best = dist
		}
	}

	return best
}

func pointToSegmentDistanceSq(point Point3D, start Point3D, end Point3D) float64 {
	dx := end.X - start.X
	dy := end.Y - start.Y
	if dx == 0 && dy == 0 {
		return squaredDistance(point.X, point.Y, start.X, start.Y)
	}

	t := ((point.X-start.X)*dx + (point.Y-start.Y)*dy) / (dx*dx + dy*dy)
	switch {
	case t <= 0:
		return squaredDistance(point.X, point.Y, start.X, start.Y)
	case t >= 1:
		return squaredDistance(point.X, point.Y, end.X, end.Y)
	default:
		projX := start.X + t*dx
		projY := start.Y + t*dy
		return squaredDistance(point.X, point.Y, projX, projY)
	}
}

func squaredDistance(ax float64, ay float64, bx float64, by float64) float64 {
	dx := ax - bx
	dy := ay - by

	return dx*dx + dy*dy
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func decodeWindows1252(data []byte) string {
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return string(data)
	}

	return string(decoded)
}

func readU32(data []byte, off int) uint32 {
	_ = data[off+3]

	return uint32(data[off]) | uint32(data[off+1])<<8 | uint32(data[off+2])<<16 | uint32(data[off+3])<<24
}
