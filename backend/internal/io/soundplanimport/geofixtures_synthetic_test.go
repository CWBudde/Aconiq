package soundplanimport

import (
	"encoding/binary"
	"math"
)

// Byte builders for the SoundPLAN *.geo record stream.
//
// Every layout below is authored from the record descriptions the parsers
// themselves carry — objHeaderLen/objIDOff and the :D tag list in geoobjs.go,
// the four-float64 :G payload, and the 14-byte :D prefix whose last four bytes
// are the payload length. Nothing is copied from the licensed reference
// project, so these fixtures run on a clean checkout.
//
// The builders follow the shape hardening_test.go established: the bytes are
// assembled in Go source rather than committed as files, which keeps the
// layout being tested readable next to the assertions about it.

// Record markers and the filler the scanner walks past. The filler must never
// be a colon: parseGeoObjsData resumes its byte-wise scan inside the tail of
// an :O& record, and a stray marker there would start a phantom record.
const geoFiller = 0x00

// The on-the-wire layout, spelled out as literals rather than reused from the
// parser's own constants.
//
// A fixture built from geoobjs.go's constants cannot fail when one of them
// changes, because the bytes move with the parser — and swapping two object
// type codes is precisely the defect that shipped once here and that
// ImmissionPoint's doc comment exists to prevent recurring. Writing the values
// out a second time is what turns these tests into evidence about the format
// instead of a restatement of the code.
const (
	wireTypeBuilding       = 0x03ec
	wireTypeImmissionPoint = 0x03e9
	wireTypeMapLabel       = 0x0028
	wireTypeWall           = 0x03eb
	wireTypeElevPoint      = 0x040b
	wireTypeContour        = 0x040a
	wireTypeTerrainBreak   = 0x046e

	wireTagName        = '1'
	wireTagBuildingAtt = 0xa0
	wireTagFloors      = 'T'
	wireTagLimits      = '('
	wireTagLabelText   = 0x59
	wireTagBarrierAcou = '!'

	wireObjRecordLen   = 44
	wireObjTypeOff     = 6
	wireObjIDOff       = 39
	wirePointRecordLen = 38
	wireDataPrefixLen  = 14
	wireDataLengthOff  = 10

	wireBuildingHeightOff = 88
	wireFloorAttrLen      = 48
	wireFloorFirstOff     = 28
	wireFloorSpacingOff   = 36
	wireFloorCountOff     = 44
	wireLimitsLen         = 32
	wireLimitsDayOff      = 8
	wireLimitsNightOff    = 24
	wireBarrierAcouLen    = 24

	wireDGMVertexCountIndex  = 3
	wireDGMVertexBlockOffset = 100
	wireDGMVertexRecordSize  = 24
)

func leF64(value float64) []byte {
	out := make([]byte, 8)
	binary.LittleEndian.PutUint64(out, math.Float64bits(value))

	return out
}

func leF32(value float32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, math.Float32bits(value))

	return out
}

func leU32(value uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, value)

	return out
}

func leU64(value uint64) []byte {
	out := make([]byte, 8)
	binary.LittleEndian.PutUint64(out, value)

	return out
}

// geoObjectRecord builds one 44-byte :O& object header.
//
// Layout, from objHeaderLen/objIDOff in geoobjs.go: the three marker bytes,
// three bytes of padding, the u32 type code, 28 bytes of bounding box and
// flags, one spare byte, the u32 object id at offset 39, and one trailing
// byte. The record must be followed immediately by the next record's colon,
// which is the condition objectIDAt requires before it trusts the id.
func geoObjectRecord(typeCode uint32, objID uint32) []byte {
	rec := make([]byte, wireObjRecordLen)
	for i := range rec {
		rec[i] = geoFiller
	}

	copy(rec[0:3], ":O&")
	copy(rec[wireObjTypeOff:wireObjTypeOff+4], leU32(typeCode))
	copy(rec[wireObjIDOff:wireObjIDOff+4], leU32(objID))

	return rec
}

// geoPointRecord builds one 38-byte :G point record: the marker, three bytes
// of padding, and four float64s — X, Y, the object's reference elevation and
// the ground elevation cached beside it.
func geoPointRecord(x float64, y float64, z float64, ground float64) []byte {
	rec := make([]byte, wirePointRecordLen)
	for i := range rec {
		rec[i] = geoFiller
	}

	copy(rec[0:3], ":G ")
	copy(rec[6:14], leF64(x))
	copy(rec[14:22], leF64(y))
	copy(rec[22:30], leF64(z))
	copy(rec[30:38], leF64(ground))

	return rec
}

// geoDataRecord builds one :D record: the two marker bytes, the tag, seven
// bytes the parsers walk past, the u32 payload length, and the payload.
//
// declaredLen is written into the length field instead of the payload's real
// length when it is non-negative, which is how the truncation and overrun
// cases are constructed.
func geoDataRecord(tag byte, payload []byte, declaredLen int) []byte {
	length := len(payload)
	if declaredLen >= 0 {
		length = declaredLen
	}

	rec := make([]byte, wireDataPrefixLen, wireDataPrefixLen+len(payload))
	for i := range rec {
		rec[i] = geoFiller
	}

	rec[0] = ':'
	rec[1] = 'D'
	rec[2] = tag

	copy(rec[wireDataLengthOff:wireDataLengthOff+4], leU32(uint32(length)))

	return append(rec, payload...)
}

// pascalStringPayload prefixes raw Windows-1252 bytes with their length, which
// is the encoding readPascalString expects. declaredLen overrides the length
// byte so that a test can declare more text than the payload holds.
func pascalStringPayload(encoded []byte, declaredLen int) []byte {
	length := len(encoded)
	if declaredLen >= 0 {
		length = declaredLen
	}

	return append([]byte{byte(length)}, encoded...)
}

// buildingAttrPayload places a building height at buildingAttrHeightOff.
func buildingAttrPayload(heightM float64) []byte {
	payload := make([]byte, wireBuildingHeightOff+8)
	copy(payload[wireBuildingHeightOff:], leF64(heightM))

	return payload
}

// floorAttrPayload lays out an Immissionsort's floor stacking at the three
// offsets handleFloorAttributes reads.
func floorAttrPayload(firstOffsetM float64, spacingM float64, count uint32) []byte {
	payload := make([]byte, wireFloorAttrLen)
	copy(payload[wireFloorFirstOff:], leF64(firstOffsetM))
	copy(payload[wireFloorSpacingOff:], leF64(spacingM))
	copy(payload[wireFloorCountOff:], leU32(count))

	return payload
}

// noiseLimitsPayload lays out the day and night thresholds at the offsets
// handleNoiseLimits reads.
func noiseLimitsPayload(dayDB float64, nightDB float64) []byte {
	payload := make([]byte, wireLimitsLen)
	copy(payload[wireLimitsDayOff:], leF64(dayDB))
	copy(payload[wireLimitsNightOff:], leF64(nightDB))

	return payload
}

// barrierAcousticsPayload lays out a GeoWand :D! record: two absorption values
// and the material code.
func barrierAcousticsPayload(sideADB float64, sideBDB float64, material uint64) []byte {
	payload := make([]byte, wireBarrierAcouLen)
	copy(payload[0:8], leF64(sideADB))
	copy(payload[8:16], leF64(sideBDB))
	copy(payload[16:24], leU64(material))

	return payload
}

// concat joins record fragments into one file image.
func concat(parts ...[]byte) []byte {
	out := make([]byte, 0, 256)
	for _, part := range parts {
		out = append(out, part...)
	}

	return out
}
