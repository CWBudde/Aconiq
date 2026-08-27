package terrain

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"strings"
	"testing"
)

// The tests in this file feed deliberately malformed GeoTIFF headers to the
// parser. Every one of them must come back as an error: a header is a few
// hundred bytes of attacker-controlled length declarations, and the decoder
// must never turn one into an allocation it cannot refuse.

type tiffTag struct {
	tag   uint16
	dtype uint16
	count uint32
	value uint32
}

// buildClassicTIFF assembles a little-endian classic TIFF: an 8-byte header,
// the given payload, then an IFD holding exactly the given tags.
func buildClassicTIFF(payload []byte, tags []tiffTag) []byte {
	order := binary.LittleEndian
	ifdOffset := 8 + len(payload)
	buf := make([]byte, ifdOffset+2+len(tags)*12+4)

	buf[0], buf[1] = 'I', 'I'
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	copy(buf[8:], payload)

	pos := ifdOffset
	order.PutUint16(buf[pos:], uint16(len(tags)))
	pos += 2

	for _, t := range tags {
		order.PutUint16(buf[pos:], t.tag)
		order.PutUint16(buf[pos+2:], t.dtype)
		order.PutUint32(buf[pos+4:], t.count)
		order.PutUint32(buf[pos+8:], t.value)
		pos += 12
	}

	order.PutUint32(buf[pos:], 0) // next IFD

	return buf
}

// buildBigTIFF assembles a little-endian BigTIFF whose IFD declares entryCount
// entries but carries only the entries actually given.
func buildBigTIFF(entryCount uint64, entries []byte) []byte {
	order := binary.LittleEndian
	buf := make([]byte, 16+8+len(entries))

	buf[0], buf[1] = 'I', 'I'
	order.PutUint16(buf[2:], 43)
	order.PutUint16(buf[4:], 8) // offset size
	order.PutUint16(buf[6:], 0)
	order.PutUint64(buf[8:], 16) // IFD offset
	order.PutUint64(buf[16:], entryCount)

	copy(buf[24:], entries)

	return buf
}

func bigTIFFEntry(tag, dtype uint16, count, value uint64) []byte {
	order := binary.LittleEndian
	e := make([]byte, 20)
	order.PutUint16(e[0:], tag)
	order.PutUint16(e[2:], dtype)
	order.PutUint64(e[4:], count)
	order.PutUint64(e[12:], value)

	return e
}

func assertParseError(t *testing.T, data []byte, wantSubstring string) {
	t.Helper()

	_, err := parseGeoTIFF(data)
	if err == nil {
		t.Fatalf("expected an error, got none (input was %d bytes)", len(data))
	}

	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("error %q does not mention %q", err.Error(), wantSubstring)
	}
}

// A header that omits BitsPerSample used to leave bytesPerSample at zero, so
// the "is there enough pixel data?" guard compared against zero and failed
// open, and make([]float64, width*height) then ran with 2^40 elements.
func TestParseGeoTIFF_MissingBitsPerSampleIsRejected(t *testing.T) {
	data := buildClassicTIFF(nil, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 1 << 20},
		{tagImageLength, tiffTypeLong, 1, 1 << 20},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagSampleFormat, tiffTypeShort, 1, sampleFormatFloat},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, 0},
	})

	if len(data) > 300 {
		t.Fatalf("proof-of-concept input grew to %d bytes; it should stay tiny", len(data))
	}

	assertParseError(t, data, "BitsPerSample")
}

func TestParseGeoTIFF_UnsupportedBitsPerSampleIsRejected(t *testing.T) {
	data := buildClassicTIFF(nil, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 4},
		{tagImageLength, tiffTypeLong, 1, 4},
		{tagBitsPerSample, tiffTypeShort, 1, 7},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, 0},
	})

	assertParseError(t, data, "BitsPerSample")
}

func TestParseGeoTIFF_OversizedDimensionIsRejected(t *testing.T) {
	data := buildClassicTIFF(nil, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 0xFFFFFFFF},
		{tagImageLength, tiffTypeLong, 1, 4},
		{tagBitsPerSample, tiffTypeShort, 1, 32},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, 0},
	})

	assertParseError(t, data, "per-side limit")
}

func TestParseGeoTIFF_OversizedPixelCountIsRejected(t *testing.T) {
	data := buildClassicTIFF(nil, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 1 << 20},
		{tagImageLength, tiffTypeLong, 1, 1 << 20},
		{tagBitsPerSample, tiffTypeShort, 1, 32},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, 0},
	})

	assertParseError(t, data, "exceeding the limit")
}

// A raster small enough to pass the pixel-count limit can still promise far
// more pixel data than a tiny file could ever hold; the tiled path used to
// allocate the full image buffer before looking at the file at all.
func TestParseGeoTIFF_ImplausiblePayloadForFileSizeIsRejected(t *testing.T) {
	data := buildClassicTIFF(nil, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 8192},
		{tagImageLength, tiffTypeLong, 1, 8192},
		{tagBitsPerSample, tiffTypeShort, 1, 32},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagTileWidth, tiffTypeLong, 1, 256},
		{tagTileLength, tiffTypeLong, 1, 256},
		{tagTileOffsets, tiffTypeLong, 1, 8},
		{tagTileByteCounts, tiffTypeLong, 1, 0},
	})

	assertParseError(t, data, "implausible")
}

func TestParseGeoTIFF_OversizedTileDimensionIsRejected(t *testing.T) {
	data := buildClassicTIFF(make([]byte, 64), []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 4},
		{tagImageLength, tiffTypeLong, 1, 4},
		{tagBitsPerSample, tiffTypeShort, 1, 32},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagTileWidth, tiffTypeLong, 1, 0xFFFFFF},
		{tagTileLength, tiffTypeLong, 1, 16},
		{tagTileOffsets, tiffTypeLong, 1, 8},
		{tagTileByteCounts, tiffTypeLong, 1, 64},
	})

	assertParseError(t, data, "per-side limit")
}

// BigTIFF stores the IFD entry count in 64 bits, so it alone could size the
// entry slice.
func TestParseGeoTIFF_BigTIFFEntryCountIsBounded(t *testing.T) {
	data := buildBigTIFF(1<<62, nil)

	if len(data) > 64 {
		t.Fatalf("proof-of-concept input grew to %d bytes", len(data))
	}

	assertParseError(t, data, "exceeding the limit")
}

// count * typeSize is computed in uint64 and used both as a length and as an
// end offset, so a count near 2^64/8 must be rejected rather than wrapped.
func TestParseGeoTIFF_TagValueCountOverflowIsRejected(t *testing.T) {
	entry := bigTIFFEntry(tagStripOffsets, tiffTypeLong8, 0xFFFFFFFFFFFFFFF0, 16)
	data := buildBigTIFF(1, entry)

	assertParseError(t, data, "overflow")
}

// getUintSlice sizes a slice from the entry's element count. The count is
// attacker-controlled, so it must be bounded and cross-checked against the
// bytes the entry actually carries.
func TestGetUintSlice_RejectsOversizedCount(t *testing.T) {
	tags := map[uint16]ifdEntry{
		tagStripOffsets: {tag: tagStripOffsets, dataType: tiffTypeShort, count: 1 << 40, data: []byte{0, 0}},
	}

	_, err := getUintSlice(tags, tagStripOffsets, binary.LittleEndian)
	if err == nil {
		t.Fatal("expected an error for a 2^40 element count")
	}
}

func TestGetUintSlice_RejectsCountBeyondEntryData(t *testing.T) {
	tags := map[uint16]ifdEntry{
		tagStripOffsets: {tag: tagStripOffsets, dataType: tiffTypeLong, count: 64, data: []byte{0, 0, 0, 0}},
	}

	_, err := getUintSlice(tags, tagStripOffsets, binary.LittleEndian)
	if err == nil {
		t.Fatal("expected an error when the entry carries fewer bytes than it declares values")
	}
}

// A deflate stream expands by up to ~1000:1, so an unbounded io.ReadAll over
// one turns a few kilobytes of strip data into gigabytes of pixels.
func TestParseGeoTIFF_DeflateBombIsRejected(t *testing.T) {
	var compressed bytes.Buffer

	zw := zlib.NewWriter(&compressed)

	_, err := zw.Write(make([]byte, 8<<20))
	if err != nil {
		t.Fatalf("write bomb payload: %v", err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatalf("close bomb payload: %v", err)
	}

	payload := compressed.Bytes()

	data := buildClassicTIFF(payload, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 2},
		{tagImageLength, tiffTypeLong, 1, 2},
		{tagBitsPerSample, tiffTypeShort, 1, 32},
		{tagSampleFormat, tiffTypeShort, 1, sampleFormatFloat},
		{tagCompression, tiffTypeShort, 1, compressionDeflate},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, uint32(len(payload))},
	})

	assertParseError(t, data, "budget")
}

func TestParseGeoTIFF_StripOffsetBeyondFileIsRejected(t *testing.T) {
	data := buildClassicTIFF(make([]byte, 64), []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 4},
		{tagImageLength, tiffTypeLong, 1, 4},
		{tagBitsPerSample, tiffTypeShort, 1, 32},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, 0xFFFFFF00},
	})

	assertParseError(t, data, "exceeds file size")
}

// A BigTIFF IFD offset near 2^63 narrows to a negative int, which used to slip
// past the signed "beyond file" comparison and then index the slice.
func TestParseGeoTIFF_NegativeIFDOffsetIsRejected(t *testing.T) {
	assertParseError(t, []byte("II+\x0000000000000\xff"), "IFD offset beyond file")
}

// A well-formed file must still parse after all the added bounds.
func TestParseGeoTIFF_ValidFileStillParses(t *testing.T) {
	data := buildMinimalGeoTIFF(2, 2, []float32{1, 2, 3, 4}, 100, 200, 10, 10)

	grid, err := parseGeoTIFF(data)
	if err != nil {
		t.Fatalf("parse valid GeoTIFF: %v", err)
	}

	if grid.width != 2 || grid.height != 2 {
		t.Fatalf("grid = %dx%d, want 2x2", grid.width, grid.height)
	}
}

func FuzzParseGeoTIFF(f *testing.F) {
	f.Add(buildMinimalGeoTIFF(2, 2, []float32{1, 2, 3, 4}, 100, 200, 10, 10))
	f.Add(buildClassicTIFF(nil, []tiffTag{
		{tagImageWidth, tiffTypeLong, 1, 1 << 20},
		{tagImageLength, tiffTypeLong, 1, 1 << 20},
		{tagCompression, tiffTypeShort, 1, compressionNone},
		{tagStripOffsets, tiffTypeLong, 1, 8},
		{tagStripByteCounts, tiffTypeLong, 1, 0},
	}))
	f.Add(buildBigTIFF(1<<62, nil))
	f.Add(buildBigTIFF(1, bigTIFFEntry(tagStripOffsets, tiffTypeLong8, 0xFFFFFFFFFFFFFFF0, 16)))
	f.Add([]byte("II*\x00\x08\x00\x00\x00"))
	f.Add([]byte("MM\x00+"))
	// A BigTIFF whose 64-bit IFD offset is negative when narrowed to int.
	f.Add([]byte("II+\x0000000000000\xff"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Only panics and runaway allocations are failures here; any error is
		// an acceptable outcome for arbitrary bytes.
		_, _ = parseGeoTIFF(data)
	})
}
