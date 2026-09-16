package soundplanimport

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Object type codes in GeoObjs.geo :O& headers.
//
// The two point types were previously named the other way round, which is the
// defect this naming exists to prevent recurring. 0x0028 carries no object id,
// no name, Z = 0 and an Arial font record — it is map furniture. 0x03e9
// carries the object id, the address, the floor attributes and the noise
// limits of an Immissionsort, and its ids are exactly the ObjID column of
// RREC*.abs. See ImmissionPoint and MapLabel.
const (
	objTypeBuilding       = 0x03ec // closed polygon (Gebäude)
	objTypeImmissionPoint = 0x03e9 // Immissionsort: a column of receivers, one per floor
	objTypeMapLabel       = 0x0028 // drawing annotation (house numbers, bridge captions)
	buildingAttrHeightOff = 88
)

// :D record tags this parser decodes. The tag is the third byte of the record
// marker; the remaining tags are read past without interpretation.
const (
	dataTagName        = '1'  // Pascal string: object name / address
	dataTagBuildingAtt = 0xa0 // building attributes, including the height
	dataTagFloors      = 'T'  // Immissionsort floor attributes
	dataTagLimits      = '('  // Immissionsort noise limits
	dataTagLabelText   = 0x59 // map label caption
)

// Byte offsets inside the records this parser decodes, all little-endian.
//
// Reverse-engineered from SoundPLAN Essential 4.1 (Productversion 8.0.0.0).
// Nothing here is documented by the vendor, so every read is length-guarded
// and an unrecognised layout leaves the corresponding field unset rather than
// substituting a plausible default — see objectIDAt and handleFloorAttributes.
const (
	// objHeaderLen is the fixed width of a :O& record. Every one of the 27015
	// such records in the reference project is exactly this long and is
	// followed by the next record's ':' marker, which is what makes the
	// trailing object id safe to read.
	objHeaderLen = 44
	// objIDOff is the object id's offset inside the :O& record. It is the
	// value RREC*.abs joins on.
	objIDOff = 39

	floorAttrMinLen   = 48
	floorAttrFirstOff = 28
	floorAttrSpaceOff = 36
	floorAttrCountOff = 44

	limitsMinLen  = 32
	limitsDayOff  = 8
	limitsNighOff = 24

	// pointRecordLen is the :G record's payload: four float64s, of which the
	// parser previously read only the first three.
	pointRecordLen = 32
	pointGroundOff = 24
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

// ImmissionPoint is a SoundPLAN Immissionsort from GeoObjs.geo (object type
// 0x03e9).
//
// It is not one receiver. It is a facade position plus the floor geometry of
// the building behind it, and SoundPLAN evaluates it once per floor: RREC*.abs
// carries FloorCount rows for it, joined by (ObjID, Floor). FloorZ and
// FloorHeightM reproduce those rows' Z and height.
//
// FloorRefZ is the reference elevation the floors are stacked from — the third
// value of the :G point record — and GroundHeightM the ground elevation
// SoundPLAN cached alongside it, which is the fourth. They are close but not
// equal, and the difference is what makes a first floor sit anywhere between
// 0.5 m and 4.2 m above ground in the reference project.
//
// HasFloorAttrs is false when the :D'T' record was absent or too short. The
// floor fields are then all zero and a caller must not stack anything on them.
type ImmissionPoint struct {
	ObjID             int64
	Name              string
	X                 float64
	Y                 float64
	FloorRefZ         float64
	GroundHeightM     float64
	FloorCount        int
	FirstFloorOffsetM float64
	FloorSpacingM     float64
	LimitDayDB        float64
	LimitNightDB      float64
	HasFloorAttrs     bool
}

// FloorZ returns the absolute elevation of a floor, counting from 1.
func (p ImmissionPoint) FloorZ(floor int) float64 {
	return p.FloorRefZ + p.FirstFloorOffsetM + float64(floor-1)*p.FloorSpacingM
}

// FloorHeightM returns a floor's height above ground, which is what the model
// schema's height_m means.
func (p ImmissionPoint) FloorHeightM(floor int) float64 {
	return p.FloorZ(floor) - p.GroundHeightM
}

// MapLabel is a drawing annotation from GeoObjs.geo (object type 0x0028):
// house numbers, bridge captions, dimension texts. It carries no elevation and
// no acoustic meaning, and exists here only so that the parser can say what
// those 77 objects are instead of mistaking them for receivers.
//
// Text is SoundPLAN's caption verbatim, in which "|" is the line break.
type MapLabel struct {
	X    float64
	Y    float64
	Text string
}

// GeoObjects holds all parsed geometry objects from a GeoObjs.geo file.
type GeoObjects struct {
	Buildings       []Building
	ImmissionPoints []ImmissionPoint
	MapLabels       []MapLabel
}

// ParseGeoObjsFile reads a SoundPlan GeoObjs.geo binary file and extracts
// building footprints, immission points and map labels.
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
	groupObjID     int64
	points         []Point3D
	groundHeights  []float64
	buildingHeight float64
	groupName      string
	groupLabelText string
	floors         floorAttributes
	limits         noiseLimits
	addressAnchors []addressAnchor
}

// floorAttributes is the :D'T' record of an Immissionsort. Present is false
// when the record was missing or shorter than the layout requires, in which
// case none of the other fields mean anything.
type floorAttributes struct {
	FirstOffsetM float64
	SpacingM     float64
	Count        int
	Present      bool
}

// noiseLimits is the :D'(' record of an Immissionsort: the day and night
// thresholds the project assesses it against.
type noiseLimits struct {
	DayDB   float64
	NightDB float64
	Present bool
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
		case data[i+1] == 'D':
			i = p.handleDataRecord(i)
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
// Layout: :O& + 3 padding + u32(typeCode) + 28 bytes bbox/flags + u32(objectID).
func (p *objsParser) handleObjectGroup(i int) int {
	p.flushGroup()

	hdrEnd := i + 3 + 3 + 4 + 28 // marker + pad + type + rest
	if hdrEnd > len(p.data) {
		return i + 3
	}

	p.groupType = readU32(p.data, i+6)
	p.groupObjID = p.objectIDAt(i)
	p.points = p.points[:0]
	p.groundHeights = p.groundHeights[:0]
	p.buildingHeight = 0
	p.groupName = ""
	p.groupLabelText = ""
	p.floors = floorAttributes{}
	p.limits = noiseLimits{}

	return hdrEnd
}

// objectIDAt reads the object id from the tail of an :O& record, or returns 0
// when the record does not have the layout that id was found in.
//
// The id sits past the point the header scan itself consumes, so reading it
// requires knowing the record's full width. Every :O& record in the reference
// project is objHeaderLen bytes and is followed by the next record's ':'
// marker; requiring that is what distinguishes "the id is here" from "these
// bytes belong to something else". A file whose headers are laid out
// differently yields 0, and callers fall back to positional identity rather
// than to a fabricated id.
func (p *objsParser) objectIDAt(i int) int64 {
	if i+objIDOff+4 > len(p.data) || i+objHeaderLen >= len(p.data) {
		return 0
	}

	if p.data[i+objHeaderLen] != ':' {
		return 0
	}

	return int64(readU32(p.data, i+objIDOff))
}

// handlePoint parses a :G point record. It holds four float64s: X, Y, the
// object's reference elevation, and the ground elevation SoundPLAN cached for
// it. The fourth was ignored until immission points needed it to express a
// floor's height above ground.
func (p *objsParser) handlePoint(i int) int {
	recEnd := i + 6 + pointRecordLen
	if recEnd > len(p.data) {
		return i + 3
	}

	off := i + 6

	p.points = append(p.points, Point3D{
		X: readF64(p.data, off),
		Y: readF64(p.data, off+8),
		Z: readF64(p.data, off+16),
	})
	p.groundHeights = append(p.groundHeights, readF64(p.data, off+pointGroundOff))

	return recEnd
}

// handleDataRecord reads one :D record and dispatches on its tag. Tags this
// parser does not decode are skipped by returning the record's end.
func (p *objsParser) handleDataRecord(i int) int {
	payload, recEnd, ok := p.readDataRecordPayload(i)
	if !ok {
		return i + 3
	}

	switch p.data[i+2] {
	case dataTagName:
		p.groupName = readPascalString(payload)
	case dataTagLabelText:
		p.groupLabelText = readPascalString(payload)
	case dataTagBuildingAtt:
		if p.groupType == objTypeBuilding && len(payload) >= buildingAttrHeightOff+8 {
			p.buildingHeight = readF64(payload, buildingAttrHeightOff)
		}
	case dataTagFloors:
		p.handleFloorAttributes(payload)
	case dataTagLimits:
		p.handleNoiseLimits(payload)
	}

	return recEnd
}

// handleFloorAttributes decodes an Immissionsort's floor stacking. A payload
// shorter than the layout leaves Present false: the alternative — defaulting
// the spacing or the count — would invent receivers that SoundPLAN never
// evaluated.
func (p *objsParser) handleFloorAttributes(payload []byte) {
	if p.groupType != objTypeImmissionPoint || len(payload) < floorAttrMinLen {
		return
	}

	p.floors = floorAttributes{
		FirstOffsetM: readF64(payload, floorAttrFirstOff),
		SpacingM:     readF64(payload, floorAttrSpaceOff),
		Count:        int(readU32(payload, floorAttrCountOff)),
		Present:      true,
	}
}

func (p *objsParser) handleNoiseLimits(payload []byte) {
	if p.groupType != objTypeImmissionPoint || len(payload) < limitsMinLen {
		return
	}

	p.limits = noiseLimits{
		DayDB:   readF64(payload, limitsDayOff),
		NightDB: readF64(payload, limitsNighOff),
		Present: true,
	}
}

// readPascalString decodes a length-prefixed Windows-1252 string.
func readPascalString(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	textLen := min(int(payload[0]), len(payload)-1)

	return strings.TrimSpace(decodeWindows1252(payload[1 : 1+textLen]))
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

	case objTypeMapLabel:
		p.result.MapLabels = append(p.result.MapLabels, MapLabel{
			X:    p.points[0].X,
			Y:    p.points[0].Y,
			Text: p.groupLabelText,
		})

	case objTypeImmissionPoint:
		p.result.ImmissionPoints = append(p.result.ImmissionPoints, p.immissionPoint())

		// The Immissionsort's name is the building's address, and the
		// building layer carries no addresses of its own, so the same
		// object stays the anchor that labels the footprint behind it.
		if p.groupName != "" {
			p.addressAnchors = append(p.addressAnchors, addressAnchor{
				Point: p.points[0],
				Text:  p.groupName,
			})
		}
	}

	p.points = p.points[:0]
	p.groundHeights = p.groundHeights[:0]
}

// immissionPoint assembles the group currently being flushed. The caller has
// already established that at least one point was read.
func (p *objsParser) immissionPoint() ImmissionPoint {
	point := ImmissionPoint{
		ObjID:     p.groupObjID,
		Name:      p.groupName,
		X:         p.points[0].X,
		Y:         p.points[0].Y,
		FloorRefZ: p.points[0].Z,
	}

	if len(p.groundHeights) > 0 {
		point.GroundHeightM = p.groundHeights[0]
	}

	if p.floors.Present {
		point.FirstFloorOffsetM = p.floors.FirstOffsetM
		point.FloorSpacingM = p.floors.SpacingM
		point.FloorCount = p.floors.Count
		point.HasFloorAttrs = true
	}

	if p.limits.Present {
		point.LimitDayDB = p.limits.DayDB
		point.LimitNightDB = p.limits.NightDB
	}

	return point
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
	if slices.Contains(values, value) {
		return values
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
