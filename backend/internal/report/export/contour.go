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
		bandName := fmt.Sprintf("band%d", band)
		if band < len(meta.BandNames) && meta.BandNames[band] != "" {
			bandName = meta.BandNames[band]
		}

		// Extract band values as a 2D grid.
		grid := make([][]float64, meta.Height)
		dataMin := math.Inf(1)
		dataMax := math.Inf(-1)

		for y := range meta.Height {
			grid[y] = make([]float64, meta.Width)

			for x := range meta.Width {
				val, err := raster.At(x, y, band)
				if err != nil {
					return nil, err
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

		if math.IsInf(dataMin, 1) || math.IsInf(dataMax, -1) {
			continue // no valid data in this band
		}

		// Determine contour levels.
		minLevel := opts.MinLevel
		if minLevel == 0 {
			minLevel = math.Floor(dataMin/opts.Interval) * opts.Interval
		}

		maxLevel := opts.MaxLevel
		if maxLevel == 0 {
			maxLevel = math.Ceil(dataMax/opts.Interval) * opts.Interval
		}

		for level := minLevel; level <= maxLevel; level += opts.Interval {
			segments := marchingSquares(grid, level, meta.NoData)
			lines := joinSegments(segments)

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

				allContours = append(allContours, ContourLine{
					Level:    level,
					BandName: bandName,
					Points:   projected,
				})
			}
		}
	}

	return allContours, nil
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
			// Four corners of the cell (bottom-left origin convention).
			// bl=grid[row][col], br=grid[row][col+1], tr=grid[row+1][col+1], tl=grid[row+1][col]
			bl := grid[row][col]
			br := grid[row][col+1]
			tr := grid[row+1][col+1]
			tl := grid[row+1][col]

			// Skip cells with nodata corners.
			if bl == nodata || br == nodata || tr == nodata || tl == nodata {
				continue
			}

			// Compute case index (4-bit).
			caseIdx := 0
			if bl >= level {
				caseIdx |= 1
			}

			if br >= level {
				caseIdx |= 2
			}

			if tr >= level {
				caseIdx |= 4
			}

			if tl >= level {
				caseIdx |= 8
			}

			if caseIdx == 0 || caseIdx == 15 {
				continue // fully below or above
			}

			// Interpolation helpers.
			fcol := float64(col)
			frow := float64(row)

			// Edge midpoints with linear interpolation.
			// Bottom edge (bl-br).
			bottom := [2]float64{fcol + interpolate(bl, br, level), frow}
			// Right edge (br-tr).
			right := [2]float64{fcol + 1, frow + interpolate(br, tr, level)}
			// Top edge (tl-tr).
			top := [2]float64{fcol + interpolate(tl, tr, level), frow + 1}
			// Left edge (bl-tl).
			left := [2]float64{fcol, frow + interpolate(bl, tl, level)}

			switch caseIdx {
			case 1:
				segments = append(segments, [2][2]float64{bottom, left})
			case 2:
				segments = append(segments, [2][2]float64{right, bottom})
			case 3:
				segments = append(segments, [2][2]float64{right, left})
			case 4:
				segments = append(segments, [2][2]float64{top, right})
			case 5:
				// Saddle point: use center value to disambiguate.
				center := (bl + br + tr + tl) / 4
				if center >= level {
					segments = append(segments, [2][2]float64{top, left})
					segments = append(segments, [2][2]float64{right, bottom})
				} else {
					segments = append(segments, [2][2]float64{top, right})
					segments = append(segments, [2][2]float64{bottom, left})
				}
			case 6:
				segments = append(segments, [2][2]float64{top, bottom})
			case 7:
				segments = append(segments, [2][2]float64{top, left})
			case 8:
				segments = append(segments, [2][2]float64{left, top})
			case 9:
				segments = append(segments, [2][2]float64{bottom, top})
			case 10:
				// Saddle point.
				center := (bl + br + tr + tl) / 4
				if center >= level {
					segments = append(segments, [2][2]float64{bottom, right})
					segments = append(segments, [2][2]float64{left, top})
				} else {
					segments = append(segments, [2][2]float64{bottom, left})
					segments = append(segments, [2][2]float64{right, top})
				}
			case 11:
				segments = append(segments, [2][2]float64{right, top})
			case 12:
				segments = append(segments, [2][2]float64{left, right})
			case 13:
				segments = append(segments, [2][2]float64{bottom, right})
			case 14:
				segments = append(segments, [2][2]float64{left, bottom})
			}
		}
	}

	return segments
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

// joinSegments connects contiguous segments into polylines.
func joinSegments(segments [][2][2]float64) [][][2]float64 {
	if len(segments) == 0 {
		return nil
	}

	const eps = 1e-9

	// Index: endpoint → list of segment indices.
	type endpoint struct {
		x, y float64
	}

	quantize := func(v float64) float64 {
		return math.Round(v*1e6) / 1e6
	}

	key := func(pt [2]float64) endpoint {
		return endpoint{quantize(pt[0]), quantize(pt[1])}
	}

	_ = eps // used conceptually via quantize

	// Build adjacency.
	used := make([]bool, len(segments))
	startIndex := make(map[endpoint][]int)
	endIndex := make(map[endpoint][]int)

	for i, seg := range segments {
		sk := key(seg[0])
		ek := key(seg[1])

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

		// Extend forward from the end.
		for {
			lastPt := line[len(line)-1]
			k := key(lastPt)

			found := false

			for _, idx := range startIndex[k] {
				if used[idx] {
					continue
				}

				used[idx] = true
				line = append(line, segments[idx][1])
				found = true

				break
			}

			if !found {
				break
			}
		}

		// Extend backward from the start.
		for {
			firstPt := line[0]
			k := key(firstPt)

			found := false

			for _, idx := range endIndex[k] {
				if used[idx] {
					continue
				}

				used[idx] = true
				line = append([][2]float64{segments[idx][0]}, line...)
				found = true

				break
			}

			if !found {
				break
			}
		}

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
