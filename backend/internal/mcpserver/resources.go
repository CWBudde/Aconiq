package mcpserver

import (
	"context"
	"encoding/json"
	"mime"
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
	resourceArtifact       = "aconiq://artifacts/{id}"
	artifactResourcePrefix = "aconiq://artifacts/"
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
		id, ok := pathSegment(strings.TrimPrefix(uri, runResourcePrefix))

		return "/api/v1/runs/" + id, ok
	}))

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: resourceRunProvenance,
		Name:        "run provenance",
		Description: "One run's provenance.json, byte for byte as the run recorded it: standard, resolved parameters, evidence tier, project_crs and compute_crs, input hashes, standard data digests and the build that made it.",
		MIMEType:    jsonMIMEType,
	}, api.resource(func(uri string) (string, bool) {
		segment, found := strings.CutSuffix(strings.TrimPrefix(uri, runResourcePrefix), provenanceResourceTail)
		id, ok := pathSegment(segment)

		return "/api/v1/runs/" + id + provenanceResourceTail, found && ok
	}))

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: resourceArtifact,
		Name:        "artifact",
		Description: "One artifact's content by the id a run's artifacts list names, byte for byte: a receiver table (JSON or CSV), the raster sidecar (units, band names, grid) and its float64 .bin, a report. Text formats are returned as text, the others as a blob; the MIME type is the artifact's own.",
	}, api.resource(func(uri string) (string, bool) {
		id, ok := pathSegment(strings.TrimPrefix(uri, artifactResourcePrefix))

		return "/api/v1/artifacts/" + id + "/content", ok
	}))
}

// pathSegment decodes a run or artifact id — as a resource URI or a tool
// argument carries it — and re-escapes it as one path segment of the API
// route, so no spelling of it — an encoded slash, a dot segment — can make the
// read reach a different route. Whether a run or artifact of that id exists is
// the API's to answer.
func pathSegment(segment string) (string, bool) {
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
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{resp.contents(uri)}}, nil
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

// contents is a resource read's answer as the API wrote it: text for a JSON
// or text media type, the bytes as a blob for anything else, under the media
// type the API declared.
func (r apiResponse) contents(uri string) *mcp.ResourceContents {
	contents := &mcp.ResourceContents{URI: uri, MIMEType: r.contentType}

	mediaType, _, err := mime.ParseMediaType(r.contentType)
	if err != nil {
		contents.MIMEType = "application/octet-stream"
		contents.Blob = r.body

		return contents
	}

	// The API declares JSON with a charset; a resource is listed, and read,
	// under the bare type.
	if mediaType == jsonMIMEType {
		contents.MIMEType = jsonMIMEType
	}

	if isText(mediaType) {
		contents.Text = string(r.body)
	} else {
		contents.Blob = r.body
	}

	return contents
}

// isText reports whether a media type is read as text: text/*, JSON and the
// +json types such as application/geo+json.
func isText(mediaType string) bool {
	return strings.HasPrefix(mediaType, "text/") || mediaType == jsonMIMEType || strings.HasSuffix(mediaType, "+json")
}

// envelopeData is the API's error body as JSON-RPC error data, when it is
// JSON; otherwise the error carries its code alone.
func envelopeData(body []byte) json.RawMessage {
	if !json.Valid(body) {
		return nil
	}

	return json.RawMessage(body)
}
