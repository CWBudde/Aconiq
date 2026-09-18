package wasmkernel

import (
	"encoding/json"
	"fmt"

	"github.com/aconiq/backend/internal/report/contour"
	"github.com/aconiq/backend/internal/report/results"
)

// ContourResult is what `aconiq.contours` answers with, and ContourLine is one
// line in it.
//
// Aliases rather than declarations, for the reason TransformRequest gives: the
// wire contract belongs to internal/report/contour, which
// GET /api/v1/runs/{id}/contours and `aconiq export --format contour-geojson`
// serve from the same types. Where a 55 dB line falls is read as an assessment,
// so the browser must not be able to drift into a second shape for it.
type (
	ContourResult = contour.Result
	ContourLine   = contour.Line
)

// DefaultContourInterval is the dB step a request that names none takes.
const DefaultContourInterval = contour.DefaultInterval

// ContourRequest is the JSON `aconiq.contours` takes beside the raster bytes.
//
// Raster is the run's `.json` sidecar verbatim — the browser holds it next to
// the `.bin` payload and hands both back untouched, so the shape, the nodata
// value, the band names and the georeference are the ones the run wrote rather
// than a browser's reconstruction of them. The sidecar's bookkeeping fields
// (data_file, encoding, cell_count) are simply not read here; they describe a
// file on disk, and in browser mode there is none.
//
// The options are spelled flat and in snake_case rather than nested as a
// contour.Options, which carries no JSON tags and would put PascalCase keys on
// a wire where every other key in this package is snake_case.
type ContourRequest struct {
	Raster results.RasterMetadata `json:"raster"`

	// TargetCRS is required. There is no "auto" here as there is on
	// aconiq.transform: contours are drawn on a map that already knows what CRS
	// it wants them in, and guessing on the kernel's side would put a metric
	// run's lines off the coast of Africa the one time the guess is wrong.
	TargetCRS string `json:"target_crs"`

	Interval float64 `json:"interval,omitempty"`
	MinLevel float64 `json:"min_level,omitempty"`
	MaxLevel float64 `json:"max_level,omitempty"`
}

// Contours traces ISO-band contour lines over a run's raster and returns them
// in TargetCRS.
//
// The raster values do not travel in the JSON: a 500x500 two-band grid is four
// megabytes of float64, and rendering those as JSON numbers and parsing them
// back would cost more than the marching squares does. They cross as raw bytes
// instead, exactly as loadTerrain's GeoTIFF does, and are decoded by
// results.DecodeRaster so that the browser never holds a second reading of the
// byte contract.
//
// What this function owns is only the JSON/bytes boundary. The tracing, the
// band walk and the reprojection live in internal/report/contour, which the API
// handler calls too.
func Contours(payload []byte, input []byte) ([]byte, error) {
	var req ContourRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	raster, err := results.DecodeRaster(req.Raster, payload)
	if err != nil {
		// Bare %w, as in Transform and TerrainStore.Load: DecodeRaster's
		// refusals already name the shape and the byte count that disagree, and
		// cmd/wasm prefixes the entry point.
		return nil, fmt.Errorf("%w", err)
	}

	opts := contour.Options{Interval: req.Interval, MinLevel: req.MinLevel, MaxLevel: req.MaxLevel}

	result, err := contour.FromRaster(raster, opts, req.TargetCRS)
	if err != nil {
		// Bare %w again, and here it is load-bearing: contour.FromRaster's
		// refusals are the ones the CLI and the API print — "declares no
		// georeference", "carries no EPSG code" — and a reader comparing
		// browser mode against a bundle has to see the same sentence. Context
		// added here would make the two look like different failures.
		return nil, fmt.Errorf("%w", err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return out, nil
}
