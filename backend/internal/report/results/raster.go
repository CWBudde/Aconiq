package results

import (
	"errors"
	"fmt"
	"math"
)

// RasterMetadata describes a dense raster container.
type RasterMetadata struct {
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Bands     int      `json:"bands"`
	NoData    float64  `json:"nodata"`
	Unit      string   `json:"unit"`
	BandNames []string `json:"band_names,omitempty"`
	CRS       string   `json:"crs,omitempty"`
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
