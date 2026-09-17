package wasmkernel

import (
	"encoding/json"
	"fmt"

	"github.com/aconiq/backend/internal/geo/crstransform"
)

// AutoTargetCRS asks Transform to make the same decision `aconiq run` makes:
// project a geographic model into the ETRS89 / UTM zone its centre falls in,
// and leave a model that is already metric exactly where it is.
const AutoTargetCRS = crstransform.AutoTarget

// TransformRequest is the JSON `aconiq.transform` takes, and TransformResponse
// is what it answers with.
//
// Both are aliases rather than declarations: the wire contract belongs to
// crstransform, which POST /api/v1/transform serves from the same types, so the
// browser and the local API cannot drift apart in their shape any more than
// they can in their zone decision.
type (
	TransformRequest  = crstransform.Request
	TransformResponse = crstransform.Response
)

// Transform projects a batch of coordinates between two CRS.
//
// It is the browser's access to the transform the Go kernel already links in.
// The projection itself, and the AutoTargetCRS decision behind it, live in
// crstransform; what this function owns is the JSON boundary, because
// syscall/js hands strings across and nothing else.
func Transform(input []byte) ([]byte, error) {
	var req TransformRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	resp, err := crstransform.Transform(req)
	if err != nil {
		// Wrapped with a bare %w, which adds no text at all. That is deliberate
		// and it is the one place in this repository where it is right: the
		// refusal has to reach the browser worded exactly as `aconiq run` and
		// POST /api/v1/transform word it, so any context added here would break
		// a parity the callers rely on. Context travels on crstransform.Error's
		// Reason instead, which errors.As still reaches through this.
		return nil, fmt.Errorf("%w", err)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return out, nil
}
