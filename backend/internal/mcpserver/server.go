// Package mcpserver is the Model Context Protocol face of the local API, as
// docs/adr/002-mcp-server-boundaries.md specifies it. Each tool is a thin
// adapter: it serves one request through the httpv1 handler in process and
// hands the answer back verbatim. The package holds no rule of its own about
// models, runs or standards — only what MCP needs and HTTP does not have: the
// write gate, the confirm check and the audit log.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool names. They are the contract an agent plans with, and ADR-002 fixes
// them.
const (
	toolProjectStatus = "project_status"
	toolListStandards = "list_standards"
	toolGetModel      = "get_model"
	toolTransform     = "transform"
	toolSaveModel     = "save_model"
	toolListRuns      = "list_runs"
	toolGetRun        = "get_run"
	toolGetRunLog     = "get_run_log"
	toolGetRunResults = "get_run_results"
	toolGetContours   = "get_contours"
	toolEstimateRun   = "estimate_run"
	toolStartRun      = "start_run"
	toolCancelRun     = "cancel_run"
	// toolProposeWindows is offered only to a server started with a photo
	// transmission decision (Options.PhotoTransmission).
	toolProposeWindows = "propose_windows"
)

// The MCP layer's own refusals, beside the API's codes.
const (
	// errorCodeConfirmRequired refuses a destructive call that names no state
	// it read.
	errorCodeConfirmRequired = "confirm_required"
	// errorCodeInvalidArguments refuses arguments the tool's input schema does
	// not accept.
	errorCodeInvalidArguments = "invalid_arguments"
	// errorCodeAuditUnavailable refuses a state-changing call whose audit
	// entry could not be opened, so that no write happens unaudited.
	errorCodeAuditUnavailable = "audit_unavailable"
)

// Options configures the server.
type Options struct {
	// AllowWrites registers the state-changing and destructive tools. Without
	// it they are not listed at all, so an agent cannot plan around a tool it
	// may not call.
	AllowWrites bool
	// Version is the server version announced at initialization.
	Version string
	// AuditPath is the JSONL file every state-changing call is appended to.
	// Required when AllowWrites is set.
	AuditPath string
	// PhotoTransmission is the reference to the per-engagement decision under
	// which a site photo may be sent to the model that reads windows off it
	// (P18 D2; docs/policies/data-handling.md). It is the person starting the
	// server who records it, never the agent: with it, and only with it and
	// AllowWrites, propose_windows is offered, every proposal's evidence is
	// stamped with it, and every propose_windows audit line names it.
	PhotoTransmission string
	// Clock stamps audit entries. Nil falls back to time.Now.
	Clock func() time.Time
	// Logger receives what no tool result can carry on its own, a failed
	// audit write. Nil falls back to slog.Default().
	Logger *slog.Logger
}

// New builds the MCP server over handler, which must be the handler
// `aconiq serve` mounts (httpv1.NewServeHandler): it carries the standards
// registry and the run supervisor the tools rely on.
func New(handler http.Handler, opts Options) (*mcp.Server, error) {
	err := checkOptions(opts)
	if err != nil {
		return nil, err
	}

	proposals := opts.AllowWrites && opts.PhotoTransmission != ""

	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "aconiq", Version: opts.Version},
		&mcp.ServerOptions{Instructions: instructions(opts.AllowWrites, proposals)},
	)

	t := tools{
		api:               apiClient{handler: handler},
		audit:             &auditLog{path: opts.AuditPath, clock: clock},
		logger:            logger,
		photoTransmission: strings.TrimSpace(opts.PhotoTransmission),
	}

	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)}

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolProjectStatus,
		Description: "The project's status: name, CRS, scenario and run counts, the open runs with their progress, and the hashes of the saved model and terrain. The model hash is what save_model's confirm takes.",
		Annotations: readOnly,
	}, t.projectStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name: toolListStandards,
		Description: "The standards this server can run, with each one's evidence tier (normative, preview, scaffold or test-fixture), versions, profiles, supported source types and indicators, and parameters — each parameter with its kind, unit, default, enum and bounds. " +
			"A scaffold-tier standard is not an implementation of the method its name suggests, and start_run refuses it without experimental: true.",
		Annotations: readOnly,
	}, t.listStandards)

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolGetModel,
		Description: "The saved model — every source, building, barrier, receiver and calc-area feature — as a GeoJSON FeatureCollection, with the CRS it is expressed in (crs), the project CRS (project_crs), its feature count and its hash. Heights are metres (height_m).",
		Annotations: readOnly,
	}, t.getModel)

	mcp.AddTool(server, &mcp.Tool{
		Name: toolTransform,
		Description: "Project a batch of coordinates between two CRS; this is the only projection to use. " +
			"target_crs \"auto\" (or omitted) picks the ETRS89 / UTM zone `aconiq run` would compute in, and refuses a site no supported zone covers in the same words. " +
			"The answer names the CRS the coordinates are in (target_crs) and whether anything moved (applied). It reads and changes nothing in the project.",
		Annotations: readOnly,
	}, t.transform)

	err = t.addRunReadTools(server, readOnly)
	if err != nil {
		return nil, err
	}

	addResources(server, t.api)

	if !opts.AllowWrites {
		return server, nil
	}

	err = t.addWriteTools(server, proposals)
	if err != nil {
		return nil, err
	}

	return server, nil
}

// checkOptions refuses options New cannot honour: writes without an audit
// log, and a photo transmission decision without writes or not spelled as a
// reference.
func checkOptions(opts Options) error {
	if opts.AllowWrites && opts.AuditPath == "" {
		return errors.New("mcpserver: writes need an audit log path")
	}

	if opts.PhotoTransmission == "" {
		return nil
	}

	if !opts.AllowWrites {
		return errors.New("mcpserver: a photo transmission decision only applies with writes allowed")
	}

	err := ValidatePhotoTransmission(opts.PhotoTransmission)
	if err != nil {
		return fmt.Errorf("mcpserver: %w", err)
	}

	return nil
}

// addWriteTools registers the tools --allow-writes offers, and
// propose_windows when the server was started with a photo transmission
// decision.
func (t tools) addWriteTools(server *mcp.Server, proposals bool) error {
	schema, resolved, err := inputSchema[saveModelInput](rawModelAsObject())
	if err != nil {
		return err
	}

	// The low-level AddTool, not the generic one: mcp.AddTool validates the
	// arguments and refuses a bad call before any handler runs, and that
	// refusal of a destructive call would never reach the audit log.
	// serveSaveModel validates against the same schema itself.
	server.AddTool(&mcp.Tool{
		Name: toolSaveModel,
		Description: "Replace the whole project model. To change one feature, read the model with get_model, change it, and save it back. " +
			"The model is validated against the GeoJSON schema v1 and refused with the validation issues if it does not pass; nothing is corrected. " +
			"confirm binds the save to the model you read: the hash get_model or project_status returned, or \"none\" when the project has no model yet. " +
			"A model that changed since is refused with hash_mismatch and the current hash, not overwritten.",
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)},
	}, t.audited(toolSaveModel, func(ctx context.Context, arguments json.RawMessage) served {
		return t.serveSaveModel(ctx, resolved, arguments)
	}))

	err = t.addRunWriteTools(server)
	if err != nil {
		return err
	}

	if !proposals {
		return nil
	}

	return t.addProposeWindows(server)
}

func instructions(allowWrites, proposals bool) string {
	const common = "Aconiq environmental noise modelling, for the one project this server was started on. " +
		"Tool results are the local API's JSON answers, verbatim; a refused call returns the API's error envelope (code, message, details, hint). " +
		"Coordinates are in the CRS named beside them. " +
		"Runs: list_runs, get_run (poll it for progress; there is no stream), get_run_log, get_run_results (the receiver table, with its units), get_contours, and estimate_run, which prices a run without starting it. " +
		"Resources: aconiq://project (status), aconiq://standards (the catalogue, with evidence tiers and parameter units), " +
		"aconiq://runs/{id} (a run's summary and progress), aconiq://runs/{id}/provenance (its provenance.json, as recorded) " +
		"and aconiq://artifacts/{id} (an artifact's content, by the id a run's artifacts list names)."

	if !allowWrites {
		return common + " This server was started read-only (without --allow-writes): no tool changes the project, and none is listed."
	}

	writes := common + " Writes are enabled (--allow-writes): start_run and cancel_run, and save_model. Every state-changing call is recorded in the project's MCP audit log. " +
		"Destructive tools require confirm, a value you must have read first. Runs this server started are stopped when it exits."

	if !proposals {
		return writes
	}

	return writes + " propose_windows adds windows you read off a facade image as proposals for a person to review; " +
		"this server was started under a recorded decision that such images may be sent to the model reading them, and it stamps that decision on every proposal."
}

type tools struct {
	api    apiClient
	audit  *auditLog
	logger *slog.Logger
	// photoTransmission is Options.PhotoTransmission, trimmed.
	photoTransmission string
}

type projectStatusInput struct{}

type getModelInput struct {
	CRS string `json:"crs,omitempty" jsonschema:"EPSG identifier to answer in, such as EPSG:4326. Omitted, the model is returned in the project CRS, byte-identical to what is stored."`
}

type listStandardsInput struct{}

// transformInput carries the fields of the API's transform request, which
// judges them: an odd count, an unknown CRS or an unprojectable site is the
// API's refusal, not this layer's.
type transformInput struct {
	SourceCRS   string    `json:"source_crs"           jsonschema:"EPSG identifier the coordinates are in, such as EPSG:4326 or EPSG:25832."`
	TargetCRS   string    `json:"target_crs,omitempty" jsonschema:"EPSG identifier to project into, or auto (the default) for the zone aconiq run would compute in."`
	Coordinates []float64 `json:"coordinates"          jsonschema:"Flat and interleaved: x0, y0, x1, y1, … — longitude before latitude in a geographic CRS."`
}

type saveModelInput struct {
	Model   json.RawMessage `json:"model"         jsonschema:"The whole model as a GeoJSON FeatureCollection whose features carry kind = source, building, barrier, receiver or calc-area."`
	CRS     string          `json:"crs,omitempty" jsonschema:"EPSG identifier of the model's coordinates. Omitted, they are taken to be in the project CRS already."`
	Confirm string          `json:"confirm"       jsonschema:"The hash of the model being replaced, as get_model or project_status returned it, or none when the project has no model yet."`
}

// rawModelAsObject types save_model's model as the object it is. It is held
// as json.RawMessage so it reaches the API byte for byte, and inference would
// type those bytes as an array. A fresh map per call: the schema it holds ends
// up inside the tool's schema, and every server resolves its own.
func rawModelAsObject() map[reflect.Type]*jsonschema.Schema {
	return map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[json.RawMessage](): {Type: "object"},
	}
}

// inputSchema derives a tool's input schema from In, with typeSchemas
// overriding inference for the types it names and each nonEmpty property held
// to at least one character: required only says a property is present, and
// an empty identifier would read as an omitted one. It returns the schema the
// tool advertises and its resolved form, which a low-level handler validates
// against.
func inputSchema[In any](typeSchemas map[reflect.Type]*jsonschema.Schema, nonEmpty ...string) (*jsonschema.Schema, *jsonschema.Resolved, error) {
	name := reflect.TypeFor[In]().Name()

	schema, err := jsonschema.For[In](&jsonschema.ForOptions{TypeSchemas: typeSchemas})
	if err != nil {
		return nil, nil, fmt.Errorf("mcpserver: %s schema: %w", name, err)
	}

	for _, property := range nonEmpty {
		field, ok := schema.Properties[property]
		if !ok {
			return nil, nil, fmt.Errorf("mcpserver: %s has no property %q", name, property)
		}

		minLength := 1
		field.MinLength = &minLength
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return nil, nil, fmt.Errorf("mcpserver: resolve %s schema: %w", name, err)
	}

	return schema, resolved, nil
}

func (t tools) projectStatus(ctx context.Context, _ *mcp.CallToolRequest, _ projectStatusInput) (*mcp.CallToolResult, any, error) {
	resp, err := t.api.get(ctx, "/api/v1/project/status")
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

func (t tools) listStandards(ctx context.Context, _ *mcp.CallToolRequest, _ listStandardsInput) (*mcp.CallToolResult, any, error) {
	resp, err := t.api.get(ctx, "/api/v1/standards")
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

func (t tools) transform(ctx context.Context, _ *mcp.CallToolRequest, in transformInput) (*mcp.CallToolResult, any, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("encode transform request: %w", err)
	}

	resp, err := t.api.postJSON(ctx, "/api/v1/transform", body)
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

func (t tools) getModel(ctx context.Context, _ *mcp.CallToolRequest, in getModelInput) (*mcp.CallToolResult, any, error) {
	path := "/api/v1/model"
	if in.CRS != "" {
		path += "?crs=" + url.QueryEscape(in.CRS)
	}

	resp, err := t.api.get(ctx, path)
	if err != nil {
		return nil, nil, err
	}

	return resp.toolResult(), nil, nil
}

// modelSaveBody is the body of POST /api/v1/model; confirm travels as the
// API's expected_hash, so the compare is the store's, atomic with the write.
type modelSaveBody struct {
	CRS          string          `json:"crs,omitempty"`
	Model        json.RawMessage `json:"model"`
	ExpectedHash string          `json:"expected_hash"`
}

// served is a state-changing call's result, with what its audit entry
// records of it.
type served struct {
	result  *mcp.CallToolResult
	outcome string
	touched affected
}

// audited returns the handler of a state-changing tool. Every call it
// receives is audited, whatever its outcome — refused arguments included —
// and none is served unless its audit entry could be opened first.
func (t tools) audited(tool string, serve func(context.Context, json.RawMessage) served) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.Params.Arguments

		entry, err := t.audit.open()
		if err != nil {
			t.logger.Error("mcp audit log unavailable; write refused", "tool", tool, "error", err)

			return toolError{
				Code:    errorCodeAuditUnavailable,
				Message: "the MCP audit log cannot be written, so " + tool + " was not attempted: " + err.Error(),
				Hint:    "Make .noise/logs/ in the project writable; every write through this server is recorded there.",
			}.result(), nil
		}

		return t.recorded(tool, entry, arguments, serve(ctx, arguments)), nil
	}
}

// invalidArguments refuses arguments the tool's input schema does not accept.
func invalidArguments(tool string, err error, hint string) served {
	refusal := toolError{
		Code:    errorCodeInvalidArguments,
		Message: tool + " arguments do not match its input schema: " + err.Error(),
		Hint:    hint,
	}

	return served{result: refusal.result(), outcome: errorCodeInvalidArguments}
}

// internalError is a call this layer could not serve at all.
func internalError(err error) served {
	return served{result: toolError{Code: "internal_error", Message: err.Error()}.result(), outcome: "internal_error"}
}

// serveSaveModel validates and serves one save_model call.
func (t tools) serveSaveModel(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	in, err := decodeArguments[saveModelInput](schema, arguments)
	if err != nil {
		return invalidArguments(toolSaveModel, err, "Pass model (a GeoJSON FeatureCollection object) and confirm, and optionally crs; nothing else.")
	}

	return t.postConfirmed(ctx, "/api/v1/model", in.Confirm, toolError{
		Code:    errorCodeConfirmRequired,
		Message: "save_model replaces the whole model and needs confirm",
		Hint:    "Pass the hash get_model or project_status returned for the current model, or \"none\" if the project has no model yet.",
	}, modelSaveBody{CRS: in.CRS, Model: in.Model, ExpectedHash: in.Confirm})
}

// postConfirmed serves a write bound by confirm to the model the agent read:
// it posts body to the API and records the model the write left. An empty
// confirm is the one value that would turn the API's compare-and-write back
// into an unconditional write, so it is refused here with refusal; anything
// else is the API's to judge: a malformed hash is refused there, not
// corrected here.
func (t tools) postConfirmed(ctx context.Context, path, confirm string, refusal toolError, body any) served {
	if confirm == "" {
		return served{result: refusal.result(), outcome: errorCodeConfirmRequired}
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return internalError(fmt.Errorf("encode %s request: %w", path, err))
	}

	resp, err := t.api.postJSON(ctx, path, encoded)
	if err != nil {
		return internalError(err)
	}

	outcome, modelHash := saveOutcome(resp)

	return served{result: resp.toolResult(), outcome: outcome, touched: affected{modelHash: modelHash}}
}

// decodeArguments validates the arguments against the tool's input schema —
// the check mcp.AddTool would have made before the handler — and decodes
// them.
func decodeArguments[In any](schema *jsonschema.Resolved, arguments json.RawMessage) (In, error) {
	var in In

	raw := arguments
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}

	var instance any

	err := json.Unmarshal(raw, &instance)
	if err != nil {
		return in, fmt.Errorf("decode arguments: %w", err)
	}

	err = schema.Validate(instance)
	if err != nil {
		return in, fmt.Errorf("validate arguments: %w", err)
	}

	err = json.Unmarshal(raw, &in)
	if err != nil {
		return in, fmt.Errorf("decode arguments: %w", err)
	}

	return in, nil
}

// saveOutcome reads what the audit log records of a model save: "ok" and the
// hash of the model now in place, or the code the API refused with.
func saveOutcome(resp apiResponse) (string, string) {
	if resp.ok() {
		var saved struct {
			Hash string `json:"hash"`
		}

		_ = json.Unmarshal(resp.body, &saved)

		return auditOutcomeOK, saved.Hash
	}

	return errorCode(resp), ""
}

// errorCode is the code of the API's error envelope, or the bare status when
// the body carries none.
func errorCode(resp apiResponse) string {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}

	if json.Unmarshal(resp.body, &envelope) == nil && envelope.Error.Code != "" {
		return envelope.Error.Code
	}

	return fmt.Sprintf("http_%d", resp.status)
}

// recorded writes a state-changing call's audit entry and returns its
// result. The call has already been served by then, so an entry that fails to
// write neither undoes nor hides it: it is logged at ERROR, and the agent is
// told beside the result.
func (t tools) recorded(tool string, entry *pendingEntry, arguments json.RawMessage, out served) *mcp.CallToolResult {
	err := entry.finish(tool, arguments, out.outcome, out.touched)
	if err == nil {
		return out.result
	}

	t.logger.Error("mcp audit entry not written", "tool", tool, "outcome", out.outcome, "error", err)

	out.result.Content = append(out.result.Content, &mcp.TextContent{
		Text: "warning: this call was not recorded in the MCP audit log: " + err.Error(),
	})

	return out.result
}

// boolPtr is for the annotation hints MCP leaves tri-state, where an absent
// hint means the spec's default rather than false.
func boolPtr(v bool) *bool {
	return &v
}
