package geo

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// PlacementTALaermWindow is TA Lärm Anhang A.1.3 a): 0.5 m in front of the
// centre of the opened window of the most affected schutzbedürftiger Raum. It
// places only at a window: a facade without one carries no TA Lärm point.
const PlacementTALaermWindow = "talaerm-window"

// windowOnlyPlacementRules are the geometric rules a footprint alone cannot
// apply: they place at windows and nowhere else.
var windowOnlyPlacementRules = []string{PlacementTALaermWindow}

// IsWindowOnlyPlacementRule reports whether rule places only at windows.
func IsWindowOnlyPlacementRule(rule string) bool {
	return slices.Contains(windowOnlyPlacementRules, rule)
}

// Height bases of a point placed at a window (facade-receivers.md, Section 5,
// Decision): what its height was derived from, recorded beside it because a
// window replaces the storey arithmetic slab_height_source describes.
const (
	// HeightBasisWindowHead is Schall 03's own relation: 0.2 m above the
	// window head (16. BImSchV Anl. 2 Nr. 2.2.10).
	HeightBasisWindowHead = "window-head"
	// HeightBasisSchall03WindowHead is the same relation applied under
	// RLS-19, which gives none: the most defensible estimate of the slab
	// height, but borrowed from Schall 03, and recorded as borrowed.
	HeightBasisSchall03WindowHead = "schall03-window-head"
	// HeightBasisWindowCentre is TA Lärm's: the window centre.
	HeightBasisWindowCentre = "window-centre"
)

// windowHeadToSlabM is Schall 03's distance from the window head up to the
// slab height.
const windowHeadToSlabM = 0.2

// FacadeWindowFitToleranceM is how far a window may reach past the end of
// its edge and still fit it: the 1 mm a facade key's endpoints may move, so a
// window drawn to the corner fits after any reprojection.
const FacadeWindowFitToleranceM = 0.001

// FacadeWindow is a window as a rectangle in its facade's frame
// (facade-receivers.md, The facade frame): UM is the u of its left edge,
// SillZM the height of its sill above the ground at the facade, WidthM and
// HeightM its size, all in metres.
type FacadeWindow struct {
	UM      float64 `json:"u_m"`
	SillZM  float64 `json:"sill_z_m"`
	WidthM  float64 `json:"width_m"`
	HeightM float64 `json:"height_m"`
}

// CentreUM is the u of the window's centre.
func (w FacadeWindow) CentreUM() float64 { return w.UM + w.WidthM/2 }

// CentreZM is the height of the window's centre.
func (w FacadeWindow) CentreZM() float64 { return w.SillZM + w.HeightM/2 }

// HeadZM is the height of the window's head.
func (w FacadeWindow) HeadZM() float64 { return w.SillZM + w.HeightM }

// Validate checks the rectangle on its own, without the edge it stands on.
func (w FacadeWindow) Validate() error {
	for _, v := range []float64{w.UM, w.SillZM, w.WidthM, w.HeightM} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.New("u_m, sill_z_m, width_m and height_m must be finite")
		}
	}

	if w.UM < 0 || w.SillZM < 0 {
		return errors.New("u_m and sill_z_m must be >= 0")
	}

	if w.WidthM <= 0 || w.HeightM <= 0 {
		return errors.New("width_m and height_m must be > 0")
	}

	return nil
}

// Fits reports whether the window lies on an edge of the given length.
func (w FacadeWindow) Fits(edgeLengthM float64) bool {
	return w.UM+w.WidthM <= edgeLengthM+FacadeWindowFitToleranceM
}

// FacadeBuildingWindow is one window of a FacadeBuilding, on the edge its
// indices name in the canonical footprint. Generate places at every window it
// is given: choosing the sensitive ones, and dropping rejected ones, is the
// caller's.
type FacadeBuildingWindow struct {
	ID           string
	Part         int
	Ring         int
	Edge         int
	Frame        FacadeWindow
	ReviewStatus string
}

// windowPlacement is where a rule puts its point at a window: the offset out
// of the facade, the height, and what the height was derived from.
func windowPlacement(rule string, w FacadeWindow) (offsetM, z float64, basis string) {
	switch rule {
	case PlacementTALaermWindow:
		return geometricPlacementOffsetM[rule], w.CentreZM(), HeightBasisWindowCentre
	case PlacementSchall03Facade:
		return geometricPlacementOffsetM[rule], w.HeadZM() + windowHeadToSlabM, HeightBasisWindowHead
	default:
		return geometricPlacementOffsetM[rule], w.HeadZM() + windowHeadToSlabM, HeightBasisSchall03WindowHead
	}
}

type edgeIndex struct{ part, ring, edge int }

// buildingWindows checks a building's windows against its canonical
// footprint and files them by edge, each edge's in order along it.
func buildingWindows(building FacadeBuilding, parts [][][]Point2D) (map[edgeIndex][]FacadeBuildingWindow, error) {
	byEdge := make(map[edgeIndex][]FacadeBuildingWindow)
	seen := make(map[string]struct{}, len(building.Windows))

	for _, w := range building.Windows {
		if strings.TrimSpace(w.ID) == "" || strings.Contains(w.ID, "/") {
			return nil, fmt.Errorf("window id %q must be non-empty and must not contain '/'", w.ID)
		}

		if _, dup := seen[w.ID]; dup {
			return nil, fmt.Errorf("window %q is given twice", w.ID)
		}

		seen[w.ID] = struct{}{}

		if !slices.Contains([]string{ReviewStatusProposed, ReviewStatusConfirmed}, w.ReviewStatus) {
			return nil, fmt.Errorf("window %q: review status %q is neither %s nor %s",
				w.ID, w.ReviewStatus, ReviewStatusProposed, ReviewStatusConfirmed)
		}

		err := w.Frame.Validate()
		if err != nil {
			return nil, fmt.Errorf("window %q: %w", w.ID, err)
		}

		if w.Part < 0 || w.Part >= len(parts) || w.Ring < 0 || w.Ring >= len(parts[w.Part]) ||
			w.Edge < 0 || w.Edge >= len(parts[w.Part][w.Ring]) {
			return nil, fmt.Errorf("window %q: part %d, ring %d, edge %d is not an edge of the footprint", w.ID, w.Part, w.Ring, w.Edge)
		}

		ring := parts[w.Part][w.Ring]
		a, b := ring[w.Edge], ring[(w.Edge+1)%len(ring)]

		if length := Distance(a, b); !w.Frame.Fits(length) {
			return nil, fmt.Errorf("window %q: u_m %g + width_m %g runs past the end of its %g m edge", w.ID, w.Frame.UM, w.Frame.WidthM, length)
		}

		key := edgeIndex{w.Part, w.Ring, w.Edge}
		byEdge[key] = append(byEdge[key], w)
	}

	for _, windows := range byEdge {
		slices.SortFunc(windows, func(x, y FacadeBuildingWindow) int {
			return cmp.Or(cmp.Compare(x.Frame.CentreUM(), y.Frame.CentreUM()), cmp.Compare(x.ID, y.ID))
		})
	}

	return byEdge, nil
}

// storeyAt is the storey whose ceiling slab is the first at or above z: a
// label for the point, never its height. A window above the last slab
// belongs to the top storey.
func (s storeys) storeyAt(z float64) int {
	for n, slab := range s.z {
		if z <= slab {
			return n
		}
	}

	return len(s.z) - 1
}
