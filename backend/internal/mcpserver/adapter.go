package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aconiq/backend/internal/jsonio"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// clientName is the X-Aconiq-Client value the adapter sends on writes. The API
// requires the header on every state-changing request and only checks that it
// is present; naming the caller makes a server log line attributable.
const clientName = "aconiq-mcp"

// loopbackHost is the Host the adapter's requests carry. The API's security
// middleware admits loopback hosts only, and an in-process request comes from
// nowhere else.
const loopbackHost = "127.0.0.1"

// apiClient serves one request at a time through the httpv1 handler, in
// process, and returns what the handler wrote. It is the whole of the MCP
// layer's contact with the backend: request validation, body caps, the
// evidence-tier gate and the error envelope stay the API's.
type apiClient struct {
	handler http.Handler
}

// apiResponse is what the handler wrote: the status, the media type it
// declared, and the body, byte for byte.
type apiResponse struct {
	status      int
	contentType string
	body        []byte
}

func (r apiResponse) ok() bool {
	return r.status >= http.StatusOK && r.status < http.StatusMultipleChoices
}

// toolResult hands the API's answer to the agent unchanged: a 2xx body as the
// result, anything else as a tool error whose text is the API's error envelope
// — code, message, details and hint — exactly as the API wrote it. Re-encoding
// either would make the agent read a summary instead of the record.
func (r apiResponse) toolResult() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(r.body)}},
		IsError: !r.ok(),
	}
}

func (c apiClient) get(ctx context.Context, path string) (apiResponse, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c apiClient) postJSON(ctx context.Context, path string, body []byte) (apiResponse, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

func (c apiClient) do(ctx context.Context, method string, path string, body []byte) (apiResponse, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, "http://"+loopbackHost+path, reader)
	if err != nil {
		return apiResponse{}, fmt.Errorf("build %s %s: %w", method, path, err)
	}

	req.Host = loopbackHost

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if method != http.MethodGet {
		req.Header.Set("X-Aconiq-Client", clientName)
	}

	recorder := &responseRecorder{header: http.Header{}}
	c.handler.ServeHTTP(recorder, req)

	return apiResponse{
		status:      recorder.statusOrOK(),
		contentType: recorder.header.Get("Content-Type"),
		body:        recorder.body.Bytes(),
	}, nil
}

// responseRecorder is the http.ResponseWriter an in-process request writes
// into. httptest.ResponseRecorder would do, but it is a test helper and this
// is the production path.
type responseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	n, err := r.body.Write(p)
	if err != nil {
		return n, fmt.Errorf("buffer response: %w", err)
	}

	return n, nil
}

func (r *responseRecorder) statusOrOK() int {
	if r.status == 0 {
		return http.StatusOK
	}

	return r.status
}

// toolError is a refusal the MCP layer makes itself, before any request: the
// only rules it holds, the write gate's confirm among them. It is written in
// the API's envelope so an agent reads one shape whichever layer refused.
type toolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e toolError) result() *mcp.CallToolResult {
	encoded, err := jsonio.Marshal(struct {
		Error toolError `json:"error"`
	}{Error: e})
	if err != nil {
		// A struct of three strings always encodes; this keeps the refusal a
		// refusal even if it did not.
		encoded = []byte(`{"error":{"code":"` + e.Code + `"}}` + "\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		IsError: true,
	}
}
