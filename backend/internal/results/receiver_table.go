package results

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// ReceiverRecord stores one receiver location and indicator values.
//
// PlacementRule, Facade and Storey name the building, facade and storey of a
// facade receiver (docs/research/facade-receivers.md, "Receiver properties").
// They come as a set: a facade receiver carries all three, any other receiver
// none of them. All three are omitted from the JSON when absent, so a grid or
// custom table without them keeps the bytes it had before they existed.
type ReceiverRecord struct {
	ID      string             `json:"id"`
	X       float64            `json:"x"`
	Y       float64            `json:"y"`
	HeightM float64            `json:"height_m"`
	Values  map[string]float64 `json:"values"`

	PlacementRule string          `json:"placement_rule,omitempty"`
	Facade        *ReceiverFacade `json:"facade,omitempty"`
	// Storey is the storey index, 0 being the ground floor.
	Storey *int `json:"storey,omitempty"`
}

// ReceiverFacade names the footprint edge a facade receiver belongs to: the
// building, the polygon within it, the ring and the edge, by their indices in
// the canonical footprint, plus the edge's endpoints as a check.
//
// It has the JSON shape of geo.FacadeKey, with From and To as [x, y] arrays,
// and is declared here rather than imported so that results stays free of geo.
type ReceiverFacade struct {
	BuildingRef string     `json:"building_ref"`
	Part        int        `json:"part"`
	Ring        int        `json:"ring"`
	Edge        int        `json:"edge"`
	From        [2]float64 `json:"from"`
	To          [2]float64 `json:"to"`
}

// ReceiverTable stores tabular receiver output for one run.
type ReceiverTable struct {
	IndicatorOrder []string `json:"indicator_order"`
	// Units maps each entry of IndicatorOrder to the unit its values carry.
	// See units.go for why it is a map and not a parallel slice.
	Units   map[string]string `json:"units"`
	Records []ReceiverRecord  `json:"records"`
}

func (t ReceiverTable) Validate() error {
	err := t.validateIndicatorOrder()
	if err != nil {
		return err
	}

	err = validateUnits("receiver table", t.IndicatorOrder, t.Units)
	if err != nil {
		return err
	}

	seenReceivers := make(map[string]struct{}, len(t.Records))
	for i, record := range t.Records {
		err = t.validateRecord(i, record, seenReceivers)
		if err != nil {
			return err
		}

		seenReceivers[record.ID] = struct{}{}
	}

	return nil
}

func (t ReceiverTable) SortedRecordIDs() []string {
	ids := make([]string, 0, len(t.Records))
	for _, r := range t.Records {
		ids = append(ids, r.ID)
	}

	sort.Strings(ids)

	return ids
}

// validateIndicatorOrder checks that indicator_order is present, non-empty and free of duplicates.
func (t ReceiverTable) validateIndicatorOrder() error {
	if len(t.IndicatorOrder) == 0 {
		return errors.New("receiver table indicator_order is required")
	}

	seenIndicators := make(map[string]struct{}, len(t.IndicatorOrder))
	for _, indicator := range t.IndicatorOrder {
		if indicator == "" {
			return errors.New("receiver table indicator cannot be empty")
		}

		if _, exists := seenIndicators[indicator]; exists {
			return fmt.Errorf("receiver table indicator %q is duplicated", indicator)
		}

		seenIndicators[indicator] = struct{}{}
	}

	return nil
}

// validateRecord checks a single receiver record in the same order as the original inline checks.
func (t ReceiverTable) validateRecord(i int, record ReceiverRecord, seenReceivers map[string]struct{}) error {
	if record.ID == "" {
		return fmt.Errorf("receiver record[%d] id is required", i)
	}

	if _, exists := seenReceivers[record.ID]; exists {
		return fmt.Errorf("receiver record id %q is duplicated", record.ID)
	}

	if !isFinite(record.X) || !isFinite(record.Y) || !isFinite(record.HeightM) || record.HeightM < 0 {
		return fmt.Errorf("receiver record %q has invalid coordinates or height", record.ID)
	}

	err := validateRecordFacade(record)
	if err != nil {
		return err
	}

	return t.validateRecordValues(record)
}

// validateRecordFacade checks the optional facade attributes of a record:
// all present or all absent, and well-formed when present.
func validateRecordFacade(record ReceiverRecord) error {
	hasFacade := record.Facade != nil
	hasStorey := record.Storey != nil
	hasRule := record.PlacementRule != ""

	if !hasFacade && !hasStorey && !hasRule {
		return nil
	}

	if !hasFacade || !hasStorey || !hasRule {
		return fmt.Errorf("receiver record %q has incomplete facade attributes: placement_rule, facade and storey come together", record.ID)
	}

	if *record.Storey < 0 {
		return fmt.Errorf("receiver record %q has a negative storey", record.ID)
	}

	return validateFacade(record.ID, *record.Facade)
}

// validateFacade checks the facade key of the record named id.
func validateFacade(id string, facade ReceiverFacade) error {
	if facade.BuildingRef == "" {
		return fmt.Errorf("receiver record %q facade building_ref is required", id)
	}

	if facade.Part < 0 || facade.Ring < 0 || facade.Edge < 0 {
		return fmt.Errorf("receiver record %q facade has a negative part, ring or edge", id)
	}

	if !isFinite(facade.From[0]) || !isFinite(facade.From[1]) || !isFinite(facade.To[0]) || !isFinite(facade.To[1]) {
		return fmt.Errorf("receiver record %q facade has a non-finite from or to", id)
	}

	return nil
}

// validateRecordValues checks that a record carries a finite value for every ordered indicator.
func (t ReceiverTable) validateRecordValues(record ReceiverRecord) error {
	if len(record.Values) == 0 {
		return fmt.Errorf("receiver record %q has no indicator values", record.ID)
	}

	for _, indicator := range t.IndicatorOrder {
		value, ok := record.Values[indicator]
		if !ok {
			return fmt.Errorf("receiver record %q missing indicator %q", record.ID, indicator)
		}

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("receiver record %q indicator %q has non-finite value", record.ID, indicator)
		}
	}

	return nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
