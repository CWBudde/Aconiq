package facadeset

import (
	"fmt"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// WindowPlacement is one window as the elevation view draws or moves it
// (P18 C2): the building, the canonical part, ring and edge of the facade it
// is on, the rectangle in that facade's frame, and the review fields a
// person sets. ReviewStatus is proposed when empty, as a window list's is.
type WindowPlacement struct {
	ID           string           `json:"id"`
	BuildingRef  string           `json:"building_ref"`
	Part         int              `json:"part"`
	Ring         int              `json:"ring"`
	Edge         int              `json:"edge"`
	Frame        geo.FacadeWindow `json:"window"`
	Sensitive    bool             `json:"sensitive"`
	ReviewStatus string           `json:"review_status,omitempty"`
	Evidence     []any            `json:"evidence,omitempty"`
}

// PlaceWindow turns a placement into the window feature the model stores,
// through modelgeojson.WindowPlacer — the code `aconiq import --windows`
// places a window list with — so a window drawn in the view carries the same
// key, from / to in the model's CRS and Point at its centre as one imported
// from a list. It refuses what that import refuses: a building with no stable
// id, an edge the building does not have, a rectangle its edge has no room
// for.
func PlaceWindow(model modelgeojson.Model, w WindowPlacement) (modelgeojson.GeoJSONFeature, error) {
	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		return modelgeojson.GeoJSONFeature{}, fmt.Errorf("no metric frame to place window %q in: %w", w.ID, err)
	}

	feature, err := placer.Feature(modelgeojson.WindowSpec{
		ID:           w.ID,
		BuildingRef:  strings.TrimSpace(w.BuildingRef),
		Part:         w.Part,
		Ring:         w.Ring,
		Edge:         w.Edge,
		Frame:        w.Frame,
		Sensitive:    w.Sensitive,
		ReviewStatus: w.ReviewStatus,
		Evidence:     w.Evidence,
	})
	if err != nil {
		return modelgeojson.GeoJSONFeature{}, fmt.Errorf("the window cannot be placed: %w", err)
	}

	encoded := modelgeojson.Model{Features: []modelgeojson.Feature{feature}}.ToFeatureCollection().Features[0]
	encoded.ID = feature.ID

	return encoded, nil
}
