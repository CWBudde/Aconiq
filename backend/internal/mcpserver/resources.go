package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resource URIs. ADR-002 fixes them, beside the tool names.
const (
	resourceProject        = "aconiq://project"
	resourceStandards      = "aconiq://standards"
	resourceRun            = "aconiq://runs/{id}"
	resourceRunProvenance  = "aconiq://runs/{id}/provenance"
	runResourcePrefix      = "aconiq://runs/"
	provenanceResourceTail = "/provenance"
)

const jsonMIMEType = "application/json"

// addResources registers the read-only resources. They are listed whatever
// the write gate says: they are what an agent reads before it writes.
func addResources(server *mcp.Server, api apiClient) {
	server.AddResource(&mcp.Resource{
		URI:         resourceProject,
		Name:        "project",
		Description: "The project's status, as project_status returns it: name, CRS (crs), scenario and run counts, the open runs with their progress, and the hashes of the saved model and terrain.",
		MIMEType:    jsonMIMEType,
	}, api.resource(func(string) (string, bool) { return "/api/v1/project/status", true }))

	server.AddResource(&mcp.Resource{
		URI:         resourceStandards,
		Name:        "standards",
		Description: "The standards catalogue, as list_standards returns it: every standard with its evidence tier, versions, profiles and parameters, each parameter with its unit and default.",
		MIMEType:    jsonMIMEType,
	}, api.resource(func(string) (string, bool) { return "/api/v1/standards", true }))

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: resourceRun,
		Name:        "run",
		Description: "One run's summary: status, standard, project_crs and compute_crs, output_hash, source and receiver counts, and progress while it computes.",
		MIMEType:    jsonMIMEType,
	}, api.resource(func(uri string) (string, bool) {
		id, ok := runID(strings.TrimPrefix(uri, runResourcePrefix))

		return "/api/v1/runs/" + id, ok
	}))

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: resourceRunProvenance,
		Name:        "run provenance",
		Description: "One run's provenance.json, byte for byte as the run recorded it: standard, resolved parameters, evidence tier, project_crs and compute_crs, input hashes, standard data digests and the build that made it.",
		MIMEType:    jsonMIMEType,
	}, api.resource(func(uri string) (string, bool) {
		segment, found := strings.CutSuffix(strings.TrimPrefix(uri, runResourcePrefix), provenanceResourceTail)
		id, ok := runID(segment)

		return "/api/v1/runs/" + id + provenanceResourceTail, found && ok
	}))
}

// runID decodes the run id a resource URI carries and re-escapes it as one
// path segment of the API route, so no spelling of it — an encoded slash, a
// dot segment — can make the read reach a different route. Whether a run of
// that id exists is the API's to answer.
func runID(segment string) (string, bool) {
	id, err := url.PathUnescape(segment)
	if err != nil || id == "" || id == "." || id == ".." || strings.Contains(id, "/") {
		return "", false
	}

	return url.PathEscape(id), true
}

// resource returns the handler of a resource backed by one API GET. path maps
// the requested URI to the API path, or refuses it as not found. The contents
// are the API's answer verbatim; a 404 becomes MCP's resource-not-found, and
// any other refusal an error carrying the API's envelope as written.
func (c apiClient) resource(path func(uri string) (string, bool)) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI

		apiPath, ok := path(uri)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}

		resp, err := c.get(ctx, apiPath)
		if err != nil {
			return nil, err
		}

		switch {
		case resp.ok():
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: jsonMIMEType,
				Text:     string(resp.body),
			}}}, nil
		case resp.status == http.StatusNotFound:
			return nil, mcp.ResourceNotFoundError(uri)
		default:
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInternalError,
				Message: "read " + uri + ": " + errorCode(resp),
				Data:    envelopeData(resp.body),
			}
		}
	}
}

// envelopeData is the API's error body as JSON-RPC error data, when it is
// JSON; otherwise the error carries its code alone.
func envelopeData(body []byte) json.RawMessage {
	if !json.Valid(body) {
		return nil
	}

	return json.RawMessage(body)
}
