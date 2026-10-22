package cli

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
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// facadeReceiverSetID names the generated set, and so prefixes every
// generated receiver's id (geo.FacadeReceiverSet).
const facadeReceiverSetID = "facade"

// The keys a run with facade receivers adds to provenance metadata and the run
// summary. Each count is written only when it is non-zero, so a run without
// facade receivers records what it always did.
const (
	facadeProposedCountKey      = "facade_proposed_count"
	facadeAssumedHeightCountKey = "facade_assumed_height_count"
	facadeRejectedCountKey      = "facade_rejected_count"
	allowUnconfirmedKey         = "allow_unconfirmed"
)

// facadePlacementRuleByStandard is the geometric placement rule each standard
// takes its facade Immissionsorte from (facade-receivers.md, Section 4). A
// standard missing here has no rule a footprint alone can apply: TA Lärm's is
// the window, and the others assess no facade.
var facadePlacementRuleByStandard = map[string]string{
	rls19road.StandardID: geo.PlacementRLS19Facade,
	schall03.StandardID:  geo.PlacementSchall03Facade,
}

// placementRuleStandardPrefixes say which standard a placement rule belongs
// to, by the prefix every rule id of Section 4 carries. TA Lärm is computed by
// the ISO 9613-2 module.
var placementRuleStandardPrefixes = []struct {
	prefix     string
	standardID string
}{
	{"rls19-", rls19road.StandardID},
	{"schall03-", schall03.StandardID},
	{"talaerm-", iso9613.StandardID},
}

// facadeRequest is the facade part of a run request: the --facade-* flags and
// the opt-in.
type facadeRequest struct {
	buildings        []string
	spacingM         float64
	spacingSet       bool
	includeCourtyard bool
	allowUnconfirmed bool
}

// facadeAttributes is what a receiver carries about the facade it stands at.
// Every property is optional on a custom receiver, so each field may be unset.
type facadeAttributes struct {
	placementRule    string
	facade           *geo.FacadeKey
	storey           *int
	slabHeightSource string
	reviewStatus     string
}

// proposed reports whether nobody has confirmed the point yet. A
// facade-linked point that carries no review state has not been confirmed
// either.
func (a facadeAttributes) proposed() bool {
	return a.reviewStatus == geo.ReviewStatusProposed || a.reviewStatus == ""
}

func (a facadeAttributes) assumedHeight() bool {
	return a.slabHeightSource == geo.SlabHeightAssumed
}

// receiverSet is the receiver set a run computes at, as far as it is settled
// before the standard's module runs. For an automatic grid that is only the
// mode — the grid depends on the module's sources. For custom and facade
// receivers it is the whole set, read or generated once by the pipeline, so
// every module, the estimate and the gate see the same points.
type receiverSet struct {
	mode string

	// listed is the set of a custom or facade run, in the order it is
	// computed in.
	listed []geo.PointReceiver

	// facade holds the facade attributes of the listed receivers that carry
	// any, by receiver id.
	facade map[string]facadeAttributes

	proposed      int
	assumedHeight int
	rejected      int

	// metadata is what provenance.json records about the set beyond its mode.
	metadata map[string]string
}

// isListed reports whether the set is a list of points rather than a grid, so
// the run writes a receiver table and no raster.
func (s receiverSet) isListed() bool {
	return s.mode == receiverModeCustom || s.mode == receiverModeFacade
}

// tableRecord adds a receiver's facade attributes to its table row. They go in
// only as a complete set — rule, facade and storey — because that is the one
// shape results.ReceiverTable accepts; a partial set stays in the model.
func (s receiverSet) tableRecord(record results.ReceiverRecord) results.ReceiverRecord {
	attrs, ok := s.facade[record.ID]
	if !ok || attrs.placementRule == "" || attrs.facade == nil || attrs.storey == nil {
		return record
	}

	key := attrs.facade
	storey := *attrs.storey
	record.PlacementRule = attrs.placementRule
	record.Storey = &storey
	record.Facade = &results.ReceiverFacade{
		BuildingRef: key.BuildingRef,
		Part:        key.Part,
		Ring:        key.Ring,
		Edge:        key.Edge,
		From:        [2]float64{key.From.X, key.From.Y},
		To:          [2]float64{key.To.X, key.To.Y},
	}

	return record
}

// decorateSummary stamps the counts of unconfirmed and rejected facade points
// onto a run summary.
func (s receiverSet) decorateSummary(summary map[string]any) {
	for key, count := range s.counts() {
		if count > 0 {
			summary[key] = count
		}
	}
}

func (s receiverSet) counts() map[string]int {
	return map[string]int{
		facadeProposedCountKey:      s.proposed,
		facadeAssumedHeightCountKey: s.assumedHeight,
		facadeRejectedCountKey:      s.rejected,
	}
}

// resolveRunReceivers settles the receiver set of a run before the run exists,
// and refuses one that may not run: a facade request a standard has no rule
// for, and a normative run over facade points nobody has confirmed. The run
// and its estimate both call it, so both refuse in the same words.
func resolveRunReceivers(
	mode string,
	request facadeRequest,
	standard framework.ResolvedProfile,
	model modelgeojson.Model,
) (receiverSet, error) {
	err := request.validateFor(mode)
	if err != nil {
		return receiverSet{}, err
	}

	set := receiverSet{mode: mode, metadata: map[string]string{}}

	switch mode {
	case receiverModeCustom:
		set, err = customReceiverSet(set, model)
	case receiverModeFacade:
		set, err = generatedFacadeReceiverSet(set, request, standard, model)
	default:
		return set, nil
	}

	if err != nil {
		return receiverSet{}, err
	}

	set.proposed, set.assumedHeight = set.countUnconfirmed()

	err = gateFacadeReceivers(set, standard, model, request.allowUnconfirmed)
	if err != nil {
		return receiverSet{}, err
	}

	for key, count := range set.counts() {
		if count > 0 {
			set.metadata[key] = strconv.Itoa(count)
		}
	}

	if request.allowUnconfirmed {
		set.metadata[allowUnconfirmedKey] = "true"
	}

	return set, nil
}

// validateFor refuses facade flags outside the facade mode, and an incomplete
// facade request inside it. There is no default spacing: how densely a facade
// is sampled is the engineer's choice, and a default would make it for them.
func (r facadeRequest) validateFor(mode string) error {
	if mode != receiverModeFacade {
		if len(r.buildings) > 0 || r.spacingSet || r.includeCourtyard {
			return domainerrors.New(domainerrors.KindUserInput, "cli.run",
				"--facade-building, --facade-spacing-m and --facade-courtyard need --receiver-mode facade", nil)
		}

		return nil
	}

	if len(r.buildings) == 0 {
		return domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"receiver mode facade needs at least one --facade-building <building feature id>", nil)
	}

	if !r.spacingSet {
		return domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"receiver mode facade needs --facade-spacing-m, the station spacing along each facade; it has no default", nil)
	}

	return nil
}

// customReceiverSet reads the model's receivers and their facade properties.
// A rejected point is dropped here, before anything can compute at it.
func customReceiverSet(set receiverSet, model modelgeojson.Model) (receiverSet, error) {
	receivers, err := extractExplicitReceivers(model)
	if err != nil {
		return receiverSet{}, err
	}

	byID := make(map[string]modelgeojson.Feature, len(receivers))

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindReceiver {
			byID[feature.ID] = feature
		}
	}

	set.facade = make(map[string]facadeAttributes)
	set.listed = make([]geo.PointReceiver, 0, len(receivers))

	for _, receiver := range receivers {
		attrs, carries, err := facadeAttributesOf(byID[receiver.ID])
		if err != nil {
			return receiverSet{}, domainerrors.New(domainerrors.KindValidation, "cli.run",
				fmt.Sprintf("receiver %q: %v", receiver.ID, err), nil)
		}

		if carries && attrs.reviewStatus == geo.ReviewStatusRejected {
			set.rejected++

			continue
		}

		if carries {
			set.facade[receiver.ID] = attrs
		}

		set.listed = append(set.listed, receiver)
	}

	if len(set.listed) == 0 {
		return receiverSet{}, domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"custom receiver mode found no receiver to compute at: every receiver in the model is rejected", nil)
	}

	return set, nil
}

// facadeAttributesOf reads the facade properties of a receiver feature, and
// reports whether it carries any. Validation has already refused a malformed
// one, so an error here is a model that reached the run unvalidated.
func facadeAttributesOf(feature modelgeojson.Feature) (facadeAttributes, bool, error) {
	var attrs facadeAttributes

	carries := false

	for key, target := range map[string]*string{
		modelgeojson.PropertyPlacementRule:    &attrs.placementRule,
		modelgeojson.PropertySlabHeightSource: &attrs.slabHeightSource,
		modelgeojson.PropertyReviewStatus:     &attrs.reviewStatus,
	} {
		if value, ok := feature.Properties[key].(string); ok {
			*target = value
			carries = true
		}
	}

	if raw, ok := feature.Properties[modelgeojson.PropertyFacade]; ok && raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return facadeAttributes{}, false, fmt.Errorf("encode facade: %w", err)
		}

		var key geo.FacadeKey

		err = json.Unmarshal(encoded, &key)
		if err != nil {
			return facadeAttributes{}, false, fmt.Errorf("decode facade: %w", err)
		}

		attrs.facade = &key
		carries = true
	}

	if raw, ok := feature.Properties[modelgeojson.PropertyStorey].(float64); ok {
		if raw != math.Trunc(raw) || raw < 0 || raw > math.MaxInt32 {
			return facadeAttributes{}, false, fmt.Errorf("storey %v is not a non-negative integer", raw)
		}

		storey := int(raw)
		attrs.storey = &storey
		carries = true
	}

	return attrs, carries, nil
}

// generatedFacadeReceiverSet places the facade receivers of the requested
// buildings at the standard's placement rule. Every building of the model is
// handed over, not only the requested ones: each masks the others' party
// walls.
func generatedFacadeReceiverSet(
	set receiverSet,
	request facadeRequest,
	standard framework.ResolvedProfile,
	model modelgeojson.Model,
) (receiverSet, error) {
	rule, ok := facadePlacementRuleByStandard[standard.StandardID]
	if !ok {
		return receiverSet{}, domainerrors.New(domainerrors.KindUserInput, "cli.run", fmt.Sprintf(
			"standard %q has no facade placement rule a footprint alone can apply, so receiver mode facade is not available for it; "+
				"it is available for %s", standard.StandardID, strings.Join(slices.Sorted(maps.Keys(facadePlacementRuleByStandard)), " and "),
		), nil)
	}

	buildings := make([]geo.FacadeBuilding, 0)

	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindBuilding {
			continue
		}

		building, err := facadeBuildingFromFeature(feature)
		if err != nil {
			return receiverSet{}, domainerrors.New(domainerrors.KindValidation, "cli.run", fmt.Sprintf("building %q", feature.ID), err)
		}

		buildings = append(buildings, building)
	}

	generated, err := geo.FacadeReceiverSet{
		ID:               facadeReceiverSetID,
		BuildingIDs:      request.buildings,
		PlacementRule:    rule,
		SpacingM:         request.spacingM,
		IncludeCourtyard: request.includeCourtyard,
	}.Generate(buildings)
	if err != nil {
		return receiverSet{}, domainerrors.New(domainerrors.KindUserInput, "cli.run", "receiver mode facade: "+err.Error(), err)
	}

	if len(generated) == 0 {
		return receiverSet{}, domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"receiver mode facade placed no receiver: every facade of the requested buildings is a party wall", nil)
	}

	set.listed = make([]geo.PointReceiver, 0, len(generated))
	set.facade = make(map[string]facadeAttributes, len(generated))

	for _, receiver := range generated {
		key := receiver.Facade
		storey := receiver.Storey

		set.listed = append(set.listed, receiver.PointReceiver)
		set.facade[receiver.ID] = facadeAttributes{
			placementRule:    receiver.PlacementRule,
			facade:           &key,
			storey:           &storey,
			slabHeightSource: receiver.SlabHeightSource,
			reviewStatus:     receiver.ReviewStatus,
		}
	}

	// The request defines the set as much as the model does, so provenance
	// records it. The building ids are a JSON list, not a joined string: an id
	// is the model's spelling and may hold any separator.
	encodedBuildings, err := json.Marshal(slices.Sorted(slices.Values(request.buildings)))
	if err != nil {
		return receiverSet{}, domainerrors.New(domainerrors.KindInternal, "cli.run", "encode facade buildings", err)
	}

	set.metadata["facade_placement_rule"] = rule
	set.metadata["facade_buildings"] = string(encodedBuildings)
	set.metadata["facade_spacing_m"] = strconv.FormatFloat(request.spacingM, 'g', -1, 64)
	set.metadata["facade_include_courtyard"] = strconv.FormatBool(request.includeCourtyard)

	return set, nil
}

// facadeBuildingFromFeature maps a model building onto the generator's input.
//
// building:levels counts only as a positive integer; anything else is an
// unknown storey count, which the generator turns into an assumed slab height.
// A building with levels and no height_source is `levels`, never `measured`:
// the projects imported before osmimport wrote `levels` carry none, and their
// heights are levels × 3 m all the same. The ground-floor height is 0 until
// something records a Hochparterre (facade-receivers.md §7, question 1).
func facadeBuildingFromFeature(feature modelgeojson.Feature) (geo.FacadeBuilding, error) {
	polygons, err := polygonsFromFeature(feature, modelgeojson.FeatureKindBuilding)
	if err != nil {
		return geo.FacadeBuilding{}, err
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

	if building.HeightSource == "" && building.Levels > 0 {
		building.HeightSource = geo.SlabHeightLevels
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

// gateFacadeReceivers refuses a normative run over facade points it must not
// assess (facade-receivers.md, Decision: what the run does with `proposed`).
//
// A broken facade link — a building that no longer resolves, or an edge that
// moved — and a point placed by another standard's rule refuse the run
// outright: the opt-in does not lift them, because re-attaching a point is a
// reviewed step and the opt-in is not one. Proposed points and assumed slab
// heights refuse it unless the run opts in. A non-normative standard assesses
// nothing, so it runs over all of them.
func gateFacadeReceivers(set receiverSet, standard framework.ResolvedProfile, model modelgeojson.Model, allowUnconfirmed bool) error {
	if standard.EvidenceTier != framework.EvidenceTierNormative || len(set.facade) == 0 {
		return nil
	}

	blocked := brokenFacadeLinks(set, standard, model)
	if len(blocked) > 0 {
		return domainerrors.New(domainerrors.KindUserInput, "cli.run", fmt.Sprintf(
			"standard %q is normative and %d facade receiver finding(s) keep it from assessing them: %s; "+
				"re-attach or remove those receivers, --allow-unconfirmed does not apply to these",
			standard.StandardID, len(blocked), summarizeValidationErrors(blocked, 5),
		), nil)
	}

	if allowUnconfirmed || set.proposed+set.assumedHeight == 0 {
		return nil
	}

	return domainerrors.New(domainerrors.KindUserInput, "cli.run", fmt.Sprintf(
		"standard %q is normative and its receivers include %d proposed facade point(s) and %d with an assumed slab height: "+
			"nobody has confirmed where these Immissionsorte are, so their levels are not an assessment; "+
			"confirm them in the model (review_status: confirmed), or pass --allow-unconfirmed to run anyway and have the counts recorded with the result",
		standard.StandardID, set.proposed, set.assumedHeight,
	), nil)
}

// countUnconfirmed counts the listed points that are proposed, and those whose
// slab height is assumed. One point can be both.
func (s receiverSet) countUnconfirmed() (proposed, assumed int) {
	for _, receiver := range s.listed {
		attrs, ok := s.facade[receiver.ID]
		if !ok {
			continue
		}

		if attrs.proposed() {
			proposed++
		}

		if attrs.assumedHeight() {
			assumed++
		}
	}

	return proposed, assumed
}

// brokenFacadeLinks lists why a normative standard must not compute at some of
// the listed receivers: validation found their facade link broken, or another
// standard's rule placed them. Validation runs again here, on the model in the
// compute CRS, so its 1 mm tolerance is metres even when the project CRS is
// geographic.
func brokenFacadeLinks(set receiverSet, standard framework.ResolvedProfile, model modelgeojson.Model) []string {
	findings := make(map[string][]string)

	for _, issue := range modelgeojson.Validate(model).Warnings {
		if issue.Code == modelgeojson.CodeReceiverFacadeUnresolved || issue.Code == modelgeojson.CodeReceiverFacadeMoved {
			findings[issue.FeatureID] = append(findings[issue.FeatureID], issue.Code+": "+issue.Message)
		}
	}

	var blocked []string

	for _, receiver := range set.listed {
		attrs, ok := set.facade[receiver.ID]
		if !ok {
			continue
		}

		blocked = append(blocked, findings[receiver.ID]...)

		if attrs.placementRule == "" {
			continue
		}

		if owner := placementRuleStandard(attrs.placementRule); owner != standard.StandardID {
			if owner == "" {
				owner = "no standard"
			}

			blocked = append(blocked, fmt.Sprintf(
				"receiver %q: placement_rule %q places an Immissionsort of %s, not of %s",
				receiver.ID, attrs.placementRule, owner, standard.StandardID,
			))
		}
	}

	return blocked
}

func placementRuleStandard(rule string) string {
	for _, p := range placementRuleStandardPrefixes {
		if strings.HasPrefix(rule, p.prefix) {
			return p.standardID
		}
	}

	return ""
}
