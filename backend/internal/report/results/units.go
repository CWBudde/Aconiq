package results

import (
	"fmt"
	"maps"
	"sort"
)

// A result container names several channels — a receiver table's indicators, a
// raster's bands — and until now could declare only one unit for all of them.
// Where the channels disagreed, the writer had to pick a word that was true of
// none of them: beb-exposure wrote "mixed" over two decibel indicators and six
// counts, and every consumer then had to treat the whole container as
// unreadable. The unit is per channel because the values are.
//
// Keyed by name and not a slice parallel to the order, for the reason the band
// index is resolved by name everywhere else: a reorder of the names would
// silently relabel every value, and nothing downstream could detect it.

// The units this project's own containers declare. A module is free to write
// something else — the field is a string because the vocabulary is not closed
// — but these two are spelled once so the decibel gate on the frontend and the
// six count indicators of beb-exposure cannot drift apart by a capital letter.
const (
	// UnitDecibel is what every normative module writes.
	UnitDecibel = "dB"
	// UnitCount is a population or dwelling tally: a number of things, not a
	// level, and not paintable by a decibel ramp.
	UnitCount = "count"
)

// UniformUnits declares one unit for every channel.
//
// Most containers really do hold one kind of value throughout — every
// normative module writes decibels and nothing else — and this keeps their
// writers one line rather than making each spell out a map whose values are
// all the same. It is not a default: a writer whose channels disagree has to
// say so, which is the whole point of the field.
func UniformUnits(names []string, unit string) map[string]string {
	units := make(map[string]string, len(names))
	for _, name := range names {
		units[name] = unit
	}

	return units
}

// validateUnits checks that a unit map names every channel and nothing else.
//
// Both halves matter. A missing entry is a channel whose values cannot be
// labelled, which is the state this field exists to remove; an extra entry is
// a name that no longer exists, which is what a renamed indicator leaves
// behind and what would otherwise sit in the file until somebody noticed.
//
// The report is deterministic: the offending names are sorted before they are
// named, because they come out of a map.
func validateUnits(what string, names []string, units map[string]string) error {
	for _, name := range names {
		unit, ok := units[name]
		if !ok {
			return fmt.Errorf("%s units missing %q", what, name)
		}

		if unit == "" {
			return fmt.Errorf("%s unit for %q is empty", what, name)
		}
	}

	if len(units) == len(names) {
		return nil
	}

	declared := make(map[string]struct{}, len(names))
	for _, name := range names {
		declared[name] = struct{}{}
	}

	unknown := make([]string, 0, len(units)-len(names))

	for name := range units {
		if _, ok := declared[name]; !ok {
			unknown = append(unknown, name)
		}
	}

	sort.Strings(unknown)

	return fmt.Errorf("%s units name %v, which is not declared", what, unknown)
}

// CopyUnits returns a map the caller cannot write back through.
//
// The same reason Metadata() copies BandNames and Georeference: a shallow
// struct copy hands out the container's own map, and a caller that edited it
// would be editing the raster. Exported because a container rebuilt from
// another one — a filtered receiver table, say — has to carry the units
// across rather than re-deriving them from a channel list that did not change.
func CopyUnits(units map[string]string) map[string]string {
	if units == nil {
		return nil
	}

	out := make(map[string]string, len(units))
	maps.Copy(out, units)

	return out
}
