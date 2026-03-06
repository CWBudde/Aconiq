package geo

import (
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
		return fmt.Errorf("point receiver set id is required")
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
		return fmt.Errorf("grid receiver set id is required")
	}
	if !g.Extent.IsFinite() || !g.Extent.IsValid() {
		return fmt.Errorf("grid receiver extent is invalid")
	}
	if g.Resolution <= 0 || math.IsNaN(g.Resolution) || math.IsInf(g.Resolution, 0) {
		return fmt.Errorf("grid receiver resolution must be finite and > 0")
	}
	if g.HeightM < 0 || math.IsNaN(g.HeightM) || math.IsInf(g.HeightM, 0) {
		return fmt.Errorf("grid receiver height must be finite and >= 0")
	}

	return nil
}

// Generate creates deterministic point receivers from the grid definition.
func (g GridReceiverSet) Generate() ([]PointReceiver, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	points := make([]PointReceiver, 0)
	index := 0
	for y := g.Extent.MinY; y <= g.Extent.MaxY+1e-9; y += g.Resolution {
		for x := g.Extent.MinX; x <= g.Extent.MaxX+1e-9; x += g.Resolution {
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
		return fmt.Errorf("facade receiver set id is required")
	}
	if len(f.BuildingIDs) == 0 {
		return fmt.Errorf("facade receiver set must reference at least one building id")
	}
	if f.OffsetM < 0 || math.IsNaN(f.OffsetM) || math.IsInf(f.OffsetM, 0) {
		return fmt.Errorf("facade offset must be finite and >= 0")
	}
	if f.VerticalStepM <= 0 || math.IsNaN(f.VerticalStepM) || math.IsInf(f.VerticalStepM, 0) {
		return fmt.Errorf("facade vertical_step_m must be finite and > 0")
	}

	return nil
}
