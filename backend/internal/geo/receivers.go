package geo

import (
	"errors"
	"fmt"
	"math"
)

// PointReceiver is a single receiver location with a reference height.
type PointReceiver struct {
	ID      string  `json:"id"`
	Point   Point2D `json:"point"`
	HeightM float64 `json:"height_m"`
}

// PointReceiverSet is an explicit list of receiver points.
type PointReceiverSet struct {
	ID        string          `json:"id"`
	Receivers []PointReceiver `json:"receivers"`
}

func (s PointReceiverSet) Validate() error {
	if s.ID == "" {
		return errors.New("point receiver set id is required")
	}

	seen := make(map[string]struct{}, len(s.Receivers))
	for i, receiver := range s.Receivers {
		if receiver.ID == "" {
			return fmt.Errorf("point receiver[%d] id is required", i)
		}

		if _, exists := seen[receiver.ID]; exists {
			return fmt.Errorf("point receiver id %q is duplicated", receiver.ID)
		}

		if !receiver.Point.IsFinite() {
			return fmt.Errorf("point receiver %q has invalid coordinates", receiver.ID)
		}

		if receiver.HeightM < 0 || math.IsNaN(receiver.HeightM) || math.IsInf(receiver.HeightM, 0) {
			return fmt.Errorf("point receiver %q has invalid height", receiver.ID)
		}

		seen[receiver.ID] = struct{}{}
	}

	return nil
}

// GridReceiverSet defines a regular receiver grid over a bbox.
type GridReceiverSet struct {
	ID         string  `json:"id"`
	Extent     BBox    `json:"extent"`
	Resolution float64 `json:"resolution"`
	HeightM    float64 `json:"height_m"`
}

func (g GridReceiverSet) Validate() error {
	if g.ID == "" {
		return errors.New("grid receiver set id is required")
	}

	if !g.Extent.IsFinite() || !g.Extent.IsValid() {
		return errors.New("grid receiver extent is invalid")
	}

	if g.Resolution <= 0 || math.IsNaN(g.Resolution) || math.IsInf(g.Resolution, 0) {
		return errors.New("grid receiver resolution must be finite and > 0")
	}

	if g.HeightM < 0 || math.IsNaN(g.HeightM) || math.IsInf(g.HeightM, 0) {
		return errors.New("grid receiver height must be finite and >= 0")
	}

	return nil
}

// gridBoundTolerance is how far past an extent's maximum a sample may still
// land and count. It absorbs the rounding error of an accumulated step, so a
// grid whose last row misses the edge by 1e-12 keeps that row. CellCount and
// Generate share it: a count derived from a different bound than the loop uses
// would not be a count of what the loop produces.
const gridBoundTolerance = 1e-9

// CellCount reports how many receivers Generate would produce, derived from the
// extent and the resolution rather than by iterating — so a grid too large to
// hold can be refused before it is built. Generate allocates every receiver
// before its caller sees a single one, and a drawn calculation area makes a
// 100 km extent at 10 m spacing (about 10^8 receivers) two gestures away.
//
// The result is a float64 because an extent and a resolution can describe more
// cells than an int holds, and a guard that overflows is not a guard.
//
// Drift over hundreds of thousands of accumulated steps can put this one row or
// column away from what Generate's loop emits. That only matters to a caller
// comparing against a threshold, and only within one receiver of it, which is
// why an oversized grid is still refused on the generated length afterwards.
func (g GridReceiverSet) CellCount() (float64, error) {
	err := g.Validate()
	if err != nil {
		return 0, err
	}

	cols := math.Floor((g.Extent.MaxX-g.Extent.MinX+gridBoundTolerance)/g.Resolution) + 1
	rows := math.Floor((g.Extent.MaxY-g.Extent.MinY+gridBoundTolerance)/g.Resolution) + 1

	return cols * rows, nil
}

// Generate creates deterministic point receivers from the grid definition.
func (g GridReceiverSet) Generate() ([]PointReceiver, error) {
	err := g.Validate()
	if err != nil {
		return nil, err
	}

	points := make([]PointReceiver, 0)
	index := 0

	for y := g.Extent.MinY; y <= g.Extent.MaxY+gridBoundTolerance; y += g.Resolution {
		for x := g.Extent.MinX; x <= g.Extent.MaxX+gridBoundTolerance; x += g.Resolution {
			points = append(points, PointReceiver{
				ID:      fmt.Sprintf("%s-%06d", g.ID, index),
				Point:   Point2D{X: x, Y: y},
				HeightM: g.HeightM,
			})
			index++
		}
	}

	return points, nil
}

// FacadeReceiverSet is a deferred data model for facade-oriented receivers.
// Full geometric generation is intentionally deferred beyond Phase 5.
type FacadeReceiverSet struct {
	ID               string   `json:"id"`
	BuildingIDs      []string `json:"building_ids"`
	OffsetM          float64  `json:"offset_m"`
	VerticalStepM    float64  `json:"vertical_step_m"`
	IncludeCourtyard bool     `json:"include_courtyard"`
}

// IsImplemented reports whether facade receiver generation is currently implemented.
func (f FacadeReceiverSet) IsImplemented() bool {
	return false
}

// Validate validates basic facade set shape without generating points.
func (f FacadeReceiverSet) Validate() error {
	if f.ID == "" {
		return errors.New("facade receiver set id is required")
	}

	if len(f.BuildingIDs) == 0 {
		return errors.New("facade receiver set must reference at least one building id")
	}

	if f.OffsetM < 0 || math.IsNaN(f.OffsetM) || math.IsInf(f.OffsetM, 0) {
		return errors.New("facade offset must be finite and >= 0")
	}

	if f.VerticalStepM <= 0 || math.IsNaN(f.VerticalStepM) || math.IsInf(f.VerticalStepM, 0) {
		return errors.New("facade vertical_step_m must be finite and > 0")
	}

	return nil
}
