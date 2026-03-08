package gpkgimport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	wkbPoint              = 1
	wkbLineString         = 2
	wkbPolygon            = 3
	wkbMultiPoint         = 4
	wkbMultiLineString    = 5
	wkbMultiPolygon       = 6
	gpkgMagic0            = 0x47
	gpkgMagic1            = 0x50
	gpkgEnvelopeMaskBits  = 0x07
	gpkgEmptyGeomBit      = 0x08
	gpkgEnvelopeBox2DSize = 32
	gpkgEnvelopeBox3DSize = 48
	gpkgEnvelopeBox4DSize = 64
	wkbHeaderSize         = 5 // 1 byte order + 4 type
	float64Size           = 8
	uint32Size            = 4
)

// DecodeGPKGBlob decodes a GeoPackage geometry blob into a GeoJSON-compatible
// geometry type name and coordinate tree ([]any of float64 values).
// Returns empty strings and nil coordinates when the geometry is flagged as empty.
func DecodeGPKGBlob(blob []byte) (geomType string, coords any, err error) {
	if len(blob) < 4 {
		return "", nil, fmt.Errorf("gpkg blob too short: %d bytes", len(blob))
	}

	if blob[0] != gpkgMagic0 || blob[1] != gpkgMagic1 {
		return "", nil, fmt.Errorf("gpkg blob: invalid magic bytes %02x %02x", blob[0], blob[1])
	}

	flags := blob[3]

	envelopeSize, err := envelopeSizeFromFlags(flags)
	if err != nil {
		return "", nil, err
	}

	if (flags & gpkgEmptyGeomBit) != 0 {
		return "", nil, nil
	}

	wkbOffset := 4 + uint32Size + envelopeSize // magic(2)+version(1)+flags(1) + SRS ID(4) + envelope

	if len(blob) < wkbOffset {
		return "", nil, errors.New("gpkg blob: truncated header")
	}

	wkbData := blob[wkbOffset:]
	if len(wkbData) == 0 {
		return "", nil, nil
	}

	geomType, coords, _, err = decodeWKB(wkbData)

	return geomType, coords, err
}

func envelopeSizeFromFlags(flags byte) (int, error) {
	envelopeType := flags & gpkgEnvelopeMaskBits

	switch envelopeType {
	case 0:
		return 0, nil
	case 1:
		return gpkgEnvelopeBox2DSize, nil
	case 2, 3:
		return gpkgEnvelopeBox3DSize, nil
	case 4:
		return gpkgEnvelopeBox4DSize, nil
	default:
		return 0, fmt.Errorf("gpkg blob: unknown envelope type %d", envelopeType)
	}
}

func decodeWKB(data []byte) (geomType string, coords any, consumed int, err error) {
	if len(data) < wkbHeaderSize {
		return "", nil, 0, fmt.Errorf("wkb: data too short for header: %d bytes", len(data))
	}

	order, err := byteOrderFromByte(data[0])
	if err != nil {
		return "", nil, 0, err
	}

	wkbType := order.Uint32(data[1:5])
	offset := wkbHeaderSize

	geomType, coords, consumed, err = decodeWKBGeometry(data, offset, wkbType, order)

	return geomType, coords, consumed, err
}

func byteOrderFromByte(b byte) (binary.ByteOrder, error) {
	switch b {
	case 0:
		return binary.BigEndian, nil
	case 1:
		return binary.LittleEndian, nil
	default:
		return nil, fmt.Errorf("wkb: unknown byte order %d", b)
	}
}

func decodeWKBGeometry(data []byte, offset int, wkbType uint32, order binary.ByteOrder) (string, any, int, error) {
	switch wkbType {
	case wkbPoint:
		return decodePoint(data, offset, order)
	case wkbLineString:
		return decodeLineString(data, offset, order)
	case wkbPolygon:
		return decodePolygon(data, offset, order)
	case wkbMultiPoint:
		return decodeMultiGeometry("MultiPoint", data, offset, order)
	case wkbMultiLineString:
		return decodeMultiGeometry("MultiLineString", data, offset, order)
	case wkbMultiPolygon:
		return decodeMultiGeometry("MultiPolygon", data, offset, order)
	default:
		return "", nil, 0, fmt.Errorf("wkb: unsupported geometry type %d", wkbType)
	}
}

func decodePoint(data []byte, offset int, order binary.ByteOrder) (string, any, int, error) {
	if len(data) < offset+2*float64Size {
		return "", nil, 0, errors.New("wkb: Point data too short")
	}

	x := readFloat64(data[offset:], order)
	y := readFloat64(data[offset+float64Size:], order)

	return "Point", []any{x, y}, offset + 2*float64Size, nil
}

func decodeLineString(data []byte, offset int, order binary.ByteOrder) (string, any, int, error) {
	if len(data) < offset+uint32Size {
		return "", nil, 0, errors.New("wkb: LineString numPoints too short")
	}

	numPoints := int(order.Uint32(data[offset:]))
	offset += uint32Size

	pts, n, err := readPoints(data[offset:], numPoints, order)
	if err != nil {
		return "", nil, 0, err
	}

	return "LineString", pts, offset + n, nil
}

func decodePolygon(data []byte, offset int, order binary.ByteOrder) (string, any, int, error) {
	if len(data) < offset+uint32Size {
		return "", nil, 0, errors.New("wkb: Polygon numRings too short")
	}

	numRings := int(order.Uint32(data[offset:]))
	offset += uint32Size

	rings := make([]any, 0, numRings)

	for range numRings {
		if len(data) < offset+uint32Size {
			return "", nil, 0, errors.New("wkb: Polygon ring numPoints too short")
		}

		numPoints := int(order.Uint32(data[offset:]))
		offset += uint32Size

		pts, n, err := readPoints(data[offset:], numPoints, order)
		if err != nil {
			return "", nil, 0, err
		}

		offset += n

		rings = append(rings, pts)
	}

	return "Polygon", rings, offset, nil
}

func decodeMultiGeometry(geomType string, data []byte, offset int, order binary.ByteOrder) (string, any, int, error) {
	if len(data) < offset+uint32Size {
		return "", nil, 0, fmt.Errorf("wkb: %s count too short", geomType)
	}

	numParts := int(order.Uint32(data[offset:]))
	offset += uint32Size

	parts := make([]any, 0, numParts)

	for range numParts {
		_, partCoords, n, err := decodeWKB(data[offset:])
		if err != nil {
			return "", nil, 0, fmt.Errorf("wkb: %s part: %w", geomType, err)
		}

		offset += n

		parts = append(parts, partCoords)
	}

	return geomType, parts, offset, nil
}

func readPoints(data []byte, numPoints int, order binary.ByteOrder) ([]any, int, error) {
	needed := numPoints * 2 * float64Size

	if len(data) < needed {
		return nil, 0, fmt.Errorf("wkb: not enough data for %d points", numPoints)
	}

	pts := make([]any, 0, numPoints)
	offset := 0

	for range numPoints {
		x := readFloat64(data[offset:], order)
		y := readFloat64(data[offset+float64Size:], order)
		offset += 2 * float64Size

		pts = append(pts, []any{x, y})
	}

	return pts, offset, nil
}

func readFloat64(data []byte, order binary.ByteOrder) float64 {
	bits := order.Uint64(data[:float64Size])

	return math.Float64frombits(bits)
}
