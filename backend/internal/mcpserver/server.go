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
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool names. They are the contract an agent plans with, and ADR-002 fixes
// them.
const (
	toolProjectStatus = "project_status"
	toolGetModel      = "get_model"
	toolSaveModel     = "save_model"
)

// errorCodeConfirmRequired is the MCP layer's own refusal of a destructive
// call that names no state it read.
const errorCodeConfirmRequired = "confirm_required"

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
	if opts.AllowWrites && opts.AuditPath == "" {
		return nil, errors.New("mcpserver: writes need an audit log path")
	}

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
		&mcp.ServerOptions{Instructions: instructions(opts.AllowWrites)},
	)

	t := tools{
		api:    apiClient{handler: handler},
		audit:  &auditLog{path: opts.AuditPath, clock: clock},
		logger: logger,
	}

	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)}

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolProjectStatus,
		Description: "The project's status: name, CRS, scenario and run counts, the open runs with their progress, and the hashes of the saved model and terrain. The model hash is what save_model's confirm takes.",
		Annotations: readOnly,
	}, t.projectStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolGetModel,
		Description: "The saved model — every source, building, barrier, receiver and calc-area feature — as a GeoJSON FeatureCollection, with the CRS it is expressed in (crs), the project CRS (project_crs), its feature count and its hash. Heights are metres (height_m).",
		Annotations: readOnly,
	}, t.getModel)

	if opts.AllowWrites {
		schema, err := saveModelSchema()
		if err != nil {
			return nil, err
		}

		mcp.AddTool(server, &mcp.Tool{
			Name: toolSaveModel,
			Description: "Replace the whole project model. To change one feature, read the model with get_model, change it, and save it back. " +
				"The model is validated against the GeoJSON schema v1 and refused with the validation issues if it does not pass; nothing is corrected. " +
				"confirm binds the save to the model you read: the hash get_model or project_status returned, or \"none\" when the project has no model yet. " +
				"A model that changed since is refused with hash_mismatch and the current hash, not overwritten.",
			InputSchema: schema,
			Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)},
		}, t.saveModel)
	}

	return server, nil
}

func instructions(allowWrites bool) string {
	const common = "Aconiq environmental noise modelling, for the one project this server was started on. " +
		"Tool results are the local API's JSON answers, verbatim; a refused call returns the API's error envelope (code, message, details, hint). " +
		"Coordinates are in the CRS named beside them."

	if !allowWrites {
		return common + " This server was started read-only (without --allow-writes): no tool changes the project, and none is listed."
	}

	return common + " Writes are enabled (--allow-writes). Every state-changing call is recorded in the project's MCP audit log. " +
		"Destructive tools require confirm, a value you must have read first."
}

type tools struct {
	api    apiClient
	audit  *auditLog
	logger *slog.Logger
}

type projectStatusInput struct{}

type getModelInput struct {
	CRS string `json:"crs,omitempty" jsonschema:"EPSG identifier to answer in, such as EPSG:4326. Omitted, the model is returned in the project CRS, byte-identical to what is stored."`
}

type saveModelInput struct {
	Model   json.RawMessage `json:"model"         jsonschema:"The whole model as a GeoJSON FeatureCollection whose features carry kind = source, building, barrier, receiver or calc-area."`
	CRS     string          `json:"crs,omitempty" jsonschema:"EPSG identifier of the model's coordinates. Omitted, they are taken to be in the project CRS already."`
	Confirm string          `json:"confirm"       jsonschema:"The hash of the model being replaced, as get_model or project_status returned it, or none when the project has no model yet."`
}

// saveModelSchema derives save_model's input schema from saveModelInput, with
// one correction: the model is held as json.RawMessage so it reaches the API
// byte for byte, and inference would type those bytes as an array.
func saveModelSchema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[saveModelInput](&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[json.RawMessage](): {Type: "object"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mcpserver: save_model input schema: %w", err)
	}

	return schema, nil
}

func (t tools) projectStatus(ctx context.Context, _ *mcp.CallToolRequest, _ projectStatusInput) (*mcp.CallToolResult, any, error) {
	resp, err := t.api.get(ctx, "/api/v1/project/status")
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

func (t tools) saveModel(ctx context.Context, req *mcp.CallToolRequest, in saveModelInput) (*mcp.CallToolResult, any, error) {
	arguments := req.Params.Arguments

	// An empty confirm is the one value that would turn the API's
	// compare-and-write back into an unconditional replace. Anything else is
	// the API's to judge: a malformed hash is refused there, not corrected
	// here.
	if in.Confirm == "" {
		refusal := toolError{
			Code:    errorCodeConfirmRequired,
			Message: "save_model replaces the whole model and needs confirm",
			Hint:    "Pass the hash get_model or project_status returned for the current model, or \"none\" if the project has no model yet.",
		}

		return t.auditSave(arguments, refusal.result(), errorCodeConfirmRequired, ""), nil, nil
	}

	body, err := json.Marshal(modelSaveBody{CRS: in.CRS, Model: in.Model, ExpectedHash: in.Confirm})
	if err != nil {
		t.auditSave(arguments, nil, "invalid_arguments", "")

		return nil, nil, fmt.Errorf("encode model save: %w", err)
	}

	resp, err := t.api.postJSON(ctx, "/api/v1/model", body)
	if err != nil {
		t.auditSave(arguments, nil, "internal_error", "")

		return nil, nil, err
	}

	outcome, modelHash := saveOutcome(resp)

	return t.auditSave(arguments, resp.toolResult(), outcome, modelHash), nil, nil
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

// auditSave records a save_model call and returns its result. The call has
// already happened by then, so a failed audit write neither undoes nor hides
// it: it is logged, and the agent is told beside the result.
func (t tools) auditSave(arguments json.RawMessage, result *mcp.CallToolResult, outcome string, modelHash string) *mcp.CallToolResult {
	const tool = toolSaveModel

	err := t.audit.record(tool, arguments, outcome, modelHash)
	if err == nil {
		return result
	}

	t.logger.Error("mcp audit entry not written", "tool", tool, "outcome", outcome, "error", err)

	if result != nil {
		result.Content = append(result.Content, &mcp.TextContent{
			Text: "warning: this call was not recorded in the MCP audit log: " + err.Error(),
		})
	}

	return result
}

// boolPtr is for the annotation hints MCP leaves tri-state, where an absent
// hint means the spec's default rather than false.
func boolPtr(v bool) *bool {
	return &v
}
