package results

import (
	"encoding/json"
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
// A run whose receivers are not a grid (explicit receiver mode) carries the
// **zero** layout: no width, no height, no georeference. Not a 1xN shape — a
// scatter of points the user placed has no raster shape to report, and
// `persistDummyRunOutputs` keys its no-raster short circuit on `Width <= 0`,
// so a layout invented to look grid-like would make it write one.
type GridLayout struct {
	Width  int
	Height int
	CRS    string
	Geo    *Georeference

	// NoDataCells marks the cells a raster writer must leave at the nodata
	// sentinel, indexed as the receivers are (row-major, south-up). It is
	// either empty or Width*Height long. Today it marks grid receivers that
	// stand inside a building footprint: they are not Immissionsorte.
	//
	// A masked receiver is still computed and still in the receiver table.
	// Dropping it instead would punch a hole in the receiver slice, and the
	// grid would stop being a raster — inferGridShape needs len % width == 0,
	// and every writer maps a cell as i % Width. Keeping it also keeps the
	// table's row order, and with it output_hash.
	NoDataCells []bool
}

// IsNoData reports whether cell i must be written as nodata.
func (l GridLayout) IsNoData(i int) bool {
	return i >= 0 && i < len(l.NoDataCells) && l.NoDataCells[i]
}

// NoDataCount reports how many cells are masked.
func (l GridLayout) NoDataCount() int {
	n := 0

	for _, masked := range l.NoDataCells {
		if masked {
			n++
		}
	}

	return n
}

// RasterMetadata describes a dense raster container.
type RasterMetadata struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Bands  int     `json:"bands"`
	NoData float64 `json:"nodata"`
	// Units maps each entry of BandNames to the unit that band's cells carry.
	// See units.go for why it is a map and not a parallel slice.
	Units     map[string]string `json:"units"`
	BandNames []string          `json:"band_names,omitempty"`
	CRS       string            `json:"crs,omitempty"`
	Geo       *Georeference     `json:"georeference,omitempty"`
}

// UnmarshalJSON reads a sidecar written before the unit was per band.
//
// The same expansion ReceiverTable.UnmarshalJSON makes, for the same reason
// and with one extra consequence worth naming: a sidecar that declared no band
// names has nowhere to put the scalar, so it comes back with no units at all
// rather than one filed under an invented key. Such a raster could not say
// which band held what in the first place.
func (m *RasterMetadata) UnmarshalJSON(data []byte) error {
	// An alias, so unmarshalling into it does not call this method again.
	type plain RasterMetadata

	aux := struct {
		*plain

		LegacyUnit string `json:"unit"`
	}{plain: (*plain)(m)}

	err := json.Unmarshal(data, &aux)
	if err != nil {
		return fmt.Errorf("raster metadata: %w", err)
	}

	if len(m.Units) == 0 && aux.LegacyUnit != "" {
		m.Units = UniformUnits(m.BandNames, aux.LegacyUnit)
	}

	return nil
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

	// Bands are named or they are not, and a unit hangs off the name. A raster
	// that declares no band names has nothing to attach a unit to, so it may
	// not carry one — rather than carrying one under a key that names nothing.
	err := validateUnits("raster", meta.BandNames, meta.Units)
	if err != nil {
		return nil, err
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

	copyMeta.Units = CopyUnits(copyMeta.Units)

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

// SetReceiver writes grid receiver index's values into its cell, one per band
// in band order. Receiver order is grid order — row-major from the origin — so
// the index stands in for coordinates: cell (index % Width, index / Width).
//
// A cell the layout masks is left at nodata. Every raster writer goes through
// here rather than calling Set in a loop of its own, so a writer cannot publish
// a level the grid has declared unpublishable by forgetting the mask.
func (r *Raster) SetReceiver(layout GridLayout, index int, values ...float64) error {
	if layout.IsNoData(index) {
		return nil
	}

	x := index % r.meta.Width
	y := index / r.meta.Width

	for band, value := range values {
		err := r.Set(x, y, band, value)
		if err != nil {
			return fmt.Errorf("receiver %d, band %d: %w", index, band, err)
		}
	}

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
