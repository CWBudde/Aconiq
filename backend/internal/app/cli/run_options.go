package cli

import (
	"fmt"
	"maps"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	dummyfreefield "github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

const (
	dummyResultUnit       = "dB"
	defaultModelPath      = ".noise/model/model.normalized.geojson"
	maxDummyReceivers     = 250000
	receiverModeAutoGrid  = "auto-grid"
	receiverModeCustom    = "custom"
	explicitReceiverSetID = "explicit-manual"

	// evidenceTierKey names the tier field in provenance metadata, run
	// summaries and JSON command output alike, so the three never drift apart.
	evidenceTierKey = "evidence_tier"
)

type dummyRunOptions struct {
	GridResolutionM float64
	GridPaddingM    float64
	ReceiverHeightM float64
	SourceEmission  float64
	Workers         int
	ChunkSize       int
	DisableCache    bool
}

type persistedRunOutputs struct {
	ReceiverJSONPath   string
	ReceiverCSVPath    string
	RasterMetadataPath string
	RasterDataPath     string
	SummaryPath        string
}

func parseKeyValueFlags(values []string) (map[string]string, error) {
	params := make(map[string]string, len(values))
	for _, item := range values {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.parseKeyValueFlags", fmt.Sprintf("invalid --param %q (expected key=value)", item), nil)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.parseKeyValueFlags", fmt.Sprintf("invalid --param %q (empty key)", item), nil)
		}

		params[key] = value
	}

	return params, nil
}

// buildRunProvenanceMetadata assembles the provenance metadata for one run.
// The evidence tier is stamped centrally rather than by every module, so that
// the machine-readable tier and the free-text compliance_boundary a module may
// contribute stay independent of one another.
func buildRunProvenanceMetadata(resolved framework.ResolvedProfile, params map[string]string, receiverMode string) map[string]string {
	metadata := map[string]string{
		"receiver_mode": receiverMode,
		evidenceTierKey: string(resolved.EvidenceTier),
	}

	switch resolved.StandardID {
	case cnossosroad.StandardID:
		return mergeMetadata(metadata, cnossosroad.ProvenanceMetadata(params))
	case cnossosrail.StandardID:
		return mergeMetadata(metadata, cnossosrail.ProvenanceMetadata(params))
	case cnossosindustry.StandardID:
		return mergeMetadata(metadata, cnossosindustry.ProvenanceMetadata(params))
	case cnossosaircraft.StandardID:
		return mergeMetadata(metadata, cnossosaircraft.ProvenanceMetadata(params))
	case bubroad.StandardID:
		return mergeMetadata(metadata, bubroad.ProvenanceMetadata(params))
	case iso9613.StandardID:
		return mergeMetadata(metadata, iso9613.ProvenanceMetadata(params))
	case bufaircraft.StandardID:
		return mergeMetadata(metadata, bufaircraft.ProvenanceMetadata(params))
	case bebexposure.StandardID:
		return mergeMetadata(metadata, bebexposure.ProvenanceMetadata(params))
	case rls19road.StandardID:
		return mergeMetadata(metadata, rls19road.ProvenanceMetadata(params))
	case schall03.StandardID:
		return mergeMetadata(metadata, schall03.ProvenanceMetadata(params))
	default:
		return metadata
	}
}

// buildRunStandardData assembles the standard-data digest for one run.
//
// The digest answers a question the parameter and metadata maps cannot: which
// coefficient tables produced these numbers. It is recorded as its own
// provenance field rather than as an entry in input_hashes, which is defined as
// input-file path to SHA-256 and is rendered as an "Input files" table in
// reports; an embedded coefficient table is not a file the user supplied.
//
// A module that carries no coefficient data at all — dummy-freefield computes
// from its parameters alone — yields the zero value, and the field is omitted
// from the manifest rather than written empty.
func buildRunStandardData(resolved framework.ResolvedProfile) (project.StandardDataRef, error) {
	data, ok := standardDataForID(resolved.StandardID)
	if !ok {
		return project.StandardDataRef{}, nil
	}

	digest, err := data.Digest(resolved.StandardID, resolved.EvidenceTier)
	if err != nil {
		return project.StandardDataRef{}, domainerrors.New(domainerrors.KindInternal, "cli.buildRunStandardData", "compute standard data digest", err)
	}

	if digest.IsZero() {
		return project.StandardDataRef{}, nil
	}

	tables := make([]project.StandardDataTableRef, 0, len(digest.Tables))
	for _, table := range digest.Tables {
		tables = append(tables, project.StandardDataTableRef{Name: table.Name, Digest: table.Digest})
	}

	return project.StandardDataRef{
		Algorithm:    digest.Algorithm,
		Digest:       digest.Digest,
		EvidenceTier: string(digest.EvidenceTier),
		Tables:       tables,
	}, nil
}

// standardDataForID returns the coefficient data one module carries. The second
// result is false for a module that carries none.
//
// bub-rail and bub-industry are aliases over the cnossos rail and industry
// packages and share their coefficients, so they share their tables too.
func standardDataForID(standardID string) (framework.StandardData, bool) {
	switch standardID {
	case cnossosroad.StandardID:
		return cnossosroad.StandardData(), true
	case cnossosrail.StandardID, bubrail.StandardID:
		return cnossosrail.StandardData(), true
	case cnossosindustry.StandardID, bubindustry.StandardID:
		return cnossosindustry.StandardData(), true
	case cnossosaircraft.StandardID:
		return cnossosaircraft.StandardData(), true
	case bubroad.StandardID:
		return bubroad.StandardData(), true
	case bufaircraft.StandardID:
		return bufaircraft.StandardData(), true
	case bebexposure.StandardID:
		return bebexposure.StandardData(), true
	case iso9613.StandardID:
		return iso9613.StandardData(), true
	case rls19road.StandardID:
		return rls19road.StandardData(), true
	case schall03.StandardID:
		return schall03.StandardData(), true
	default:
		return framework.StandardData{}, false
	}
}

// runOptionParamKeys returns the normalized parameter names the CLI binds for
// one standard ID, in binding order. The second result is false for an ID the
// run pipeline does not parse options for.
//
// It exists so TestRunOptionsCoverParameterSchema can compare the binding tables
// against the parameter schemas the standards modules publish. Reading the keys
// off the same tables the parsers run is the point: a table that stops matching
// its schema fails the test rather than silently dropping a parameter.
func runOptionParamKeys(standardID string) ([]string, bool) {
	switch standardID {
	case dummyfreefield.StandardID:
		return boundParamKeys(dummyParamBindings(&dummyRunOptions{})), true
	case cnossosroad.StandardID:
		return boundParamKeys(cnossosRoadParamBindings(&cnossosRoadRunOptions{})), true
	case cnossosrail.StandardID:
		return boundParamKeys(cnossosRailParamBindings(&cnossosRailRunOptions{})), true
	case bubrail.StandardID:
		return boundParamKeys(bubRailParamBindings(&bubRailRunOptions{})), true
	case cnossosindustry.StandardID:
		return boundParamKeys(cnossosIndustryParamBindings(&cnossosIndustryRunOptions{})), true
	case bubindustry.StandardID:
		return boundParamKeys(bubIndustryParamBindings(&bubIndustryRunOptions{})), true
	case cnossosaircraft.StandardID, bufaircraft.StandardID:
		return boundParamKeys(aircraftParamBindings(&aircraftRunOptions{})), true
	case bubroad.StandardID:
		return boundParamKeys(bubRoadParamBindings(&bubRoadRunOptions{})), true
	case rls19road.StandardID:
		return boundParamKeys(rls19RoadParamBindings(&rls19RoadRunOptions{})), true
	case schall03.StandardID:
		return boundParamKeys(schall03ParamBindings(&schall03RunOptions{})), true
	case bebexposure.StandardID:
		return boundParamKeys(bebExposureParamBindings(&bebExposureRunOptions{})), true
	case iso9613.StandardID:
		return boundParamKeys(iso9613ParamBindings(&iso9613RunOptions{})), true
	default:
		return nil, false
	}
}

func mergeMetadata(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(extra))
	maps.Copy(merged, base)

	maps.Copy(merged, extra)

	return merged
}

func validateReceiverMode(mode string) error {
	switch mode {
	case receiverModeAutoGrid, receiverModeCustom:
		return nil
	default:
		return domainerrors.New(domainerrors.KindUserInput, "cli.run", fmt.Sprintf("invalid receiver mode %q", mode), nil)
	}
}

func receiverSetID(mode string) string {
	if mode == receiverModeCustom {
		return explicitReceiverSetID
	}

	return ""
}

// dummyParamBindings binds the dummy-freefield parameter schema.
//
// It is the only table that carries the engine controls, and the only one whose
// grid terms are range-checked here rather than by the schema alone.
func dummyParamBindings(options *dummyRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.minFloat(&options.GridResolutionM, 0.001),
		runParams.GridPaddingM.minFloat(&options.GridPaddingM, 0),
		runParams.ReceiverHeightM.minFloat(&options.ReceiverHeightM, 0),
		runParams.SourceEmissionDB.minFloat(&options.SourceEmission, 0),
		runParams.Workers.minInt(&options.Workers, 0),
		runParams.ChunkSize.minInt(&options.ChunkSize, 1),
		runParams.DisableCache.boolean(&options.DisableCache),
	}
}

func parseDummyRunOptions(params map[string]string) (dummyRunOptions, error) {
	options := dummyRunOptions{}

	err := applyBoundParams("cli.parseDummyRunOptions", params, dummyParamBindings(&options))
	if err != nil {
		return dummyRunOptions{}, err
	}

	return options, nil
}
