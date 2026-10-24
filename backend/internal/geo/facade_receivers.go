package geo

import (
	"cmp"
	"encoding/json"
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

// geometricPlacementOffsetM holds the rules a facade receiver set can place,
// and their distance out from the facade. TA Lärm's is measured from the
// window plane, which the model puts in the facade (facade_window.go).
var geometricPlacementOffsetM = map[string]float64{
	PlacementRLS19Facade:    0.05,
	PlacementSchall03Facade: 0,
	PlacementTALaermWindow:  0.5,
}

// nonGeometricPlacementRules are the remaining Section 4 rules. They need a
// balcony line or an area, which neither a footprint nor a window carries.
var nonGeometricPlacementRules = []string{
	"rls19-balcony",
	"rls19-outdoor-area",
	"schall03-outdoor-area",
	"talaerm-buildable-edge",
}

// IsPlacementRule reports whether rule is one of Section 4's identifiers,
// geometric or not. The comparison is exact, as Validate's is.
func IsPlacementRule(rule string) bool {
	_, geometric := geometricPlacementOffsetM[rule]

	return geometric || slices.Contains(nonGeometricPlacementRules, rule)
}

// ErrPlacementRuleNotGeometric refuses a known placement rule that cannot be
// applied to a footprint and its windows.
var ErrPlacementRuleNotGeometric = errors.New("placement rule needs a balcony line or an area, not a footprint")

// Slab height sources (facade-receivers.md Section 5, Decision). The same
// three values describe where a building's height_m came from, as a
// FacadeBuilding input, and where a receiver's storey height came from, as its
// output.
const (
	SlabHeightMeasured = "measured"
	SlabHeightLevels   = "levels"
	SlabHeightAssumed  = "assumed"
)

// Review states of a facade receiver (facade-receivers.md, Receiver
// properties). ReviewStatusProposed is the state of every receiver placed from
// geometry alone: a person has not checked it against the building. A
// confirmed point has been checked; a rejected one never enters a run.
const (
	ReviewStatusProposed  = "proposed"
	ReviewStatusConfirmed = "confirmed"
	ReviewStatusRejected  = "rejected"
)

// StoreyHeightFallbackM is the storey height used when a building's height
// says nothing about its storeys. It is the OSM importer's metres per level,
// so the codebase holds one storey height.
const StoreyHeightFallbackM = 3.0

// MaxFacadeReceivers caps the receivers one set may place. Validate accepts
// any finite spacing and height, and a tiny spacing or a huge height would
// otherwise ask for more stations or storeys than an int holds, or than memory
// takes; the counts are checked against it before anything is allocated.
const MaxFacadeReceivers = 100_000

// facadeProbeOffsetM is how far out a station is tested for lying inside a
// building. It is independent of the rule's offset, so a Schall 03 point on a
// party wall, which the strict-interior test would keep, is dropped too.
const facadeProbeOffsetM = 0.1

// FacadeReceiverSet places facade Immissionsorte on the selected buildings'
// footprint edges under one placement rule: one per window on an edge that
// has windows, and one per station and storey on an edge without, unless the
// rule places only at windows.
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
// Windows are the windows a point is placed at.
type FacadeBuilding struct {
	ID           string
	Polygons     [][][]Point2D
	HeightM      float64
	Levels       int
	HeightSource string
	GroundFloorM float64
	Windows      []FacadeBuildingWindow
}

// FacadeKey names the footprint edge a receiver belongs to, by its indices in
// the canonical footprint. From and To are that edge's endpoints, kept as a
// check that the building has not moved since.
//
// In JSON, From and To are [x, y] positions, the shape facade-receivers.md
// documents and modelgeojson reprojects, not Point2D's {"x", "y"} object.
type FacadeKey struct {
	BuildingRef string
	Part        int
	Ring        int
	Edge        int
	From        Point2D
	To          Point2D
}

// facadeKeyJSON is FacadeKey as the model stores it.
type facadeKeyJSON struct {
	BuildingRef string     `json:"building_ref"`
	Part        int        `json:"part"`
	Ring        int        `json:"ring"`
	Edge        int        `json:"edge"`
	From        [2]float64 `json:"from"`
	To          [2]float64 `json:"to"`
}

// MarshalJSON writes the key in the model's shape.
func (k FacadeKey) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(facadeKeyJSON{
		BuildingRef: k.BuildingRef,
		Part:        k.Part,
		Ring:        k.Ring,
		Edge:        k.Edge,
		From:        [2]float64{k.From.X, k.From.Y},
		To:          [2]float64{k.To.X, k.To.Y},
	})
	if err != nil {
		return nil, fmt.Errorf("encode facade key of %q: %w", k.BuildingRef, err)
	}

	return data, nil
}

// UnmarshalJSON reads the key from the model's shape.
func (k *FacadeKey) UnmarshalJSON(data []byte) error {
	var raw facadeKeyJSON

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("decode facade key: %w", err)
	}

	*k = FacadeKey{
		BuildingRef: raw.BuildingRef,
		Part:        raw.Part,
		Ring:        raw.Ring,
		Edge:        raw.Edge,
		From:        Point2D{X: raw.From[0], Y: raw.From[1]},
		To:          Point2D{X: raw.To[0], Y: raw.To[1]},
	}

	return nil
}

// FacadeReceiver is one generated facade Immissionsort. HeightM is the slab
// height of its storey above ground, or for a point at a window the height its
// rule derives from the window, which HeightBasis names.
//
// A point at a window carries the window's id and frame, inherits its review
// status, and has no SlabHeightSource: the window fixes its height. Its
// Station counts the windows along the edge, and StationUM is the window
// centre's u.
type FacadeReceiver struct {
	PointReceiver

	PlacementRule    string        `json:"placement_rule"`
	Facade           FacadeKey     `json:"facade"`
	Station          int           `json:"station"`
	StationUM        float64       `json:"station_u_m"`
	Storey           int           `json:"storey"`
	SlabHeightSource string        `json:"slab_height_source,omitempty"`
	HeightBasis      string        `json:"height_basis,omitempty"`
	WindowID         string        `json:"window_id,omitempty"`
	Window           *FacadeWindow `json:"window,omitempty"`
	ReviewStatus     string        `json:"review_status"`
}

// Validate checks the set's own fields, without the buildings. Neither the set
// id nor a building id may contain '/', the separator of the receiver ids
// <set>/<building>/p…r…e…/s…/f… (or …/w/<window>) they are spelled into.
func (f FacadeReceiverSet) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return errors.New("facade receiver set id is required")
	}

	if strings.Contains(f.ID, "/") {
		return fmt.Errorf("facade receiver set id %q must not contain '/'", f.ID)
	}

	if len(f.BuildingIDs) == 0 {
		return errors.New("facade receiver set must reference at least one building id")
	}

	seen := make(map[string]struct{}, len(f.BuildingIDs))
	for i, id := range f.BuildingIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("facade receiver set building_ids[%d] is empty", i)
		}

		if strings.Contains(id, "/") {
			return fmt.Errorf("facade receiver set building_ids[%d] %q must not contain '/'", i, id)
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

	windows, err := buildingWindows(building, parts)
	if err != nil {
		return nil, nil, err
	}

	for part, rings := range parts {
		for ring, vertices := range rings {
			if ring > 0 && !f.IncludeCourtyard {
				break
			}

			for edge := range vertices {
				a, b := vertices[edge], vertices[(edge+1)%len(vertices)]
				key := FacadeKey{BuildingRef: building.ID, Part: part, Ring: ring, Edge: edge, From: a, To: b}

				atWindows := windows[edgeIndex{part, ring, edge}]

				switch {
				case len(atWindows) > 0:
					if len(receivers)+len(atWindows) > MaxFacadeReceivers {
						return nil, nil, fmt.Errorf("facade receiver set %q needs more than %d receivers", f.ID, MaxFacadeReceivers)
					}

					receivers, probes = f.placeAtWindows(atWindows, key, storeys, receivers, probes)
				case !IsWindowOnlyPlacementRule(f.PlacementRule):
					receivers, probes, err = f.placeAtStations(key, storeys, receivers, probes)
					if err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}

	return receivers, probes, nil
}

// placeAtStations appends one receiver per station and storey of an edge,
// with the point each station is probed at.
func (f FacadeReceiverSet) placeAtStations(
	key FacadeKey, storeys storeys, receivers []FacadeReceiver, probes []Point2D,
) ([]FacadeReceiver, []Point2D, error) {
	a, b := key.From, key.To
	offsetM := geometricPlacementOffsetM[f.PlacementRule]

	stations, ok := facadeStations(a, b, f.SpacingM, (MaxFacadeReceivers-len(receivers))/len(storeys.z))
	if !ok {
		return nil, nil, fmt.Errorf("facade receiver set %q needs more than %d receivers; raise spacing_m", f.ID, MaxFacadeReceivers)
	}

	for station, u := range stations {
		point := FacadePoint(a, b, u, offsetM)
		probe := FacadePoint(a, b, u, facadeProbeOffsetM)

		for storey, z := range storeys.z {
			receivers = append(receivers, FacadeReceiver{
				PointReceiver: PointReceiver{
					ID:      fmt.Sprintf("%s/%s/p%dr%de%d/s%d/f%d", f.ID, key.BuildingRef, key.Part, key.Ring, key.Edge, station, storey),
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

	return receivers, probes, nil
}

// placeAtWindows appends one receiver per window of an edge, in the order
// along it, with the point each is probed at. Its id names the window in place
// of the station and storey: <set>/<building>/p…r…e…/w/<window>.
func (f FacadeReceiverSet) placeAtWindows(
	windows []FacadeBuildingWindow, key FacadeKey, storeys storeys, receivers []FacadeReceiver, probes []Point2D,
) ([]FacadeReceiver, []Point2D) {
	for station, w := range windows {
		frame := w.Frame
		u := frame.CentreUM()
		offsetM, z, basis := windowPlacement(f.PlacementRule, frame)

		receivers = append(receivers, FacadeReceiver{
			PointReceiver: PointReceiver{
				ID: fmt.Sprintf("%s/%s/p%dr%de%d/w/%s",
					f.ID, key.BuildingRef, key.Part, key.Ring, key.Edge, w.ID),
				Point:   FacadePoint(key.From, key.To, u, offsetM),
				HeightM: z,
			},
			PlacementRule: f.PlacementRule,
			Facade:        key,
			Station:       station,
			StationUM:     u,
			Storey:        storeys.storeyAt(frame.CentreZM()),
			HeightBasis:   basis,
			WindowID:      w.ID,
			Window:        &frame,
			ReviewStatus:  w.ReviewStatus,
		})
		probes = append(probes, FacadePoint(key.From, key.To, u, facadeProbeOffsetM))
	}

	return receivers, probes
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

// FacadeFacingDeg is the bearing the facade from a to b faces: its
// right-hand normal, the direction FacadePoint steps out in, in degrees
// clockwise from the frame's +y axis, in [0, 360). In a projected CRS that is
// grid north, which a view names a facade by ("the south facade"); it is not
// a geodetic azimuth.
func FacadeFacingDeg(a, b Point2D) float64 {
	deg := math.Atan2(b.Y-a.Y, -(b.X-a.X)) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}

	return deg
}

// facadeStations splits an edge into n = max(1, ceil(L / spacing)) equal parts
// and returns the u of each part's centre, so every edge gets at least one
// station, even when L / spacing underflows to 0, and none stands on a
// corner. It reports false, allocating nothing, when n would exceed limit.
func facadeStations(a, b Point2D, spacingM float64, limit int) ([]float64, bool) {
	length := math.Hypot(b.X-a.X, b.Y-a.Y)

	count := max(math.Ceil(length/spacingM), 1)
	if !(count <= float64(limit)) {
		return nil, false
	}

	n := int(count)
	stations := make([]float64, n)

	for k := range n {
		stations[k] = (float64(k) + 0.5) * length / float64(n)
	}

	return stations, true
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

	if b.Levels > MaxFacadeReceivers {
		return storeys{}, fmt.Errorf("levels %d needs more than %d receivers per station", b.Levels, MaxFacadeReceivers)
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
		fit := math.Floor(b.HeightM / StoreyHeightFallbackM)
		if fit > MaxFacadeReceivers {
			return storeys{}, fmt.Errorf("height_m %g needs more than %d receivers per station", b.HeightM, MaxFacadeReceivers)
		}

		count = max(1, int(fit))
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

// StoreySlabs is the ceiling slab height of each of b's storeys above its
// ground, the ground floor first, and where their storey height came from:
// the heights facade mode places a storey's points at, for a view that draws
// the floors those points stand on (P18 C2). They include b.GroundFloorM, as
// the points do, so a raised ground floor moves every line up with them.
func StoreySlabs(b FacadeBuilding) ([]float64, string, error) {
	s, err := storeyHeights(b)
	if err != nil {
		return nil, "", err
	}

	return s.z, s.source, nil
}

// FacadeEdge returns the endpoints of edge `edge` of ring `ring` of part
// `part` in the canonical footprint of polygons (PointInPolygon's ring
// format, as FacadeBuilding.Polygons), the indices a FacadeKey stores. It is
// the edge Generate would stamp on a receiver with that key, so comparing the
// result with the key's From and To tells whether the building moved since.
// The edge's end is the ring's next vertex, wrapping to its first. ok is false
// when an index is out of range or the footprint cannot be canonicalised.
func FacadeEdge(polygons [][][]Point2D, part, ring, edge int) (from, to Point2D, ok bool) {
	parts, err := canonicalFootprint(polygons)
	if err != nil || part < 0 || part >= len(parts) {
		return Point2D{}, Point2D{}, false
	}

	rings := parts[part]
	if ring < 0 || ring >= len(rings) {
		return Point2D{}, Point2D{}, false
	}

	vertices := rings[ring]
	if edge < 0 || edge >= len(vertices) {
		return Point2D{}, Point2D{}, false
	}

	return vertices[edge], vertices[(edge+1)%len(vertices)], true
}

// CanonicalFacadeEdge is one edge of a canonical footprint: the indices a
// FacadeKey stores and the endpoints FacadeEdge returns for them.
type CanonicalFacadeEdge struct {
	Part, Ring, Edge int
	From, To         Point2D
}

// FacadeEdges lists every edge of the canonical footprint of polygons, in
// part, ring and edge order: what FacadeEdge answers one key at a time, for a
// caller that has to search the edges rather than look one up.
func FacadeEdges(polygons [][][]Point2D) ([]CanonicalFacadeEdge, error) {
	parts, err := canonicalFootprint(polygons)
	if err != nil {
		return nil, err
	}

	var edges []CanonicalFacadeEdge

	for p, rings := range parts {
		for r, vertices := range rings {
			for e := range vertices {
				edges = append(edges, CanonicalFacadeEdge{
					Part: p, Ring: r, Edge: e,
					From: vertices[e], To: vertices[(e+1)%len(vertices)],
				})
			}
		}
	}

	return edges, nil
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
