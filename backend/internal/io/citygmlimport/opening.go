package citygmlimport

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Opening is a bldg:Window of a LoD3 building (P18 C1b): its gml:id, the
// Building or BuildingPart it belongs to, the vertices of its outline's
// exterior rings in the document's CRS with z above its datum, and the base
// of that building in the same datum, from which the window's sill is
// measured. Placing it on a facade needs the model the buildings become, and
// is modelgeojson.WindowPlacer.OpeningFeature's.
type Opening struct {
	ID         string
	BuildingID string
	Vertices   [][3]float64
	BaseZ      float64
}

// OpeningReport summarises a document's openings. Windows counts every
// bldg:Window, Imported the ones that became window features, which only the
// caller placing them knows; Doors counts the bldg:Doors, which are not
// imported. Skipped names each window that was not imported, and why.
type OpeningReport struct {
	Windows  int              `json:"windows"`
	Imported int              `json:"imported"`
	Doors    int              `json:"doors_not_imported"`
	Skipped  []SkippedOpening `json:"skipped_windows,omitempty"`
}

// SkippedOpening records a window that was not imported.
type SkippedOpening struct {
	ID       string `json:"id"`
	Building string `json:"building,omitempty"`
	Reason   string `json:"reason"`
}

// The CityGML elements the walks over the tokens act on.
const (
	elemBuilding     = "Building"
	elemBuildingPart = "BuildingPart"
	elemWindow       = "Window"
	elemDoor         = "Door"
)

// boundarySurfaces are the bldg:_BoundarySurface elements an opening can sit
// in. Only one in a WallSurface is in a facade.
var boundarySurfaces = map[string]bool{
	"WallSurface": true, "RoofSurface": true, "GroundSurface": true, "ClosureSurface": true,
	"CeilingSurface": true, "FloorSurface": true, "OuterCeilingSurface": true, "OuterFloorSurface": true,
	"InteriorWallSurface": true,
}

// readOpenings collects the bldg:Window elements of a document and the base
// height of the buildings they belong to. go-citygml decodes neither lod3
// geometry nor openings, so this is a second, light walk over the tokens, as
// readAddresses is; a document that names no Window or Door skips it, and
// reports nil. Malformed XML ends the walk quietly: the parser reports it.
//
// A window is read from the vertices of the exterior rings of its explicit
// geometry (posList or pos). Each Building or BuildingPart's base is the
// lowest z of its GroundSurface, or of its geometry when it has none; an
// opening belongs to the innermost one enclosing it.
func readOpenings(data []byte) ([]Opening, *OpeningReport) {
	if !bytes.Contains(data, []byte(elemWindow)) && !bytes.Contains(data, []byte(elemDoor)) {
		return nil, nil
	}

	var walk openingWalk

	dec := xml.NewDecoder(bytes.NewReader(data))

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			walk.start(t)
		case xml.CharData:
			if walk.text != nil {
				walk.text.Write(t)
			}
		case xml.EndElement:
			walk.end(t.Name.Local)
		}
	}

	return walk.finish()
}

// openingOwner is a Building or BuildingPart and the lowest z of its
// geometry, overall and in its GroundSurface.
type openingOwner struct {
	id                  string
	groundMinZ, anyMinZ float64
	hasGround, hasAnyZ  bool
}

// pendingOpening is a Window or Door being read, or a window read and kept.
type pendingOpening struct {
	id       string
	door     bool
	owner    *openingOwner
	surface  string
	vertices [][3]float64
	problem  string
}

// openingWalk is readOpenings' state between tokens.
type openingWalk struct {
	owners   []*openingOwner // open Building / BuildingPart, innermost last
	surfaces []string        // open boundary surfaces, innermost last
	dims     []int           // each open element's srsDimension, 0 where unstated
	exterior int             // open gml:exterior elements
	current  *pendingOpening
	text     *strings.Builder // the posList or pos being read
	textDim  int
	kept     []pendingOpening
	report   OpeningReport
}

func (w *openingWalk) start(t xml.StartElement) {
	dim := 0

	for _, attr := range t.Attr {
		if attr.Name.Local == "srsDimension" {
			dim, _ = strconv.Atoi(strings.TrimSpace(attr.Value))
		}
	}

	w.dims = append(w.dims, dim)

	switch local := t.Name.Local; {
	case local == elemBuilding || local == elemBuildingPart:
		w.owners = append(w.owners, &openingOwner{id: gmlID(t)})
	case boundarySurfaces[local]:
		w.surfaces = append(w.surfaces, local)
	case local == elemWindow || local == elemDoor:
		if w.current == nil {
			w.current = &pendingOpening{id: gmlID(t), door: local == elemDoor, owner: last(w.owners), surface: lastString(w.surfaces)}
		}
	case local == "exterior":
		w.exterior++
	case local == "posList" || local == "pos":
		w.text = &strings.Builder{}
		w.textDim = w.dimension()
	}
}

func (w *openingWalk) end(local string) {
	switch {
	case local == "posList" || local == "pos":
		if w.text != nil {
			w.points(w.text.String())
			w.text = nil
		}
	case local == "exterior":
		w.exterior--
	case local == elemWindow || local == elemDoor:
		if w.current != nil && w.current.door == (local == elemDoor) {
			w.close(*w.current)
			w.current = nil
		}
	case boundarySurfaces[local]:
		if len(w.surfaces) > 0 {
			w.surfaces = w.surfaces[:len(w.surfaces)-1]
		}
	case local == elemBuilding || local == elemBuildingPart:
		if len(w.owners) > 0 {
			w.owners = w.owners[:len(w.owners)-1]
		}
	}

	if len(w.dims) > 0 {
		w.dims = w.dims[:len(w.dims)-1]
	}
}

// dimension is the srsDimension of the innermost element stating one, 3
// where none does: LoD3 is 3D.
func (w *openingWalk) dimension() int {
	for _, dim := range slices.Backward(w.dims) {
		if dim > 0 {
			return dim
		}
	}

	return 3
}

// points files the coordinates just read: into the open window's outline
// when they are in one of its exterior rings, else into the lowest z of the
// innermost building. A door's are read by nothing.
func (w *openingWalk) points(text string) {
	if w.current != nil && w.current.door {
		return
	}

	dim := w.textDim
	if dim < 3 {
		w.fail("its outline is 2D: it states no z to measure a sill or a height from")

		return
	}

	fields := strings.Fields(text)
	values := make([]float64, len(fields))

	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || len(fields)%dim != 0 {
			w.fail(fmt.Sprintf("its coordinates are not %d finite numbers per vertex", dim))

			return
		}

		values[i] = v
	}

	for i := 0; i+dim <= len(values); i += dim {
		x, y, z := values[i], values[i+1], values[i+2]

		if w.current != nil {
			if w.exterior > 0 {
				w.current.vertices = append(w.current.vertices, [3]float64{x, y, z})
			}

			continue
		}

		w.base(z)
	}
}

// fail records the first reason the open window cannot be read; geometry
// outside a window that cannot be read only leaves its building's base unset.
func (w *openingWalk) fail(problem string) {
	if w.current != nil && w.current.problem == "" {
		w.current.problem = problem
	}
}

// base lowers the innermost building's z, overall and in its GroundSurface.
func (w *openingWalk) base(z float64) {
	owner := last(w.owners)
	if owner == nil {
		return
	}

	if !owner.hasAnyZ || z < owner.anyMinZ {
		owner.anyMinZ, owner.hasAnyZ = z, true
	}

	if lastString(w.surfaces) == "GroundSurface" && (!owner.hasGround || z < owner.groundMinZ) {
		owner.groundMinZ, owner.hasGround = z, true
	}
}

// close counts a closed opening and keeps a window that can be placed,
// recording why one cannot.
func (w *openingWalk) close(o pendingOpening) {
	if o.door {
		w.report.Doors++

		return
	}

	w.report.Windows++

	building := ""
	if o.owner != nil {
		building = o.owner.id
	}

	var reason string

	switch {
	case o.id == "":
		reason = "it has no gml:id to name the window by"
	case building == "":
		reason = "it belongs to no Building or BuildingPart with a gml:id"
	case o.surface == "":
		reason = "it is in no boundary surface, so not in a facade"
	case o.surface != "WallSurface":
		reason = "it is in a " + o.surface + ", not a WallSurface, so not in a facade"
	case o.problem != "":
		reason = o.problem
	case len(o.vertices) < 3:
		reason = "it has no inline outline of 3 or more vertices: an outline given by reference (xlink:href) " +
			"or as an implicit geometry is not read"
	}

	if reason != "" {
		w.report.Skipped = append(w.report.Skipped, SkippedOpening{ID: o.id, Building: building, Reason: reason})

		return
	}

	w.kept = append(w.kept, o)
}

// finish resolves each kept window's base, now that every building has been
// read whole: a GroundSurface may follow the WallSurface the window is in.
func (w *openingWalk) finish() ([]Opening, *OpeningReport) {
	openings := make([]Opening, 0, len(w.kept))

	for _, o := range w.kept {
		var base float64

		switch {
		case o.owner.hasGround:
			base = o.owner.groundMinZ
		case o.owner.hasAnyZ:
			base = o.owner.anyMinZ
		default:
			w.report.Skipped = append(w.report.Skipped, SkippedOpening{
				ID: o.id, Building: o.owner.id, Reason: "its building states no z to measure the sill from",
			})

			continue
		}

		openings = append(openings, Opening{ID: o.id, BuildingID: o.owner.id, Vertices: o.vertices, BaseZ: base})
	}

	return openings, &w.report
}

func last(owners []*openingOwner) *openingOwner {
	if len(owners) == 0 {
		return nil
	}

	return owners[len(owners)-1]
}

func lastString(list []string) string {
	if len(list) == 0 {
		return ""
	}

	return list[len(list)-1]
}
