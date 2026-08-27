package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/aconiq/backend/internal/report/results"
)

// ContourLine represents a single contour at a given dB level.
type ContourLine struct {
	Level    float64      `json:"level"`
	BandName string       `json:"band_name"`
	Points   [][2]float64 `json:"points"`
}

// ContourOptions configures contour generation.
type ContourOptions struct {
	// Interval is the dB step between contour levels (default 5 per EU END convention).
	Interval float64

	// MinLevel is the minimum contour level to generate. Zero means auto-detect from data.
	MinLevel float64

	// MaxLevel is the maximum contour level to generate. Zero means auto-detect from data.
	MaxLevel float64
}

// DefaultContourInterval is 5 dB per EU Environmental Noise Directive convention.
const DefaultContourInterval = 5.0

// GenerateContours generates ISO-band contour lines from a raster using marching squares.
func GenerateContours(raster *results.Raster, gt GeoTransform, opts ContourOptions) ([]ContourLine, error) {
	if raster == nil {
		return nil, errors.New("raster is nil")
	}

	meta := raster.Metadata()
	if meta.Width < 2 || meta.Height < 2 {
		return nil, fmt.Errorf("raster too small for contour generation (%dx%d)", meta.Width, meta.Height)
	}

	if opts.Interval <= 0 {
		opts.Interval = DefaultContourInterval
	}

	var allContours []ContourLine

	for band := range meta.Bands {
		bandName := contourBandName(meta, band)

		grid, dataMin, dataMax, err := extractBandGrid(raster, meta, band)
		if err != nil {
			return nil, err
		}

		if math.IsInf(dataMin, 1) || math.IsInf(dataMax, -1) {
			continue // no valid data in this band
		}

		minLevel, maxLevel := contourLevelRange(opts, dataMin, dataMax)

		for level := minLevel; level <= maxLevel; level += opts.Interval {
			allContours = append(allContours, contoursForLevel(grid, meta, gt, level, bandName)...)
		}
	}

	return allContours, nil
}

// contourBandName resolves the display name of a raster band.
func contourBandName(meta results.RasterMetadata, band int) string {
	if band < len(meta.BandNames) && meta.BandNames[band] != "" {
		return meta.BandNames[band]
	}

	return fmt.Sprintf("band%d", band)
}

// extractBandGrid reads one band into a 2D grid and reports its valid data range.
func extractBandGrid(raster *results.Raster, meta results.RasterMetadata, band int) ([][]float64, float64, float64, error) {
	grid := make([][]float64, meta.Height)
	dataMin := math.Inf(1)
	dataMax := math.Inf(-1)

	for y := range meta.Height {
		grid[y] = make([]float64, meta.Width)

		for x := range meta.Width {
			val, err := raster.At(x, y, band)
			if err != nil {
				return nil, 0, 0, err
			}

			grid[y][x] = val

			if val != meta.NoData && !math.IsNaN(val) {
				if val < dataMin {
					dataMin = val
				}

				if val > dataMax {
					dataMax = val
				}
			}
		}
	}

	return grid, dataMin, dataMax, nil
}

// contourLevelRange determines the min/max contour levels, auto-detecting from data when unset.
func contourLevelRange(opts ContourOptions, dataMin float64, dataMax float64) (float64, float64) {
	minLevel := opts.MinLevel
	if minLevel == 0 {
		minLevel = math.Floor(dataMin/opts.Interval) * opts.Interval
	}

	maxLevel := opts.MaxLevel
	if maxLevel == 0 {
		maxLevel = math.Ceil(dataMax/opts.Interval) * opts.Interval
	}

	return minLevel, maxLevel
}

// contoursForLevel builds the projected contour lines of a single level for one band.
func contoursForLevel(grid [][]float64, meta results.RasterMetadata, gt GeoTransform, level float64, bandName string) []ContourLine {
	segments := marchingSquares(grid, level, meta.NoData)
	lines := joinSegments(segments)

	out := make([]ContourLine, 0, len(lines))

	for _, pts := range lines {
		// Transform pixel coordinates to projected coordinates.
		projected := make([][2]float64, len(pts))
		for i, pt := range pts {
			// pt is in grid coordinates (col, row from bottom).
			// GeoTransform: origin is top-left corner of top-left pixel.
			// Our grid row 0 = bottom (MinY), so we flip.
			gx := gt.OriginX + (pt[0]+0.5)*gt.PixelSizeX
			gy := gt.OriginY + (float64(meta.Height)-pt[1]-0.5)*gt.PixelSizeY
			projected[i] = [2]float64{gx, gy}
		}

		out = append(out, ContourLine{
			Level:    level,
			BandName: bandName,
			Points:   projected,
		})
	}

	return out
}

// marchingSquares extracts line segments for a single contour level.
// Returns segments as pairs of [2]float64 (start, end) in grid coordinates.
func marchingSquares(grid [][]float64, level float64, nodata float64) [][2][2]float64 {
	height := len(grid)
	if height < 2 {
		return nil
	}

	width := len(grid[0])
	if width < 2 {
		return nil
	}

	var segments [][2][2]float64

	for row := range height - 1 {
		for col := range width - 1 {
			segments = appendCellSegments(segments, grid, row, col, level, nodata)
		}
	}

	return segments
}

// cellCorners holds the four corner values of a marching-squares cell
// (bottom-left origin convention).
type cellCorners struct {
	bl float64
	br float64
	tr float64
	tl float64
}

// cellEdges holds the interpolated crossing points on the four cell edges.
type cellEdges struct {
	bottom [2]float64
	right  [2]float64
	top    [2]float64
	left   [2]float64
}

// appendCellSegments appends the contour segments produced by a single cell.
// It preserves the original per-cell evaluation and append order exactly.
func appendCellSegments(segments [][2][2]float64, grid [][]float64, row int, col int, level float64, nodata float64) [][2][2]float64 {
	// bl=grid[row][col], br=grid[row][col+1], tr=grid[row+1][col+1], tl=grid[row+1][col]
	c := cellCorners{
		bl: grid[row][col],
		br: grid[row][col+1],
		tr: grid[row+1][col+1],
		tl: grid[row+1][col],
	}

	// Skip cells with nodata corners.
	if c.bl == nodata || c.br == nodata || c.tr == nodata || c.tl == nodata {
		return segments
	}

	caseIdx := marchingSquaresCase(c, level)
	if caseIdx == 0 || caseIdx == 15 {
		return segments // fully below or above
	}

	// Interpolation helpers.
	fcol := float64(col)
	frow := float64(row)

	// Edge midpoints with linear interpolation.
	edges := cellEdges{
		// Bottom edge (bl-br).
		bottom: [2]float64{fcol + interpolate(c.bl, c.br, level), frow},
		// Right edge (br-tr).
		right: [2]float64{fcol + 1, frow + interpolate(c.br, c.tr, level)},
		// Top edge (tl-tr).
		top: [2]float64{fcol + interpolate(c.tl, c.tr, level), frow + 1},
		// Left edge (bl-tl).
		left: [2]float64{fcol, frow + interpolate(c.bl, c.tl, level)},
	}

	return appendCaseSegments(segments, caseIdx, edges, c, level)
}

// marchingSquaresCase computes the 4-bit case index of a cell.
func marchingSquaresCase(c cellCorners, level float64) int {
	caseIdx := 0
	if c.bl >= level {
		caseIdx |= 1
	}

	if c.br >= level {
		caseIdx |= 2
	}

	if c.tr >= level {
		caseIdx |= 4
	}

	if c.tl >= level {
		caseIdx |= 8
	}

	return caseIdx
}

// appendCaseSegments appends the segments of one non-degenerate marching-squares case.
// Cases 5 and 10 are ambiguous saddles resolved via the cell center value.
func appendCaseSegments(segments [][2][2]float64, caseIdx int, e cellEdges, c cellCorners, level float64) [][2][2]float64 {
	switch caseIdx {
	case 5:
		// Saddle point: use center value to disambiguate.
		if cellCenter(c) >= level {
			return append(segments, [2][2]float64{e.top, e.left}, [2][2]float64{e.right, e.bottom})
		}

		return append(segments, [2][2]float64{e.top, e.right}, [2][2]float64{e.bottom, e.left})
	case 10:
		// Saddle point.
		if cellCenter(c) >= level {
			return append(segments, [2][2]float64{e.bottom, e.right}, [2][2]float64{e.left, e.top})
		}

		return append(segments, [2][2]float64{e.bottom, e.left}, [2][2]float64{e.right, e.top})
	default:
		return appendUnambiguousCaseSegments(segments, caseIdx, e)
	}
}

// appendUnambiguousCaseSegments appends the single segment of a non-saddle case.
func appendUnambiguousCaseSegments(segments [][2][2]float64, caseIdx int, e cellEdges) [][2][2]float64 {
	switch caseIdx {
	case 1:
		return append(segments, [2][2]float64{e.bottom, e.left})
	case 2:
		return append(segments, [2][2]float64{e.right, e.bottom})
	case 3:
		return append(segments, [2][2]float64{e.right, e.left})
	case 4:
		return append(segments, [2][2]float64{e.top, e.right})
	case 6:
		return append(segments, [2][2]float64{e.top, e.bottom})
	case 7:
		return append(segments, [2][2]float64{e.top, e.left})
	case 8:
		return append(segments, [2][2]float64{e.left, e.top})
	case 9:
		return append(segments, [2][2]float64{e.bottom, e.top})
	case 11:
		return append(segments, [2][2]float64{e.right, e.top})
	case 12:
		return append(segments, [2][2]float64{e.left, e.right})
	case 13:
		return append(segments, [2][2]float64{e.bottom, e.right})
	case 14:
		return append(segments, [2][2]float64{e.left, e.bottom})
	default:
		return segments
	}
}

// cellCenter is the mean of the four corner values, used to disambiguate saddle cases.
func cellCenter(c cellCorners) float64 {
	return (c.bl + c.br + c.tr + c.tl) / 4
}

// interpolate computes the fractional position between a and b where the level crosses.
func interpolate(a float64, b float64, level float64) float64 {
	denom := b - a
	if math.Abs(denom) < 1e-12 {
		return 0.5
	}

	t := (level - a) / denom
	if t < 0 {
		t = 0
	}

	if t > 1 {
		t = 1
	}

	return t
}

// endpoint is the quantized key of a segment endpoint used for adjacency lookup.
type endpoint struct {
	x, y float64
}

func quantizeCoord(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func endpointKey(pt [2]float64) endpoint {
	return endpoint{quantizeCoord(pt[0]), quantizeCoord(pt[1])}
}

// firstUnused returns the first not-yet-consumed segment index from candidates,
// preserving the insertion order of the adjacency lists.
func firstUnused(candidates []int, used []bool) (int, bool) {
	for _, idx := range candidates {
		if used[idx] {
			continue
		}

		return idx, true
	}

	return 0, false
}

// extendLineForward repeatedly appends segments whose start matches the line end.
func extendLineForward(line [][2]float64, segments [][2][2]float64, startIndex map[endpoint][]int, used []bool) [][2]float64 {
	for {
		idx, found := firstUnused(startIndex[endpointKey(line[len(line)-1])], used)
		if !found {
			return line
		}

		used[idx] = true
		line = append(line, segments[idx][1])
	}
}

// extendLineBackward repeatedly prepends segments whose end matches the line start.
func extendLineBackward(line [][2]float64, segments [][2][2]float64, endIndex map[endpoint][]int, used []bool) [][2]float64 {
	for {
		idx, found := firstUnused(endIndex[endpointKey(line[0])], used)
		if !found {
			return line
		}

		used[idx] = true
		line = append([][2]float64{segments[idx][0]}, line...)
	}
}

// joinSegments connects contiguous segments into polylines.
func joinSegments(segments [][2][2]float64) [][][2]float64 {
	if len(segments) == 0 {
		return nil
	}

	const eps = 1e-9

	_ = eps // used conceptually via quantize

	// Build adjacency.
	used := make([]bool, len(segments))
	startIndex := make(map[endpoint][]int)
	endIndex := make(map[endpoint][]int)

	for i, seg := range segments {
		sk := endpointKey(seg[0])
		ek := endpointKey(seg[1])

		startIndex[sk] = append(startIndex[sk], i)
		endIndex[ek] = append(endIndex[ek], i)
	}

	var lines [][][2]float64

	for i := range segments {
		if used[i] {
			continue
		}

		used[i] = true
		line := [][2]float64{segments[i][0], segments[i][1]}

		// Extend forward from the end, then backward from the start.
		line = extendLineForward(line, segments, startIndex, used)
		line = extendLineBackward(line, segments, endIndex, used)

		if len(line) >= 2 {
			lines = append(lines, line)
		}
	}

	// Sort for deterministic output.
	sort.Slice(lines, func(i, j int) bool {
		if lines[i][0][0] != lines[j][0][0] {
			return lines[i][0][0] < lines[j][0][0]
		}

		return lines[i][0][1] < lines[j][0][1]
	})

	return lines
}

// ExportContourGeoJSON writes contour lines as a GeoJSON FeatureCollection.
func ExportContourGeoJSON(path string, contours []ContourLine) error {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("create contour geojson directory: %w", err)
	}

	fc := buildContourFeatureCollection(contours)

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal contour geojson: %w", err)
	}

	data = append(data, '\n')

	return os.WriteFile(path, data, 0o600)
}

func buildContourFeatureCollection(contours []ContourLine) map[string]any {
	features := make([]map[string]any, 0, len(contours))

	for _, c := range contours {
		if len(c.Points) < 2 {
			continue
		}

		coords := make([][]float64, len(c.Points))
		for i, pt := range c.Points {
			coords[i] = []float64{pt[0], pt[1]}
		}

		features = append(features, map[string]any{
			"type": "Feature",
			"properties": map[string]any{
				"level_db":  c.Level,
				"band_name": c.BandName,
			},
			"geometry": map[string]any{
				"type":        "LineString",
				"coordinates": coords,
			},
		})
	}

	return map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	}
}
