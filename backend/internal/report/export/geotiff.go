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
	return exportRasterBands(basePath, raster, gt, crs, ".tif", writeGeoTIFFFile)
}

// exportRasterBands is a shared helper that iterates bands and writes each via the provided writer function.
func exportRasterBands(
	basePath string,
	raster *results.Raster,
	gt GeoTransform,
	crs string,
	ext string,
	writer func(path string, data []float64, w, h int, nodata float64, gt GeoTransform, epsg int) error,
) ([]string, error) {
	if raster == nil {
		return nil, errors.New("raster is nil")
	}

	meta := raster.Metadata()
	if meta.Width <= 0 || meta.Height <= 0 {
		return nil, errors.New("raster has invalid dimensions")
	}

	err := os.MkdirAll(filepath.Dir(basePath), 0o755)
	if err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	epsgCode := parseEPSGCode(crs)
	paths := make([]string, 0, meta.Bands)

	for band := range meta.Bands {
		bandName := fmt.Sprintf("band%d", band)
		if band < len(meta.BandNames) && meta.BandNames[band] != "" {
			bandName = meta.BandNames[band]
		}

		outPath := fmt.Sprintf("%s_%s%s", basePath, bandName, ext)

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

		err = writer(outPath, bandData, meta.Width, meta.Height, meta.NoData, gt, epsgCode)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", outPath, err)
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
			binary.LittleEndian.PutUint32(ifdEntries[i].value[:], mustUint32(extraBlocks[idx].offset))
		}

		if ifdEntries[i].resolveImage {
			binary.LittleEndian.PutUint32(ifdEntries[i].value[:], mustUint32(imageDataOffset))
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
	binary.LittleEndian.PutUint16(buf[pos:], mustUint16(len(ifdEntries)))
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

	return os.WriteFile(path, buf, 0o600)
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
		value: uint32ToBytes(mustUint32(width)), resolveExtra: -1,
	})

	// ImageLength.
	entries = append(entries, ifdEntry{
		tag: tiffTagImageLength, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(mustUint32(height)), resolveExtra: -1,
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
		value: uint32ToBytes(mustUint32(height)), resolveExtra: -1,
	})

	// StripByteCounts.
	entries = append(entries, ifdEntry{
		tag: tiffTagStripByteCounts, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(mustUint32(imageByteCount)), resolveExtra: -1,
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
		tag: tiffTagGeoKeyDirectory, typ: tiffTypeShort, count: mustUint32(len(geoKeys)),
		resolveExtra: geoKeyIdx,
	})

	// GDAL NoData tag (ASCII representation).
	nodataStr := fmt.Sprintf("%g", nodata)
	nodataASCII := append([]byte(nodataStr), 0) // null-terminated

	if len(nodataASCII) <= 4 {
		var val [4]byte
		copy(val[:], nodataASCII)

		entries = append(entries, ifdEntry{
			tag: tiffTagGDALNoData, typ: tiffTypeASCII, count: mustUint32(len(nodataASCII)),
			value: val, resolveExtra: -1,
		})
	} else {
		nodataIdx := addExtra(nodataASCII)
		entries = append(entries, ifdEntry{
			tag: tiffTagGDALNoData, typ: tiffTypeASCII, count: mustUint32(len(nodataASCII)),
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
			geoKeyGeographicType, 0, 1, mustUint16(epsgCode),
		}
	}

	return []uint16{
		geoKeyDirectoryVersion, geoKeyRevisionMajor, geoKeyRevisionMinor, 3,
		geoKeyGTModelType, 0, 1, modelTypeProjected,
		geoKeyGTRasterType, 0, 1, rasterTypePixelIsArea,
		geoKeyProjectedCSType, 0, 1, mustUint16(epsgCode),
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

// COG tile tag IDs.
const (
	tiffTagTileWidth      = 322
	tiffTagTileLength     = 323
	tiffTagTileOffsets    = 324
	tiffTagTileByteCounts = 325

	cogTileSize = 256
)

// ExportCOG writes an Aconiq raster as Cloud Optimized GeoTIFF files.
// Each band is written as a separate file: <basePath>_<bandName>.cog.tif.
// COG files use 256x256 tiles and include overview pyramids.
// Returns the list of written file paths.
func ExportCOG(basePath string, raster *results.Raster, gt GeoTransform, crs string) ([]string, error) {
	return exportRasterBands(basePath, raster, gt, crs, ".cog.tif", writeCOGFile)
}

// cogLevel holds the image data and dimensions for one COG resolution level.
type cogLevel struct {
	data          []float64
	width, height int
}

// cogLevelLayout holds the tile geometry for one COG resolution level.
type cogLevelLayout struct {
	tilesX, tilesY int
	tileCount      int
	dataSize       int // total tile data bytes for this level
}

// cogFileLayout holds the computed IFD entries, offsets, and tile data positions.
type cogFileLayout struct {
	ifdEntries      [][]ifdEntry
	ifdExtras       [][]extraBlock
	ifdOffsets      []int
	tileDataOffsets []int
	totalSize       int
}

// writeCOGFile writes a single-band float64 Cloud Optimized GeoTIFF.
// The file is tiled (256x256) and includes overview pyramid levels.
func writeCOGFile(path string, data []float64, width, height int, nodata float64, gt GeoTransform, epsgCode int) error {
	levels := buildCOGOverviews(data, width, height, nodata)
	layouts := computeCOGLevelLayouts(levels)
	fileLayout := computeCOGFileLayout(levels, layouts, nodata, gt, epsgCode)

	buf := assembleCOGFile(fileLayout, levels, nodata)

	return os.WriteFile(path, buf, 0o600)
}

// computeCOGLevelLayouts computes tile counts and data sizes for each COG level.
func computeCOGLevelLayouts(levels []cogLevel) []cogLevelLayout {
	layouts := make([]cogLevelLayout, len(levels))
	for i, lvl := range levels {
		tx := (lvl.width + cogTileSize - 1) / cogTileSize
		ty := (lvl.height + cogTileSize - 1) / cogTileSize
		tc := tx * ty
		layouts[i] = cogLevelLayout{
			tilesX:    tx,
			tilesY:    ty,
			tileCount: tc,
			dataSize:  tc * cogTileSize * cogTileSize * 8,
		}
	}

	return layouts
}

// computeCOGFileLayout builds IFD entries and computes all file offsets.
func computeCOGFileLayout(levels []cogLevel, layouts []cogLevelLayout, nodata float64, gt GeoTransform, epsgCode int) cogFileLayout {
	allEntries := make([][]ifdEntry, len(levels))
	allExtras := make([][]extraBlock, len(levels))

	for i, lvl := range levels {
		lvlGT := cogGeoTransformForLevel(gt, i)

		entries, extras := buildCOGIFD(lvl.width, lvl.height, cogTileSize, cogTileSize,
			nodata, lvlGT, epsgCode, layouts[i].tilesX, layouts[i].tilesY)
		allEntries[i] = entries
		allExtras[i] = extras
	}

	// Compute positions: IFDs, then extras, then tile data.
	pos := 8 // after TIFF header

	ifdOffsets := make([]int, len(levels))
	for i := range levels {
		ifdOffsets[i] = pos
		pos += 2 + len(allEntries[i])*12 + 4
	}

	for i := range levels {
		for j := range allExtras[i] {
			allExtras[i][j].offset = pos
			pos += len(allExtras[i][j].data)
		}
	}

	tileDataOffsets := make([]int, len(levels))
	for i := range levels {
		tileDataOffsets[i] = pos
		pos += layouts[i].dataSize
	}

	// Resolve IFD entry references.
	for i := range levels {
		for j := range allEntries[i] {
			entry := &allEntries[i][j]
			if entry.resolveExtra >= 0 {
				idx := entry.resolveExtra
				binary.LittleEndian.PutUint32(entry.value[:], mustUint32(allExtras[i][idx].offset))
			}
		}

		resolveCOGTileOffsets(allEntries[i], allExtras[i], tileDataOffsets[i], layouts[i].tileCount)
	}

	return cogFileLayout{
		ifdEntries:      allEntries,
		ifdExtras:       allExtras,
		ifdOffsets:      ifdOffsets,
		tileDataOffsets: tileDataOffsets,
		totalSize:       pos,
	}
}

// assembleCOGFile writes the TIFF header, IFDs, extra blocks, and tile data into a byte buffer.
func assembleCOGFile(layout cogFileLayout, levels []cogLevel, nodata float64) []byte {
	buf := make([]byte, layout.totalSize)

	// TIFF header.
	buf[0] = 'I'
	buf[1] = 'I'
	binary.LittleEndian.PutUint16(buf[2:], 42)
	binary.LittleEndian.PutUint32(buf[4:], mustUint32(layout.ifdOffsets[0]))

	// Write IFDs.
	for i := range levels {
		p := layout.ifdOffsets[i]
		binary.LittleEndian.PutUint16(buf[p:], mustUint16(len(layout.ifdEntries[i])))
		p += 2

		for _, entry := range layout.ifdEntries[i] {
			binary.LittleEndian.PutUint16(buf[p:], entry.tag)
			binary.LittleEndian.PutUint16(buf[p+2:], entry.typ)
			binary.LittleEndian.PutUint32(buf[p+4:], entry.count)
			copy(buf[p+8:p+12], entry.value[:])
			p += 12
		}

		if i+1 < len(levels) {
			binary.LittleEndian.PutUint32(buf[p:], mustUint32(layout.ifdOffsets[i+1]))
		} else {
			binary.LittleEndian.PutUint32(buf[p:], 0)
		}
	}

	// Write extra blocks.
	for i := range levels {
		for _, block := range layout.ifdExtras[i] {
			copy(buf[block.offset:], block.data)
		}
	}

	// Write tile data for each level.
	for i, lvl := range levels {
		writeCOGTileData(buf[layout.tileDataOffsets[i]:], lvl.data, lvl.width, lvl.height, nodata)
	}

	return buf
}

// buildCOGOverviews creates the full-res level plus overview pyramid levels.
// Each overview halves the dimensions by averaging 2x2 blocks.
// The first element is always the full-resolution data.
func buildCOGOverviews(data []float64, width, height int, nodata float64) []cogLevel {
	levels := []cogLevel{{data: data, width: width, height: height}}

	curData := data
	curW := width
	curH := height

	for curW >= cogTileSize*2 || curH >= cogTileSize*2 {
		newW := (curW + 1) / 2
		newH := (curH + 1) / 2
		newData := downsample2x(curData, curW, curH, newW, newH, nodata)

		levels = append(levels, cogLevel{data: newData, width: newW, height: newH})
		curData = newData
		curW = newW
		curH = newH
	}

	return levels
}

// downsample2x produces a half-resolution image by averaging 2x2 blocks.
func downsample2x(src []float64, srcW, srcH, dstW, dstH int, nodata float64) []float64 {
	dst := make([]float64, dstW*dstH)

	for ny := range dstH {
		for nx := range dstW {
			sum := 0.0
			count := 0

			for dy := range 2 {
				for dx := range 2 {
					sx := nx*2 + dx
					sy := ny*2 + dy

					if sx < srcW && sy < srcH {
						val := src[sy*srcW+sx]
						if val != nodata {
							sum += val
							count++
						}
					}
				}
			}

			if count > 0 {
				dst[ny*dstW+nx] = sum / float64(count)
			} else {
				dst[ny*dstW+nx] = nodata
			}
		}
	}

	return dst
}

// cogGeoTransformForLevel scales a geo-transform for the given overview level index.
// Level 0 returns the original transform; level N scales pixel size by 2^N.
func cogGeoTransformForLevel(gt GeoTransform, level int) GeoTransform {
	if level == 0 {
		return gt
	}

	scaleFactor := 1 << level

	return GeoTransform{
		OriginX:    gt.OriginX,
		OriginY:    gt.OriginY,
		PixelSizeX: gt.PixelSizeX * float64(scaleFactor),
		PixelSizeY: gt.PixelSizeY * float64(scaleFactor),
	}
}

// buildCOGIFD builds IFD entries for a COG level using tile tags instead of strip tags.
func buildCOGIFD(width, height, tileW, tileH int, nodata float64, gt GeoTransform, epsgCode int, tilesX, tilesY int) ([]ifdEntry, []extraBlock) {
	entries := make([]ifdEntry, 0, 16)
	extras := make([]extraBlock, 0, 8)

	tileCount := tilesX * tilesY
	tileByteCount := tileW * tileH * 8

	addExtra := func(data []byte) int {
		idx := len(extras)
		extras = append(extras, extraBlock{data: data})

		return idx
	}

	// Common image description tags.
	entries = append(entries, buildFloat64ImageTags(width, height)...)

	// TileWidth.
	entries = append(entries, ifdEntry{
		tag: tiffTagTileWidth, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(mustUint32(tileW)), resolveExtra: -1,
	})

	// TileLength.
	entries = append(entries, ifdEntry{
		tag: tiffTagTileLength, typ: tiffTypeLong, count: 1,
		value: uint32ToBytes(mustUint32(tileH)), resolveExtra: -1,
	})

	// TileOffsets — array of uint32, one per tile. Values resolved later.
	tileOffsetsData := make([]byte, tileCount*4)
	tileOffsetsIdx := addExtra(tileOffsetsData)

	entries = append(entries, ifdEntry{
		tag: tiffTagTileOffsets, typ: tiffTypeLong, count: mustUint32(tileCount),
		resolveExtra: tileOffsetsIdx,
	})

	// TileByteCounts — array of uint32, one per tile.
	tileByteCountsData := make([]byte, tileCount*4)
	for i := range tileCount {
		binary.LittleEndian.PutUint32(tileByteCountsData[i*4:], mustUint32(tileByteCount))
	}

	tileByteCountsIdx := addExtra(tileByteCountsData)

	entries = append(entries, ifdEntry{
		tag: tiffTagTileByteCounts, typ: tiffTypeLong, count: mustUint32(tileCount),
		resolveExtra: tileByteCountsIdx,
	})

	// Geo metadata and nodata tags.
	geoEntries, geoExtras := buildGeoIFDEntries(gt, epsgCode, nodata, len(extras))
	extras = append(extras, geoExtras...)
	entries = append(entries, geoEntries...)

	return entries, extras
}

// buildFloat64ImageTags returns the common IFD entries that describe a single-band float64 image.
func buildFloat64ImageTags(width, height int) []ifdEntry {
	return []ifdEntry{
		{tag: tiffTagImageWidth, typ: tiffTypeLong, count: 1, value: uint32ToBytes(mustUint32(width)), resolveExtra: -1},
		{tag: tiffTagImageLength, typ: tiffTypeLong, count: 1, value: uint32ToBytes(mustUint32(height)), resolveExtra: -1},
		{tag: tiffTagBitsPerSample, typ: tiffTypeShort, count: 1, value: uint16ToBytes(64), resolveExtra: -1},
		{tag: tiffTagCompression, typ: tiffTypeShort, count: 1, value: uint16ToBytes(tiffCompressionNone), resolveExtra: -1},
		{tag: tiffTagPhotometric, typ: tiffTypeShort, count: 1, value: uint16ToBytes(tiffPhotometricMinIsBlack), resolveExtra: -1},
		{tag: tiffTagSamplesPerPixel, typ: tiffTypeShort, count: 1, value: uint16ToBytes(1), resolveExtra: -1},
		{tag: tiffTagSampleFormat, typ: tiffTypeShort, count: 1, value: uint16ToBytes(tiffSampleFormatFloat), resolveExtra: -1},
	}
}

// buildGeoIFDEntries creates the GeoTIFF metadata entries (pixel scale, tiepoint, geo keys, nodata).
// extraBaseIdx is the current length of the extras slice, used to compute correct resolveExtra indices.
func buildGeoIFDEntries(gt GeoTransform, epsgCode int, nodata float64, extraBaseIdx int) ([]ifdEntry, []extraBlock) {
	var entries []ifdEntry
	var extras []extraBlock

	addExtra := func(data []byte) int {
		idx := extraBaseIdx + len(extras)
		extras = append(extras, extraBlock{data: data})

		return idx
	}

	// ModelPixelScaleTag.
	scaleData := make([]byte, 24)
	binary.LittleEndian.PutUint64(scaleData[0:], math.Float64bits(gt.PixelSizeX))
	binary.LittleEndian.PutUint64(scaleData[8:], math.Float64bits(-gt.PixelSizeY))
	binary.LittleEndian.PutUint64(scaleData[16:], 0)

	scaleIdx := addExtra(scaleData)

	entries = append(entries, ifdEntry{
		tag: tiffTagModelPixelScale, typ: tiffTypeDouble, count: 3,
		resolveExtra: scaleIdx,
	})

	// ModelTiepointTag.
	tieData := make([]byte, 48)
	binary.LittleEndian.PutUint64(tieData[24:], math.Float64bits(gt.OriginX))
	binary.LittleEndian.PutUint64(tieData[32:], math.Float64bits(gt.OriginY))

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
		tag: tiffTagGeoKeyDirectory, typ: tiffTypeShort, count: mustUint32(len(geoKeys)),
		resolveExtra: geoKeyIdx,
	})

	// GDAL NoData tag.
	nodataStr := fmt.Sprintf("%g", nodata)
	nodataASCII := append([]byte(nodataStr), 0)

	if len(nodataASCII) <= 4 {
		var val [4]byte
		copy(val[:], nodataASCII)

		entries = append(entries, ifdEntry{
			tag: tiffTagGDALNoData, typ: tiffTypeASCII, count: mustUint32(len(nodataASCII)),
			value: val, resolveExtra: -1,
		})
	} else {
		nodataIdx := addExtra(nodataASCII)
		entries = append(entries, ifdEntry{
			tag: tiffTagGDALNoData, typ: tiffTypeASCII, count: mustUint32(len(nodataASCII)),
			resolveExtra: nodataIdx,
		})
	}

	return entries, extras
}

// resolveCOGTileOffsets fills the tile offsets extra block with the actual file positions.
func resolveCOGTileOffsets(entries []ifdEntry, extras []extraBlock, tileDataStart int, tileCount int) {
	tileByteCount := cogTileSize * cogTileSize * 8

	for i := range entries {
		if entries[i].tag == tiffTagTileOffsets {
			idx := entries[i].resolveExtra
			for t := range tileCount {
				binary.LittleEndian.PutUint32(extras[idx].data[t*4:], mustUint32(tileDataStart+t*tileByteCount))
			}

			// The IFD value field should point to the extra block (already resolved by resolveExtra).
			break
		}
	}
}

// writeCOGTileData writes tiled image data (row-flipped) into buf.
// Tiles are written row-major: tile(0,0), tile(1,0), ..., tile(0,1), ...
func writeCOGTileData(buf []byte, data []float64, width, height int, nodata float64) {
	tilesX := (width + cogTileSize - 1) / cogTileSize
	tilesY := (height + cogTileSize - 1) / cogTileSize
	tileBytes := cogTileSize * cogTileSize * 8

	for ty := range tilesY {
		for tx := range tilesX {
			tileIdx := ty*tilesX + tx
			tileStart := tileIdx * tileBytes

			for row := range cogTileSize {
				for col := range cogTileSize {
					// Map tile pixel to image pixel (with row flip for GeoTIFF convention).
					imgX := tx*cogTileSize + col
					imgRow := ty*cogTileSize + row // row in TIFF (top-to-bottom)
					srcRow := height - 1 - imgRow  // flipped row in our raster (bottom-to-top)

					var val float64
					if imgX < width && imgRow < height && srcRow >= 0 {
						val = data[srcRow*width+imgX]
					} else {
						val = nodata // padding for edge tiles
					}

					off := tileStart + (row*cogTileSize+col)*8
					binary.LittleEndian.PutUint64(buf[off:], math.Float64bits(val))
				}
			}
		}
	}
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
