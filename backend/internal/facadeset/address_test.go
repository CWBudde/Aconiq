package facadeset

import (
	"slices"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func addressedBuilding(id string, properties map[string]any) modelgeojson.Feature {
	return modelgeojson.Feature{ID: id, Kind: modelgeojson.FeatureKindBuilding, Properties: properties}
}

// A --facade-building value is a feature id first and an address second,
// matched exactly on the street and one of the house numbers, without case or
// extra spaces. An address names one building: the parts LoD2 splits a
// building into share it and are selected together, while two buildings
// sharing it are refused rather than guessed between.
func TestResolveBuildings(t *testing.T) {
	t.Parallel()

	model := modelgeojson.Model{Features: []modelgeojson.Feature{
		addressedBuilding("a", map[string]any{"addr:street": "Musterstraße", "addr:housenumber": "1"}),
		addressedBuilding("b", map[string]any{"addr:street": "Musterstraße", "addr:housenumber": "3;5"}),
		addressedBuilding("part_1", map[string]any{"addr:street": "Beispielweg", "addr:housenumber": "2", "citygml_parent_id": "B"}),
		addressedBuilding("part_2", map[string]any{"addr:street": "Beispielweg", "addr:housenumber": "2", "citygml_parent_id": "B"}),
		addressedBuilding("c", map[string]any{"addr:street": "Doppelweg", "addr:housenumber": "1"}),
		addressedBuilding("d", map[string]any{"addr:street": "Doppelweg", "addr:housenumber": "1"}),
		addressedBuilding("e", nil),
		{ID: "r", Kind: modelgeojson.FeatureKindReceiver, Properties: map[string]any{"addr:street": "Empfängerweg", "addr:housenumber": "1"}},
	}}

	for _, tc := range []struct {
		name      string
		selectors []string
		want      []string
	}{
		{"feature id", []string{"e"}, []string{"e"}},
		{"address", []string{"Musterstraße 1"}, []string{"a"}},
		{"address without case or extra spaces", []string{" musterSTRAßE   1 "}, []string{"a"}},
		{"one number of a list", []string{"Musterstraße 5"}, []string{"b"}},
		{"every part of one building", []string{"Beispielweg 2"}, []string{"part_1", "part_2"}},
		{"id and address of one building once", []string{"a", "Musterstraße 1", "b"}, []string{"a", "b"}},
	} {
		got, err := ResolveBuildings(tc.selectors, model)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: ResolveBuildings(%q) = %v, %v; want %v", tc.name, tc.selectors, got, err, tc.want)
		}
	}

	for _, tc := range []struct {
		name     string
		selector string
		parts    []string
	}{
		{"unknown", "Musterstraße 99", []string{`"Musterstraße 99"`, "no building"}},
		{"no abbreviation", "Musterstr. 1", []string{`"Musterstr. 1"`}},
		{"case is folded, spelling is not", "Musterstrasse 1", []string{`"Musterstrasse 1"`}},
		{"ambiguous", "Doppelweg 1", []string{"2 buildings", `"c"`, `"d"`, "feature id"}},
		{"a receiver is no building", "Empfängerweg 1", []string{"no building"}},
	} {
		_, err := ResolveBuildings([]string{tc.selector}, model)
		if err == nil || !domainerrors.IsUserError(err) {
			t.Errorf("%s: err = %v, want a user error", tc.name, err)

			continue
		}

		for _, part := range tc.parts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: refusal %q does not contain %q", tc.name, err, part)
			}
		}
	}
}
