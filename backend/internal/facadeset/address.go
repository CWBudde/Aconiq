package facadeset

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// ResolveBuildings turns the --facade-building values into building feature
// ids, in the order given and each once. A value that is the id of a building
// is that building. Otherwise it is read as "<street> <number>" and compared
// with each building's addr:street and addr:housenumber (OSM, and LGLN LoD2
// through citygmlimport, write both), exactly, but without case and extra
// spaces; addr:housenumber may be a ";" list, as OSM spells it. No
// abbreviation is expanded: "Musterstr. 1" is not "Musterstraße 1".
//
// An address must name one building. The parts LoD2 splits a building into
// share its address and their citygml_parent_id, and are selected together;
// the party walls between them carry no receivers. Two buildings sharing an
// address are refused instead of guessed between, and are named so the user
// can pick one by id.
func ResolveBuildings(selectors []string, model modelgeojson.Model) ([]string, error) {
	index := newAddressIndex(model)

	var ids []string

	for _, selector := range selectors {
		resolved, err := index.resolve(selector)
		if err != nil {
			return nil, err
		}

		for _, id := range resolved {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}

	return ids, nil
}

// addressIndex is the model's buildings by id and by address.
type addressIndex struct {
	isBuilding map[string]bool
	// buildings maps a normalised "<street> <number>" to the buildings
	// carrying it, in model order; members maps a building to its feature
	// ids (one, or its LoD2 parts).
	buildings map[string][]string
	members   map[string]map[string][]string
}

func newAddressIndex(model modelgeojson.Model) addressIndex {
	index := addressIndex{
		isBuilding: make(map[string]bool),
		buildings:  make(map[string][]string),
		members:    make(map[string]map[string][]string),
	}

	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindBuilding {
			continue
		}

		index.isBuilding[feature.ID] = true

		street, _ := feature.Properties["addr:street"].(string)
		numbers, _ := feature.Properties["addr:housenumber"].(string)

		building := feature.ID
		if parent, ok := feature.Properties["citygml_parent_id"].(string); ok && parent != "" {
			building = parent
		}

		for number := range strings.SplitSeq(numbers, ";") {
			if strings.TrimSpace(street) != "" && strings.TrimSpace(number) != "" {
				index.add(NormalizeAddress(street+" "+number), building, feature.ID)
			}
		}
	}

	return index
}

func (x addressIndex) add(key, building, featureID string) {
	if x.members[key] == nil {
		x.members[key] = make(map[string][]string)
	}

	if _, seen := x.members[key][building]; !seen {
		x.buildings[key] = append(x.buildings[key], building)
	}

	x.members[key][building] = append(x.members[key][building], featureID)
}

// resolve returns the feature ids one --facade-building value names.
func (x addressIndex) resolve(selector string) ([]string, error) {
	if x.isBuilding[selector] {
		return []string{selector}, nil
	}

	key := NormalizeAddress(selector)
	buildings := x.buildings[key]

	switch len(buildings) {
	case 0:
		return nil, domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
			"--facade-building %q names no building: no building feature has that id, and none has that address "+
				"(addr:street and addr:housenumber, compared without case and extra spaces)", selector,
		), nil)
	case 1:
		return x.members[key][buildings[0]], nil
	}

	var candidates []string

	for _, building := range buildings {
		for _, id := range x.members[key][building] {
			candidates = append(candidates, strconv.Quote(id))
		}
	}

	return nil, domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
		"--facade-building %q is the address of %d buildings (%s); name the one you mean by its feature id",
		selector, len(buildings), strings.Join(candidates, ", "),
	), nil)
}

// NormalizeAddress folds case and collapses whitespace, and nothing else.
func NormalizeAddress(address string) string {
	return strings.ToLower(strings.Join(strings.Fields(address), " "))
}
