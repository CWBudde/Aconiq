package terrain

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// TIFF tag IDs.
const (
	tagImageWidth      = 256
	tagImageLength     = 257
	tagBitsPerSample   = 258
	tagCompression     = 259
	tagStripOffsets    = 273
	tagStripByteCounts = 279
	tagSampleFormat    = 339
	tagTileWidth       = 322
	tagTileLength      = 323
	tagTileOffsets     = 324
	tagTileByteCounts  = 325
	tagGDALNoData      = 42113

	// GeoTIFF tags.
	tagModelPixelScale = 33550
	tagModelTiepoint   = 33922
)

// TIFF type IDs.
const (
	tiffTypeByte   = 1
	tiffTypeShort  = 3
	tiffTypeLong   = 4
	tiffTypeDouble = 12
	tiffTypeLong8  = 16
)

// Compression types.
const (
	compressionNone         = 1
	compressionDeflate      = 8
	compressionAdobeDeflate = 32946
)

// Sample format.
const (
	sampleFormatUInt  = 1
	sampleFormatInt   = 2
	sampleFormatFloat = 3
)

type ifdEntry struct {
	tag      uint16
	dataType uint16
	count    uint64
	data     []byte // raw value bytes
}

// readGeoTIFF reads a single-band GeoTIFF elevation raster into a gridModel.
func readGeoTIFF(path string) (*gridModel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return parseGeoTIFF(data)
}

func parseGeoTIFF(data []byte) (*gridModel, error) {
	if len(data) < 8 {
		return nil, errors.New("file too short for TIFF header")
	}

	var order binary.ByteOrder

	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid TIFF byte order: %q", string(data[:2]))
	}

	magic := order.Uint16(data[2:4])

	var ifdOffset uint64

	var bigtiff bool

	switch magic {
	case 42: // Classic TIFF
		ifdOffset = uint64(order.Uint32(data[4:8]))
	case 43: // BigTIFF
		bigtiff = true

		if len(data) < 16 {
			return nil, errors.New("file too short for BigTIFF header")
		}

		ifdOffset = order.Uint64(data[8:16])
	default:
		return nil, fmt.Errorf("unsupported TIFF magic: %d", magic)
	}

	entries, err := readIFD(data, order, ifdOffset, bigtiff)
	if err != nil {
		return nil, fmt.Errorf("read IFD: %w", err)
	}

	return buildGrid(data, order, entries)
}

func readIFD(data []byte, order binary.ByteOrder, offset uint64, bigtiff bool) ([]ifdEntry, error) {
	if int(offset) >= len(data) { //nolint:gosec // TIFF IFD offset bounded by file size
		return nil, errors.New("IFD offset beyond file")
	}

	pos := int(offset) //nolint:gosec // TIFF IFD offset bounded by file size

	var numEntries int
	var entrySize int

	if bigtiff {
		if pos+8 > len(data) {
			return nil, errors.New("truncated BigTIFF IFD count")
		}

		numEntries = int(order.Uint64(data[pos : pos+8])) //nolint:gosec // BigTIFF entry count bounded by file size
		pos += 8
		entrySize = 20
	} else {
		if pos+2 > len(data) {
			return nil, errors.New("truncated IFD count")
		}

		numEntries = int(order.Uint16(data[pos : pos+2]))
		pos += 2
		entrySize = 12
	}

	entries := make([]ifdEntry, 0, numEntries)

	for range numEntries {
		if pos+entrySize > len(data) {
			return nil, errors.New("truncated IFD entry")
		}

		e, err := parseIFDEntry(data, order, pos, bigtiff)
		if err != nil {
			return nil, err
		}

		entries = append(entries, e)
		pos += entrySize
	}

	return entries, nil
}

func parseIFDEntry(data []byte, order binary.ByteOrder, pos int, bigtiff bool) (ifdEntry, error) {
	tag := order.Uint16(data[pos : pos+2])
	dataType := order.Uint16(data[pos+2 : pos+4])

	var count uint64
	var valueBytes []byte

	if bigtiff {
		count = order.Uint64(data[pos+4 : pos+12])
		valueBytes = data[pos+12 : pos+20]
	} else {
		count = uint64(order.Uint32(data[pos+4 : pos+8]))
		valueBytes = data[pos+8 : pos+12]
	}

	totalSize := count * typeSize(dataType)

	var maxInline uint64
	if bigtiff {
		maxInline = 8
	} else {
		maxInline = 4
	}

	var entryData []byte

	if totalSize <= maxInline {
		entryData = make([]byte, totalSize)
		copy(entryData, valueBytes[:totalSize])
	} else {
		var offset uint64
		if bigtiff {
			offset = order.Uint64(valueBytes[:8])
		} else {
			offset = uint64(order.Uint32(valueBytes[:4]))
		}

		end := offset + totalSize
		if end > uint64(len(data)) {
			return ifdEntry{}, fmt.Errorf("tag %d: data offset %d+%d exceeds file size %d", tag, offset, totalSize, len(data))
		}

		entryData = data[offset:end]
	}

	return ifdEntry{tag: tag, dataType: dataType, count: count, data: entryData}, nil
}

func typeSize(dataType uint16) uint64 {
	switch dataType {
	case tiffTypeByte:
		return 1
	case tiffTypeShort:
		return 2
	case tiffTypeLong:
		return 4
	case tiffTypeDouble, tiffTypeLong8:
		return 8
	default:
		return 1
	}
}

func buildGrid(data []byte, order binary.ByteOrder, entries []ifdEntry) (*gridModel, error) {
	tags := make(map[uint16]ifdEntry, len(entries))
	for _, e := range entries {
		tags[e.tag] = e
	}

	width := getUint(tags, tagImageWidth, order)
	height := getUint(tags, tagImageLength, order)

	if width == 0 || height == 0 {
		return nil, errors.New("missing or zero image dimensions")
	}

	bps := getUint(tags, tagBitsPerSample, order)
	sampleFmt := getUint(tags, tagSampleFormat, order)

	if sampleFmt == 0 {
		sampleFmt = sampleFormatUInt // default per TIFF spec
	}

	compression := getUint(tags, tagCompression, order)
	if compression == 0 {
		compression = compressionNone
	}

	// Read pixel data.
	var rawPixels []byte
	var readErr error

	//nolint:gosec // TIFF dimensions bounded by uint16/uint32, safe on 64-bit
	iWidth, iHeight, iBps, iSampleFmt, iCompression := int(width), int(height), int(bps), int(sampleFmt), int(compression)

	if _, hasTiles := tags[tagTileOffsets]; hasTiles {
		rawPixels, readErr = readTiledData(data, order, tags, iWidth, iHeight, iBps/8)
	} else {
		rawPixels, readErr = readStrippedData(data, order, tags, iCompression)
	}

	if readErr != nil {
		return nil, fmt.Errorf("read pixel data: %w", readErr)
	}

	grid, err := decodePixels(rawPixels, order, iWidth, iHeight, iBps, iSampleFmt)
	if err != nil {
		return nil, err
	}

	// Parse GeoTIFF transform.
	originX, originY, pxSizeX, pxSizeY, err := parseGeoTransform(tags, order)
	if err != nil {
		return nil, fmt.Errorf("parse geotransform: %w", err)
	}

	g := &gridModel{
		data:       grid,
		width:      iWidth,
		height:     iHeight,
		originX:    originX,
		originY:    originY,
		pixelSizeX: pxSizeX,
		pixelSizeY: pxSizeY,
	}

	// Parse nodata.
	if nd, ok := tags[tagGDALNoData]; ok {
		s := string(bytes.TrimRight(nd.data, "\x00"))

		var ndVal float64

		_, err := fmt.Sscanf(s, "%f", &ndVal)
		if err == nil {
			g.noData = ndVal
			g.hasNoData = true
		}
	}

	return g, nil
}

func readStrippedData(data []byte, order binary.ByteOrder, tags map[uint16]ifdEntry, compression int) ([]byte, error) {
	offsets, err := getUintSlice(tags, tagStripOffsets, order)
	if err != nil {
		return nil, fmt.Errorf("strip offsets: %w", err)
	}

	counts, err := getUintSlice(tags, tagStripByteCounts, order)
	if err != nil {
		return nil, fmt.Errorf("strip byte counts: %w", err)
	}

	if len(offsets) != len(counts) {
		return nil, fmt.Errorf("strip offsets (%d) != byte counts (%d)", len(offsets), len(counts))
	}

	var result []byte

	for i := range offsets {
		off := int(offsets[i]) //nolint:gosec // TIFF strip offsets bounded by file size
		cnt := int(counts[i])  //nolint:gosec // TIFF strip byte counts bounded by file size

		if off+cnt > len(data) {
			return nil, fmt.Errorf("strip %d: offset %d + count %d exceeds file size", i, off, cnt)
		}

		chunk := data[off : off+cnt]

		decoded, decErr := decompressChunk(chunk, compression)
		if decErr != nil {
			return nil, fmt.Errorf("strip %d: %w", i, decErr)
		}

		result = append(result, decoded...)
	}

	return result, nil
}

func readTiledData(data []byte, order binary.ByteOrder, tags map[uint16]ifdEntry, imgWidth, imgHeight, bytesPerSample int) ([]byte, error) {
	tileW := int(getUint(tags, tagTileWidth, order))  //nolint:gosec // TIFF tile dimensions bounded by uint16/uint32
	tileH := int(getUint(tags, tagTileLength, order)) //nolint:gosec // TIFF tile dimensions bounded by uint16/uint32

	if tileW == 0 || tileH == 0 {
		return nil, errors.New("missing tile dimensions")
	}

	compression := int(getUint(tags, tagCompression, order)) //nolint:gosec // TIFF compression code bounded by uint16

	offsets, err := getUintSlice(tags, tagTileOffsets, order)
	if err != nil {
		return nil, fmt.Errorf("tile offsets: %w", err)
	}

	counts, err := getUintSlice(tags, tagTileByteCounts, order)
	if err != nil {
		return nil, fmt.Errorf("tile byte counts: %w", err)
	}

	tilesAcross := (imgWidth + tileW - 1) / tileW
	tilesDown := (imgHeight + tileH - 1) / tileH
	rowBytes := imgWidth * bytesPerSample
	result := make([]byte, imgHeight*rowBytes)

	for ty := range tilesDown {
		for tx := range tilesAcross {
			idx := ty*tilesAcross + tx

			if idx >= len(offsets) || idx >= len(counts) {
				return nil, fmt.Errorf("tile index %d out of range", idx)
			}

			off := int(offsets[idx]) //nolint:gosec // TIFF tile offsets bounded by file size
			cnt := int(counts[idx])  //nolint:gosec // TIFF tile byte counts bounded by file size

			if off+cnt > len(data) {
				return nil, fmt.Errorf("tile %d: offset exceeds file size", idx)
			}

			decoded, decErr := decompressChunk(data[off:off+cnt], compression)
			if decErr != nil {
				return nil, fmt.Errorf("tile %d: %w", idx, decErr)
			}

			copyTileToImage(result, decoded, tx, ty, tileW, tileH, imgWidth, imgHeight, bytesPerSample, rowBytes)
		}
	}

	return result, nil
}

// copyTileToImage copies decoded tile pixel rows into the full image buffer,
// handling edge tiles that extend beyond the image boundary.
func copyTileToImage(dst, src []byte, tx, ty, tileW, tileH, imgWidth, imgHeight, bytesPerSample, rowBytes int) {
	tileRowBytes := tileW * bytesPerSample

	for row := range tileH {
		imgRow := ty*tileH + row
		if imgRow >= imgHeight {
			break
		}

		srcStart := row * tileRowBytes
		dstStart := imgRow*rowBytes + tx*tileW*bytesPerSample
		copyLen := tileW * bytesPerSample

		if tx*tileW+tileW > imgWidth {
			copyLen = (imgWidth - tx*tileW) * bytesPerSample
		}

		if srcStart+copyLen > len(src) {
			break
		}

		copy(dst[dstStart:dstStart+copyLen], src[srcStart:srcStart+copyLen])
	}
}

func decompressChunk(chunk []byte, compression int) ([]byte, error) {
	switch compression {
	case compressionNone:
		return chunk, nil
	case compressionDeflate, compressionAdobeDeflate:
		r, err := zlib.NewReader(bytes.NewReader(chunk))
		if err != nil {
			return nil, fmt.Errorf("deflate init: %w", err)
		}

		defer r.Close()

		return io.ReadAll(r)
	default:
		return nil, fmt.Errorf("unsupported compression: %d", compression)
	}
}

func decodePixels(raw []byte, order binary.ByteOrder, width, height, bps, sampleFmt int) ([]float64, error) {
	n := width * height
	bytesPerSample := bps / 8

	if len(raw) < n*bytesPerSample {
		return nil, fmt.Errorf("pixel data too short: got %d bytes, expected %d", len(raw), n*bytesPerSample)
	}

	grid := make([]float64, n)

	switch {
	case sampleFmt == sampleFormatFloat && bps == 32:
		decodeFloat32(grid, raw, order, n)
	case sampleFmt == sampleFormatFloat && bps == 64:
		decodeFloat64(grid, raw, order, n)
	case sampleFmt == sampleFormatInt && bps == 16:
		decodeInt16(grid, raw, order, n)
	case sampleFmt == sampleFormatUInt && bps == 16:
		decodeUint16(grid, raw, order, n)
	case sampleFmt == sampleFormatInt && bps == 32:
		decodeInt32(grid, raw, order, n)
	case sampleFmt == sampleFormatUInt && bps == 32:
		decodeUint32(grid, raw, order, n)
	default:
		return nil, fmt.Errorf("unsupported sample format %d with %d bits per sample", sampleFmt, bps)
	}

	return grid, nil
}

func decodeFloat32(grid []float64, raw []byte, order binary.ByteOrder, n int) {
	for i := range n {
		off := i * 4
		grid[i] = float64(math.Float32frombits(order.Uint32(raw[off : off+4])))
	}
}

func decodeFloat64(grid []float64, raw []byte, order binary.ByteOrder, n int) {
	for i := range n {
		off := i * 8
		grid[i] = math.Float64frombits(order.Uint64(raw[off : off+8]))
	}
}

func decodeInt16(grid []float64, raw []byte, order binary.ByteOrder, n int) {
	for i := range n {
		off := i * 2
		grid[i] = float64(int16(order.Uint16(raw[off : off+2]))) //nolint:gosec // intentional reinterpretation of uint16 as int16
	}
}

func decodeUint16(grid []float64, raw []byte, order binary.ByteOrder, n int) {
	for i := range n {
		off := i * 2
		grid[i] = float64(order.Uint16(raw[off : off+2]))
	}
}

func decodeInt32(grid []float64, raw []byte, order binary.ByteOrder, n int) {
	for i := range n {
		off := i * 4
		grid[i] = float64(int32(order.Uint32(raw[off : off+4]))) //nolint:gosec // intentional reinterpretation of uint32 as int32
	}
}

func decodeUint32(grid []float64, raw []byte, order binary.ByteOrder, n int) {
	for i := range n {
		off := i * 4
		grid[i] = float64(order.Uint32(raw[off : off+4]))
	}
}

func parseGeoTransform(tags map[uint16]ifdEntry, order binary.ByteOrder) (originX, originY, pixelSizeX, pixelSizeY float64, err error) {
	scaleEntry, hasScale := tags[tagModelPixelScale]
	tpEntry, hasTiepoint := tags[tagModelTiepoint]

	if !hasScale || !hasTiepoint {
		return 0, 0, 0, 0, errors.New("missing ModelPixelScale or ModelTiepoint tags")
	}

	if len(scaleEntry.data) < 16 {
		return 0, 0, 0, 0, errors.New("ModelPixelScale data too short")
	}

	pixelSizeX = math.Float64frombits(order.Uint64(scaleEntry.data[0:8]))
	pixelSizeY = math.Float64frombits(order.Uint64(scaleEntry.data[8:16]))

	if pixelSizeX <= 0 || pixelSizeY <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("invalid pixel scale: %f x %f", pixelSizeX, pixelSizeY)
	}

	if len(tpEntry.data) < 48 {
		return 0, 0, 0, 0, errors.New("ModelTiepoint data too short")
	}

	// Tiepoint: [pixelI, pixelJ, pixelK, geoX, geoY, geoZ]
	pixelI := math.Float64frombits(order.Uint64(tpEntry.data[0:8]))
	pixelJ := math.Float64frombits(order.Uint64(tpEntry.data[8:16]))
	geoX := math.Float64frombits(order.Uint64(tpEntry.data[24:32]))
	geoY := math.Float64frombits(order.Uint64(tpEntry.data[32:40]))

	// Origin is the pixel center of pixel (0,0), derived from the tiepoint.
	originX = geoX - pixelI*pixelSizeX
	originY = geoY + pixelJ*pixelSizeY

	return originX, originY, pixelSizeX, pixelSizeY, nil
}

// getUint reads a single unsigned integer value from an IFD entry.
func getUint(tags map[uint16]ifdEntry, tag uint16, order binary.ByteOrder) uint64 {
	e, ok := tags[tag]
	if !ok {
		return 0
	}

	return readUintValue(e, order)
}

func readUintValue(e ifdEntry, order binary.ByteOrder) uint64 {
	switch e.dataType {
	case tiffTypeShort:
		if len(e.data) >= 2 {
			return uint64(order.Uint16(e.data[:2]))
		}
	case tiffTypeLong:
		if len(e.data) >= 4 {
			return uint64(order.Uint32(e.data[:4]))
		}
	case tiffTypeLong8:
		if len(e.data) >= 8 {
			return order.Uint64(e.data[:8])
		}
	case tiffTypeByte:
		if len(e.data) >= 1 {
			return uint64(e.data[0])
		}
	}

	return 0
}

func getUintSlice(tags map[uint16]ifdEntry, tag uint16, order binary.ByteOrder) ([]uint64, error) {
	e, ok := tags[tag]
	if !ok {
		return nil, fmt.Errorf("tag %d not found", tag)
	}

	result := make([]uint64, e.count)
	elemSize := typeSize(e.dataType)

	for i := range e.count {
		off := i * elemSize

		switch e.dataType {
		case tiffTypeShort:
			result[i] = uint64(order.Uint16(e.data[off : off+2]))
		case tiffTypeLong:
			result[i] = uint64(order.Uint32(e.data[off : off+4]))
		case tiffTypeLong8:
			result[i] = order.Uint64(e.data[off : off+8])
		default:
			return nil, fmt.Errorf("unsupported type %d for uint slice", e.dataType)
		}
	}

	return result, nil
}
