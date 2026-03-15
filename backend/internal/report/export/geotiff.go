package export

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/report/results"
)

// GeoTIFF tag IDs.
const (
	tiffTagImageWidth      = 256
	tiffTagImageLength     = 257
	tiffTagBitsPerSample   = 258
	tiffTagCompression     = 259
	tiffTagPhotometric     = 262
	tiffTagStripOffsets    = 273
	tiffTagSamplesPerPixel = 277
	tiffTagRowsPerStrip    = 278
	tiffTagStripByteCounts = 279
	tiffTagSampleFormat    = 339

	// GeoTIFF extension tags.
	tiffTagModelTiepoint   = 33922
	tiffTagModelPixelScale = 33550
	tiffTagGeoKeyDirectory = 34735
	tiffTagGeoDoubleParams = 34736
	tiffTagGeoASCIIParams  = 34737
	tiffTagGDALNoData      = 42113

	// TIFF type IDs.
	tiffTypeShort  = 3
	tiffTypeLong   = 4
	tiffTypeDouble = 12
	tiffTypeASCII  = 2

	// Compression: none.
	tiffCompressionNone = 1

	// Photometric: min-is-black.
	tiffPhotometricMinIsBlack = 1

	// Sample format: IEEE floating point.
	tiffSampleFormatFloat = 3

	// GeoTIFF key IDs.
	geoKeyDirectoryVersion = 1
	geoKeyRevisionMajor    = 1
	geoKeyRevisionMinor    = 0

	geoKeyGTModelType     = 1024
	geoKeyGTRasterType    = 1025
	geoKeyGeographicType  = 2048
	geoKeyProjectedCSType = 3072

	// Model types.
	modelTypeProjected  = 1
	modelTypeGeographic = 2

	// Raster type: PixelIsArea means tie-point is at pixel corner.
	rasterTypePixelIsArea = 1
)

// ExportGeoTIFF writes an Aconiq raster as a GeoTIFF file.
// Each band is written as a separate file: <basePath>_<bandName>.tif.
// Returns the list of written file paths.
func ExportGeoTIFF(basePath string, raster *results.Raster, gt GeoTransform, crs string) ([]string, error) {
	if raster == nil {
		return nil, errors.New("raster is nil")
	}

	meta := raster.Metadata()
	if meta.Width <= 0 || meta.Height <= 0 {
		return nil, errors.New("raster has invalid dimensions")
	}

	err := os.MkdirAll(filepath.Dir(basePath), 0o755)
	if err != nil {
		return nil, fmt.Errorf("create geotiff output directory: %w", err)
	}

	epsgCode := parseEPSGCode(crs)
	paths := make([]string, 0, meta.Bands)

	for band := range meta.Bands {
		bandName := fmt.Sprintf("band%d", band)
		if band < len(meta.BandNames) && meta.BandNames[band] != "" {
			bandName = meta.BandNames[band]
		}

		outPath := fmt.Sprintf("%s_%s.tif", basePath, bandName)

		bandData := make([]float64, meta.Width*meta.Height)
		for y := range meta.Height {
			for x := range meta.Width {
				val, valErr := raster.At(x, y, band)
				if valErr != nil {
					return nil, fmt.Errorf("read raster at (%d,%d,%d): %w", x, y, band, valErr)
				}

				bandData[y*meta.Width+x] = val
			}
		}

		err = writeGeoTIFFFile(outPath, bandData, meta.Width, meta.Height, meta.NoData, gt, epsgCode)
		if err != nil {
			return nil, fmt.Errorf("write geotiff %s: %w", outPath, err)
		}

		paths = append(paths, outPath)
	}

	return paths, nil
}

// writeGeoTIFFFile writes a single-band float64 GeoTIFF.
func writeGeoTIFFFile(path string, data []float64, width int, height int, nodata float64, gt GeoTransform, epsgCode int) error {
	// Image data: float64 little-endian, row-major, top-to-bottom.
	// GeoTIFF convention: row 0 is the northernmost row.
	// Our raster is stored Y=0 at the bottom (MinY), so we flip rows.
	imageBytes := make([]byte, width*height*8)

	for row := range height {
		srcRow := height - 1 - row // flip: top row in TIFF = last row in our raster
		for col := range width {
			val := data[srcRow*width+col]
			binary.LittleEndian.PutUint64(imageBytes[(row*width+col)*8:], math.Float64bits(val))
		}
	}

	// Build IFD entries and extra data blocks.
	ifdEntries, extraBlocks := buildGeoTIFFIFD(width, height, nodata, gt, epsgCode)

	// Layout: header(8) + IFD(2 + entries*12 + 4) + extra blocks + image data.
	ifdSize := 2 + len(ifdEntries)*12 + 4

	extraOffset := 8 + ifdSize
	for i := range extraBlocks {
		extraBlocks[i].offset = extraOffset
		extraOffset += len(extraBlocks[i].data)
	}

	imageDataOffset := extraOffset

	// Now resolve offsets in IFD entries that reference extra blocks or image data.
	for i := range ifdEntries {
		if ifdEntries[i].resolveExtra >= 0 {
			idx := ifdEntries[i].resolveExtra
			binary.LittleEndian.PutUint32(ifdEntries[i].value[:], uint32(extraBlocks[idx].offset))
		}

		if ifdEntries[i].resolveImage {
			binary.LittleEndian.PutUint32(ifdEntries[i].value[:], uint32(imageDataOffset))
		}
	}

	// Assemble the file.
	totalSize := imageDataOffset + len(imageBytes)
	buf := make([]byte, totalSize)

	// TIFF header: little-endian, magic 42, IFD offset = 8.
	buf[0] = 'I'
	buf[1] = 'I'
	binary.LittleEndian.PutUint16(buf[2:], 42)
	binary.LittleEndian.PutUint32(buf[4:], 8)

	// IFD.
	pos := 8
	binary.LittleEndian.PutUint16(buf[pos:], uint16(len(ifdEntries)))
	pos += 2

	for _, entry := range ifdEntries {
		binary.LittleEndian.PutUint16(buf[pos:], entry.tag)
		binary.LittleEndian.PutUint16(buf[pos+2:], entry.typ)
		binary.LittleEndian.PutUint32(buf[pos+4:], entry.count)
		copy(buf[pos+8:pos+12], entry.value[:])
		pos += 12
	}

	// Next IFD offset = 0 (no more IFDs).
	binary.LittleEndian.PutUint32(buf[pos:], 0)

	// Extra data blocks.
	for _, block := range extraBlocks {
		copy(buf[block.offset:], block.data)
	}

	// Image data.
	copy(buf[imageDataOffset:], imageBytes)

	return os.WriteFile(path, buf, 0o644)
}

type ifdEntry struct {
	tag          uint16
	typ          uint16
	count        uint32
	value        [4]byte
	resolveExtra int  // index into extraBlocks, or -1
	resolveImage bool // true if value should be image data offset
}

type extraBlock struct {
	data   []byte
	offset int
}

func buildGeoTIFFIFD(width int, height int, nodata float64, gt GeoTransform, epsgCode int) ([]ifdEntry, []extraBlock) {
	var entries []ifdEntry
	var extras []extraBlock

	imageByteCount := width * height * 8

	// Helper to add an extra block and return its index.
	addExtra := func(data []byte) int {
		idx := len(extras)
		extras = append(extras, extraBlock{data: data})
		return idx
	}

	// ImageWidth.
	entries = append(entries, ifdEntry{
		tag: tiffTagImageWidth, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(uint32(width)), resolveExtra: -1,
	})

	// ImageLength.
	entries = append(entries, ifdEntry{
		tag: tiffTagImageLength, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(uint32(height)), resolveExtra: -1,
	})

	// BitsPerSample.
	entries = append(entries, ifdEntry{
		tag: tiffTagBitsPerSample, typ: tiffTypeShort, count: 1,
		value: uint16ToBytes(64), resolveExtra: -1,
	})

	// Compression: none.
	entries = append(entries, ifdEntry{
		tag: tiffTagCompression, typ: tiffTypeShort, count: 1,
		value: uint16ToBytes(tiffCompressionNone), resolveExtra: -1,
	})

	// PhotometricInterpretation.
	entries = append(entries, ifdEntry{
		tag: tiffTagPhotometric, typ: tiffTypeShort, count: 1,
		value: uint16ToBytes(tiffPhotometricMinIsBlack), resolveExtra: -1,
	})

	// StripOffsets (single strip = image data offset, resolved later).
	entries = append(entries, ifdEntry{
		tag: tiffTagStripOffsets, typ: tiffTypeLong, count: 1,
		resolveExtra: -1, resolveImage: true,
	})

	// SamplesPerPixel.
	entries = append(entries, ifdEntry{
		tag: tiffTagSamplesPerPixel, typ: tiffTypeShort, count: 1,
		value: uint16ToBytes(1), resolveExtra: -1,
	})

	// RowsPerStrip (all rows in one strip).
	entries = append(entries, ifdEntry{
		tag: tiffTagRowsPerStrip, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(uint32(height)), resolveExtra: -1,
	})

	// StripByteCounts.
	entries = append(entries, ifdEntry{
		tag: tiffTagStripByteCounts, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(uint32(imageByteCount)), resolveExtra: -1,
	})

	// SampleFormat: float.
	entries = append(entries, ifdEntry{
		tag: tiffTagSampleFormat, typ: tiffTypeShort, count: 1,
		value: uint16ToBytes(tiffSampleFormatFloat), resolveExtra: -1,
	})

	// ModelPixelScaleTag: [ScaleX, ScaleY, ScaleZ].
	scaleData := make([]byte, 24)
	binary.LittleEndian.PutUint64(scaleData[0:], math.Float64bits(gt.PixelSizeX))
	binary.LittleEndian.PutUint64(scaleData[8:], math.Float64bits(-gt.PixelSizeY)) // positive value
	binary.LittleEndian.PutUint64(scaleData[16:], 0)                               // ScaleZ = 0

	scaleIdx := addExtra(scaleData)

	entries = append(entries, ifdEntry{
		tag: tiffTagModelPixelScale, typ: tiffTypeDouble, count: 3,
		resolveExtra: scaleIdx,
	})

	// ModelTiepointTag: [I, J, K, X, Y, Z] — pixel (0,0,0) maps to origin.
	tieData := make([]byte, 48)
	// I=0, J=0, K=0 (first three doubles are zero).
	binary.LittleEndian.PutUint64(tieData[24:], math.Float64bits(gt.OriginX))
	binary.LittleEndian.PutUint64(tieData[32:], math.Float64bits(gt.OriginY))
	// Z = 0.

	tieIdx := addExtra(tieData)

	entries = append(entries, ifdEntry{
		tag: tiffTagModelTiepoint, typ: tiffTypeDouble, count: 6,
		resolveExtra: tieIdx,
	})

	// GeoKeyDirectoryTag.
	geoKeys := buildGeoKeys(epsgCode)
	geoKeyData := make([]byte, len(geoKeys)*2)

	for i, v := range geoKeys {
		binary.LittleEndian.PutUint16(geoKeyData[i*2:], v)
	}

	geoKeyIdx := addExtra(geoKeyData)

	entries = append(entries, ifdEntry{
		tag: tiffTagGeoKeyDirectory, typ: tiffTypeShort, count: uint32(len(geoKeys)),
		resolveExtra: geoKeyIdx,
	})

	// GDAL NoData tag (ASCII representation).
	nodataStr := fmt.Sprintf("%g", nodata)
	nodataASCII := append([]byte(nodataStr), 0) // null-terminated

	if len(nodataASCII) <= 4 {
		var val [4]byte
		copy(val[:], nodataASCII)

		entries = append(entries, ifdEntry{
			tag: tiffTagGDALNoData, typ: tiffTypeASCII, count: uint32(len(nodataASCII)),
			value: val, resolveExtra: -1,
		})
	} else {
		nodataIdx := addExtra(nodataASCII)
		entries = append(entries, ifdEntry{
			tag: tiffTagGDALNoData, typ: tiffTypeASCII, count: uint32(len(nodataASCII)),
			resolveExtra: nodataIdx,
		})
	}

	return entries, extras
}

func buildGeoKeys(epsgCode int) []uint16 {
	// GeoKeyDirectory header: version=1, revision=1.0, numberOfKeys=N.
	// Then N key entries, each: KeyID, TIFFTagLocation, Count, ValueOffset.
	if epsgCode <= 0 {
		// No CRS info: just the header with model type undefined.
		return []uint16{
			geoKeyDirectoryVersion, geoKeyRevisionMajor, geoKeyRevisionMinor, 1,
			geoKeyGTRasterType, 0, 1, rasterTypePixelIsArea,
		}
	}

	// Determine if geographic or projected.
	if epsgCode == 4326 || epsgCode == 4258 {
		return []uint16{
			geoKeyDirectoryVersion, geoKeyRevisionMajor, geoKeyRevisionMinor, 3,
			geoKeyGTModelType, 0, 1, modelTypeGeographic,
			geoKeyGTRasterType, 0, 1, rasterTypePixelIsArea,
			geoKeyGeographicType, 0, 1, uint16(epsgCode),
		}
	}

	return []uint16{
		geoKeyDirectoryVersion, geoKeyRevisionMajor, geoKeyRevisionMinor, 3,
		geoKeyGTModelType, 0, 1, modelTypeProjected,
		geoKeyGTRasterType, 0, 1, rasterTypePixelIsArea,
		geoKeyProjectedCSType, 0, 1, uint16(epsgCode),
	}
}

func uint32ToBytes(v uint32) [4]byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)

	return b
}

func uint16ToBytes(v uint16) [4]byte {
	var b [4]byte
	binary.LittleEndian.PutUint16(b[:], v)

	return b
}

func parseEPSGCode(crs string) int {
	if crs == "" {
		return 0
	}

	var code int

	_, err := fmt.Sscanf(crs, "EPSG:%d", &code)
	if err != nil {
		return 0
	}

	return code
}
