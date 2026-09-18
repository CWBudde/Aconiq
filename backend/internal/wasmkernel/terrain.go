package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// ComputeProjection is the CRS pair a browser-mode request computed in.
//
// It mirrors `cli.computeProjection` field for field, and its two CRS keys are
// spelled the way every other CRS on a wire in this project is spelled — the
// same `project_crs`/`compute_crs` a run's provenance carries, and the same
// snake_case `aconiq.transform` takes. The browser already resolved this before
// it built the scene, and it is the only thing that can tell the kernel which
// CRS the coordinates in a request are in.
//
// The kernel cannot resolve this for itself. `aconiq run` reads the project
// CRS off the manifest and projects the model; browser mode projects the model
// in TypeScript, through `aconiq.transform`, and hands the kernel coordinates
// that are already metric. By the time a request arrives there is nothing in it
// that names a CRS — so a terrain query has nothing to transform through unless
// the request says.
type ComputeProjection struct {
	ProjectCRS string `json:"project_crs"`
	ComputeCRS string `json:"compute_crs"`
	Applied    bool   `json:"applied"`
}

// TerrainStore holds the DTM the kernel computes over, together with the CRS it
// was declared in.
//
// The CRS is stored beside the model because nothing else knows it. The GeoTIFF
// loader in internal/geo/terrain reads the tie point (33922) and the pixel scale
// (33550) and no GeoKeyDirectory, so a raster's own file does not tell this
// kernel what it is in — the caller does, once, at load.
type TerrainStore struct {
	model terrain.Model
	crs   string
}

// kernelTerrain is the one store `window.aconiq` operates on. The package-level
// functions below are what cmd/wasm/main.go calls; a TerrainStore value is what
// a test uses, so the tests do not have to share this one.
var kernelTerrain TerrainStore

// Load parses a GeoTIFF DTM and keeps it, replacing whatever was loaded before.
// It answers with the terrain.Info JSON the browser reads back.
//
// crs names the CRS the raster's own coordinates are in — `EPSG:25832`, say.
// It is required: without it every later elevation query is a query into an
// unknown CRS, which is exactly the failure this store exists to prevent, and
// guessing "the same as the run" would reintroduce it silently.
func (s *TerrainStore) Load(data []byte, crs string) ([]byte, error) {
	if crs == "" {
		return nil, errors.New(
			"the DTM's CRS must be given: this loader reads no CRS from the GeoTIFF, " +
				"so the caller has to declare what the raster is in (e.g. \"EPSG:25832\")",
		)
	}

	parsed, err := geo.ParseCRS(crs)
	if err != nil {
		return nil, fmt.Errorf("terrain CRS %q: %w", crs, err)
	}

	model, err := terrain.LoadFromBytes(data)
	if err != nil {
		// Bare %w, as in Transform: terrain.LoadFromBytes already says
		// "terrain: parse: …" and cmd/wasm prefixes the entry point. A third
		// layer of context would name the same failure three times.
		return nil, fmt.Errorf("%w", err)
	}

	s.model, s.crs = model, parsed.ID

	info, err := json.Marshal(model.Info())
	if err != nil {
		return nil, fmt.Errorf("marshal terrain info: %w", err)
	}

	return info, nil
}

// Clear drops the loaded terrain. A run afterwards computes without one.
func (s *TerrainStore) Clear() {
	s.model, s.crs = nil, ""
}

// Loaded reports whether a terrain model is held.
func (s *TerrainStore) Loaded() bool { return s.model != nil }

// CRS returns the CRS the loaded terrain was declared in, or "" when none is
// loaded.
func (s *TerrainStore) CRS() string { return s.crs }

// InComputeCRS returns the loaded model ready to be queried with coordinates in
// computeCRS, and a nil model when nothing is loaded.
//
// It compares the two CRS rather than trusting a caller's "applied" flag: a
// browser-mode run whose workspace was already metric reports applied=false and
// may still hold a DTM in a different CRS, and a run that did project may have
// landed in the CRS the DTM is already in.
func (s *TerrainStore) InComputeCRS(computeCRS string) (terrain.Model, error) {
	if s.model == nil {
		return nil, nil //nolint:nilnil // no terrain is not an error: the run computes without one.
	}

	model, err := terrain.InComputeCRS(s.model, s.crs, computeCRS)
	if err != nil {
		// Bare %w again: terrain.InComputeCRS's refusals already name the CRS
		// they could not parse or could not build a transform between.
		return nil, fmt.Errorf("%w", err)
	}

	return model, nil
}

// Apply resolves the loaded terrain into the CRS a request computes in and
// attaches it to the propagation config — both the model itself, which is the
// ground h_m is measured above, and the single elevation the receiver heights
// are stacked on.
//
// A request that carries no projection while a terrain is loaded is refused
// rather than computed. The alternative is what this whole change removes: the
// DTM queried with coordinates in a CRS it is not in, every lookup a miss, and
// every miss read as sea level without a word.
func (s *TerrainStore) Apply(
	cfg road.PropagationConfig,
	receivers []geo.PointReceiver,
	projection *ComputeProjection,
) (road.PropagationConfig, error) {
	if s.model == nil {
		return cfg, nil
	}

	if projection == nil || projection.ComputeCRS == "" {
		return cfg, errors.New(
			"a terrain model is loaded but the request declares no compute CRS: the browser " +
				"projects the model and the kernel cannot resolve the CRS from the coordinates it " +
				"is handed, so send \"projection\" with the request, or call clearTerrain()",
		)
	}

	model, err := s.InComputeCRS(projection.ComputeCRS)
	if err != nil {
		return cfg, err
	}

	cfg.TerrainModel = model

	if len(receivers) > 0 {
		cfg.ReceiverTerrainZ = TerrainAtGridCenter(model, receivers)
	}

	return cfg, nil
}

// LoadTerrain loads a GeoTIFF DTM into the kernel's own store, declared to be
// in crs. It and the three functions below are what cmd/wasm/main.go calls, and
// they are thin so that the file `go test` cannot reach stays thin.
func LoadTerrain(data []byte, crs string) ([]byte, error) { return kernelTerrain.Load(data, crs) }

// ClearTerrain drops the kernel's loaded terrain.
func ClearTerrain() { kernelTerrain.Clear() }

// TerrainLoaded reports whether the kernel holds a terrain model.
func TerrainLoaded() bool { return kernelTerrain.Loaded() }

// ApplyTerrain attaches the kernel's terrain to a request's config, resolved
// into the CRS that request computes in.
func ApplyTerrain(
	cfg road.PropagationConfig,
	receivers []geo.PointReceiver,
	projection *ComputeProjection,
) (road.PropagationConfig, error) {
	return kernelTerrain.Apply(cfg, receivers, projection)
}

// TerrainAtGridCenter returns the terrain elevation at the centroid of the
// receivers, which is the single ReceiverTerrainZ an RLS-19 request carries.
//
// A centroid the DTM does not cover reads 0, deliberately: `cli.terrainElevationAt`
// does the same, and the two targets have to agree on what an uncovered receiver
// grid means or the same model answers differently in the browser and on the
// command line. It is a poor reading — sea level is not "unknown" — but changing
// it is a decision that has to move both targets at once, not a divergence
// introduced here.
func TerrainAtGridCenter(model terrain.Model, receivers []geo.PointReceiver) float64 {
	if model == nil || len(receivers) == 0 {
		return 0
	}

	var sumX, sumY float64

	for _, r := range receivers {
		sumX += r.Point.X
		sumY += r.Point.Y
	}

	n := float64(len(receivers))

	elevation, ok := model.ElevationAt(sumX/n, sumY/n)
	if !ok {
		return 0
	}

	return elevation
}
