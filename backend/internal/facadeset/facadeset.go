// Package facadeset turns a model and a facade request into the facade
// receivers a run computes at, and decides whether a normative run may assess
// them (facade-receivers.md, Sections 4 and 5).
//
// `aconiq run --receiver-mode facade` and the WASM kernel's `facadeReceivers`
// both come through here, so the two targets select the same buildings by the
// same address rule, place the same points and refuse the same runs in the
// same words. The words name the CLI's flags: the API and MCP already pass a
// refusal on verbatim, and a browser user reads the same sentence.
package facadeset

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// op names this package in the errors it returns.
const op = "run.facade"

// The standards facade receivers concern, by ID. They are spelled out rather
// than imported from the modules because the WASM kernel links this package,
// and importing schall03 and iso9613 for a constant would link both modules
// into a kernel that runs neither (wasmkernel.go). TestStandardIDsAreTheModules
// pins each to its module's StandardID.
const (
	rls19RoadID = "rls19-road"
	schall03ID  = "schall03"
	iso9613ID   = "iso9613"
)

// SetID names the generated set, and so prefixes every generated receiver's id
// (geo.FacadeReceiverSet).
const SetID = "facade"

// placementRuleByStandard is the geometric placement rule each standard takes
// its facade Immissionsorte from (facade-receivers.md, Section 4). A standard
// missing here has no rule a footprint alone can apply: TA Lärm's is the
// window, and the others assess no facade.
var placementRuleByStandard = map[string]string{
	rls19RoadID: geo.PlacementRLS19Facade,
	schall03ID:  geo.PlacementSchall03Facade,
}

// placementRuleStandardPrefixes say which standard a placement rule belongs
// to, by the prefix every rule id of Section 4 carries. TA Lärm is computed by
// the ISO 9613-2 module.
var placementRuleStandardPrefixes = []struct {
	prefix     string
	standardID string
}{
	{"rls19-", rls19RoadID},
	{"schall03-", schall03ID},
	{"talaerm-", iso9613ID},
}

// PlacementRule is the rule a standard places generated facade receivers by,
// and false for a standard that has none.
func PlacementRule(standardID string) (string, bool) {
	rule, ok := placementRuleByStandard[standardID]

	return rule, ok
}

// RuleStandard is the standard a placement rule belongs to, or "" for a rule
// of none.
func RuleStandard(rule string) string {
	for _, p := range placementRuleStandardPrefixes {
		if strings.HasPrefix(rule, p.prefix) {
			return p.standardID
		}
	}

	return ""
}

// Attributes is what a receiver carries about the facade it stands at. Every
// property is optional on a custom receiver, so each field may be unset.
type Attributes struct {
	PlacementRule    string
	Facade           *geo.FacadeKey
	Storey           *int
	SlabHeightSource string
	ReviewStatus     string
}

// Proposed reports whether nobody has confirmed the point yet. A
// facade-linked point that carries no review state has not been confirmed
// either.
func (a Attributes) Proposed() bool {
	return a.ReviewStatus == geo.ReviewStatusProposed || a.ReviewStatus == ""
}

// AssumedHeight reports whether the point's slab height is assumed.
func (a Attributes) AssumedHeight() bool {
	return a.SlabHeightSource == geo.SlabHeightAssumed
}

// AttributesOf reads the facade properties of a receiver feature, and reports
// whether it carries any. Validation has already refused a malformed one, so
// an error here is a model that reached the run unvalidated.
func AttributesOf(feature modelgeojson.Feature) (Attributes, bool, error) {
	var attrs Attributes

	carries := false

	for key, target := range map[string]*string{
		modelgeojson.PropertyPlacementRule:    &attrs.PlacementRule,
		modelgeojson.PropertySlabHeightSource: &attrs.SlabHeightSource,
		modelgeojson.PropertyReviewStatus:     &attrs.ReviewStatus,
	} {
		if value, ok := feature.Properties[key].(string); ok {
			*target = value
			carries = true
		}
	}

	if raw, ok := feature.Properties[modelgeojson.PropertyFacade]; ok && raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return Attributes{}, false, fmt.Errorf("encode facade: %w", err)
		}

		var key geo.FacadeKey

		err = json.Unmarshal(encoded, &key)
		if err != nil {
			return Attributes{}, false, fmt.Errorf("decode facade: %w", err)
		}

		attrs.Facade = &key
		carries = true
	}

	if raw, ok := feature.Properties[modelgeojson.PropertyStorey].(float64); ok {
		if raw != math.Trunc(raw) || raw < 0 || raw > math.MaxInt32 {
			return Attributes{}, false, fmt.Errorf("storey %v is not a non-negative integer", raw)
		}

		storey := int(raw)
		attrs.Storey = &storey
		carries = true
	}

	return attrs, carries, nil
}

// Request is the facade receiver set a run asks for: the buildings, by
// feature id or "<street> <number>", the station spacing along each facade,
// and whether courtyard facades carry points.
type Request struct {
	Buildings        []string
	SpacingM         float64
	IncludeCourtyard bool
}

// Listed is a run's listed receivers, in the order they are computed in, with
// the facade attributes of those that carry any, by receiver id.
type Listed struct {
	Receivers []geo.PointReceiver
	Facade    map[string]Attributes
}

// CountUnconfirmed counts the listed points that are proposed, and those
// whose slab height is assumed. One point can be both.
func (l Listed) CountUnconfirmed() (proposed, assumed int) {
	for _, receiver := range l.Receivers {
		attrs, ok := l.Facade[receiver.ID]
		if !ok {
			continue
		}

		if attrs.Proposed() {
			proposed++
		}

		if attrs.AssumedHeight() {
			assumed++
		}
	}

	return proposed, assumed
}

// Generate places the facade receivers of the requested buildings at the
// standard's placement rule, and returns them with what provenance records
// about the request. Every building of the model is handed over, not only the
// requested ones: each masks the others' party walls.
func Generate(request Request, standardID string, model modelgeojson.Model) (Listed, map[string]string, error) {
	rule, ok := placementRuleByStandard[standardID]
	if !ok {
		return Listed{}, nil, domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
			"standard %q has no facade placement rule a footprint alone can apply, so receiver mode facade is not available for it; "+
				"it is available for %s", standardID, strings.Join(slices.Sorted(maps.Keys(placementRuleByStandard)), " and "),
		), nil)
	}

	buildings := make([]geo.FacadeBuilding, 0)

	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindBuilding {
			continue
		}

		building, err := BuildingFromFeature(feature)
		if err != nil {
			return Listed{}, nil, domainerrors.New(domainerrors.KindValidation, op, fmt.Sprintf("building %q", feature.ID), err)
		}

		buildings = append(buildings, building)
	}

	buildingIDs, err := ResolveBuildings(request.Buildings, model)
	if err != nil {
		return Listed{}, nil, err
	}

	generated, err := geo.FacadeReceiverSet{
		ID:               SetID,
		BuildingIDs:      buildingIDs,
		PlacementRule:    rule,
		SpacingM:         request.SpacingM,
		IncludeCourtyard: request.IncludeCourtyard,
	}.Generate(buildings)
	if err != nil {
		return Listed{}, nil, domainerrors.New(domainerrors.KindUserInput, op, "receiver mode facade: "+err.Error(), err)
	}

	if len(generated) == 0 {
		return Listed{}, nil, domainerrors.New(domainerrors.KindUserInput, op,
			"receiver mode facade placed no receiver: every facade of the requested buildings is a party wall", nil)
	}

	listed := Listed{
		Receivers: make([]geo.PointReceiver, 0, len(generated)),
		Facade:    make(map[string]Attributes, len(generated)),
	}

	for _, receiver := range generated {
		key := receiver.Facade
		storey := receiver.Storey

		listed.Receivers = append(listed.Receivers, receiver.PointReceiver)
		listed.Facade[receiver.ID] = Attributes{
			PlacementRule:    receiver.PlacementRule,
			Facade:           &key,
			Storey:           &storey,
			SlabHeightSource: receiver.SlabHeightSource,
			ReviewStatus:     receiver.ReviewStatus,
		}
	}

	// The request defines the set as much as the model does, so provenance
	// records it. The building ids are a JSON list, not a joined string: an id
	// is the model's spelling and may hold any separator. They are the ids an
	// address resolved to, since those are what key the receivers.
	encodedBuildings, err := json.Marshal(slices.Sorted(slices.Values(buildingIDs)))
	if err != nil {
		return Listed{}, nil, domainerrors.New(domainerrors.KindInternal, op, "encode facade buildings", err)
	}

	metadata := map[string]string{
		"facade_placement_rule":    rule,
		"facade_buildings":         string(encodedBuildings),
		"facade_spacing_m":         strconv.FormatFloat(request.SpacingM, 'g', -1, 64),
		"facade_include_courtyard": strconv.FormatBool(request.IncludeCourtyard),
	}

	return listed, metadata, nil
}

// BuildingFromFeature maps a model building onto the generator's input.
//
// building:levels counts only as a positive integer; anything else is an
// unknown storey count, which the generator turns into an assumed slab height.
// A building with levels and no height_source is `levels`, never `measured`:
// the projects imported before osmimport wrote `levels` carry none, and their
// heights are levels × 3 m all the same. A building with neither — an OSM
// `height` tag alone, a CityGML or LGLN building — has no storey count, so it
// is `assumed`: that is what the generator makes of its slab heights anyway,
// and it must not stop the run as an unknown source. The ground-floor height
// is 0 until something records a Hochparterre (facade-receivers.md §7,
// question 1).
func BuildingFromFeature(feature modelgeojson.Feature) (geo.FacadeBuilding, error) {
	polygons, err := modelgeojson.FeaturePolygons(feature)
	if err != nil {
		return geo.FacadeBuilding{}, fmt.Errorf("footprint: %w", err)
	}

	building := geo.FacadeBuilding{
		ID:       feature.ID,
		Polygons: polygons,
		Levels:   buildingLevels(feature.Properties["building:levels"]),
	}

	if feature.HeightM != nil {
		building.HeightM = *feature.HeightM
	}

	if source, ok := feature.Properties["height_source"].(string); ok {
		building.HeightSource = source
	}

	if building.HeightSource == "" {
		building.HeightSource = geo.SlabHeightAssumed
		if building.Levels > 0 {
			building.HeightSource = geo.SlabHeightLevels
		}
	}

	return building, nil
}

func buildingLevels(raw any) int {
	var levels float64

	switch value := raw.(type) {
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}

		levels = float64(parsed)
	case float64:
		levels = value
	default:
		return 0
	}

	if levels <= 0 || levels != math.Trunc(levels) || levels > math.MaxInt32 {
		return 0
	}

	return int(levels)
}
