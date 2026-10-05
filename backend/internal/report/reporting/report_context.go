package reporting

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/standards/framework"
)

type reportContext struct {
	Title string `json:"title"`

	TemplateVersion string `json:"template_version"`
	GeneratedAt     string `json:"generated_at"`

	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	ProjectCRS  string `json:"project_crs"`

	RunID      string `json:"run_id"`
	RunStatus  string `json:"run_status"`
	ScenarioID string `json:"scenario_id"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`

	SourceCount        string `json:"source_count,omitempty"`
	ParkingSourceCount string `json:"parking_source_count,omitempty"`
	ReceiverCount      string `json:"receiver_count,omitempty"`
	GridWidth          string `json:"grid_width,omitempty"`
	GridHeight         string `json:"grid_height,omitempty"`
	OutputHash         string `json:"output_hash,omitempty"`

	InputFiles      []inputFileView `json:"input_files"`
	ModelSourcePath string          `json:"model_source_path,omitempty"`
	ModelFeatureCnt string          `json:"model_feature_count,omitempty"`
	CountsByKind    []kindCountView `json:"counts_by_kind,omitempty"`

	StandardID      string `json:"standard_id"`
	StandardContext string `json:"standard_context,omitempty"`
	StandardVersion string `json:"standard_version"`
	StandardProfile string `json:"standard_profile,omitempty"`
	EvidenceTier    string `json:"evidence_tier"`

	StandardDataDigest string                  `json:"standard_data_digest"`
	StandardDataTables []standardDataTableView `json:"standard_data_tables"`

	Parameters []kvPairView    `json:"parameters"`
	Maps       []rasterMapView `json:"maps"`
	Indicators []indicatorView `json:"indicators"`
	Assessment *assessmentView `json:"assessment,omitempty"`
	QASuites   []qaSuiteView   `json:"qa_suites"`
	Notes      []string        `json:"notes,omitempty"`
}

type inputFileView struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type standardDataTableView struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type kindCountView struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type kvPairView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type rasterMapView struct {
	MetadataPath string `json:"metadata_path"`
	DataPath     string `json:"data_path"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Bands        int    `json:"bands"`
	// BandNames reads "name (unit)" per band — see bandsWithUnits for why the
	// unit is not a column of its own.
	BandNames string `json:"band_names,omitempty"`
}

type indicatorView struct {
	Indicator string `json:"indicator"`
	// Unit belongs on the row because it varies by row: beb-exposure lists
	// two decibel indicators beside six counts. A single unit above the table
	// was true of neither.
	Unit string  `json:"unit"`
	Min  float64 `json:"min"`
	Mean float64 `json:"mean"`
	Max  float64 `json:"max"`
}

type qaSuiteView struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

type assessmentView struct {
	Law            string          `json:"law"`
	SourceStandard string          `json:"source_standard"`
	AssessedCount  int             `json:"assessed_count"`
	ExceedingCount int             `json:"exceeding_count"`
	SkippedCount   int             `json:"skipped_count"`
	Categories     []kindCountView `json:"categories,omitempty"`
	ExamplesDE     []string        `json:"examples_de,omitempty"`
}

func buildContext(opts BuildOptions, generatedAt time.Time) (reportContext, error) {
	ctx := reportContext{
		Title:           defaultReportTitle,
		TemplateVersion: reportTypstTemplateVersion,
		GeneratedAt:     generatedAt.Format(time.RFC3339),
		ProjectID:       opts.Project.ProjectID,
		ProjectName:     opts.Project.Name,
		ProjectCRS:      opts.Project.CRS,
		RunID:           opts.Run.ID,
		RunStatus:       opts.Run.Status,
		ScenarioID:      opts.Run.ScenarioID,
		StartedAt:       formatTime(opts.Run.StartedAt),
		FinishedAt:      formatTime(opts.Run.FinishedAt),
		StandardID:      opts.Run.Standard.ID,
		StandardContext: opts.Run.Standard.Context,
		StandardVersion: opts.Run.Standard.Version,
		StandardProfile: opts.Run.Standard.Profile,
		InputFiles:      make([]inputFileView, 0),
		Parameters:      make([]kvPairView, 0),
		Maps:            make([]rasterMapView, 0),
		Indicators:      make([]indicatorView, 0),
		Notes:           make([]string, 0),
	}

	err := applyProvenance(&ctx, opts.ProvenancePath)
	if err != nil {
		return reportContext{}, err
	}

	summary, hasSummary, err := loadRunSummary(opts.RunSummaryPath)
	if err != nil {
		return reportContext{}, err
	}

	if hasSummary {
		applyRunSummary(&ctx, summary)
	}

	modelDump, hasModelDump, err := loadModelDump(opts.ModelDumpPath)
	if err != nil {
		return reportContext{}, err
	}

	if hasModelDump {
		ctx.ModelSourcePath = modelDump.SourcePath
		ctx.ModelFeatureCnt = strconv.Itoa(modelDump.FeatureCount)
		ctx.CountsByKind = kindCountsFromMap(modelDump.CountsByKind)
	}

	table, hasTable, err := loadReceiverTable(opts.ReceiverTablePath)
	if err != nil {
		return reportContext{}, err
	}

	if hasTable {
		ctx.Indicators = buildIndicatorStats(table)
		if ctx.ReceiverCount == "" {
			ctx.ReceiverCount = strconv.Itoa(len(table.Records))
		}
	}

	maps, err := loadRasterMaps(opts.BundleDir, opts.RasterMetaPaths)
	if err != nil {
		return reportContext{}, err
	}

	ctx.Maps = maps

	assessment, hasAssessment, err := loadAssessment(opts.AssessmentPath)
	if err != nil {
		return reportContext{}, err
	}

	if hasAssessment {
		ctx.Assessment = assessment
	}

	ctx.QASuites = normalizeQASuites(opts.QASuites)

	appendContextNotes(&ctx)

	return ctx, nil
}

// applyProvenance loads provenance data into the report context.
func applyProvenance(ctx *reportContext, provenancePath string) error {
	provenance, hasProvenance, err := loadProvenance(provenancePath)
	if err != nil {
		return err
	}

	ctx.EvidenceTier = UnknownEvidenceTier
	ctx.StandardDataDigest = NoStandardData
	ctx.StandardDataTables = []standardDataTableView{}

	if hasProvenance {
		if provenance.Standard.ID != "" {
			ctx.StandardID = provenance.Standard.ID
			ctx.StandardContext = provenance.Standard.Context
			ctx.StandardVersion = provenance.Standard.Version
			ctx.StandardProfile = provenance.Standard.Profile
		}

		tier := strings.TrimSpace(provenance.Metadata[ProvenanceEvidenceTierKey])
		if tier != "" {
			ctx.EvidenceTier = tier
		}

		applyStandardData(ctx, provenance.StandardData)

		ctx.InputFiles = inputFilesFromHashes(provenance.InputHashes)
		ctx.Parameters = kvPairsFromMap(provenance.Parameters)
	}

	if len(ctx.Parameters) == 0 {
		ctx.Parameters = []kvPairView{{Key: "(none)", Value: ""}}
	}

	return nil
}

// applyStandardData renders the standard-data digest, which identifies the
// coefficient tables that produced the run's numbers.
//
// It is deliberately kept out of the "Input files" table: that table is built
// from provenance input_hashes, which is defined as input-file path to SHA-256,
// and an embedded coefficient table is not a file anyone supplied.
func applyStandardData(ctx *reportContext, ref *project.StandardDataRef) {
	if ref == nil || strings.TrimSpace(ref.Digest) == "" {
		return
	}

	algorithm := strings.TrimSpace(ref.Algorithm)
	if algorithm == "" {
		ctx.StandardDataDigest = ref.Digest
	} else {
		ctx.StandardDataDigest = algorithm + ":" + ref.Digest
	}

	tables := make([]standardDataTableView, 0, len(ref.Tables))
	for _, table := range ref.Tables {
		name := strings.TrimSpace(table.Name)
		if name == "" {
			continue
		}

		tables = append(tables, standardDataTableView{Name: name, Digest: strings.TrimSpace(table.Digest)})
	}

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})

	ctx.StandardDataTables = tables
}

// appendContextNotes appends the evidence-tier disclosure and the explanatory
// notes for missing report sections, in fixed order.
func appendContextNotes(ctx *reportContext) {
	if ctx.EvidenceTier == string(framework.EvidenceTierScaffold) {
		ctx.Notes = append(ctx.Notes, "This run used a scaffold-tier standards module: it carries no normative coefficients, its base levels are invented, and its levels are not usable for assessment.")
	}

	if len(ctx.Maps) == 0 {
		ctx.Notes = append(ctx.Notes, "No raster map artifacts were found in the export bundle.")
	}

	if len(ctx.Indicators) == 0 {
		ctx.Notes = append(ctx.Notes, "No receiver table statistics were available.")
	}

	if len(ctx.InputFiles) == 0 {
		ctx.Notes = append(ctx.Notes, "No input hashes were found in provenance.")
	}

	if ctx.Assessment == nil && (ctx.StandardID == "rls19-road" || ctx.StandardID == "schall03") {
		ctx.Notes = append(ctx.Notes, "No 16. BImSchV assessment artifact was generated. This usually means the run used no explicit receivers with area-category properties.")
	}
}

func normalizeQASuites(in []QASuiteStatus) []qaSuiteView {
	if len(in) == 0 {
		return []qaSuiteView{
			{
				Name:    "acceptance-baseline",
				Status:  "not_configured",
				Details: "No QA suite artifacts were found for this run.",
			},
		}
	}

	out := make([]qaSuiteView, 0, len(in))
	for _, suite := range in {
		name := strings.TrimSpace(suite.Name)
		status := strings.TrimSpace(suite.Status)

		if name == "" {
			continue
		}

		if status == "" {
			status = "unknown"
		}

		out = append(out, qaSuiteView{
			Name:    name,
			Status:  status,
			Details: strings.TrimSpace(suite.Details),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	if len(out) == 0 {
		return []qaSuiteView{
			{
				Name:    "acceptance-baseline",
				Status:  "not_configured",
				Details: "No QA suite artifacts were found for this run.",
			},
		}
	}

	return out
}

func inputFilesFromHashes(hashes map[string]string) []inputFileView {
	if len(hashes) == 0 {
		return []inputFileView{}
	}

	keys := make([]string, 0, len(hashes))
	for key := range hashes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]inputFileView, 0, len(keys))
	for _, key := range keys {
		out = append(out, inputFileView{
			Path:   key,
			SHA256: hashes[key],
		})
	}

	return out
}
