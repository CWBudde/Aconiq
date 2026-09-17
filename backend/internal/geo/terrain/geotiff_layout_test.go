package terrain

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"sort"
	"strings"
	"testing"
)

// --- A configurable GeoTIFF writer ---
//
// buildMinimalGeoTIFF in terrain_test.go writes exactly one shape: uncompressed
// little-endian float32 in a single strip. The sample formats, the tiled
// layout and the deflate path are all reached only through a file that says so,
// so this builder exists to produce those files.

// tiffSpec describes the file to write.
type tiffSpec struct {
	width, height int

	// bitsPerSample and sampleFormat go into the tags of the same name and must
	// match how pixels were encoded.
	bitsPerSample uint32
	sampleFormat  uint32

	// pixels is the already-encoded, row-major sample data.
	pixels []byte

	// tileWidth and tileHeight switch the file to the tiled layout when both
	// are non-zero. pixels is then cut into tiles by the builder, padding edge
	// tiles the way a TIFF encoder does.
	tileWidth, tileHeight int

	// stripRows splits the image into strips of that many rows, which is what a
	// TIFF with a RowsPerStrip smaller than the image looks like. Zero keeps
	// the whole image in one strip.
	stripRows int

	// deflate compresses every strip or tile with zlib.
	deflate bool

	// bigEndian writes an "MM" file.
	bigEndian bool

	originX, originY       float64
	pixelSizeX, pixelSizeY float64

	// noData, when non-empty, is written as the GDAL_NODATA ASCII tag.
	noData string

	// omitPixelScale and omitTiepoint drop the geotransform tags.
	omitPixelScale, omitTiepoint bool
}

// rawTag is one IFD entry before offsets are assigned.
type rawTag struct {
	tag   uint16
	dtype uint16
	count uint32
	data  []byte
}

// tiffWriter lays out header, payload blobs and IFD.
type tiffWriter struct {
	order binary.ByteOrder
	body  []byte // everything between the header and the IFD
	tags  []rawTag
}

// appendBlob places p after the header and returns its file offset.
func (w *tiffWriter) appendBlob(p []byte) uint32 {
	if len(w.body)%2 == 1 {
		w.body = append(w.body, 0)
	}

	offset := 8 + len(w.body)
	w.body = append(w.body, p...)

	return uint32(offset)
}

func (w *tiffWriter) addTag(tag, dtype uint16, count uint32, data []byte) {
	w.tags = append(w.tags, rawTag{tag: tag, dtype: dtype, count: count, data: data})
}

// addShort adds a single SHORT-typed tag.
func (w *tiffWriter) addShort(tag uint16, value uint32) {
	buf := make([]byte, 2)
	w.order.PutUint16(buf, uint16(value))
	w.addTag(tag, tiffTypeShort, 1, buf)
}

// addLongs adds a LONG-typed tag holding the given values.
func (w *tiffWriter) addLongs(tag uint16, values []uint32) {
	buf := make([]byte, 4*len(values))
	for i, v := range values {
		w.order.PutUint32(buf[i*4:], v)
	}

	w.addTag(tag, tiffTypeLong, uint32(len(values)), buf)
}

// addDoubles adds a DOUBLE-typed tag holding the given values.
func (w *tiffWriter) addDoubles(tag uint16, values []float64) {
	buf := make([]byte, 8*len(values))
	for i, v := range values {
		w.order.PutUint64(buf[i*8:], math.Float64bits(v))
	}

	w.addTag(tag, tiffTypeDouble, uint32(len(values)), buf)
}

// putU16 and putU32 append one value in the writer's byte order.
// binary.ByteOrder itself has no Append methods.
func (w *tiffWriter) putU16(dst []byte, v uint16) []byte {
	var buf [2]byte

	w.order.PutUint16(buf[:], v)

	return append(dst, buf[:]...)
}

func (w *tiffWriter) putU32(dst []byte, v uint32) []byte {
	var buf [4]byte

	w.order.PutUint32(buf[:], v)

	return append(dst, buf[:]...)
}

// bytes assembles the file: header, payload, then the IFD with every tag whose
// value does not fit in four bytes pointing back into the payload.
func (w *tiffWriter) bytes() []byte {
	sort.Slice(w.tags, func(i, j int) bool { return w.tags[i].tag < w.tags[j].tag })

	// Values longer than four bytes have to live in the payload, which must be
	// laid out before the IFD offsets can be written.
	offsets := make([]uint32, len(w.tags))

	for i, tag := range w.tags {
		if len(tag.data) > 4 {
			offsets[i] = w.appendBlob(tag.data)
		}
	}

	if len(w.body)%2 == 1 {
		w.body = append(w.body, 0)
	}

	ifdOffset := 8 + len(w.body)

	out := make([]byte, 0, ifdOffset+2+len(w.tags)*12+4)

	if w.order == binary.BigEndian {
		out = append(out, 'M', 'M')
	} else {
		out = append(out, 'I', 'I')
	}

	out = w.putU16(out, 42)
	out = w.putU32(out, uint32(ifdOffset))
	out = append(out, w.body...)

	out = w.putU16(out, uint16(len(w.tags)))

	for i, tag := range w.tags {
		out = w.putU16(out, tag.tag)
		out = w.putU16(out, tag.dtype)
		out = w.putU32(out, tag.count)

		if len(tag.data) > 4 {
			out = w.putU32(out, offsets[i])

			continue
		}

		// An inline value is left-justified in the four value bytes.
		inline := make([]byte, 4)
		copy(inline, tag.data)
		out = append(out, inline...)
	}

	// No next IFD.
	out = w.putU32(out, 0)

	return out
}

// deflateBytes zlib-compresses p the way a DEFLATE-compressed TIFF strip is.
func deflateBytes(t *testing.T, p []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := zlib.NewWriter(&buf)

	_, err := zw.Write(p)
	if err != nil {
		t.Fatalf("deflate write: %v", err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatalf("deflate close: %v", err)
	}

	return buf.Bytes()
}

// cutTiles slices row-major sample data into TIFF tiles, padding the edge tiles
// with zeroes as an encoder does.
func cutTiles(spec tiffSpec, bytesPerSample int) [][]byte {
	across := (spec.width + spec.tileWidth - 1) / spec.tileWidth
	down := (spec.height + spec.tileHeight - 1) / spec.tileHeight
	rowBytes := spec.width * bytesPerSample
	tileRowBytes := spec.tileWidth * bytesPerSample

	tiles := make([][]byte, 0, across*down)

	for ty := range down {
		for tx := range across {
			tile := make([]byte, spec.tileHeight*tileRowBytes)

			for row := range spec.tileHeight {
				imgRow := ty*spec.tileHeight + row
				if imgRow >= spec.height {
					break
				}

				copyLen := tileRowBytes
				if tx*spec.tileWidth+spec.tileWidth > spec.width {
					copyLen = (spec.width - tx*spec.tileWidth) * bytesPerSample
				}

				src := imgRow*rowBytes + tx*spec.tileWidth*bytesPerSample
				copy(tile[row*tileRowBytes:row*tileRowBytes+copyLen], spec.pixels[src:src+copyLen])
			}

			tiles = append(tiles, tile)
		}
	}

	return tiles
}

// cutStrips splits row-major sample data into strips of spec.stripRows rows.
// The last strip is short when the rows do not divide evenly, exactly as a TIFF
// encoder writes it.
func cutStrips(spec tiffSpec, bytesPerSample int) [][]byte {
	rowBytes := spec.width * bytesPerSample
	strips := make([][]byte, 0, (spec.height+spec.stripRows-1)/spec.stripRows)

	for start := 0; start < spec.height; start += spec.stripRows {
		end := min(start+spec.stripRows, spec.height)

		strips = append(strips, spec.pixels[start*rowBytes:end*rowBytes])
	}

	return strips
}

// buildGeoTIFF writes a GeoTIFF matching spec.
func buildGeoTIFF(t *testing.T, spec tiffSpec) []byte {
	t.Helper()

	writer := &tiffWriter{order: binary.LittleEndian}
	if spec.bigEndian {
		writer.order = binary.BigEndian
	}

	bytesPerSample := int(spec.bitsPerSample) / 8

	compression := uint32(compressionNone)
	if spec.deflate {
		compression = compressionDeflate
	}

	chunks := [][]byte{spec.pixels}

	switch {
	case spec.tileWidth > 0 && spec.tileHeight > 0:
		chunks = cutTiles(spec, bytesPerSample)
	case spec.stripRows > 0:
		chunks = cutStrips(spec, bytesPerSample)
	}

	chunkOffsets := make([]uint32, 0, len(chunks))
	chunkCounts := make([]uint32, 0, len(chunks))

	for _, chunk := range chunks {
		payload := chunk
		if spec.deflate {
			payload = deflateBytes(t, chunk)
		}

		chunkOffsets = append(chunkOffsets, writer.appendBlob(payload))
		chunkCounts = append(chunkCounts, uint32(len(payload)))
	}

	writer.addShort(tagImageWidth, uint32(spec.width))
	writer.addShort(tagImageLength, uint32(spec.height))
	writer.addShort(tagBitsPerSample, spec.bitsPerSample)
	writer.addShort(tagCompression, compression)
	writer.addShort(tagSampleFormat, spec.sampleFormat)

	if spec.tileWidth > 0 && spec.tileHeight > 0 {
		writer.addShort(tagTileWidth, uint32(spec.tileWidth))
		writer.addShort(tagTileLength, uint32(spec.tileHeight))
		writer.addLongs(tagTileOffsets, chunkOffsets)
		writer.addLongs(tagTileByteCounts, chunkCounts)
	} else {
		writer.addLongs(tagStripOffsets, chunkOffsets)
		writer.addLongs(tagStripByteCounts, chunkCounts)
	}

	if !spec.omitPixelScale {
		writer.addDoubles(tagModelPixelScale, []float64{spec.pixelSizeX, spec.pixelSizeY, 0})
	}

	if !spec.omitTiepoint {
		writer.addDoubles(tagModelTiepoint, []float64{0, 0, 0, spec.originX, spec.originY, 0})
	}

	if spec.noData != "" {
		writer.addTag(tagGDALNoData, tiffTypeByte, uint32(len(spec.noData)+1), append([]byte(spec.noData), 0))
	}

	return writer.bytes()
}

// encodeSamples renders values with enc, which must write exactly
// bytesPerSample bytes per value.
func encodeSamples(order binary.ByteOrder, values []float64, bytesPerSample int, enc func(binary.ByteOrder, []byte, float64)) []byte {
	out := make([]byte, len(values)*bytesPerSample)
	for i, v := range values {
		enc(order, out[i*bytesPerSample:], v)
	}

	return out
}

// --- Sample formats ---

// Each (sample format, bits per sample) pair the decoder claims to support has
// to actually round-trip. Only float32 was exercised before, so a wrong offset
// or a wrong sign reinterpretation in any of the others would have gone
// unnoticed — including the int16 case, which is the one that carries terrain
// below sea level.
func TestLoadFromBytesDecodesEverySupportedSampleFormat(t *testing.T) {
	t.Parallel()

	// A 2x2 grid whose values are representable in every format under test.
	values := []float64{100, 101, 102, 103}

	cases := []struct {
		name           string
		bits           uint32
		format         uint32
		bytesPerSample int
		encode         func(binary.ByteOrder, []byte, float64)
		values         []float64
	}{
		{
			name: "float32", bits: 32, format: sampleFormatFloat, bytesPerSample: 4,
			encode: func(o binary.ByteOrder, b []byte, v float64) { o.PutUint32(b, math.Float32bits(float32(v))) },
			values: []float64{100.5, 101.25, -3.75, 0},
		},
		{
			name: "float64", bits: 64, format: sampleFormatFloat, bytesPerSample: 8,
			encode: func(o binary.ByteOrder, b []byte, v float64) { o.PutUint64(b, math.Float64bits(v)) },
			values: []float64{100.1, 101.2, -3.3, 0.4},
		},
		{
			name: "int16", bits: 16, format: sampleFormatInt, bytesPerSample: 2,
			encode: func(o binary.ByteOrder, b []byte, v float64) { o.PutUint16(b, uint16(int16(v))) },
			// Below sea level is the case a uint16 decoder would silently turn
			// into +65 000 m.
			values: []float64{-410, -1, 0, 32767},
		},
		{
			name: "uint16", bits: 16, format: sampleFormatUInt, bytesPerSample: 2,
			encode: func(o binary.ByteOrder, b []byte, v float64) { o.PutUint16(b, uint16(v)) },
			values: []float64{0, 1, 8848, 65535},
		},
		{
			name: "int32", bits: 32, format: sampleFormatInt, bytesPerSample: 4,
			encode: func(o binary.ByteOrder, b []byte, v float64) { o.PutUint32(b, uint32(int32(v))) },
			values: []float64{-2147483648, -1, 0, 2147483647},
		},
		{
			name: "uint32", bits: 32, format: sampleFormatUInt, bytesPerSample: 4,
			encode: func(o binary.ByteOrder, b []byte, v float64) { o.PutUint32(b, uint32(v)) },
			values: []float64{0, 1, 4294967295, 12345},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			want := testCase.values
			if want == nil {
				want = values
			}

			data := buildGeoTIFF(t, tiffSpec{
				width: 2, height: 2,
				bitsPerSample: testCase.bits,
				sampleFormat:  testCase.format,
				pixels:        encodeSamples(binary.LittleEndian, want, testCase.bytesPerSample, testCase.encode),
				originX:       0, originY: 10,
				pixelSizeX: 10, pixelSizeY: 10,
			})

			model, err := LoadFromBytes(data)
			if err != nil {
				t.Fatalf("LoadFromBytes: %v", err)
			}

			// Pixel centers, row-major: (0,10) (10,10) (0,0) (10,0).
			centers := [][2]float64{{0, 10}, {10, 10}, {0, 0}, {10, 0}}
			for i, center := range centers {
				got, ok := model.ElevationAt(center[0], center[1])
				if !ok {
					t.Fatalf("pixel %d at %v is outside the bounds", i, center)
				}

				if got != want[i] {
					t.Fatalf("pixel %d = %v, want %v", i, got, want[i])
				}
			}
		})
	}
}

// A (format, width) pair the decoders do not implement must be refused before
// anything is decoded, rather than silently read as some other width.
func TestLoadFromBytesRefusesUnsupportedSampleFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		bits   uint32
		format uint32
	}{
		{name: "float16", bits: 16, format: sampleFormatFloat},
		{name: "int64", bits: 64, format: sampleFormatInt},
		{name: "uint64", bits: 64, format: sampleFormatUInt},
		{name: "int8", bits: 8, format: sampleFormatInt},
		{name: "uint8", bits: 8, format: sampleFormatUInt},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			data := buildGeoTIFF(t, tiffSpec{
				width: 2, height: 2,
				bitsPerSample: testCase.bits,
				sampleFormat:  testCase.format,
				pixels:        make([]byte, 2*2*int(testCase.bits)/8),
				originX:       0, originY: 10,
				pixelSizeX: 10, pixelSizeY: 10,
			})

			_, err := LoadFromBytes(data)
			if err == nil {
				t.Fatalf("expected %d-bit sample format %d to be refused", testCase.bits, testCase.format)
			}
		})
	}
}

// --- Layout ---

// float32Pixels encodes a ramp of width*height float32 samples.
func float32Pixels(order binary.ByteOrder, width, height int) ([]byte, []float64) {
	values := make([]float64, width*height)
	for i := range values {
		values[i] = float64(i)
	}

	return encodeSamples(order, values, 4, func(o binary.ByteOrder, b []byte, v float64) {
		o.PutUint32(b, math.Float32bits(float32(v)))
	}), values
}

// assertGridValues checks every pixel center of a grid built by float32Pixels.
//
// The step between centres is read back from the model rather than passed in,
// so the reported pixel size and the positions the samples actually sit at have
// to agree: a model that reported one scale and interpolated on another would
// pass a caller-supplied step and fail here.
func assertGridValues(t *testing.T, model Model, width, height int, want []float64, originX, originY float64) {
	t.Helper()

	pixelSize := model.Info().PixelSize

	for row := range height {
		for col := range width {
			x := originX + float64(col)*pixelSize[0]
			y := originY - float64(row)*pixelSize[1]

			got, ok := model.ElevationAt(x, y)
			if !ok {
				t.Fatalf("pixel (%d,%d) at (%g,%g) is outside the bounds", col, row, x, y)
			}

			if got != want[row*width+col] {
				t.Fatalf("pixel (%d,%d) = %v, want %v", col, row, got, want[row*width+col])
			}
		}
	}
}

// The tiled layout is what GDAL writes for any DTM of size, and the reassembly
// has to place every tile at the right offset. A 5x5 image in 2x2 tiles is
// deliberately not a multiple of the tile size, so the right column and the
// bottom row are edge tiles whose padding must be discarded rather than copied
// into the neighbouring row.
func TestLoadFromBytesReadsTiledRastersIncludingEdgeTiles(t *testing.T) {
	t.Parallel()

	const (
		width  = 5
		height = 5
	)

	pixels, want := float32Pixels(binary.LittleEndian, width, height)

	data := buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		tileWidth:     2, tileHeight: 2,
		originX: 0, originY: 40,
		pixelSizeX: 10, pixelSizeY: 10,
	})

	model, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	if model.Info().GridSize != [2]int{width, height} {
		t.Fatalf("GridSize = %v, want [5 5]", model.Info().GridSize)
	}

	assertGridValues(t, model, width, height, want, 0, 40)
}

// A tiled raster whose tiles divide the image exactly takes a different branch
// of the edge handling, so it is pinned separately.
func TestLoadFromBytesReadsTiledRastersThatDivideExactly(t *testing.T) {
	t.Parallel()

	const (
		width  = 4
		height = 4
	)

	pixels, want := float32Pixels(binary.LittleEndian, width, height)

	data := buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		tileWidth:     2, tileHeight: 2,
		originX: 0, originY: 30,
		pixelSizeX: 10, pixelSizeY: 10,
	})

	model, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	assertGridValues(t, model, width, height, want, 0, 30)
}

// A tiled file whose TileOffsets vector is shorter than the tile grid it
// declares must be refused, not read with whatever happened to be at index 0.
func TestLoadFromBytesRefusesTruncatedTileIndex(t *testing.T) {
	t.Parallel()

	const (
		width  = 4
		height = 4
	)

	pixels, _ := float32Pixels(binary.LittleEndian, width, height)

	data := buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		tileWidth:     2, tileHeight: 2,
		originX: 0, originY: 30,
		pixelSizeX: 10, pixelSizeY: 10,
	})

	// Cut the declared TileOffsets count from four to one. The entry keeps its
	// data, so only the count is a lie — which is exactly the shape of a
	// truncated file.
	patched := patchTagCount(t, data, tagTileOffsets, 1)

	_, err := LoadFromBytes(patched)
	if err == nil {
		t.Fatal("expected an error for a tile index shorter than the tile grid")
	}
}

// patchTagCount rewrites the element count of one IFD entry in a
// little-endian classic TIFF built by buildGeoTIFF.
func patchTagCount(t *testing.T, data []byte, tag uint16, count uint32) []byte {
	t.Helper()

	out := bytes.Clone(data)
	order := binary.LittleEndian

	ifdOffset := int(order.Uint32(out[4:8]))
	entries := int(order.Uint16(out[ifdOffset:]))

	for i := range entries {
		pos := ifdOffset + 2 + i*12
		if order.Uint16(out[pos:]) == tag {
			order.PutUint32(out[pos+4:], count)

			return out
		}
	}

	t.Fatalf("tag %d not present in the fixture", tag)

	return nil
}

// DEFLATE is the compression GDAL writes by default. The deflate-bomb guard is
// already pinned; this pins that an honest compressed raster still decodes,
// which is what the guard must not break.
func TestLoadFromBytesReadsDeflateCompressedRasters(t *testing.T) {
	t.Parallel()

	const (
		width  = 4
		height = 3
	)

	pixels, want := float32Pixels(binary.LittleEndian, width, height)

	for _, tiled := range []bool{false, true} {
		spec := tiffSpec{
			width: width, height: height,
			bitsPerSample: 32,
			sampleFormat:  sampleFormatFloat,
			pixels:        pixels,
			deflate:       true,
			originX:       0, originY: 20,
			pixelSizeX: 10, pixelSizeY: 10,
		}

		if tiled {
			spec.tileWidth, spec.tileHeight = 2, 2
		}

		model, err := LoadFromBytes(buildGeoTIFF(t, spec))
		if err != nil {
			t.Fatalf("LoadFromBytes (tiled=%t): %v", tiled, err)
		}

		assertGridValues(t, model, width, height, want, 0, 20)
	}
}

// A big-endian file is still a valid GeoTIFF, and the byte order has to reach
// the sample decoders rather than only the header parser.
func TestLoadFromBytesReadsBigEndianRasters(t *testing.T) {
	t.Parallel()

	const (
		width  = 3
		height = 2
	)

	pixels, want := float32Pixels(binary.BigEndian, width, height)

	model, err := LoadFromBytes(buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		bigEndian:     true,
		originX:       0, originY: 10,
		pixelSizeX: 10, pixelSizeY: 10,
	}))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	assertGridValues(t, model, width, height, want, 0, 10)
}

// An unsupported compression code must be named rather than produce a grid of
// garbage.
func TestLoadFromBytesRefusesUnsupportedCompression(t *testing.T) {
	t.Parallel()

	pixels, _ := float32Pixels(binary.LittleEndian, 2, 2)

	data := buildGeoTIFF(t, tiffSpec{
		width: 2, height: 2,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		originX:       0, originY: 10,
		pixelSizeX: 10, pixelSizeY: 10,
	})

	// 5 is LZW, which this reader does not implement.
	patched := patchShortTagValue(t, data, tagCompression, 5)

	_, err := LoadFromBytes(patched)
	if err == nil {
		t.Fatal("expected an error for LZW-compressed pixel data")
	}
}

// patchShortTagValue rewrites the inline SHORT value of one IFD entry.
func patchShortTagValue(t *testing.T, data []byte, tag uint16, value uint16) []byte {
	t.Helper()

	out := bytes.Clone(data)
	order := binary.LittleEndian

	ifdOffset := int(order.Uint32(out[4:8]))
	entries := int(order.Uint16(out[ifdOffset:]))

	for i := range entries {
		pos := ifdOffset + 2 + i*12
		if order.Uint16(out[pos:]) == tag {
			order.PutUint16(out[pos+8:], value)

			return out
		}
	}

	t.Fatalf("tag %d not present in the fixture", tag)

	return nil
}

// --- Geotransform ---

func TestLoadFromBytesRefusesABrokenGeotransform(t *testing.T) {
	t.Parallel()

	pixels, _ := float32Pixels(binary.LittleEndian, 2, 2)

	base := tiffSpec{
		width: 2, height: 2,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		originX:       0, originY: 10,
		pixelSizeX: 10, pixelSizeY: 10,
	}

	cases := []struct {
		name  string
		spec  func(s tiffSpec) tiffSpec
		wants string
	}{
		{
			name:  "no pixel scale",
			spec:  func(s tiffSpec) tiffSpec { s.omitPixelScale = true; return s },
			wants: "ModelPixelScale",
		},
		{
			name:  "no tiepoint",
			spec:  func(s tiffSpec) tiffSpec { s.omitTiepoint = true; return s },
			wants: "ModelTiepoint",
		},
		{
			name:  "zero pixel scale",
			spec:  func(s tiffSpec) tiffSpec { s.pixelSizeX = 0; return s },
			wants: "invalid pixel scale",
		},
		{
			name:  "negative pixel scale",
			spec:  func(s tiffSpec) tiffSpec { s.pixelSizeY = -10; return s },
			wants: "invalid pixel scale",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := LoadFromBytes(buildGeoTIFF(t, testCase.spec(base)))
			if err == nil {
				t.Fatal("expected a geotransform error")
			}

			if !strings.Contains(err.Error(), testCase.wants) {
				t.Fatalf("error %q does not mention %q", err, testCase.wants)
			}
		})
	}
}

// The tiepoint need not sit on pixel (0,0). A non-zero raster tiepoint shifts
// the origin, and getting the sign wrong moves every elevation query by whole
// pixels.
func TestLoadFromBytesHonoursANonZeroTiepointPixel(t *testing.T) {
	t.Parallel()

	pixels, want := float32Pixels(binary.LittleEndian, 3, 3)

	writer := &tiffWriter{order: binary.LittleEndian}
	offset := writer.appendBlob(pixels)

	writer.addShort(tagImageWidth, 3)
	writer.addShort(tagImageLength, 3)
	writer.addShort(tagBitsPerSample, 32)
	writer.addShort(tagCompression, compressionNone)
	writer.addShort(tagSampleFormat, sampleFormatFloat)
	writer.addLongs(tagStripOffsets, []uint32{offset})
	writer.addLongs(tagStripByteCounts, []uint32{uint32(len(pixels))})
	writer.addDoubles(tagModelPixelScale, []float64{10, 10, 0})
	// The tiepoint names raster pixel (1, 1) as world (1000, 2000), so pixel
	// (0,0) is at (990, 2010).
	writer.addDoubles(tagModelTiepoint, []float64{1, 1, 0, 1000, 2000, 0})

	model, err := LoadFromBytes(writer.bytes())
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	got, ok := model.ElevationAt(1000, 2000)
	if !ok {
		t.Fatal("the tiepoint itself is outside the bounds")
	}

	// Pixel (1,1) is index 4 in a 3x3 ramp.
	if got != want[4] {
		t.Fatalf("elevation at the tiepoint = %v, want %v", got, want[4])
	}

	assertGridValues(t, model, 3, 3, want, 990, 2010)
}

// A raster split into several strips is the ordinary GDAL layout for anything
// larger than a few rows; the single-strip fixtures never walk the loop that
// concatenates them. A strip appended in the wrong order, or one dropped, shows
// up here as a value from the wrong row.
func TestLoadFromBytesReadsMultiStripRasters(t *testing.T) {
	t.Parallel()

	const (
		width  = 3
		height = 7
	)

	pixels, want := float32Pixels(binary.LittleEndian, width, height)

	cases := []struct {
		name      string
		stripRows int
		deflate   bool
	}{
		{name: "one row per strip", stripRows: 1},
		// 7 rows in strips of 2 leaves a short final strip.
		{name: "two rows per strip", stripRows: 2},
		{name: "two rows per strip, deflate", stripRows: 2, deflate: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model, err := LoadFromBytes(buildGeoTIFF(t, tiffSpec{
				width: width, height: height,
				bitsPerSample: 32,
				sampleFormat:  sampleFormatFloat,
				pixels:        pixels,
				stripRows:     testCase.stripRows,
				deflate:       testCase.deflate,
				originX:       0, originY: 60,
				pixelSizeX: 10, pixelSizeY: 10,
			}))
			if err != nil {
				t.Fatalf("LoadFromBytes: %v", err)
			}

			assertGridValues(t, model, width, height, want, 0, 60)
		})
	}
}

// StripOffsets and StripByteCounts are parallel vectors. A file whose counts
// vector is shorter than its offsets vector is truncated, and reading it by
// index would walk off the end of one of them.
func TestLoadFromBytesRefusesMismatchedStripVectors(t *testing.T) {
	t.Parallel()

	const (
		width  = 3
		height = 4
	)

	pixels, _ := float32Pixels(binary.LittleEndian, width, height)

	data := buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		stripRows:     1,
		originX:       0, originY: 30,
		pixelSizeX: 10, pixelSizeY: 10,
	})

	patched := patchTagCount(t, data, tagStripByteCounts, 2)

	_, err := LoadFromBytes(patched)
	if err == nil {
		t.Fatal("expected an error for a byte-counts vector shorter than the offsets vector")
	}

	if !strings.Contains(err.Error(), "byte counts") {
		t.Fatalf("error %q does not report the mismatch", err)
	}
}

// TIFF lets a writer pick any integer type wide enough for the value, and
// encoders do: GDAL writes ImageWidth as SHORT for small rasters and LONG for
// large ones, and StripOffsets follows the file size. A reader that assumed one
// type would read zeroes from a perfectly valid file.
func TestLoadFromBytesAcceptsEitherIntegerTagType(t *testing.T) {
	t.Parallel()

	const (
		width  = 3
		height = 2
	)

	pixels, want := float32Pixels(binary.LittleEndian, width, height)

	// Build the same raster with LONG dimensions and a SHORT strip index,
	// which is the mirror image of what buildGeoTIFF writes.
	writer := &tiffWriter{order: binary.LittleEndian}
	offset := writer.appendBlob(pixels)

	if offset > math.MaxUint16 {
		t.Fatalf("fixture payload starts at %d, too far for a SHORT offset", offset)
	}

	writer.addLongs(tagImageWidth, []uint32{width})
	writer.addLongs(tagImageLength, []uint32{height})
	writer.addShort(tagBitsPerSample, 32)
	writer.addShort(tagCompression, compressionNone)
	writer.addShort(tagSampleFormat, sampleFormatFloat)
	writer.addShort(tagStripOffsets, offset)
	writer.addLongs(tagStripByteCounts, []uint32{uint32(len(pixels))})
	writer.addDoubles(tagModelPixelScale, []float64{10, 10, 0})
	writer.addDoubles(tagModelTiepoint, []float64{0, 0, 0, 0, 10, 0})

	model, err := LoadFromBytes(writer.bytes())
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	assertGridValues(t, model, width, height, want, 0, 10)
}

// A file with no SampleFormat tag means unsigned integer samples, per the TIFF
// specification. Defaulting to anything else would misread every DEM that omits
// the tag.
func TestLoadFromBytesDefaultsSampleFormatToUnsignedInteger(t *testing.T) {
	t.Parallel()

	values := []float64{0, 1, 40000, 65535}

	pixels := encodeSamples(binary.LittleEndian, values, 2, func(o binary.ByteOrder, b []byte, v float64) {
		o.PutUint16(b, uint16(v))
	})

	writer := &tiffWriter{order: binary.LittleEndian}
	offset := writer.appendBlob(pixels)

	writer.addShort(tagImageWidth, 2)
	writer.addShort(tagImageLength, 2)
	writer.addShort(tagBitsPerSample, 16)
	// No Compression and no SampleFormat tag at all.
	writer.addLongs(tagStripOffsets, []uint32{offset})
	writer.addLongs(tagStripByteCounts, []uint32{uint32(len(pixels))})
	writer.addDoubles(tagModelPixelScale, []float64{10, 10, 0})
	writer.addDoubles(tagModelTiepoint, []float64{0, 0, 0, 0, 10, 0})

	model, err := LoadFromBytes(writer.bytes())
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	assertGridValues(t, model, 2, 2, values, 0, 10)
}

// A GDAL_NODATA tag that is not a number must be ignored rather than parsed
// into something arbitrary: treating a garbled tag as "nodata = 0" would blank
// out every sea-level sample in the raster.
func TestLoadFromBytesIgnoresAnUnparsableNoDataTag(t *testing.T) {
	t.Parallel()

	values := []float64{0, 1, 2, 3}

	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 2, values, 0, 10, 10, "nan-ish"))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	got, ok := model.ElevationAt(0, 10)
	if !ok {
		t.Fatal("a 0 m sample became a miss because of an unparsable nodata tag")
	}

	if got != 0 {
		t.Fatalf("elevation = %v, want 0", got)
	}
}

// A strip index whose byte count is shorter than the pixels it must supply
// leaves the grid underfilled, which has to be an error rather than a grid of
// zeroes past the truncation point.
func TestLoadFromBytesRefusesTruncatedPixelData(t *testing.T) {
	t.Parallel()

	const (
		width  = 4
		height = 4
	)

	pixels, _ := float32Pixels(binary.LittleEndian, width, height)

	data := buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		originX:       0, originY: 30,
		pixelSizeX: 10, pixelSizeY: 10,
	})

	// Halve the declared strip length: the file still holds the bytes, but the
	// header no longer claims them.
	patched := patchLongTagValue(t, data, tagStripByteCounts, uint32(len(pixels)/2))

	_, err := LoadFromBytes(patched)
	if err == nil {
		t.Fatal("expected an error for a strip shorter than the declared raster")
	}

	if !strings.Contains(err.Error(), "pixel data too short") {
		t.Fatalf("error %q does not report the short pixel data", err)
	}
}

// patchLongTagValue rewrites the value of a single-element LONG tag whose value
// is stored inline.
func patchLongTagValue(t *testing.T, data []byte, tag uint16, value uint32) []byte {
	t.Helper()

	out := bytes.Clone(data)
	order := binary.LittleEndian

	ifdOffset := int(order.Uint32(out[4:8]))
	entries := int(order.Uint16(out[ifdOffset:]))

	for i := range entries {
		pos := ifdOffset + 2 + i*12
		if order.Uint16(out[pos:]) != tag {
			continue
		}

		if order.Uint32(out[pos+4:]) != 1 {
			t.Fatalf("tag %d is not a single-element entry", tag)
		}

		order.PutUint32(out[pos+8:], value)

		return out
	}

	t.Fatalf("tag %d not present in the fixture", tag)

	return nil
}
