package geo

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Placement rules, docs/research/facade-receivers.md Section 4. A rule names
// the one passage a facade Immissionsort was placed under; its offset,
// reference line and height all follow from it.
const (
	// PlacementRLS19Facade is RLS-19 Section 1, Immissionsort: 0.05 m in
	// front of the outer facade, at the storey's slab height.
	PlacementRLS19Facade = "rls19-facade"
	// PlacementSchall03Facade is 16. BImSchV Anlage 2 Nr. 2.2.10: on the
	// facade, at the storey's slab height.
	PlacementSchall03Facade = "schall03-facade"
)

// geometricPlacementOffsetM holds the rules a facade receiver set can place
// from a footprint alone, and their distance out from the facade.
var geometricPlacementOffsetM = map[string]float64{
	PlacementRLS19Facade:    0.05,
	PlacementSchall03Facade: 0,
}

// nonGeometricPlacementRules are the remaining Section 4 rules. They need a
// window, a balcony line or an area, which a footprint does not carry.
var nonGeometricPlacementRules = []string{
	"rls19-balcony",
	"rls19-outdoor-area",
	"schall03-outdoor-area",
	"talaerm-window",
	"talaerm-buildable-edge",
}

// ErrPlacementRuleNotGeometric refuses a known placement rule that cannot be
// applied to a footprint alone.
var ErrPlacementRuleNotGeometric = errors.New("placement rule needs a window or an area, not a footprint")

// Slab height sources (facade-receivers.md Section 5, Decision). The same
// three values describe where a building's height_m came from, as a
// FacadeBuilding input, and where a receiver's storey height came from, as its
// output.
const (
	SlabHeightMeasured = "measured"
	SlabHeightLevels   = "levels"
	SlabHeightAssumed  = "assumed"
)

// ReviewStatusProposed is the review state of every receiver placed from
// geometry alone: a person has not checked it against the building.
const ReviewStatusProposed = "proposed"

// StoreyHeightFallbackM is the storey height used when a building's height
// says nothing about its storeys. It is the OSM importer's metres per level,
// so the codebase holds one storey height.
const StoreyHeightFallbackM = 3.0

// facadeProbeOffsetM is how far out a station is tested for lying inside a
// building. It is independent of the rule's offset, so a Schall 03 point on a
// party wall, which the strict-interior test would keep, is dropped too.
const facadeProbeOffsetM = 0.1

// FacadeReceiverSet places facade Immissionsorte on the selected buildings'
// footprint edges, one per station and storey, under one placement rule.
type FacadeReceiverSet struct {
	ID               string   `json:"id"`
	BuildingIDs      []string `json:"building_ids"`
	PlacementRule    string   `json:"placement_rule"`
	SpacingM         float64  `json:"spacing_m"`
	IncludeCourtyard bool     `json:"include_courtyard"`
}

// FacadeBuilding is one building as Generate reads it.
//
// Polygons are the footprint's parts in PointInPolygon's ring format: rings[0]
// the exterior, rings[1:] courtyard holes. HeightSource says where HeightM
// came from (a SlabHeight* value), Levels is the storey count or 0 when it is
// unknown, and GroundFloorM is the ground floor's level above terrain (h_EG).
type FacadeBuilding struct {
	ID           string
	Polygons     [][][]Point2D
	HeightM      float64
	Levels       int
	HeightSource string
	GroundFloorM float64
}

// FacadeKey names the footprint edge a receiver belongs to, by its indices in
// the canonical footprint. From and To are that edge's endpoints, kept as a
// check that the building has not moved since.
type FacadeKey struct {
	BuildingRef string  `json:"building_ref"`
	Part        int     `json:"part"`
	Ring        int     `json:"ring"`
	Edge        int     `json:"edge"`
	From        Point2D `json:"from"`
	To          Point2D `json:"to"`
}

// FacadeReceiver is one generated facade Immissionsort. HeightM is the slab
// height of its storey above ground.
type FacadeReceiver struct {
	PointReceiver

	PlacementRule    string    `json:"placement_rule"`
	Facade           FacadeKey `json:"facade"`
	Station          int       `json:"station"`
	StationUM        float64   `json:"station_u_m"`
	Storey           int       `json:"storey"`
	SlabHeightSource string    `json:"slab_height_source"`
	ReviewStatus     string    `json:"review_status"`
}

// Validate checks the set's own fields, without the buildings.
func (f FacadeReceiverSet) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return errors.New("facade receiver set id is required")
	}

	if len(f.BuildingIDs) == 0 {
		return errors.New("facade receiver set must reference at least one building id")
	}

	seen := make(map[string]struct{}, len(f.BuildingIDs))
	for i, id := range f.BuildingIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("facade receiver set building_ids[%d] is empty", i)
		}

		if _, dup := seen[id]; dup {
			return fmt.Errorf("facade receiver set names building %q twice", id)
		}

		seen[id] = struct{}{}
	}

	if _, ok := geometricPlacementOffsetM[f.PlacementRule]; !ok {
		if slices.Contains(nonGeometricPlacementRules, f.PlacementRule) {
			return fmt.Errorf("placement rule %q: %w", f.PlacementRule, ErrPlacementRuleNotGeometric)
		}

		return fmt.Errorf("unknown placement rule %q", f.PlacementRule)
	}

	if f.SpacingM <= 0 || math.IsNaN(f.SpacingM) || math.IsInf(f.SpacingM, 0) {
		return errors.New("facade spacing_m must be finite and > 0")
	}

	return nil
}

// Generate places the set's receivers. buildings is every building in the
// model: the selected ones get receivers, and all of them decide which
// stations are free, so a party wall carries none.
//
// The result depends on the buildings' geometry and not on how it is written:
// footprints are canonicalised before any index is taken, and the receivers
// come back sorted by (building, part, ring, edge, station, storey), the order
// their IDs spell.
func (f FacadeReceiverSet) Generate(buildings []FacadeBuilding) ([]FacadeReceiver, error) {
	err := f.Validate()
	if err != nil {
		return nil, err
	}

	model, err := canonicalBuildings(buildings)
	if err != nil {
		return nil, err
	}

	var (
		receivers []FacadeReceiver
		probes    []Point2D
	)

	for _, id := range f.BuildingIDs {
		i, ok := model.byID[id]
		if !ok {
			return nil, fmt.Errorf("building %q is not in the model", id)
		}

		receivers, probes, err = f.place(buildings[i], model.parts[i], receivers, probes)
		if err != nil {
			return nil, fmt.Errorf("building %q: %w", id, err)
		}
	}

	masked := MaskPointsInFootprints(probes, model.footprints)
	kept := receivers[:0]

	for i, r := range receivers {
		if !masked[i] {
			kept = append(kept, r)
		}
	}

	slices.SortFunc(kept, compareFacadeReceivers)

	for i := 1; i < len(kept); i++ {
		if kept[i].ID == kept[i-1].ID {
			return nil, fmt.Errorf("facade receiver id %q is not unique: a building id must not contain '/'", kept[i].ID)
		}
	}

	return kept, nil
}

// facadeModel is the model's buildings with their footprints canonicalised,
// and every part closed again for the probe test.
type facadeModel struct {
	byID       map[string]int
	parts      [][][][]Point2D
	footprints [][][]Point2D
}

func canonicalBuildings(buildings []FacadeBuilding) (facadeModel, error) {
	model := facadeModel{
		byID:       make(map[string]int, len(buildings)),
		parts:      make([][][][]Point2D, len(buildings)),
		footprints: make([][][]Point2D, 0, len(buildings)),
	}

	for i, b := range buildings {
		if _, dup := model.byID[b.ID]; dup {
			return facadeModel{}, fmt.Errorf("building %q is given twice", b.ID)
		}

		model.byID[b.ID] = i

		parts, err := canonicalFootprint(b.Polygons)
		if err != nil {
			return facadeModel{}, fmt.Errorf("building %q: %w", b.ID, err)
		}

		model.parts[i] = parts

		for _, part := range parts {
			model.footprints = append(model.footprints, closeRings(part))
		}
	}

	return model, nil
}

// place appends one building's receivers, and beside each the point its
// station is probed at.
func (f FacadeReceiverSet) place(
	building FacadeBuilding, parts [][][]Point2D, receivers []FacadeReceiver, probes []Point2D,
) ([]FacadeReceiver, []Point2D, error) {
	storeys, err := storeyHeights(building)
	if err != nil {
		return nil, nil, err
	}

	offsetM := geometricPlacementOffsetM[f.PlacementRule]

	for part, rings := range parts {
		for ring, vertices := range rings {
			if ring > 0 && !f.IncludeCourtyard {
				break
			}

			for edge := range vertices {
				a, b := vertices[edge], vertices[(edge+1)%len(vertices)]
				key := FacadeKey{BuildingRef: building.ID, Part: part, Ring: ring, Edge: edge, From: a, To: b}

				for station, u := range facadeStations(a, b, f.SpacingM) {
					point := FacadePoint(a, b, u, offsetM)
					probe := FacadePoint(a, b, u, facadeProbeOffsetM)

					for storey, z := range storeys.z {
						receivers = append(receivers, FacadeReceiver{
							PointReceiver: PointReceiver{
								ID:      fmt.Sprintf("%s/%s/p%dr%de%d/s%d/f%d", f.ID, building.ID, part, ring, edge, station, storey),
								Point:   point,
								HeightM: z,
							},
							PlacementRule:    f.PlacementRule,
							Facade:           key,
							Station:          station,
							StationUM:        u,
							Storey:           storey,
							SlabHeightSource: storeys.source,
							ReviewStatus:     ReviewStatusProposed,
						})
						probes = append(probes, probe)
					}
				}
			}
		}
	}

	return receivers, probes, nil
}

// compareFacadeReceivers orders receivers by the key their IDs spell.
func compareFacadeReceivers(a, b FacadeReceiver) int {
	return cmp.Or(
		cmp.Compare(a.Facade.BuildingRef, b.Facade.BuildingRef),
		cmp.Compare(a.Facade.Part, b.Facade.Part),
		cmp.Compare(a.Facade.Ring, b.Facade.Ring),
		cmp.Compare(a.Facade.Edge, b.Facade.Edge),
		cmp.Compare(a.Station, b.Station),
		cmp.Compare(a.Storey, b.Storey),
	)
}

// FacadePoint maps a point of a facade's frame to the world: u metres along
// the edge from a towards b, d metres out along the right-hand normal. On a
// canonical ring the right-hand normal points away from the building, on an
// exterior ring and a courtyard ring alike.
//
// Each product is rounded on its own (the float64 conversions stop fused
// multiply-adds), so every architecture places the same point to the bit.
func FacadePoint(a, b Point2D, u, d float64) Point2D {
	length := math.Hypot(b.X-a.X, b.Y-a.Y)
	tx := (b.X - a.X) / length
	ty := (b.Y - a.Y) / length

	return Point2D{
		X: a.X + float64(u*tx) + float64(d*ty),
		Y: a.Y + float64(u*ty) - float64(d*tx),
	}
}

// facadeStations splits an edge into n = ceil(L / spacing) equal parts and
// returns the u of each part's centre, so every edge gets at least one
// station and none stands on a corner.
func facadeStations(a, b Point2D, spacingM float64) []float64 {
	length := math.Hypot(b.X-a.X, b.Y-a.Y)
	n := int(math.Ceil(length / spacingM))
	stations := make([]float64, n)

	for k := range n {
		stations[k] = (float64(k) + 0.5) * length / float64(n)
	}

	return stations
}

// storeys is a building's slab heights, storey 0 (the ground floor) first,
// and where their storey height came from.
type storeys struct {
	z      []float64
	source string
}

// storeyHeights applies facade-receivers.md Section 5: the ceiling slab of
// storey n is at h_EG + (n + 1)·s, with s from a height that was not derived
// from the storey count, and StoreyHeightFallbackM otherwise.
func storeyHeights(b FacadeBuilding) (storeys, error) {
	if b.HeightM <= 0 || math.IsNaN(b.HeightM) || math.IsInf(b.HeightM, 0) {
		return storeys{}, errors.New("height_m must be finite and > 0")
	}

	if b.Levels < 0 {
		return storeys{}, errors.New("levels must be >= 0")
	}

	if math.IsNaN(b.GroundFloorM) || math.IsInf(b.GroundFloorM, 0) {
		return storeys{}, errors.New("ground floor level must be finite")
	}

	switch b.HeightSource {
	case SlabHeightMeasured, SlabHeightLevels, SlabHeightAssumed:
	default:
		return storeys{}, fmt.Errorf("unknown height source %q", b.HeightSource)
	}

	count, s, source := b.Levels, StoreyHeightFallbackM, SlabHeightLevels

	switch {
	case b.Levels == 0:
		// No storey count: as many fallback storeys as fit, at least one.
		count = max(1, int(math.Floor(b.HeightM/StoreyHeightFallbackM)))
		source = SlabHeightAssumed
	case b.HeightSource == SlabHeightMeasured:
		s = b.HeightM / float64(b.Levels)
		source = SlabHeightMeasured
	}

	z := make([]float64, count)
	for n := range count {
		z[n] = b.GroundFloorM + float64(float64(n+1)*s)
	}

	return storeys{z: z, source: source}, nil
}

// canonicalFootprint returns a footprint whose part, ring and edge indices do
// not depend on how it was written (facade-receivers.md, The facade key).
// Each ring loses its closing vertex and any repeated vertex, is oriented
// (exterior counter-clockwise, holes clockwise) and starts at its smallest
// vertex; holes, then parts, are sorted by their vertex sequences. The
// comparison is exact: a tolerance would make the order depend on which
// neighbour falls within it.
func canonicalFootprint(polygons [][][]Point2D) ([][][]Point2D, error) {
	if len(polygons) == 0 {
		return nil, errors.New("footprint has no polygon")
	}

	parts := make([][][]Point2D, 0, len(polygons))

	for p, rings := range polygons {
		if len(rings) == 0 {
			return nil, fmt.Errorf("polygon %d has no ring", p)
		}

		part := make([][]Point2D, len(rings))

		for r, ring := range rings {
			canonical, err := canonicalRing(ring, r == 0)
			if err != nil {
				return nil, fmt.Errorf("polygon %d ring %d: %w", p, r, err)
			}

			part[r] = canonical
		}

		slices.SortFunc(part[1:], compareRings)
		parts = append(parts, part)
	}

	slices.SortFunc(parts, func(a, b [][]Point2D) int { return compareRings(a[0], b[0]) })

	return parts, nil
}

func canonicalRing(ring []Point2D, exterior bool) ([]Point2D, error) {
	open := make([]Point2D, 0, len(ring))

	for _, p := range ring {
		if !p.IsFinite() {
			return nil, errors.New("vertex is not finite")
		}

		if len(open) == 0 || open[len(open)-1] != p {
			open = append(open, p)
		}
	}

	for len(open) > 1 && open[len(open)-1] == open[0] {
		open = open[:len(open)-1]
	}

	if len(open) < 3 {
		return nil, errors.New("ring has fewer than three distinct vertices")
	}

	area := signedRingArea(closeRing(open))
	if area == 0 {
		return nil, errors.New("ring encloses no area")
	}

	if (area > 0) != exterior {
		slices.Reverse(open)
	}

	start := 0

	for i, p := range open {
		if comparePoints(p, open[start]) < 0 {
			start = i
		}
	}

	return slices.Concat(open[start:], open[:start]), nil
}

func comparePoints(a, b Point2D) int {
	return cmp.Or(cmp.Compare(a.X, b.X), cmp.Compare(a.Y, b.Y))
}

func compareRings(a, b []Point2D) int {
	return slices.CompareFunc(a, b, comparePoints)
}

func closeRing(open []Point2D) []Point2D {
	return append(slices.Clone(open), open[0])
}

func closeRings(rings [][]Point2D) [][]Point2D {
	out := make([][]Point2D, len(rings))
	for i, ring := range rings {
		out[i] = closeRing(ring)
	}

	return out
}
