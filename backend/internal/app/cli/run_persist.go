package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/framework"
)

// gridMaskedCellsKey counts the grid cells a raster leaves at nodata because
// their receiver stands inside a building footprint. Browser mode writes the
// same key.
const gridMaskedCellsKey = "grid_masked_cells"

// newRunSummary builds the keys every standard-backed run summary carries.
// Callers add only what is specific to their standard on top of it.
//
// modelVersion may be empty for a module that versions nothing of its own; the
// key is then left out rather than written as an empty string.
func newRunSummary(
	runDir string,
	outputHash string,
	modelVersion string,
	receiverMode string,
	tier framework.EvidenceTier,
	projection computeProjection,
	sourceCount int,
	receiverCount int,
) map[string]any {
	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": receiverCount,
		"receiver_mode":  receiverMode,
		evidenceTierKey:  string(tier),
	}

	addComputeCRS(summary, projection)

	if modelVersion != "" {
		summary["model_version"] = modelVersion
	}

	return summary
}

// addComputeCRS stamps the CRS a run computed in onto its summary.
//
// The receiver table carries x/y and no CRS — results.ReceiverTable has no
// field for one, and adding one would fork the container format — so the run
// summary is where a consumer that only ever sees the results learns which
// CRS to read them in. The keys are provenance's own, because provenance is
// not an artifact and the API therefore never serves it: a caller reading
// `run.result.summary` would otherwise have nowhere to ask. Browser mode
// writes the same two keys (`api/browser-backend.ts`), so a receiver table
// means the same thing on both targets.
func addComputeCRS(summary map[string]any, projection computeProjection) {
	summary[provenanceProjectCRSKey] = projection.ProjectCRS
	summary[provenanceComputeCRSKey] = projection.ComputeCRS
}

// writeGridRunSummary stamps the raster grid dimensions onto a run summary and
// writes it next to the exported result bundle.
//
// grid_masked_cells is written only when a building masked a cell, so a run
// over a model without buildings writes the summary it always did. It says
// why receiver_count and the raster's valid-cell count differ.
func writeGridRunSummary(resultsDir string, summary map[string]any, layout results.GridLayout) (string, error) {
	summary["grid_width"] = layout.Width
	summary["grid_height"] = layout.Height

	if masked := layout.NoDataCount(); masked > 0 {
		summary[gridMaskedCellsKey] = masked
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err := writeJSONFile(summaryPath, summary)
	if err != nil {
		return "", err
	}

	return summaryPath, nil
}

func persistReceiverTableOnly(
	resultsDir string,
	table results.ReceiverTable,
	summary map[string]any,
) (persistedRunOutputs, error) {
	err := os.MkdirAll(resultsDir, 0o750)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "create results directory "+resultsDir, err)
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table json", err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table csv", err)
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		SummaryPath:      summaryPath,
	}, nil
}

// withComputeCRS stamps the CRS the run computed in onto the grid layout.
//
// buildReceiversFromPoints builds the layout without knowing which CRS it is
// working in — it is handed coordinates, not a projection. The persist layer
// is the first place that knows, and the raster sidecar is the only artifact
// that carries the CRS to a consumer: provenance is not an ArtifactRef, so the
// API never serves it, and a GIS export reading a sidecar has nowhere else to
// ask.
func withComputeCRS(layout results.GridLayout, projection computeProjection) results.GridLayout {
	layout.CRS = projection.ComputeCRS

	return layout
}

// exportOutputs is the shape every standards module's ExportResultBundle
// returns. The per-module structs are structurally identical but nominally
// distinct, so this constraint is what lets one adapter convert any of them.
type exportOutputs interface {
	~struct {
		ReceiverJSONPath string
		ReceiverCSVPath  string
		RasterMetaPath   string
		RasterDataPath   string
	}
}

// exportBundle adapts a module's ExportResultBundle to the export hook a
// persist plan carries: it binds the outputs and the grid to write and wraps a
// failure in the caller's scope.
func exportBundle[Output any, Bundle exportOutputs](
	scope string,
	message string,
	export func(string, []Output, results.GridLayout) (Bundle, error),
	outputs []Output,
	layout results.GridLayout,
) func(resultsDir string) (exportedBundle, error) {
	return func(resultsDir string) (exportedBundle, error) {
		exported, err := export(resultsDir, outputs, layout)
		if err != nil {
			return exportedBundle{}, domainerrors.New(domainerrors.KindInternal, scope, message, err)
		}

		return exportedBundle(exported), nil
	}
}

// receiverPersistPlan carries the whole of what one standard's receiver-output
// persist path does not share with the others: the identity its errors and its
// run summary carry, the indicator vocabulary its receiver table publishes, and
// the result bundle it writes to disk. Everything around those — the output
// hash, the run summary, the custom-receiver short circuit and the grid bundle —
// is the same for every standard that produces receiver outputs, and lives once
// in persistReceiverRunOutputs.
//
// indicators returns any rather than a second type parameter because the value
// is only ever marshalled: the hash payload is byte-identical either way.
type receiverPersistPlan[Output any] struct {
	scope           string
	hashLabel       string
	hashErrMessage  string
	modelVersion    string
	indicatorOrder  []string
	decorateSummary func(summary map[string]any)
	receiver        func(Output) geo.PointReceiver
	indicators      func(Output) any
	values          func(Output) map[string]float64
	export          func(resultsDir string) (exportedBundle, error)
}

// persistReceiverRunOutputs writes one standard's receiver outputs under runDir
// and returns the paths, the output hash and the finish time the run manifest
// records. In custom-receiver mode it writes a receiver table only; otherwise it
// writes the standard's full grid bundle beside a grid run summary.
func persistReceiverRunOutputs[Output any](
	plan receiverPersistPlan[Output],
	runDir string,
	outputs []Output,
	layout results.GridLayout,
	sourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")
	layout = withComputeCRS(layout, projection)

	outputHash, err := hashReceiverOutputs(
		plan.hashLabel,
		outputs,
		func(output Output) string { return plan.receiver(output).ID },
		plan.indicators,
	)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, plan.scope, plan.hashErrMessage, err)
	}

	summary := newRunSummary(runDir, outputHash, plan.modelVersion, receiverMode, tier, projection, sourceCount, len(outputs))
	if plan.decorateSummary != nil {
		plan.decorateSummary(summary)
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{
			IndicatorOrder: plan.indicatorOrder,
			Units:          results.UniformUnits(plan.indicatorOrder, results.UnitDecibel),
			Records:        make([]results.ReceiverRecord, 0, len(outputs)),
		}

		for _, output := range outputs {
			receiver := plan.receiver(output)
			table.Records = append(table.Records, results.ReceiverRecord{
				ID:      receiver.ID,
				X:       receiver.Point.X,
				Y:       receiver.Point.Y,
				HeightM: receiver.HeightM,
				Values:  plan.values(output),
			})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := plan.export(resultsDir)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	summaryPath, err := writeGridRunSummary(resultsDir, summary, layout)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   exported.ReceiverJSONPath,
		ReceiverCSVPath:    exported.ReceiverCSVPath,
		RasterMetadataPath: exported.RasterMetaPath,
		RasterDataPath:     exported.RasterDataPath,
		SummaryPath:        summaryPath,
	}, outputHash, nowUTC(), nil
}

// exportedBundle is the shape every standards module's ExportResultBundle
// returns. The per-module structs are structurally identical but nominally
// distinct, so a persist path shared by an alias pair needs one type to hand
// back.
type exportedBundle struct {
	ReceiverJSONPath string
	ReceiverCSVPath  string
	RasterMetaPath   string
	RasterDataPath   string
}

// hashedReceiverRecord is the unit of the run output-hash contract: one
// receiver's ID beside its indicator block. The JSON tags are part of that
// contract — changing them changes every output hash this repo has ever stored.
type hashedReceiverRecord[Indicators any] struct {
	ReceiverID string     `json:"receiver_id"`
	Indicators Indicators `json:"indicators"`
}

// hashedBuildingRecord is the BEB equivalent. Exposure is aggregated per
// building, so the key is a building ID rather than a receiver ID.
type hashedBuildingRecord[Indicators any] struct {
	BuildingID string     `json:"building_id"`
	Indicators Indicators `json:"indicators"`
}

// hashReceiverOutputs hashes one standard's receiver outputs. Every standard
// but BEB hashes the same shape over a different indicator type, so the only
// per-standard inputs are the two accessors and label, which names the payload
// in the error a marshal failure produces.
func hashReceiverOutputs[Output, Indicators any](
	label string,
	outputs []Output,
	receiverID func(Output) string,
	indicators func(Output) Indicators,
) (string, error) {
	records := make([]hashedReceiverRecord[Indicators], 0, len(outputs))
	for _, output := range outputs {
		records = append(records, hashedReceiverRecord[Indicators]{
			ReceiverID: receiverID(output),
			Indicators: indicators(output),
		})
	}

	return hashJSONPayload(label, records)
}

func hashJSONPayload(label string, payload any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", label, err)
	}

	sum := sha256.Sum256(encoded)

	return hex.EncodeToString(sum[:]), nil
}
