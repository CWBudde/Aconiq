package results

import (
	"fmt"
	"math"
	"sort"
)

// ReceiverRecord stores one receiver location and indicator values.
type ReceiverRecord struct {
	ID      string             `json:"id"`
	X       float64            `json:"x"`
	Y       float64            `json:"y"`
	HeightM float64            `json:"height_m"`
	Values  map[string]float64 `json:"values"`
}

// ReceiverTable stores tabular receiver output for one run.
type ReceiverTable struct {
	IndicatorOrder []string         `json:"indicator_order"`
	Unit           string           `json:"unit"`
	Records        []ReceiverRecord `json:"records"`
}

func (t ReceiverTable) Validate() error {
	if len(t.IndicatorOrder) == 0 {
		return fmt.Errorf("receiver table indicator_order is required")
	}

	seenIndicators := make(map[string]struct{}, len(t.IndicatorOrder))
	for _, indicator := range t.IndicatorOrder {
		if indicator == "" {
			return fmt.Errorf("receiver table indicator cannot be empty")
		}
		if _, exists := seenIndicators[indicator]; exists {
			return fmt.Errorf("receiver table indicator %q is duplicated", indicator)
		}
		seenIndicators[indicator] = struct{}{}
	}

	seenReceivers := make(map[string]struct{}, len(t.Records))
	for i, record := range t.Records {
		if record.ID == "" {
			return fmt.Errorf("receiver record[%d] id is required", i)
		}
		if _, exists := seenReceivers[record.ID]; exists {
			return fmt.Errorf("receiver record id %q is duplicated", record.ID)
		}
		if !isFinite(record.X) || !isFinite(record.Y) || !isFinite(record.HeightM) || record.HeightM < 0 {
			return fmt.Errorf("receiver record %q has invalid coordinates or height", record.ID)
		}
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

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
