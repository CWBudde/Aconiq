package soundplanimport

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// GridMapLayer describes one named layer embedded in an RRLK*.GM file.
type GridMapLayer struct {
	Name string `json:"name"`
	Unit string `json:"unit,omitempty"`
}

// GridMapValueStats describes one decoded numeric layer.
type GridMapValueStats struct {
	Name  string  `json:"name"`
	Unit  string  `json:"unit,omitempty"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
	Count int     `json:"count"`
}

// GridMapCell contains one decoded SoundPLAN grid-map cell.
type GridMapCell struct {
	GroundM float64 `json:"ground_m"`
	DayDB   float64 `json:"day_db"`
	NightDB float64 `json:"night_db"`
}

// DecodedGridMap contains the decoded GM row stream.
type DecodedGridMap struct {
	Rows            [][]GridMapCell `json:"rows"`
	MarkerCellCount int             `json:"marker_cell_count"`
	ValueCellCount  int             `json:"value_cell_count"`
}

// GridMapMetadata describes the currently decoded metadata from one SoundPLAN
// grid-map result. Value extraction is intentionally deferred until the GM
// payload layout is understood well enough to avoid guesswork.
type GridMapMetadata struct {
	ResultSubFolder   string              `json:"result_subfolder"`
	RunType           string              `json:"run_type,omitempty"`
	GMFile            string              `json:"gm_file"`
	FileSizeBytes     int64               `json:"file_size_bytes"`
	PointsTotal       int                 `json:"points_total,omitempty"`
	PointsCalculated  int                 `json:"points_calculated,omitempty"`
	OriginX           float64             `json:"origin_x,omitempty"`
	OriginY           float64             `json:"origin_y,omitempty"`
	SpacingX          float64             `json:"spacing_x,omitempty"`
	SpacingY          float64             `json:"spacing_y,omitempty"`
	DeclaredRowCount  int                 `json:"declared_row_count,omitempty"`
	AssessmentPeriods []string            `json:"assessment_periods,omitempty"`
	Layers            []GridMapLayer      `json:"layers,omitempty"`
	DecodedValues     bool                `json:"decoded_values"`
	ActiveCellCount   int                 `json:"active_cell_count,omitempty"`
	RowCount          int                 `json:"row_count,omitempty"`
	RowCellCounts     []int               `json:"row_cell_counts,omitempty"`
	ValueStats        []GridMapValueStats `json:"value_stats,omitempty"`
	Warnings          []string            `json:"warnings,omitempty"`
}

// LoadGridMapMetadata discovers grid-map result metadata from the known run directories.
func LoadGridMapMetadata(projectDir string, runs []*RunResult) []GridMapMetadata {
	out := make([]GridMapMetadata, 0, len(runs))

	for _, run := range runs {
		if run == nil || strings.TrimSpace(run.RunType) != "Grid Map Sound" {
			continue
		}

		subdir := strings.TrimSpace(run.ResultSubFolder)
		if subdir == "" {
			continue
		}

		suffix := extractRunSuffix(subdir)
		if suffix == "" {
			continue
		}

		gmPath := filepath.Join(projectDir, subdir, "RRLK"+suffix+".GM")
		item := GridMapMetadata{
			ResultSubFolder:  subdir,
			RunType:          run.RunType,
			GMFile:           filepath.Base(gmPath),
			PointsTotal:      run.Statistics.PointsTotal,
			PointsCalculated: run.Statistics.PointsCalculated,
		}

		for _, period := range run.AssessmentPeriods {
			if trimmed := strings.TrimSpace(period.Name); trimmed != "" {
				item.AssessmentPeriods = append(item.AssessmentPeriods, trimmed)
			}
		}

		parsed, err := ParseGridMapMetadata(gmPath, run.Statistics.PointsTotal)
		if err != nil {
			item.Warnings = append(item.Warnings, err.Error())
			out = append(out, item)
			continue
		}

		item.GMFile = parsed.GMFile
		item.FileSizeBytes = parsed.FileSizeBytes
		item.OriginX = parsed.OriginX
		item.OriginY = parsed.OriginY
		item.SpacingX = parsed.SpacingX
		item.SpacingY = parsed.SpacingY
		item.DeclaredRowCount = parsed.DeclaredRowCount
		item.Layers = parsed.Layers
		item.DecodedValues = parsed.DecodedValues
		item.ActiveCellCount = parsed.ActiveCellCount
		item.RowCount = parsed.RowCount
		item.RowCellCounts = parsed.RowCellCounts
		item.ValueStats = parsed.ValueStats
		item.Warnings = append(item.Warnings, parsed.Warnings...)
		out = append(out, item)
	}

	return out
}

func parseGridMapLayers(payload []byte) []GridMapLayer {
	seen := make(map[string]struct{})
	out := make([]GridMapLayer, 0, 4)

	for _, raw := range extractPrintableNullTerminatedStrings(payload) {
		name, unit, ok := parseGridMapLayer(raw)
		if !ok {
			continue
		}

		key := name + "|" + unit
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, GridMapLayer{Name: name, Unit: unit})
	}

	return out
}

type gridMapCellRecord struct {
	groundM float32
	dayDB   float32
	nightDB float32
	flag    byte
}

func ParseGridMapMetadata(path string, pointsTotal int) (GridMapMetadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return GridMapMetadata{}, fmt.Errorf("soundplan: stat GM: %w", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return GridMapMetadata{}, fmt.Errorf("soundplan: read GM: %w", err)
	}

	layers := parseGridMapLayers(payload)
	if len(layers) == 0 {
		return GridMapMetadata{}, fmt.Errorf("soundplan: parse GM: no raster layer descriptors found")
	}

	meta := GridMapMetadata{
		GMFile:        filepath.Base(path),
		FileSizeBytes: info.Size(),
		Layers:        layers,
	}
	if geometry, ok := parseGridMapGeometry(payload); ok {
		meta.OriginX = geometry.originX
		meta.OriginY = geometry.originY
		meta.SpacingX = geometry.spacingX
		meta.SpacingY = geometry.spacingY
		meta.DeclaredRowCount = geometry.rowCount
	} else {
		meta.Warnings = append(meta.Warnings, "soundplan: parse GM geometry: no plausible origin/spacing header found")
	}

	rows, decodeErr := decodeGridMapRows(payload, pointsTotal)
	if decodeErr != nil {
		meta.Warnings = append(meta.Warnings, decodeErr.Error())
		return meta, nil
	}

	meta.DecodedValues = true
	meta.RowCount = len(rows)
	meta.RowCellCounts = make([]int, 0, len(rows))

	groundStats := newGridMapStatAccumulator(layers, 0, "Ground elevation", "m")
	dayStats := newGridMapStatAccumulator(layers, 1, "Tag", "dB(A)")
	nightStats := newGridMapStatAccumulator(layers, 2, "Nacht", "dB(A)")

	for _, row := range rows {
		meta.RowCellCounts = append(meta.RowCellCounts, len(row))
		meta.ActiveCellCount += len(row)

		for _, cell := range row {
			groundStats.add(float64(cell.groundM))
			dayStats.add(float64(cell.dayDB))
			nightStats.add(float64(cell.nightDB))
		}
	}

	meta.ValueStats = compactGridMapStats([]GridMapValueStats{
		groundStats.finish(),
		dayStats.finish(),
		nightStats.finish(),
	})

	return meta, nil
}

type gridMapGeometry struct {
	originX  float64
	originY  float64
	spacingX float64
	spacingY float64
	rowCount int
}

func parseGridMapGeometry(payload []byte) (gridMapGeometry, bool) {
	const (
		gridMapHeaderMinBytes = 57
		rowCountOffset        = 4
		originXOffset         = 9
		originYOffset         = 17
		spacingXOffset        = 49
		spacingYOffset        = 53
	)

	if len(payload) < gridMapHeaderMinBytes {
		return gridMapGeometry{}, false
	}

	rowCount := int(binary.BigEndian.Uint16(payload[rowCountOffset : rowCountOffset+2]))
	originX := readF64(payload, originXOffset)
	originY := readF64(payload, originYOffset)
	spacingX := float64(readF32(payload, spacingXOffset))
	spacingY := float64(readF32(payload, spacingYOffset))

	if rowCount <= 0 || rowCount > 10000 {
		return gridMapGeometry{}, false
	}

	if !allFinite(originX, originY, spacingX, spacingY) {
		return gridMapGeometry{}, false
	}

	if spacingX <= 0 || spacingY <= 0 {
		return gridMapGeometry{}, false
	}

	// SoundPLAN fixture coordinates are in projected meters; reject obviously bogus headers.
	if originX < 1000 || originY < 1000 {
		return gridMapGeometry{}, false
	}

	return gridMapGeometry{
		originX:  originX,
		originY:  originY,
		spacingX: spacingX,
		spacingY: spacingY,
		rowCount: rowCount,
	}, true
}

// ParseDecodedGridMap decodes the current SoundPLAN GM payload into row-wise
// cells. The leading `(-1,0,0)` marker cell on each row is stripped.
func ParseDecodedGridMap(path string, pointsTotal int) (DecodedGridMap, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return DecodedGridMap{}, fmt.Errorf("soundplan: read GM: %w", err)
	}

	rows, err := decodeGridMapRows(payload, pointsTotal)
	if err != nil {
		return DecodedGridMap{}, err
	}

	out := DecodedGridMap{
		Rows: make([][]GridMapCell, 0, len(rows)),
	}

	for _, row := range rows {
		decodedRow := make([]GridMapCell, 0, len(row))
		for idx, cell := range row {
			if idx == 0 && cell.groundM == -1 && cell.dayDB == 0 && cell.nightDB == 0 {
				out.MarkerCellCount++
				continue
			}

			decodedRow = append(decodedRow, GridMapCell{
				GroundM: float64(cell.groundM),
				DayDB:   float64(cell.dayDB),
				NightDB: float64(cell.nightDB),
			})
			out.ValueCellCount++
		}

		out.Rows = append(out.Rows, decodedRow)
	}

	return out, nil
}

func decodeGridMapRows(payload []byte, pointsTotal int) ([][]gridMapCellRecord, error) {
	if pointsTotal <= 0 {
		return nil, fmt.Errorf("soundplan: decode GM rows: points_total must be > 0")
	}

	start, err := detectGridMapCellStreamStart(payload)
	if err != nil {
		return nil, err
	}

	spans := splitGridMapNonZeroSpansFromPayload(payload, start)
	startSpan := -1
	for i := range spans {
		sum := 0
		for _, row := range spans[i:] {
			sum += len(row)
		}

		if sum == pointsTotal {
			startSpan = i
			break
		}
	}

	if startSpan < 0 {
		return nil, fmt.Errorf("soundplan: decode GM rows: could not match %d points_total to decoded row spans", pointsTotal)
	}

	rows := spans[startSpan:]
	actualTotal := 0
	for _, row := range rows {
		actualTotal += len(row)
	}

	if actualTotal != pointsTotal {
		return nil, fmt.Errorf("soundplan: decode GM rows: got %d decoded cells, want %d", actualTotal, pointsTotal)
	}

	return rows, nil
}

func detectGridMapCellStreamStart(payload []byte) (int, error) {
	bestStart := -1
	bestCount := -1

	for start := 0; start < 13; start++ {
		count := 0
		for off := start; off <= len(payload)-13; off += 13 {
			if _, ok := decodeGridMapCellRecord(payload[off : off+13]); ok {
				count++
			}
		}

		if count > bestCount {
			bestStart = start
			bestCount = count
		}
	}

	if bestStart < 0 || bestCount <= 0 {
		return 0, fmt.Errorf("soundplan: decode GM rows: no plausible 13-byte cell stream found")
	}

	return bestStart, nil
}

func decodeGridMapCellRecord(chunk []byte) (gridMapCellRecord, bool) {
	if len(chunk) != 13 {
		return gridMapCellRecord{}, false
	}

	flag := chunk[12]
	if flag == 0 {
		return gridMapCellRecord{}, false
	}

	cell := gridMapCellRecord{
		groundM: math.Float32frombits(binary.LittleEndian.Uint32(chunk[0:4])),
		dayDB:   math.Float32frombits(binary.LittleEndian.Uint32(chunk[4:8])),
		nightDB: math.Float32frombits(binary.LittleEndian.Uint32(chunk[8:12])),
		flag:    flag,
	}

	if !allFinite(float64(cell.groundM), float64(cell.dayDB), float64(cell.nightDB)) {
		return gridMapCellRecord{}, false
	}

	switch {
	case cell.groundM == -1 && cell.dayDB == 0 && cell.nightDB == 0:
		return cell, true
	case cell.groundM >= 100 && cell.groundM <= 400 && cell.dayDB >= 0 && cell.dayDB <= 150 && cell.nightDB >= 0 && cell.nightDB <= 150:
		return cell, true
	default:
		return gridMapCellRecord{}, false
	}
}

func splitGridMapNonZeroSpansFromPayload(payload []byte, start int) [][]gridMapCellRecord {
	rows := make([][]gridMapCellRecord, 0, 96)
	current := make([]gridMapCellRecord, 0, 96)

	for off := start; off <= len(payload)-13; off += 13 {
		flag := payload[off+12]
		if flag == 0 {
			if len(current) > 0 {
				rows = append(rows, current)
				current = make([]gridMapCellRecord, 0, 96)
			}

			continue
		}

		cell, ok := decodeGridMapCellRecord(payload[off : off+13])
		if !ok {
			if len(current) > 0 {
				rows = append(rows, current)
				current = make([]gridMapCellRecord, 0, 96)
			}

			continue
		}

		current = append(current, cell)
	}

	if len(current) > 0 {
		rows = append(rows, current)
	}

	return rows
}

func allFinite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}

	return true
}

type gridMapStatAccumulator struct {
	name  string
	unit  string
	min   float64
	max   float64
	sum   float64
	count int
}

func newGridMapStatAccumulator(layers []GridMapLayer, index int, fallbackName, fallbackUnit string) gridMapStatAccumulator {
	acc := gridMapStatAccumulator{
		name: fallbackName,
		unit: fallbackUnit,
		min:  math.Inf(1),
		max:  math.Inf(-1),
	}

	if index >= 0 && index < len(layers) {
		if strings.TrimSpace(layers[index].Name) != "" {
			acc.name = layers[index].Name
		}
		if strings.TrimSpace(layers[index].Unit) != "" {
			acc.unit = layers[index].Unit
		}
	}

	return acc
}

func (a *gridMapStatAccumulator) add(value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}

	if value < a.min {
		a.min = value
	}
	if value > a.max {
		a.max = value
	}

	a.sum += value
	a.count++
}

func (a gridMapStatAccumulator) finish() GridMapValueStats {
	if a.count == 0 {
		return GridMapValueStats{}
	}

	return GridMapValueStats{
		Name:  a.name,
		Unit:  a.unit,
		Min:   a.min,
		Max:   a.max,
		Mean:  a.sum / float64(a.count),
		Count: a.count,
	}
}

func compactGridMapStats(items []GridMapValueStats) []GridMapValueStats {
	out := make([]GridMapValueStats, 0, len(items))
	for _, item := range items {
		if item.Count == 0 {
			continue
		}

		out = append(out, item)
	}

	slices.SortFunc(out, func(a, b GridMapValueStats) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out
}

func parseGridMapLayer(raw string) (string, string, bool) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return "", "", false
	}

	name, unit, ok := strings.Cut(clean, "|")
	if !ok {
		return "", "", false
	}

	name = strings.TrimSpace(name)
	unit = strings.TrimSpace(unit)
	if name == "" || unit == "" {
		return "", "", false
	}

	if !isPlausibleGridMapName(name) || !isPlausibleGridMapUnit(unit) {
		return "", "", false
	}

	return name, unit, true
}

func extractPrintableNullTerminatedStrings(payload []byte) []string {
	out := make([]string, 0, 16)
	var current []byte

	flush := func() {
		if len(current) < 3 {
			current = current[:0]
			return
		}

		out = append(out, string(current))
		current = current[:0]
	}

	for _, b := range payload {
		switch {
		case b == 0:
			flush()
		case b >= 32 && b <= 126:
			current = append(current, b)
		default:
			flush()
		}
	}

	flush()

	return out
}

func isPlausibleGridMapName(value string) bool {
	if value == "" {
		return false
	}

	hasLetter := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= 'a' && r <= 'z':
			hasLetter = true
		case r == ' ' || r == '-' || r == '_' || r == '(' || r == ')' || r == '/':
		default:
			return false
		}
	}

	return hasLetter
}

func isPlausibleGridMapUnit(value string) bool {
	switch value {
	case "m", "dB(A)", "dB":
		return true
	default:
		return false
	}
}
