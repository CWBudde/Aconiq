package modelgeojson

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// Validation codes of the cross-checks between a building's windows and its
// geometry (P18 D3). Both are review flags: warnings that say the windows
// mapped from an image and the geometry the model holds disagree, and that a
// person should look. Neither refuses a save or a facade run, and neither
// corrects anything; which side is wrong is the reviewer's call.
//
// CodeBuildingLevelsDisagree is a building whose building:levels disagrees
// with its windows or with its height: one facade's windows stand in more
// rows than it has storeys, or a height_m that was not derived from the
// storey count gives a storey height no storey has.
//
// CodeFacadeWindowsNoCandidate is a facade that carries windows while facade
// mode places nothing on it, because another footprint stands against all of
// it: the image shows windows where the model has a party wall.
const (
	CodeBuildingLevelsDisagree   = "building.levels.disagrees"
	CodeFacadeWindowsNoCandidate = "facade.windows.no_candidate"
)

// Building properties the cross-checks read: the storey count an import
// keeps, and where height_m came from (osmimport writes levels or assumed;
// a tagged or measured height carries none, or measured).
const (
	PropertyBuildingLevels = "building:levels"
	PropertyHeightSource   = "height_source"
)

// StoreyRowGapM is the vertical gap between two sills, sorted, above which
// the higher window starts a new row. Windows of one storey sit within
// centimetres of each other, and a storey is at least MinStoreyHeightM tall,
// so 1 m separates storeys without splitting one.
const StoreyRowGapM = 1.0

// MinStoreyHeightM and MaxStoreyHeightM bound the storey height a building's
// height_m and building:levels may give before they are flagged as
// disagreeing: below a habitable room's clear height plus a slab, and above a
// tall Altbau or ground-floor shop storey. A ridge height divided by the
// storeys under the roof is the common way to exceed the upper bound.
const (
	MinStoreyHeightM = 2.2
	MaxStoreyHeightM = 4.5
)

// IssueFacade names the facade a finding is about, by the canonical indices
// its facade key stores; the finding's feature id is the building's.
type IssueFacade struct {
	Part int `json:"part"`
	Ring int `json:"ring"`
	Edge int `json:"edge"`
}

// BuildingLevels reads building:levels as a storey count: a positive integer,
// spelled as a JSON number or as the string an OSM tag is. Anything else —
// absent, zero, fractional, unparseable — is an unknown count, 0.
func BuildingLevels(raw any) int {
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

// facadeRef names one facade of one building, for collecting the windows on
// it.
type facadeRef struct {
	building         string
	part, ring, edge int
}

func compareFacadeRefs(a, b facadeRef) int {
	return cmp.Or(
		cmp.Compare(a.building, b.building),
		cmp.Compare(a.part, b.part),
		cmp.Compare(a.ring, b.ring),
		cmp.Compare(a.edge, b.edge),
	)
}

// crossCheck flags, after resolve, what the buildings' windows and their
// geometry disagree on. Only windows whose key resolved to its edge and that
// are not rejected count: a moved one is on no facade as the building is, and
// a rejected one is a window a person said is not there.
func (t *facadeTracker) crossCheck(model Model, report *ValidationReport) {
	t.checkLevels(model, report)
	t.checkPartyWallFacades(model, report)
}

// checkLevels reports each building whose building:levels disagrees with its
// height or with the rows its windows stand in, in model order.
func (t *facadeTracker) checkLevels(model Model, report *ValidationReport) {
	rows := t.windowRows()

	for _, feature := range model.Features {
		id := strings.TrimSpace(feature.ID)
		if featureKind(feature) != FeatureKindBuilding || id == "" {
			continue
		}

		levels := BuildingLevels(feature.Properties[PropertyBuildingLevels])
		if levels == 0 {
			continue
		}

		if storey, ok := storeyHeightFromHeight(feature, levels); ok && (storey < MinStoreyHeightM || storey > MaxStoreyHeightM) {
			addWarning(report, CodeBuildingLevelsDisagree, id, fmt.Sprintf(
				"building %q: height_m %g over building:levels %d gives %.2f m a storey, outside %g–%g m; "+
					"the count, or a height measured to the ridge, may be wrong",
				id, *feature.HeightM, levels, storey, MinStoreyHeightM, MaxStoreyHeightM,
			))
		}

		if facade, count, ok := mostRows(rows, id); ok && count > levels {
			addWarning(report, CodeBuildingLevelsDisagree, id, fmt.Sprintf(
				"building %q: the windows on facade (part %d, ring %d, edge %d) stand in %d rows, more than building:levels %d",
				id, facade.part, facade.ring, facade.edge, count, levels,
			))
		}
	}
}

// storeyHeightFromHeight is the storey height a building's height_m gives
// over its storey count, when that height says something the count does not:
// a height derived from the count (levels) or a default (assumed) is not
// compared, since it cannot disagree or is no evidence.
func storeyHeightFromHeight(feature Feature, levels int) (float64, bool) {
	if feature.HeightM == nil || !(*feature.HeightM > 0) || math.IsInf(*feature.HeightM, 0) {
		return 0, false
	}

	source, _ := feature.Properties[PropertyHeightSource].(string)
	if source == geo.SlabHeightLevels || source == geo.SlabHeightAssumed {
		return 0, false
	}

	return *feature.HeightM / float64(levels), true
}

// windowRows counts, per facade, the rows its counted windows' sills stand
// in: sorted, a sill more than StoreyRowGapM above the one below starts a new
// row.
func (t *facadeTracker) windowRows() map[facadeRef]int {
	rows := make(map[facadeRef]int, len(t.onFacade))

	for ref, sills := range t.onFacade {
		sorted := slices.Sorted(slices.Values(sills))
		count := 1

		for i := 1; i < len(sorted); i++ {
			if sorted[i]-sorted[i-1] > StoreyRowGapM {
				count++
			}
		}

		rows[ref] = count
	}

	return rows
}

// mostRows is the facade of building whose windows stand in the most rows,
// the first in canonical order on a tie.
func mostRows(rows map[facadeRef]int, building string) (facadeRef, int, bool) {
	var (
		best  facadeRef
		count int
	)

	for _, ref := range slices.SortedFunc(maps.Keys(rows), compareFacadeRefs) {
		if ref.building == building && rows[ref] > count {
			best, count = ref, rows[ref]
		}
	}

	return best, count, count > 0
}

// checkPartyWallFacades reports each facade carrying counted windows that
// another footprint stands against along its whole length, where the probe
// facade mode drops a point by (geo.PartyWallSpans) finds no free stretch:
// facade mode places no candidate there at any spacing. Measured in the frame
// validation measures every key in. A model without such windows costs
// nothing; one with them indexes every building once, since any footprint
// may stand against a facade.
func (t *facadeTracker) checkPartyWallFacades(model Model, report *ValidationReport) {
	if len(t.onFacade) == 0 {
		return
	}

	index := newReattachIndex(model)
	if index.frame.failure != "" {
		return
	}

	footprints := index.footprints(model)

	for _, ref := range slices.SortedFunc(maps.Keys(t.onFacade), compareFacadeRefs) {
		building, ok := index.byID[ref.building]
		if !ok {
			continue
		}

		for _, e := range index.buildings[building].edges {
			if e.Part != ref.part || e.Ring != ref.ring || e.Edge != ref.edge {
				continue
			}

			if wholeFacadeAgainst(e.From, e.To, footprints) {
				addFacadeWarning(report, CodeFacadeWindowsNoCandidate, ref, fmt.Sprintf(
					"building %q: facade (part %d, ring %d, edge %d) carries %d window(s), but another building stands against all of it, "+
						"so facade mode places no point there; the windows or the neighbouring footprint may be wrong",
					ref.building, ref.part, ref.ring, ref.edge, len(t.onFacade[ref]),
				))
			}

			break
		}
	}
}

// wholeFacadeAgainst reports whether the party walls of the facade from a to
// b cover it from end to end, within the tolerance a window may reach past
// its edge.
func wholeFacadeAgainst(a, b geo.Point2D, footprints [][][]geo.Point2D) bool {
	spans := geo.PartyWallSpans(a, b, footprints)

	return len(spans) == 1 &&
		spans[0].FromUM <= geo.FacadeWindowFitToleranceM &&
		spans[0].ToUM >= geo.Distance(a, b)-geo.FacadeWindowFitToleranceM
}

func addFacadeWarning(report *ValidationReport, code string, ref facadeRef, message string) {
	report.Warnings = append(report.Warnings, ValidationIssue{
		Level:     "warning",
		Code:      code,
		FeatureID: ref.building,
		Facade:    &IssueFacade{Part: ref.part, Ring: ref.ring, Edge: ref.edge},
		Message:   message,
	})
}
