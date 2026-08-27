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

// Resource limits for untrusted GeoTIFF input.
//
// A TIFF header is a handful of bytes that declares how much data the decoder
// should allocate, so every allocation derived from it has to be bounded before
// it is made. The numbers below are policy choices: generous enough for any
// real digital terrain model, small enough that a malformed header cannot
// request an allocation the runtime cannot refuse.
const (
	// maxRasterDimension caps ImageWidth and ImageLength individually.
	// 2^20 samples per side is a 1000 km edge at 1 m resolution.
	maxRasterDimension = 1 << 20

	// maxRasterPixels caps width*height. At 8 bytes per decoded sample this is
	// a 2 GiB elevation grid, well beyond any tile the engine loads in practice.
	maxRasterPixels = 1 << 28

	// maxIFDEntries caps the entries in a single image file directory. Classic
	// TIFF encodes the count in 16 bits; BigTIFF uses 64 bits, so the cap is
	// what keeps a BigTIFF IFD count from driving the entry allocation.
	maxIFDEntries = 1 << 16

	// maxTagValueCount caps the element count of a single IFD entry, such as
	// StripOffsets or TileOffsets. A raster with more strips than it has rows,
	// or more tiles than a 16x16-tiled maxRasterPixels image, is malformed.
	maxTagValueCount = 1 << 20

	// maxCompressionRatio bounds the declared raster payload against the size
	// of the file that declares it. DEFLATE cannot expand data by much more
	// than 1000:1, so a header promising more than this is lying no matter what
	// the pixel-count limit allows. This is what keeps a few-hundred-byte
	// upload from sizing a gigabyte-scale buffer.
	maxCompressionRatio = 1024

	// decompressionSlackBytes is added to the expected payload size when
	// bounding decompressed strip/tile data. Some encoders pad the final strip
	// to a full strip height, so the decoded stream may legitimately exceed the
	// image payload by a little; it may never exceed it by orders of magnitude
	// (which is what a deflate bomb would do).
	decompressionSlackBytes = 1 << 20
)

// bitsPerSampleSupported reports whether bps is a sample width the pixel
// decoders understand. Anything else -- including a missing BitsPerSample tag,
// which reads back as 0 -- must be rejected before sizing an allocation.
func bitsPerSampleSupported(bps uint64) bool {
	switch bps {
	case 8, 16, 32, 64:
		return true
	default:
		return false
	}
}

// validateRasterGeometry rejects a raster header whose declared geometry would
// drive an unreasonable allocation. It runs before any allocation derived from
// width, height, or bits-per-sample, and uses uint64 arithmetic throughout so
// that no intermediate can overflow or go negative.
func validateRasterGeometry(width, height, bps uint64) error {
	if width == 0 || height == 0 {
		return errors.New("missing or zero image dimensions")
	}

	if width > maxRasterDimension || height > maxRasterDimension {
		return fmt.Errorf("image dimensions %dx%d exceed the per-side limit of %d", width, height, maxRasterDimension)
	}

	if !bitsPerSampleSupported(bps) {
		return fmt.Errorf("unsupported or missing BitsPerSample: %d", bps)
	}

	// Both factors are at most 2^20 here, so the product cannot overflow.
	if width*height > maxRasterPixels {
		return fmt.Errorf("image has %d pixels, exceeding the limit of %d", width*height, maxRasterPixels)
	}

	return nil
}

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
		return nil, fmt.Errorf("open geotiff %s: %w", path, err)
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
	// Compared in uint64: an offset near 2^63 would become a negative int and
	// slip past a signed comparison, then index the slice out of range.
	if offset >= uint64(len(data)) {
		return nil, errors.New("IFD offset beyond file")
	}

	pos := int(offset) //nolint:gosec // bounded by len(data) above

	var (
		numEntries int
		entrySize  int
	)

	var rawCount uint64

	if bigtiff {
		if pos+8 > len(data) {
			return nil, errors.New("truncated BigTIFF IFD count")
		}

		rawCount = order.Uint64(data[pos : pos+8])
		pos += 8
		entrySize = 20
	} else {
		if pos+2 > len(data) {
			return nil, errors.New("truncated IFD count")
		}

		rawCount = uint64(order.Uint16(data[pos : pos+2]))
		pos += 2
		entrySize = 12
	}

	// Bound the count before it sizes an allocation: it is attacker-controlled
	// (64 bits wide in BigTIFF) and the entries it promises must actually fit
	// in the remaining bytes of the file.
	if rawCount > maxIFDEntries {
		return nil, fmt.Errorf("IFD declares %d entries, exceeding the limit of %d", rawCount, maxIFDEntries)
	}

	numEntries = int(rawCount)
	if numEntries > (len(data)-pos)/entrySize {
		return nil, fmt.Errorf("IFD declares %d entries but only %d bytes remain", numEntries, len(data)-pos)
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

	var (
		count      uint64
		valueBytes []byte
	)

	if bigtiff {
		count = order.Uint64(data[pos+4 : pos+12])
		valueBytes = data[pos+12 : pos+20]
	} else {
		count = uint64(order.Uint32(data[pos+4 : pos+8]))
		valueBytes = data[pos+8 : pos+12]
	}

	elemSize := typeSize(dataType)

	// count is attacker-controlled (32 or 64 bits wide); reject a product that
	// would wrap before it is used as a length or an end offset.
	if count > math.MaxUint64/elemSize {
		return ifdEntry{}, fmt.Errorf("tag %d: value count %d overflows", tag, count)
	}

	totalSize := count * elemSize

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
		if end < offset || end > uint64(len(data)) {
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
	bps := getUint(tags, tagBitsPerSample, order)

	// Must happen before anything sized by these values is allocated.
	err := validateRasterGeometry(width, height, bps)
	if err != nil {
		return nil, err
	}

	sampleFmt := getUint(tags, tagSampleFormat, order)

	if sampleFmt == 0 {
		sampleFmt = sampleFormatUInt // default per TIFF spec
	}

	compression := getUint(tags, tagCompression, order)
	if compression == 0 {
		compression = compressionNone
	}

	// Read pixel data.
	var (
		rawPixels []byte
		readErr   error
	)

	//nolint:gosec // dimensions and bits-per-sample validated by validateRasterGeometry
	iWidth, iHeight, iBps, iSampleFmt := int(width), int(height), int(bps), int(sampleFmt)
	iCompression := int(compression)

	// Bounded by maxRasterPixels * 8, i.e. 2 GiB.
	payloadBytes := iWidth * iHeight * (iBps / 8)

	// Division rather than multiplication so the comparison cannot overflow.
	if payloadBytes/maxCompressionRatio > len(data) {
		return nil, fmt.Errorf("header declares %d bytes of pixel data, implausible for a %d byte file", payloadBytes, len(data))
	}

	if _, hasTiles := tags[tagTileOffsets]; hasTiles {
		rawPixels, readErr = readTiledData(data, order, tags, iWidth, iHeight, iBps/8)
	} else {
		rawPixels, readErr = readStrippedData(data, order, tags, iCompression, payloadBytes)
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

func readStrippedData(data []byte, order binary.ByteOrder, tags map[uint16]ifdEntry, compression, payloadBytes int) ([]byte, error) {
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

	budget := payloadBytes + decompressionSlackBytes

	var result []byte

	for i := range offsets {
		chunk, sliceErr := sliceChunk(data, offsets[i], counts[i])
		if sliceErr != nil {
			return nil, fmt.Errorf("strip %d: %w", i, sliceErr)
		}

		decoded, decErr := decompressChunk(chunk, compression, budget-len(result))
		if decErr != nil {
			return nil, fmt.Errorf("strip %d: %w", i, decErr)
		}

		if len(result)+len(decoded) > budget {
			return nil, fmt.Errorf("strip %d: decoded pixel data exceeds the %d byte budget", i, budget)
		}

		result = append(result, decoded...)
	}

	return result, nil
}

// sliceChunk bounds-checks an attacker-controlled (offset, count) pair against
// the file before converting it to int. The comparison is done in uint64 so a
// count near 2^63 cannot become a negative int and slip past the check.
func sliceChunk(data []byte, offset, count uint64) ([]byte, error) {
	size := uint64(len(data))

	if offset > size || count > size-offset {
		return nil, fmt.Errorf("offset %d + count %d exceeds file size %d", offset, count, size)
	}

	return data[offset : offset+count], nil
}

func readTiledData(data []byte, order binary.ByteOrder, tags map[uint16]ifdEntry, imgWidth, imgHeight, bytesPerSample int) ([]byte, error) {
	rawTileW := getUint(tags, tagTileWidth, order)
	rawTileH := getUint(tags, tagTileLength, order)

	if rawTileW == 0 || rawTileH == 0 {
		return nil, errors.New("missing tile dimensions")
	}

	if rawTileW > maxRasterDimension || rawTileH > maxRasterDimension {
		return nil, fmt.Errorf("tile dimensions %dx%d exceed the per-side limit of %d", rawTileW, rawTileH, maxRasterDimension)
	}

	tileW := int(rawTileW)
	tileH := int(rawTileH)

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
	// imgWidth, imgHeight and bytesPerSample all passed validateRasterGeometry,
	// so this product is bounded by maxRasterPixels * 8.
	result := make([]byte, imgHeight*rowBytes)
	tileBudget := tileW*tileH*bytesPerSample + decompressionSlackBytes

	for ty := range tilesDown {
		for tx := range tilesAcross {
			idx := ty*tilesAcross + tx

			if idx >= len(offsets) || idx >= len(counts) {
				return nil, fmt.Errorf("tile index %d out of range", idx)
			}

			chunk, sliceErr := sliceChunk(data, offsets[idx], counts[idx])
			if sliceErr != nil {
				return nil, fmt.Errorf("tile %d: %w", idx, sliceErr)
			}

			decoded, decErr := decompressChunk(chunk, compression, tileBudget)
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

// decompressChunk expands one strip or tile. limitBytes bounds the decoded
// output so that a highly compressible payload (a deflate bomb) cannot expand
// past the size the validated raster geometry accounts for.
func decompressChunk(chunk []byte, compression, limitBytes int) ([]byte, error) {
	if limitBytes < 0 {
		limitBytes = 0
	}

	switch compression {
	case compressionNone:
		return chunk, nil
	case compressionDeflate, compressionAdobeDeflate:
		r, err := zlib.NewReader(bytes.NewReader(chunk))
		if err != nil {
			return nil, fmt.Errorf("deflate init: %w", err)
		}

		defer r.Close()

		// Read one byte past the limit so an oversized stream is detectable.
		out, err := io.ReadAll(io.LimitReader(r, int64(limitBytes)+1))
		if err != nil {
			return nil, fmt.Errorf("deflate read: %w", err)
		}

		if len(out) > limitBytes {
			return nil, fmt.Errorf("deflate output exceeds the %d byte budget", limitBytes)
		}

		return out, nil
	default:
		return nil, fmt.Errorf("unsupported compression: %d", compression)
	}
}

func decodePixels(raw []byte, order binary.ByteOrder, width, height, bps, sampleFmt int) ([]float64, error) {
	// Resolving the decoder first pins bps to one of the supported widths, so
	// bytesPerSample below is at least 2. That matters: when BitsPerSample is
	// missing the tag reads back as 0, and a zero bytesPerSample turns the
	// "enough pixel data?" guard into a comparison against 0 that any width and
	// height would pass.
	decode, err := pixelDecoder(sampleFmt, bps)
	if err != nil {
		return nil, err
	}

	n := width * height
	bytesPerSample := bps / 8

	if len(raw) < n*bytesPerSample {
		return nil, fmt.Errorf("pixel data too short: got %d bytes, expected %d", len(raw), n*bytesPerSample)
	}

	grid := make([]float64, n)
	decode(grid, raw, order, n)

	return grid, nil
}

// pixelDecoder returns the sample decoder for a (sample format, bits per
// sample) pair, or an error when the combination is unsupported.
func pixelDecoder(sampleFmt, bps int) (func([]float64, []byte, binary.ByteOrder, int), error) {
	switch {
	case sampleFmt == sampleFormatFloat && bps == 32:
		return decodeFloat32, nil
	case sampleFmt == sampleFormatFloat && bps == 64:
		return decodeFloat64, nil
	case sampleFmt == sampleFormatInt && bps == 16:
		return decodeInt16, nil
	case sampleFmt == sampleFormatUInt && bps == 16:
		return decodeUint16, nil
	case sampleFmt == sampleFormatInt && bps == 32:
		return decodeInt32, nil
	case sampleFmt == sampleFormatUInt && bps == 32:
		return decodeUint32, nil
	default:
		return nil, fmt.Errorf("unsupported sample format %d with %d bits per sample", sampleFmt, bps)
	}
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

	switch e.dataType {
	case tiffTypeShort, tiffTypeLong, tiffTypeLong8:
	default:
		return nil, fmt.Errorf("unsupported type %d for uint slice", e.dataType)
	}

	// e.count is attacker-controlled; bound it and confirm the entry actually
	// carries that many elements before sizing the result slice.
	if e.count > maxTagValueCount {
		return nil, fmt.Errorf("tag %d declares %d values, exceeding the limit of %d", tag, e.count, maxTagValueCount)
	}

	elemSize := typeSize(e.dataType)
	if uint64(len(e.data)) < e.count*elemSize {
		return nil, fmt.Errorf("tag %d: %d values need %d bytes, entry has %d", tag, e.count, e.count*elemSize, len(e.data))
	}

	result := make([]uint64, e.count)

	for i := range e.count {
		off := i * elemSize

		switch e.dataType {
		case tiffTypeShort:
			result[i] = uint64(order.Uint16(e.data[off : off+2]))
		case tiffTypeLong:
			result[i] = uint64(order.Uint32(e.data[off : off+4]))
		case tiffTypeLong8:
			result[i] = order.Uint64(e.data[off : off+8])
		}
	}

	return result, nil
}
