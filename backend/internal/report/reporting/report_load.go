package reporting

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/results"
)

type provenanceEnvelope struct {
	Standard    project.StandardRef `json:"standard"`
	Parameters  map[string]string   `json:"parameters"`
	Metadata    map[string]string   `json:"metadata"`
	InputHashes map[string]string   `json:"input_hashes"`

	StandardData *project.StandardDataRef `json:"standard_data"`
}

type runSummaryEnvelope struct {
	SourceCount        *int
	ParkingSourceCount *int
	ReceiverCount      *int
	GridWidth          *int
	GridHeight         *int
	OutputHash         string
}

type modelDumpEnvelope struct {
	SourcePath   string         `json:"source_path"`
	FeatureCount int            `json:"feature_count"`
	CountsByKind map[string]int `json:"counts_by_kind"`
}

// rasterMetaEnvelope is reporting's own reading of a raster sidecar: it takes
// the four fields a report names and does not depend on results.RasterMetadata.
//
// LegacyUnit is the pre-per-band spelling. Sidecars already on disk carry it
// and nothing rewrites them, so a report generated over an old bundle has to
// keep naming the unit it found — see bandsWithUnits.
type rasterMetaEnvelope struct {
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Bands      int               `json:"bands"`
	Units      map[string]string `json:"units"`
	LegacyUnit string            `json:"unit"`
	BandNames  []string          `json:"band_names"`
	DataFile   string            `json:"data_file"`
}

func loadAssessment(path string) (*assessmentView, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("read assessment %s: %w", path, err)
	}

	var parsed struct {
		Law              string         `json:"law"`
		SourceStandardID string         `json:"source_standard_id"`
		AssessedCount    int            `json:"assessed_count"`
		ExceedingCount   int            `json:"exceeding_count"`
		CategoryCounts   map[string]int `json:"category_counts"`
		Skipped          []any          `json:"skipped"`
		Results          []struct {
			SummaryDE string `json:"summary_de"`
		} `json:"results"`
	}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, false, fmt.Errorf("decode assessment %s: %w", path, err)
	}

	view := &assessmentView{
		Law:            parsed.Law,
		SourceStandard: parsed.SourceStandardID,
		AssessedCount:  parsed.AssessedCount,
		ExceedingCount: parsed.ExceedingCount,
		SkippedCount:   len(parsed.Skipped),
		Categories:     kindCountsFromMap(parsed.CategoryCounts),
		ExamplesDE:     make([]string, 0, minInt(3, len(parsed.Results))),
	}
	for _, result := range parsed.Results {
		if strings.TrimSpace(result.SummaryDE) == "" {
			continue
		}

		view.ExamplesDE = append(view.ExamplesDE, result.SummaryDE)
		if len(view.ExamplesDE) == 3 {
			break
		}
	}

	return view, true, nil
}

func loadProvenance(path string) (provenanceEnvelope, bool, error) {
	if strings.TrimSpace(path) == "" {
		return provenanceEnvelope{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return provenanceEnvelope{}, false, nil
		}

		return provenanceEnvelope{}, false, fmt.Errorf("read provenance %s: %w", path, err)
	}

	var parsed provenanceEnvelope

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return provenanceEnvelope{}, false, fmt.Errorf("decode provenance %s: %w", path, err)
	}

	return parsed, true, nil
}

func loadRunSummary(path string) (runSummaryEnvelope, bool, error) {
	if strings.TrimSpace(path) == "" {
		return runSummaryEnvelope{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runSummaryEnvelope{}, false, nil
		}

		return runSummaryEnvelope{}, false, fmt.Errorf("read run summary %s: %w", path, err)
	}

	var parsed map[string]any

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return runSummaryEnvelope{}, false, fmt.Errorf("decode run summary %s: %w", path, err)
	}

	out := runSummaryEnvelope{
		SourceCount:        optionalInt(parsed["source_count"]),
		ParkingSourceCount: optionalInt(parsed["parking_source_count"]),
		ReceiverCount:      optionalInt(parsed["receiver_count"]),
		GridWidth:          optionalInt(parsed["grid_width"]),
		GridHeight:         optionalInt(parsed["grid_height"]),
	}
	if hashText, ok := parsed["output_hash"].(string); ok {
		out.OutputHash = strings.TrimSpace(hashText)
	}

	return out, true, nil
}

func loadModelDump(path string) (modelDumpEnvelope, bool, error) {
	if strings.TrimSpace(path) == "" {
		return modelDumpEnvelope{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return modelDumpEnvelope{}, false, nil
		}

		return modelDumpEnvelope{}, false, fmt.Errorf("read model dump %s: %w", path, err)
	}

	var parsed modelDumpEnvelope

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return modelDumpEnvelope{}, false, fmt.Errorf("decode model dump %s: %w", path, err)
	}

	return parsed, true, nil
}

func loadReceiverTable(path string) (results.ReceiverTable, bool, error) {
	if strings.TrimSpace(path) == "" {
		return results.ReceiverTable{}, false, nil
	}

	// Through results' own loader, not a json.Unmarshal of our own. That
	// loader is where a table written before the unit was per indicator has
	// its scalar "unit" expanded across the indicator list, and Validate below
	// refuses a table with no units at all — so decoding it here instead meant
	// `aconiq export` aborted on every run created before that change.
	table, err := results.LoadReceiverTableJSON(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return results.ReceiverTable{}, false, nil
		}

		// results names the path already; this says which consumer wanted it,
		// since the same table is read by export and by compare.
		return results.ReceiverTable{}, false, fmt.Errorf("report: %w", err)
	}

	err = table.Validate()
	if err != nil {
		return results.ReceiverTable{}, false, fmt.Errorf("validate receiver table %s: %w", path, err)
	}

	return table, true, nil
}

func loadRasterMaps(bundleDir string, metaPaths []string) ([]rasterMapView, error) {
	paths := make([]string, 0, len(metaPaths))
	for _, rawPath := range metaPaths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}

		paths = append(paths, trimmed)
	}

	sort.Strings(paths)

	views := make([]rasterMapView, 0, len(paths))
	for _, metaPath := range paths {
		payload, err := os.ReadFile(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("read raster metadata %s: %w", metaPath, err)
		}

		var meta rasterMetaEnvelope
		{
			err := json.Unmarshal(payload, &meta)
			if err != nil {
				return nil, fmt.Errorf("decode raster metadata %s: %w", metaPath, err)
			}
		}

		dataPath := meta.DataFile
		if !filepath.IsAbs(dataPath) {
			dataPath = filepath.Join(filepath.Dir(metaPath), meta.DataFile)
		}

		views = append(views, rasterMapView{
			MetadataPath: filepath.ToSlash(relativeFrom(bundleDir, metaPath)),
			DataPath:     filepath.ToSlash(relativeFrom(bundleDir, dataPath)),
			Width:        meta.Width,
			Height:       meta.Height,
			Bands:        meta.Bands,
			BandNames:    bandsWithUnits(meta),
		})
	}

	return views, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
