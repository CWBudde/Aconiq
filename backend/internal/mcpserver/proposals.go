package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxPhotoTransmissionLength bounds the --photo-transmission reference: an
// engagement's decision record is named in a few words, not pasted in.
const maxPhotoTransmissionLength = 200

// ValidatePhotoTransmission checks the reference to the per-engagement
// decision a server offering propose_windows is started under (P18 D2): a
// short, opaque reference, never a path, which could name the client
// (docs/policies/data-handling.md).
func ValidatePhotoTransmission(reference string) error {
	trimmed := strings.TrimSpace(reference)

	switch {
	case trimmed == "":
		return errors.New("the photo transmission reference is empty")
	case len(trimmed) > maxPhotoTransmissionLength:
		return fmt.Errorf("the photo transmission reference is longer than %d characters; name the decision, do not paste it", maxPhotoTransmissionLength)
	case strings.ContainsAny(trimmed, `/\`):
		return errors.New("the photo transmission reference must not be a path: name the decision in the engagement's records, not a file")
	}

	return nil
}

// proposeWindowsInput is what propose_windows takes. A proposal has no
// sensitive and no review_status: the API writes it proposed and not
// sensitive, and the schema refuses a proposal that claims either, because
// only a person confirms a window or marks its room schutzbedürftig.
type proposeWindowsInput struct {
	Windows []windowProposal `json:"windows" jsonschema:"The windows read off the image, each on one canonical facade of a building."`
	Confirm string           `json:"confirm" jsonschema:"The hash of the saved model the windows were placed against, as get_model or project_status returned it."`
}

type windowProposal struct {
	ID          string           `json:"id"           jsonschema:"A new window feature id, without '/'. An id the model already holds is refused."`
	BuildingRef string           `json:"building_ref" jsonschema:"The building's feature id."`
	Part        int              `json:"part"         jsonschema:"The canonical part index of the facade's edge."`
	Ring        int              `json:"ring"         jsonschema:"The canonical ring index: 0 the exterior, above 0 a courtyard."`
	Edge        int              `json:"edge"         jsonschema:"The canonical edge index within the ring."`
	Window      windowFrame      `json:"window"       jsonschema:"The window's rectangle in the facade's frame, in metres."`
	Evidence    []evidenceSource `json:"evidence"     jsonschema:"What the window was read from: at least one source, the image first."`
}

type windowFrame struct {
	UM      float64 `json:"u_m"      jsonschema:"Distance along the facade from its start to the window's left edge, seen from outside."`
	SillZM  float64 `json:"sill_z_m" jsonschema:"Height of the sill above the ground."`
	WidthM  float64 `json:"width_m"  jsonschema:"The window's width."`
	HeightM float64 `json:"height_m" jsonschema:"The window's height."`
}

type evidenceSource struct {
	Kind          string `json:"kind"                  jsonschema:"The kind of source. There is no Google kind: nothing seen in Google imagery is recorded."`
	SourceID      string `json:"source_id"             jsonschema:"The image's opaque reference in the engagement's store, never a path: no slash or backslash."`
	ContentSHA256 string `json:"content_sha256"        jsonschema:"The SHA-256 of the image file as 64 lowercase hex digits."`
	CapturedAt    string `json:"captured_at,omitempty" jsonschema:"When the image was taken."`
	Licence       string `json:"licence,omitempty"     jsonschema:"The source's licence, such as own."`
	Attribution   string `json:"attribution,omitempty" jsonschema:"Whom the source must be credited to."`
}

// evidenceKinds mirrors facadeset.EvidenceKinds, which the API holds a
// proposal to; the schema states it so an agent plans with it.
var evidenceKinds = []any{"orthophoto", "oblique", "street-level", "site-photo", "plan", "citygml-lod3"}

// proposeWindowsSchema derives propose_windows's input schema and tightens
// what inference cannot say: at least one window, at least one evidence
// source per window, a known kind, a non-empty id and reference, and a hash
// in its one spelling.
func proposeWindowsSchema() (*jsonschema.Schema, *jsonschema.Resolved, error) {
	schema, err := jsonschema.For[proposeWindowsInput](nil)
	if err != nil {
		return nil, nil, fmt.Errorf("mcpserver: proposeWindowsInput schema: %w", err)
	}

	one := 1
	windows := schema.Properties["windows"]
	windows.MinItems = &one

	window := windows.Items.Properties
	window["id"].MinLength = &one
	window["building_ref"].MinLength = &one

	evidence := window["evidence"]
	evidence.MinItems = &one

	source := evidence.Items.Properties
	source["kind"].Enum = evidenceKinds
	source["source_id"].MinLength = &one
	source["source_id"].Pattern = `^[^/\\]+$`
	source["content_sha256"].Pattern = "^[0-9a-f]{64}$"

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return nil, nil, fmt.Errorf("mcpserver: resolve proposeWindowsInput schema: %w", err)
	}

	return schema, resolved, nil
}

// proposeWindowsBody is the body of POST /api/v1/facades/windows: confirm
// travels as expected_hash, and the server's reference as transmission_basis.
type proposeWindowsBody struct {
	ExpectedHash      string           `json:"expected_hash"`
	TransmissionBasis string           `json:"transmission_basis"`
	Windows           []windowProposal `json:"windows"`
}

// addProposeWindows registers propose_windows, low-level for the reason
// save_model is: a refused call is audited too. It only adds windows, so it
// is annotated additive, not destructive; it still takes confirm, because
// the windows are placed on the model the agent read.
func (t tools) addProposeWindows(server *mcp.Server) error {
	schema, resolved, err := proposeWindowsSchema()
	if err != nil {
		return err
	}

	server.AddTool(&mcp.Tool{
		Name: toolProposeWindows,
		Description: "Add windows you read off an image of a facade to the saved model, as proposals a person reviews in the facade elevation. " +
			"Each window is a rectangle in one canonical facade's frame (get_model's buildings; part, ring and edge as the canonical footprint numbers them), " +
			"and is written proposed and not sensitive: confirming a window, or marking its room schutzbedürftig, is a person's act and cannot be proposed. " +
			"Each needs evidence naming the image it was read from — kind, source_id and the image's content_sha256 — and the server stamps every entry with the photo transmission decision it was started under. " +
			"Only new ids are added; a window the model holds is never replaced. confirm binds the write to the model you read: a model that changed since is refused with hash_mismatch. " +
			"The image itself never passes through this tool: only the rectangles and the evidence record reach the project.",
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, t.audited(toolProposeWindows, func(ctx context.Context, arguments json.RawMessage) served {
		return t.serveProposeWindows(ctx, resolved, arguments)
	}))

	return nil
}

// serveProposeWindows validates and serves one propose_windows call. Every
// audit line it writes names the transmission decision, refusals included.
func (t tools) serveProposeWindows(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	out := t.proposeWindows(ctx, schema, arguments)
	out.touched.photoTransmission = t.photoTransmission

	return out
}

func (t tools) proposeWindows(ctx context.Context, schema *jsonschema.Resolved, arguments json.RawMessage) served {
	in, err := decodeArguments[proposeWindowsInput](schema, arguments)
	if err != nil {
		return invalidArguments(toolProposeWindows, err,
			"Pass windows (each with id, building_ref, part, ring, edge, window and evidence) and confirm; a window takes no sensitive or review_status.")
	}

	return t.postConfirmed(ctx, "/api/v1/facades/windows", in.Confirm, toolError{
		Code:    errorCodeConfirmRequired,
		Message: "propose_windows writes the model and needs confirm",
		Hint:    "Pass the hash get_model or project_status returned for the model the windows were placed against.",
	}, proposeWindowsBody{ExpectedHash: in.Confirm, TransmissionBasis: t.photoTransmission, Windows: in.Windows})
}
