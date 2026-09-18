package cli

import (
	"fmt"

	"github.com/aconiq/backend/internal/geo/terrain"
)

// newTerrainInComputeCRS wraps a terrain model when the run computes in a CRS
// the terrain is not in. It returns the model unchanged when no transform is
// needed, so the ordinary projected project keeps querying the grid directly.
//
// The wrapper itself is terrain.InComputeCRS, next to the model it adapts:
// browser mode needs the same adaptation, and `cmd/wasm/main.go` is
// `//go:build js && wasm` and therefore unreachable from `go test`. What this
// function owns is the CLI's own fact — that a project's terrain is stored in
// the project CRS, and that a run only moves it when resolveComputeModel
// projected the model.
func newTerrainInComputeCRS(model terrain.Model, projection computeProjection) (terrain.Model, error) {
	if model == nil || !projection.Applied {
		return model, nil
	}

	wrapped, err := terrain.InComputeCRS(model, projection.ProjectCRS, projection.ComputeCRS)
	if err != nil {
		// Bare %w: the refusal already names the CRS it could not parse or
		// could not build a transform between, and both come straight from the
		// project manifest.
		return nil, fmt.Errorf("%w", err)
	}

	return wrapped, nil
}
