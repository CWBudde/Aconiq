package results

import (
	"errors"
	"fmt"
	"math"
)

// RowOrderSouthUp is the only row order this project writes: row 0 holds the
// southernmost cells, because geo.GridReceiverSet.Generate walks Y ascending
// and every raster is laid out in the order its receivers were generated.
//
// It is declared rather than assumed. The GeoTIFF writer and the contour
// tracer each flip rows independently against that convention, and until now
// neither could read it off the data — both simply knew. A raster that ever
// arrives north-up is then a value to branch on rather than a silent
// half-image.
const RowOrderSouthUp = "south-up"

// Georeference says where a raster's cells sit on the ground.
//
// OriginX/OriginY is the *centre* of cell (0,0), not a corner, because that is
// what the engine produces: receivers are points, and a grid receiver sits in
// the middle of the cell it stands for. The conversion to GDAL's corner-based
// affine transform happens once, in report/export, and nowhere else.
//
// An absent Georeference means the receivers are not a grid — which is not the
// same as a grid at the origin, and is why this hangs off RasterMetadata as a
// pointer.
type Georeference struct {
	OriginX    float64 `json:"origin_x"`
	OriginY    float64 `json:"origin_y"`
	PixelSizeM float64 `json:"pixel_size_m"`
	RowOrder   string  `json:"row_order"`
}

// Validate refuses a georeference that cannot describe a grid.
func (g Georeference) Validate() error {
	if math.IsNaN(g.OriginX) || math.IsInf(g.OriginX, 0) {
		return errors.New("georeference origin_x must be finite")
	}

	if math.IsNaN(g.OriginY) || math.IsInf(g.OriginY, 0) {
		return errors.New("georeference origin_y must be finite")
	}

	if g.PixelSizeM <= 0 || math.IsNaN(g.PixelSizeM) || math.IsInf(g.PixelSizeM, 0) {
		return errors.New("georeference pixel_size_m must be finite and > 0")
	}

	if g.RowOrder != RowOrderSouthUp {
		return fmt.Errorf("georeference row_order %q is not supported", g.RowOrder)
	}

	return nil
}

// GridLayout is everything a raster needs to know about the receiver grid it
// was computed on: its shape, where it sits and in which CRS.
//
// The three used to travel separately — width and height as a pair of ints
// through a dozen signatures, the CRS through provenance, and the origin and
// pixel size not at all, reconstructed downstream by arithmetic over the
// receiver table. Keeping them in one value is what stops the third from being
// dropped again.
//
// Geo is nil for a run whose receivers are not a grid (explicit receiver mode).
// Width is then 1 and Height the receiver count, which describes the raster's
// shape truthfully and says nothing about the ground.
type GridLayout struct {
	Width  int
	Height int
	CRS    string
	Geo    *Georeference
}

// RasterMetadata describes a dense raster container.
type RasterMetadata struct {
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	Bands     int           `json:"bands"`
	NoData    float64       `json:"nodata"`
	Unit      string        `json:"unit"`
	BandNames []string      `json:"band_names,omitempty"`
	CRS       string        `json:"crs,omitempty"`
	Geo       *Georeference `json:"georeference,omitempty"`
}

// Raster stores banded grid values in row-major order.
type Raster struct {
	meta RasterMetadata
	data []float64
}

func NewRaster(meta RasterMetadata) (*Raster, error) {
	if meta.Width <= 0 {
		return nil, errors.New("raster width must be > 0")
	}

	if meta.Height <= 0 {
		return nil, errors.New("raster height must be > 0")
	}

	if meta.Bands <= 0 {
		return nil, errors.New("raster bands must be > 0")
	}

	if math.IsNaN(meta.NoData) || math.IsInf(meta.NoData, 0) {
		return nil, errors.New("raster nodata must be finite")
	}

	if len(meta.BandNames) > 0 && len(meta.BandNames) != meta.Bands {
		return nil, fmt.Errorf("band_names length (%d) must match bands (%d)", len(meta.BandNames), meta.Bands)
	}

	if meta.Geo != nil {
		err := meta.Geo.Validate()
		if err != nil {
			return nil, fmt.Errorf("raster georeference: %w", err)
		}
	}

	cellCount := meta.Width * meta.Height * meta.Bands

	values := make([]float64, cellCount)
	for i := range values {
		values[i] = meta.NoData
	}

	return &Raster{meta: meta, data: values}, nil
}

func (r *Raster) Metadata() RasterMetadata {
	copyMeta := r.meta
	if len(copyMeta.BandNames) > 0 {
		copyMeta.BandNames = append([]string(nil), copyMeta.BandNames...)
	}

	// Geo is a pointer, so the shallow struct copy above would hand the
	// caller a handle on the raster's own georeference. Metadata() exists to
	// return a value nobody can write back through.
	if copyMeta.Geo != nil {
		geo := *copyMeta.Geo
		copyMeta.Geo = &geo
	}

	return copyMeta
}

func (r *Raster) CellCount() int {
	return len(r.data)
}

func (r *Raster) Fill(value float64) {
	for i := range r.data {
		r.data[i] = value
	}
}

func (r *Raster) At(x, y, band int) (float64, error) {
	idx, err := r.index(x, y, band)
	if err != nil {
		return 0, err
	}

	return r.data[idx], nil
}

func (r *Raster) Set(x, y, band int, value float64) error {
	idx, err := r.index(x, y, band)
	if err != nil {
		return err
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errors.New("raster value must be finite")
	}

	r.data[idx] = value

	return nil
}

func (r *Raster) Values() []float64 {
	return append([]float64(nil), r.data...)
}

func (r *Raster) index(x, y, band int) (int, error) {
	if x < 0 || x >= r.meta.Width {
		return 0, fmt.Errorf("x index out of bounds: %d", x)
	}

	if y < 0 || y >= r.meta.Height {
		return 0, fmt.Errorf("y index out of bounds: %d", y)
	}

	if band < 0 || band >= r.meta.Bands {
		return 0, fmt.Errorf("band index out of bounds: %d", band)
	}

	return (band*r.meta.Height+y)*r.meta.Width + x, nil
}
