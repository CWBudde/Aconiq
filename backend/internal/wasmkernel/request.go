package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// ComputeRequest is the JSON `rls19Road`, `rls19RoadShard`, `rls19RoadGrid`
// and `estimate` take.
//
// It lives here rather than in cmd/wasm so that a host test can reach what a
// request means; cmd/wasm only moves strings across the JS boundary.
type ComputeRequest struct {
	// Receivers lists the receivers one object at a time. A request carries
	// either this or Grid, never both: PrepareComputeRequest expands a Grid
	// into this list, so everything downstream reads Receivers alone.
	Receivers []geo.PointReceiver `json:"receivers"`

	// Grid names an automatic grid instead of listing it. Nil when the
	// request listed its receivers; still set after the expansion, so an
	// export can tell which way a request came.
	Grid *ReceiverGrid `json:"grid,omitempty"`

	Sources  []road.RoadSource      `json:"sources"`
	Barriers []road.Barrier         `json:"barriers"`
	Config   road.PropagationConfig `json:"config"`

	// Projection is the CRS pair the caller resolved before it built this
	// scene. It is optional — a request that loads no terrain needs nothing
	// from it — and required the moment a terrain model is loaded, because the
	// coordinates in this request name no CRS of their own and a DTM has to be
	// queried in the one it was written in. See ComputeProjection.
	Projection *ComputeProjection `json:"projection,omitempty"`

	// Shard names which member of a Worker pool is asking, when one is.
	// Absent means the whole run, which is what `rls19Road` always is.
	//
	// The rest of this struct is the *whole run's* even when a shard is set:
	// Receivers is every receiver, not the shard's slice. See
	// ComputeRLS19RoadShard for why that is load-bearing rather than wasteful.
	Shard *Shard `json:"shard,omitempty"`
}

// PrepareComputeRequest parses a request against the kernel's own terrain.
func PrepareComputeRequest(input []byte) (ComputeRequest, error) {
	return kernelTerrain.PrepareComputeRequest(input)
}

// PrepareComputeRequest parses a request, expands a grid into the receivers it
// names, fills the propagation defaults, and resolves the ground under it.
//
// Every compute export goes through this, so they cannot come to disagree
// about what a request means — which matters more than usual here, because
// the whole point of the shard export is that N Workers reach the identical
// configuration. The grid is expanded *before* the terrain is applied: the
// one ground elevation of the run is read at the centroid of the receiver
// list, so a grid and the list it names reach the same elevation by
// construction, and every shard expands the identical list.
func (s *TerrainStore) PrepareComputeRequest(input []byte) (ComputeRequest, error) {
	var req ComputeRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return ComputeRequest{}, fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.Grid != nil {
		if len(req.Receivers) > 0 {
			return ComputeRequest{}, errors.New("a request names its receivers either as a grid or as a list, not both")
		}

		if err := req.Grid.Validate(); err != nil {
			return ComputeRequest{}, err
		}

		req.Receivers = req.Grid.Receivers()
	}

	req.Config = withPropagationDefaults(req.Config)

	// The terrain is the ground h_m is measured above as well as the one
	// elevation the receiver heights stack on, so both travel into the
	// config — and both in the CRS this request computes in. Browser mode
	// resolves the ground the way `aconiq run` does or it is answering a
	// different question.
	cfg, err := s.Apply(req.Config, req.Receivers, req.Projection)
	if err != nil {
		return ComputeRequest{}, fmt.Errorf("%w", err)
	}

	req.Config = cfg

	return req, nil
}

// withPropagationDefaults fills the three scalars a caller may leave unset,
// one at a time.
//
// Replacing the whole struct would discard everything else the config carries
// — Buildings, Reflectors, ParkingSources, Terrain — for a caller that simply
// did not state a segment length. The browser sets all three today, so that
// was latent rather than live, but the scene fields are exactly what a request
// building a Parkplatz sends.
func withPropagationDefaults(cfg road.PropagationConfig) road.PropagationConfig {
	defaults := road.DefaultPropagationConfig()

	if cfg.SegmentLengthM == 0 {
		cfg.SegmentLengthM = defaults.SegmentLengthM
	}

	if cfg.MinDistanceM == 0 {
		cfg.MinDistanceM = defaults.MinDistanceM
	}

	if cfg.ReceiverHeightM == 0 {
		cfg.ReceiverHeightM = defaults.ReceiverHeightM
	}

	return cfg
}
