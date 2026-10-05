package reporting

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/results"
)

// bandsWithUnits renders a raster's bands as "name (unit)", in band order.
//
// One column rather than a names column beside a unit column, because the
// units are per band: a single unit cell next to four names either repeats
// itself or summarises away the disagreement it exists to show. Iterating
// BandNames and not the map also keeps the order the raster declares.
func bandsWithUnits(meta rasterMetaEnvelope) string {
	if len(meta.BandNames) == 0 {
		return ""
	}

	parts := make([]string, 0, len(meta.BandNames))

	for _, name := range meta.BandNames {
		unit, ok := meta.Units[name]
		if !ok {
			unit = meta.LegacyUnit
		}

		if unit == "" {
			parts = append(parts, name)
			continue
		}

		parts = append(parts, fmt.Sprintf("%s (%s)", name, unit))
	}

	return strings.Join(parts, ", ")
}

func kvPairsFromMap(values map[string]string) []kvPairView {
	if len(values) == 0 {
		return []kvPairView{}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]kvPairView, 0, len(keys))
	for _, key := range keys {
		out = append(out, kvPairView{Key: key, Value: values[key]})
	}

	return out
}

func kindCountsFromMap(values map[string]int) []kindCountView {
	if len(values) == 0 {
		return []kindCountView{}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]kindCountView, 0, len(keys))
	for _, key := range keys {
		out = append(out, kindCountView{Kind: key, Count: values[key]})
	}

	return out
}

func buildIndicatorStats(table results.ReceiverTable) []indicatorView {
	stats := make([]indicatorView, 0, len(table.IndicatorOrder))
	for _, indicator := range table.IndicatorOrder {
		minValue := math.Inf(1)
		maxValue := math.Inf(-1)
		sum := 0.0
		count := 0

		for _, record := range table.Records {
			value := record.Values[indicator]
			if value < minValue {
				minValue = value
			}

			if value > maxValue {
				maxValue = value
			}

			sum += value
			count++
		}

		if count == 0 {
			continue
		}

		stats = append(stats, indicatorView{
			Indicator: indicator,
			Unit:      table.Units[indicator],
			Min:       minValue,
			Mean:      sum / float64(count),
			Max:       maxValue,
		})
	}

	return stats
}

func optionalInt(value any) *int {
	switch typed := value.(type) {
	case float64:
		v := int(math.Round(typed))
		return &v
	case int:
		v := typed
		return &v
	case int64:
		v := int(typed)
		return &v
	default:
		return nil
	}
}

func optionalIntString(value *int) string {
	if value == nil {
		return ""
	}

	return strconv.Itoa(*value)
}
