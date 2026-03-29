package soundplanimport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GridMapLayer describes one named layer embedded in an RRLK*.GM file.
type GridMapLayer struct {
	Name string `json:"name"`
	Unit string `json:"unit,omitempty"`
}

// GridMapMetadata describes the currently decoded metadata from one SoundPLAN
// grid-map result. Value extraction is intentionally deferred until the GM
// payload layout is understood well enough to avoid guesswork.
type GridMapMetadata struct {
	ResultSubFolder   string         `json:"result_subfolder"`
	RunType           string         `json:"run_type,omitempty"`
	GMFile            string         `json:"gm_file"`
	FileSizeBytes     int64          `json:"file_size_bytes"`
	PointsTotal       int            `json:"points_total,omitempty"`
	PointsCalculated  int            `json:"points_calculated,omitempty"`
	AssessmentPeriods []string       `json:"assessment_periods,omitempty"`
	Layers            []GridMapLayer `json:"layers,omitempty"`
	Warnings          []string       `json:"warnings,omitempty"`
}

// ParseGridMapMetadata reads the currently supported metadata from an RRLK*.GM file.
func ParseGridMapMetadata(path string) (GridMapMetadata, error) {
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

	return GridMapMetadata{
		GMFile:        filepath.Base(path),
		FileSizeBytes: info.Size(),
		Layers:        layers,
	}, nil
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

		parsed, err := ParseGridMapMetadata(gmPath)
		if err != nil {
			item.Warnings = append(item.Warnings, err.Error())
			out = append(out, item)
			continue
		}

		item.GMFile = parsed.GMFile
		item.FileSizeBytes = parsed.FileSizeBytes
		item.Layers = parsed.Layers
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
