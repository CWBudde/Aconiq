package mcpserver_test

import (
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolContract is what a tool promises an agent before any call: its name,
// its hints and the arguments it accepts.
type toolContract struct {
	Name        string               `json:"name"`
	Annotations *mcp.ToolAnnotations `json:"annotations,omitempty"`
	InputSchema any                  `json:"input_schema"`
}

// The tool list is the contract an agent plans with (ADR-002), so every
// listed tool's schema is pinned: required fields, additionalProperties
// false, and the minLength that keeps a method field from being empty. A
// change to any of them is a reviewed diff of the golden, not a side effect.
func TestToolSchemasAreTheContract(t *testing.T) {
	t.Parallel()

	for _, server := range []struct {
		name        string
		allowWrites bool
	}{{"read-only", false}, {"allow-writes", true}} {
		t.Run(server.name, func(t *testing.T) {
			t.Parallel()

			listed, err := newFixture(t, server.allowWrites).session.ListTools(t.Context(), nil)
			if err != nil {
				t.Fatalf("list tools: %v", err)
			}

			contracts := make([]toolContract, 0, len(listed.Tools))
			for _, tool := range listed.Tools {
				contracts = append(contracts, toolContract{Name: tool.Name, Annotations: tool.Annotations, InputSchema: tool.InputSchema})
			}

			slices.SortFunc(contracts, func(a, b toolContract) int { return strings.Compare(a.Name, b.Name) })

			golden.AssertJSONSnapshot(t, filepath.Join("testdata", "tools-"+server.name+".golden.json"), contracts)
		})
	}
}

// with returns a copy of args with key set to value.
func with(args map[string]any, key string, value any) map[string]any {
	out := maps.Clone(args)
	out[key] = value

	return out
}

// without returns a copy of args with key left out.
func without(args map[string]any, key string) map[string]any {
	out := maps.Clone(args)
	delete(out, key)

	return out
}

// argumentlessTools take no arguments at all.
var argumentlessTools = []string{"list_runs", "list_standards", "project_status"}

// auditedTools are registered with the low-level AddTool so that a refused
// call is still audited; they refuse with the layer's invalid_arguments
// envelope. The SDK refuses the arguments of every other tool itself.
var auditedTools = []string{"cancel_run", "save_model", "start_run"}

// refusedArguments are, per tool that takes arguments, argument sets its
// input schema does not accept: a required field missing, a field of the
// wrong type, a property the tool does not have, and an empty method field.
func refusedArguments() map[string]map[string]map[string]any {
	runID := map[string]any{"run_id": "run-1"}
	runRequest := runRequestArgs("dummy-freefield")
	transform := map[string]any{"source_crs": "EPSG:4326", "coordinates": []float64{9.73, 52.37}}
	save := saveArgs(validModel, "none")

	runIDRefusals := map[string]map[string]any{
		"missing run_id":     {},
		"numeric run_id":     with(runID, "run_id", 7),
		"run_id as an array": with(runID, "run_id", []string{"run-1"}),
		"unknown property":   with(runID, "path", "/etc/passwd"),
	}

	// cancel_run holds its run id non-empty, as the method fields are; the
	// read tools answer an empty one as the API's not_found.
	cancelRefusals := maps.Clone(runIDRefusals)
	cancelRefusals["empty run_id"] = map[string]any{"run_id": ""}

	runRequestRefusals := map[string]map[string]any{
		"missing standard_id":      without(runRequest, "standard_id"),
		"missing receiver_mode":    without(runRequest, "receiver_mode"),
		"empty standard_version":   with(runRequest, "standard_version", ""),
		"empty scenario_id":        with(runRequest, "scenario_id", ""),
		"experimental as a string": with(runRequest, "experimental", "yes"),
		"numeric parameter value":  with(runRequest, "params", map[string]any{"grid_resolution_m": 10}),
		"a model path":             with(runRequest, "model_path", "/tmp/model.geojson"),
	}

	return map[string]map[string]map[string]any{
		"get_model": {
			"numeric crs":      {"crs": 4326},
			"unknown property": {"path": "/etc/passwd"},
		},
		"transform": {
			"missing source_crs":     without(transform, "source_crs"),
			"missing coordinates":    without(transform, "coordinates"),
			"coordinates as text":    with(transform, "coordinates", "9.73,52.37"),
			"a non-numeric ordinate": with(transform, "coordinates", []any{9.73, "52.37"}),
			"unknown property":       with(transform, "input_path", "/tmp/points.csv"),
		},
		"get_run":         runIDRefusals,
		"get_run_log":     runIDRefusals,
		"get_run_results": runIDRefusals,
		"get_contours": {
			"missing run_id":   {"interval": 5},
			"interval as text": with(runID, "interval", "5"),
			"numeric crs":      with(runID, "crs", 4326),
			"unknown property": with(runID, "output", "/tmp/contours.geojson"),
		},
		"estimate_run": runRequestRefusals,
		"start_run":    runRequestRefusals,
		"cancel_run":   cancelRefusals,
		"save_model": {
			"missing model":    without(save, "model"),
			"missing confirm":  without(save, "confirm"),
			"model as text":    with(save, "model", validModel),
			"numeric confirm":  with(save, "confirm", 0),
			"unknown property": with(save, "path", "/tmp/model.geojson"),
		},
	}
}

// Every tool refuses what its schema does not accept, before the API sees
// the call: nothing is read, nothing is written and no run is started. The
// state-changing tools refuse with invalid_arguments and audit each refusal;
// the read tools refuse through the SDK and audit nothing.
func TestEveryToolRefusesArgumentsItsSchemaDoesNot(t *testing.T) {
	t.Parallel()

	refusals := refusedArguments()

	withArguments := slices.DeleteFunc(slices.Sorted(maps.Keys(listTools(t, newFixture(t, true).session))),
		func(name string) bool { return slices.Contains(argumentlessTools, name) })
	if covered := slices.Sorted(maps.Keys(refusals)); !slices.Equal(covered, withArguments) {
		t.Fatalf("refusals cover %v; the tools that take arguments are %v", covered, withArguments)
	}

	for tool, cases := range refusals {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			var apiCalls atomic.Int64

			f := newFixtureWrapped(t, true, func(api http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					apiCalls.Add(1)
					api.ServeHTTP(w, r)
				})
			})
			before := f.modelFiles(t)

			for name, args := range cases {
				assertSchemaRefusal(t, tool, name, f.call(t, tool, args))
			}

			if n := apiCalls.Load(); n != 0 {
				t.Fatalf("refused %s calls reached the API %d times", tool, n)
			}

			f.assertUnchanged(t, before)

			assertRefusalsAudited(t, tool, len(cases), auditLines(t, f.store))
		})
	}
}

func assertSchemaRefusal(t *testing.T, tool string, name string, result *mcp.CallToolResult) {
	t.Helper()

	if slices.Contains(auditedTools, tool) {
		if apiErr := apiErrorOf(t, result); apiErr["code"] != "invalid_arguments" {
			t.Fatalf("%s, %s: envelope = %v", tool, name, apiErr)
		}

		return
	}

	if !result.IsError || !strings.Contains(resultText(t, result), `validating "arguments"`) {
		t.Fatalf("%s, %s: not refused by the schema: %s", tool, name, resultText(t, result))
	}
}

func assertRefusalsAudited(t *testing.T, tool string, calls int, lines []map[string]any) {
	t.Helper()

	if !slices.Contains(auditedTools, tool) {
		if len(lines) != 0 {
			t.Fatalf("refused %s calls were audited: %v", tool, lines)
		}

		return
	}

	if len(lines) != calls {
		t.Fatalf("%d refused %s calls left %d audit lines", calls, tool, len(lines))
	}

	for _, line := range lines {
		if line["tool"] != tool || line["outcome"] != "invalid_arguments" {
			t.Fatalf("audit line = %v", line)
		}
	}
}

// The tools that take no arguments accept none, given either way, and
// refuse a property they do not have rather than ignoring it.
func TestArgumentlessToolsAcceptNoArguments(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	for _, tool := range argumentlessTools {
		for _, args := range []any{nil, map[string]any{}} {
			if result := f.call(t, tool, args); result.IsError {
				t.Fatalf("%s(%v): %s", tool, args, resultText(t, result))
			}
		}

		assertSchemaRefusal(t, tool, "unknown property", f.call(t, tool, map[string]any{"x": 1}))
	}
}
