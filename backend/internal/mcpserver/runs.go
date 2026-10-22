package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/jsonio"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// inlineResultLimit is the largest receiver table get_run_results returns in
// its result. A larger one — a full auto grid — would fill much of an
// agent's context, so it is answered with what is needed to read it and a
// link to the artifact resource instead (ADR-002, "Large payloads").
const inlineResultLimit = 256 << 10

// errorCodeResultsNotFound refuses get_run_results for a run that has no
// receiver table: one still computing, or one that failed.
const errorCodeResultsNotFound = "results_not_found"

// errorCodeNotFound is the API's own code for an unknown id, which a run id
// that cannot name a run is answered with.
const errorCodeNotFound = "not_found"

type listRunsInput struct{}

type runInput struct {
	RunID string `json:"run_id" jsonschema:"The run's id, as list_runs or start_run returned it."`
}

// contoursInput holds the interval as a pointer, so an interval the agent
// gave — zero included — reaches the API to be judged, and only an absent
// one takes the API's default.
type contoursInput struct {
	RunID    string   `json:"run_id"             jsonschema:"The run's id, as list_runs or start_run returned it."`
	Interval *float64 `json:"interval,omitempty" jsonschema:"Contour step in dB. Omitted, the API's default step."`
	CRS      string   `json:"crs,omitempty"      jsonschema:"EPSG identifier to answer in, such as EPSG:4326. Omitted, the run's compute_crs."`
}

// runRequestInput is a run request as start_run and estimate_run take it.
// The fields that select a method carry no omitempty, so the schema requires
// them, and requiredRunFields keeps each from being empty: an omitted or
// empty one would be left out of the `aconiq run` argv and replaced by the
// CLI's default, a choice the agent never made (ADR-002, security policy
// 6). It names no model or input path: a run computes the saved model, the
// one get_model shows.
type runRequestInput struct {
	ScenarioID      string            `json:"scenario_id"            jsonschema:"The scenario to run; default in a new project."`
	StandardID      string            `json:"standard_id"            jsonschema:"A standard id from list_standards."`
	StandardVersion string            `json:"standard_version"       jsonschema:"One of the standard's versions, from list_standards."`
	StandardProfile string            `json:"standard_profile"       jsonschema:"One of that version's profiles, from list_standards."`
	ReceiverMode    string            `json:"receiver_mode"          jsonschema:"auto-grid (a receiver grid over the sources or the calc-area), custom (the model's receiver features) or facade (receivers generated on the facades of facade_buildings; rls19-road and schall03 only)."`
	Params          map[string]string `json:"params,omitempty"       jsonschema:"Parameters of the profile by name, each value as a string. Omitted ones take the default list_standards declares; provenance.json records the values the run resolved."`
	Experimental    bool              `json:"experimental,omitempty" jsonschema:"Opt in to a scaffold-tier standard. A scaffold is not an implementation of the method its name suggests; without this its run is refused."`

	FacadeBuildings        []string `json:"facade_buildings,omitempty"         jsonschema:"With receiver_mode facade: the buildings whose facades carry receivers, each a building feature id from get_model or \"<street> <number>\" as its addr:street and addr:housenumber spell it."`
	FacadeSpacingM         *float64 `json:"facade_spacing_m,omitempty"         jsonschema:"With receiver_mode facade: the station spacing along each facade in metres. Required there; there is no default."`
	FacadeIncludeCourtyard bool     `json:"facade_include_courtyard,omitempty" jsonschema:"With receiver_mode facade: also place receivers on courtyard facades."`
	AllowUnconfirmed       bool     `json:"allow_unconfirmed,omitempty"        jsonschema:"Opt in to a normative run over facade receivers that are still proposed or whose slab height is assumed. Without it such a run is refused; with it their counts are recorded in the run's provenance and report."`
}

var requiredRunFields = []string{"scenario_id", "standard_id", "standard_version", "standard_profile", "receiver_mode"}

// addRunReadTools registers the read-only run tools.
func (t tools) addRunReadTools(server *mcp.Server, readOnly *mcp.ToolAnnotations) error {
	// mcp.AddTool validates estimate_run's arguments against the schema it is
	// given, so the method fields are held non-empty here as start_run holds
	// them.
	estimateSchema, _, err := inputSchema[runRequestInput](nil, requiredRunFields...)
	if err != nil {
		return err
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolListRuns,
		Description: "Every run of the project, newest first: status, standard, scenario, its artifacts and, while it computes, its progress.",
		Annotations: readOnly,
	}, t.listRuns)

	mcp.AddTool(server, &mcp.Tool{
		Name: toolGetRun,
		Description: "One run's summary: status, standard, artifacts and, while it is pending or running, progress (done, total, eta_seconds). " +
			"Poll this after start_run until status is completed or failed; there is no progress stream.",
		Annotations: readOnly,
	}, t.getRun)

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolGetRunLog,
		Description: "One run's log, line by line: what it computed, what it refused and why it failed.",
		Annotations: readOnly,
	}, t.getRunLog)

	mcp.AddTool(server, &mcp.Tool{
		Name: toolGetRunResults,
		Description: "A finished run's receiver table: indicator_order, units (per indicator), and one record per receiver with its x, y (in the run's compute_crs), height_m and values. " +
			"A table over 256 KiB is not returned inline: the answer is its indicators, units, record count and size, with a link to the aconiq://artifacts/{id} resource holding it.",
		Annotations: readOnly,
	}, t.getRunResults)

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolGetContours,
		Description: "Contour lines traced from a finished grid run's result raster, per band, at the interval given (dB), in the CRS named in the answer.",
		Annotations: readOnly,
	}, t.getContours)

	mcp.AddTool(server, &mcp.Tool{
		Name: toolEstimateRun,
		Description: "What the run start_run would start with these arguments would cost in time, from a timed probe of its own kernel; it creates no run and changes nothing. " +
			"It refuses what start_run would refuse. Its timings are measured, so two calls differ.",
		InputSchema: estimateSchema,
		Annotations: readOnly,
	}, t.estimateRun)

	return nil
}

// addRunWriteTools registers start_run and cancel_run. They are registered
// with the low-level AddTool, as save_model is, so that a call whose
// arguments the schema refuses is still audited.
func (t tools) addRunWriteTools(server *mcp.Server) error {
	startSchema, start, err := inputSchema[runRequestInput](nil, requiredRunFields...)
	if err != nil {
		return err
	}

	cancelSchema, cancel, err := inputSchema[runInput](nil, "run_id")
	if err != nil {
		return err
	}

	stateChanging := &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)}

	server.AddTool(&mcp.Tool{
		Name: toolStartRun,
		Description: "Start a run of the saved model and return its row at once (status pending or running); it computes in the background. " +
			"Poll get_run for progress, then read get_run_results, get_contours or get_run_log. " +
			"A scaffold-tier standard is refused without experimental: true, and a normative run over proposed or assumed-height facade receivers without allow_unconfirmed: true. " +
			"Runs this server started stop when it exits.",
		InputSchema: startSchema,
		Annotations: stateChanging,
	}, t.audited(toolStartRun, func(ctx context.Context, arguments json.RawMessage) served {
		return t.serveStartRun(ctx, start, arguments)
	}))

	server.AddTool(&mcp.Tool{
		Name:        toolCancelRun,
		Description: "Ask a run this server started to stop. It ends failed, with \"run cancelled\" in its log; a run started elsewhere is refused with run_not_owned.",
		InputSchema: cancelSchema,
		Annotations: stateChanging,
	}, t.audited(toolCancelRun, func(ctx context.Context, arguments json.RawMessage) served {
		return t.serveCancelRun(ctx, cancel, arguments)
	}))

	return nil
}

func (t tools) listRuns(ctx context.Context, _ *mcp.CallToolRequest, _ listRunsInput) (*mcp.CallToolResult, any, error) {
	resp, err := t.api.get(ctx, "/api/v1/runs")
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

func (t tools) getRun(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	return t.getRunPath(ctx, in.RunID, "", nil)
}

func (t tools) getRunLog(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	return t.getRunPath(ctx, in.RunID, "/log", nil)
}

func (t tools) getContours(ctx context.Context, _ *mcp.CallToolRequest, in contoursInput) (*mcp.CallToolResult, any, error) {
	query := url.Values{}
	if in.Interval != nil {
		query.Set("interval", strconv.FormatFloat(*in.Interval, 'g', -1, 64))
	}

	if in.CRS != "" {
		query.Set("crs", in.CRS)
	}

	return t.getRunPath(ctx, in.RunID, "/contours", query)
}

// getRunPath serves GET /api/v1/runs/{id}<suffix>, with the run id as one
// path segment of the route.
func (t tools) getRunPath(ctx context.Context, runID string, suffix string, query url.Values) (*mcp.CallToolResult, any, error) {
	segment, ok := pathSegment(runID)
	if !ok {
		return runNotFound(runID).result(), nil, nil
	}

	path := "/api/v1/runs/" + segment + suffix
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	resp, err := t.api.get(ctx, path)
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

// runNotFound answers a run id that cannot name a run as the API answers one
// that names none.
func runNotFound(runID string) toolError {
	return toolError{Code: errorCodeNotFound, Message: fmt.Sprintf("run %q not found", runID)}
}

// receiverTableSummary is what get_run_results answers for a table past the
// inline limit. Every field is read from the table or the manifest, none
// derived.
type receiverTableSummary struct {
	RunID          string            `json:"run_id"`
	ArtifactID     string            `json:"artifact_id"`
	IndicatorOrder []string          `json:"indicator_order"`
	Units          map[string]string `json:"units"`
	RecordCount    int               `json:"record_count"`
	SizeBytes      int               `json:"size_bytes"`
	Resource       string            `json:"resource"`
}

func (t tools) getRunResults(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	segment, ok := pathSegment(in.RunID)
	if !ok {
		return runNotFound(in.RunID).result(), nil, nil
	}

	resp, err := t.api.get(ctx, "/api/v1/runs/"+segment)
	if err != nil {
		return nil, nil, err
	}

	if !resp.ok() {
		return resp.toolResult(), nil, nil
	}

	artifactID, status, err := receiverTableOf(resp.body)
	if err != nil {
		return nil, nil, fmt.Errorf("run %s: %w", in.RunID, err)
	}

	if artifactID == "" {
		return toolError{
			Code:    errorCodeResultsNotFound,
			Message: fmt.Sprintf("run %q is %s and has no receiver table", in.RunID, status),
			Hint:    "Poll get_run until status is completed; a failed run has no results, and get_run_log says why.",
		}.result(), nil, nil
	}

	artifactSegment, ok := pathSegment(artifactID)
	if !ok {
		return nil, nil, fmt.Errorf("run %s names artifact %q, which is not one path segment", in.RunID, artifactID)
	}

	resp, err = t.api.get(ctx, "/api/v1/artifacts/"+artifactSegment+"/content")
	if err != nil {
		return nil, nil, err
	}

	if !resp.ok() || len(resp.body) <= inlineResultLimit {
		return resp.toolResult(), nil, nil
	}

	result, err := linkedTable(in.RunID, artifactID, resp.body)

	return result, nil, err
}

// receiverTableOf reads a run summary for the id of its receiver table
// artifact, empty when it has none, and the run's status.
func receiverTableOf(summary []byte) (string, string, error) {
	var run struct {
		Status    string `json:"status"`
		Artifacts []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"artifacts"`
	}

	err := json.Unmarshal(summary, &run)
	if err != nil {
		return "", "", fmt.Errorf("decode run summary: %w", err)
	}

	for _, artifact := range run.Artifacts {
		if artifact.Kind == project.ArtifactKindRunResultReceiverTableJSON {
			return artifact.ID, run.Status, nil
		}
	}

	return "", run.Status, nil
}

// linkedTable answers for a receiver table too large to return inline.
func linkedTable(runID string, artifactID string, table []byte) (*mcp.CallToolResult, error) {
	var decoded struct {
		IndicatorOrder []string          `json:"indicator_order"`
		Units          map[string]string `json:"units"`
		Records        []json.RawMessage `json:"records"`
	}

	err := json.Unmarshal(table, &decoded)
	if err != nil {
		return nil, fmt.Errorf("decode receiver table %s: %w", artifactID, err)
	}

	uri := artifactResourcePrefix + url.PathEscape(artifactID)

	summary, err := jsonio.Marshal(receiverTableSummary{
		RunID:          runID,
		ArtifactID:     artifactID,
		IndicatorOrder: decoded.IndicatorOrder,
		Units:          decoded.Units,
		RecordCount:    len(decoded.Records),
		SizeBytes:      len(table),
		Resource:       uri,
	})
	if err != nil {
		return nil, fmt.Errorf("encode receiver table summary: %w", err)
	}

	size := int64(len(table))

	return &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: string(summary)},
		&mcp.ResourceLink{URI: uri, Name: "receiver table", MIMEType: jsonMIMEType, Size: &size},
	}}, nil
}

func (t tools) estimateRun(ctx context.Context, _ *mcp.CallToolRequest, in runRequestInput) (*mcp.CallToolResult, any, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("encode estimate request: %w", err)
	}

	resp, err := t.api.postJSON(ctx, "/api/v1/runs/estimate", body)
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

// serveStartRun validates and serves one start_run call. Its body is the
// API's run request with the agent's fields only, so the request validation
// and the scaffold gate are the API's.
func (t tools) serveStartRun(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	in, err := decodeArguments[runRequestInput](schema, arguments)
	if err != nil {
		return invalidArguments(toolStartRun, err,
			"Pass scenario_id, standard_id, standard_version, standard_profile and receiver_mode, each non-empty, and optionally params, experimental, "+
				"the facade_* fields of receiver_mode facade and allow_unconfirmed.")
	}

	body, err := json.Marshal(in)
	if err != nil {
		return internalError(fmt.Errorf("encode run request: %w", err))
	}

	// The run belongs to the server, not to this call. An agent that goes
	// away while the API waits for the run's row would leave the API without
	// an answer to write, and the audit without the id of a run that computes
	// anyway; the API bounds that wait itself.
	resp, err := t.api.postJSON(context.WithoutCancel(ctx), "/api/v1/runs", body)
	if err != nil {
		return internalError(err)
	}

	if !resp.ok() {
		return served{result: resp.toolResult(), outcome: errorCode(resp)}
	}

	var started struct {
		ID string `json:"id"`
	}

	err = json.Unmarshal(resp.body, &started)
	if err != nil || started.ID == "" {
		return internalError(fmt.Errorf("the API accepted the run request but named no run (HTTP %d, %d-byte body)", resp.status, len(resp.body)))
	}

	return served{result: resp.toolResult(), outcome: auditOutcomeOK, touched: affected{runID: started.ID}}
}

func (t tools) serveCancelRun(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	in, err := decodeArguments[runInput](schema, arguments)
	if err != nil {
		return invalidArguments(toolCancelRun, err, "Pass run_id, the id list_runs or start_run returned.")
	}

	touched := affected{runID: in.RunID}

	segment, ok := pathSegment(in.RunID)
	if !ok {
		return served{result: runNotFound(in.RunID).result(), outcome: errorCodeNotFound, touched: touched}
	}

	resp, err := t.api.postJSON(ctx, "/api/v1/runs/"+segment+"/cancel", nil)
	if err != nil {
		return internalError(err)
	}

	outcome := auditOutcomeOK
	if !resp.ok() {
		outcome = errorCode(resp)
	}

	return served{result: resp.toolResult(), outcome: outcome, touched: touched}
}
