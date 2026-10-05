package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/results"
)

// Receiver match strategies, as recorded per record and counted in
// match_strategy_counts.
//
// There used to be a third, `ordinal`, which paired an Aconiq receiver with
// whatever SoundPLAN row happened to sit at the same position in file order.
// It is deleted rather than demoted: it is not a weaker match, it is a
// fabricated one, and every way of keeping it — behind a flag, excluded from
// the statistics — still ends in an artifact full of pairs that were never
// pairs. A receiver that cannot be matched is reported unmatched, by name.
const (
	matchStrategyKey         = "soundplan_key"
	matchStrategyCoordinates = "coordinates"
)

// soundPlanReceiverKey identifies one SoundPLAN receiver: the immission point
// it belongs to and which of that point's floors it is. It is the key
// RREC*.abs itself is indexed by, and with it the correspondence between the
// two receiver sets is a bijection by construction, so no assignment algorithm
// is needed or wanted.
type soundPlanReceiverKey struct {
	ObjID int64
	Floor int
}

// soundPlanReceiverMatch pairs one Aconiq receiver with one SoundPLAN row.
type soundPlanReceiverMatch struct {
	AconiqIndex    int
	SoundPlanIndex int
	Strategy       string
	DistanceM      float64
}

// soundPlanMatchResult is everything the matcher decided, including what it
// refused to decide.
type soundPlanMatchResult struct {
	Matches            []soundPlanReceiverMatch
	UnmatchedAconiq    []string
	UnmatchedSoundPlan []string
	Warnings           []string
	StrategyCounts     map[string]int
	MaxDistanceM       float64
}

// matchSoundPlanReceivers pairs the Aconiq receiver table with the SoundPLAN
// receiver rows.
//
// Receivers that carry a SoundPLAN key — which every receiver an
// `--from-soundplan` import produced does — are matched on it alone. The
// coordinate distance of such a pair is recorded and asserted against tolM,
// not searched over: it exists to catch a key that points at the wrong place,
// and in the reference project every pair agrees to within 2.3e-6 m.
//
// Receivers with no key at all — a hand-built receiver set compared against a
// SoundPLAN run — fall back to coordinates. That fallback is a *mutual*
// nearest-neighbour within tolM: a pair is formed only when each side is the
// other's nearest, which is what makes the result independent of the order the
// two inputs arrive in. The previous greedy first-come matcher was not.
//
// Two things are refused rather than resolved, because no answer would be the
// right one: a receiver carrying a SoundPLAN identity with floor 0, which the
// import emits for an immission point it could not expand, and two receivers
// carrying the same key.
func matchSoundPlanReceivers(
	table results.ReceiverTable,
	keys map[string]soundPlanReceiverKey,
	soundPlan []soundplanimport.ReceiverResult,
	tolM float64,
) (soundPlanMatchResult, error) {
	byKey, err := indexSoundPlanReceiversByKey(soundPlan)
	if err != nil {
		return soundPlanMatchResult{}, err
	}

	out := soundPlanMatchResult{
		Matches:        make([]soundPlanReceiverMatch, 0, len(table.Records)),
		StrategyCounts: map[string]int{},
	}

	used := make([]bool, len(soundPlan))

	unkeyed, err := matchSoundPlanReceiversByKey(table, keys, soundPlan, byKey, used, &out)
	if err != nil {
		return soundPlanMatchResult{}, err
	}

	coordinateMatches := matchSoundPlanReceiversByCoordinates(table, unkeyed, soundPlan, used, tolM)
	for _, match := range coordinateMatches {
		used[match.SoundPlanIndex] = true

		out.Matches = append(out.Matches, match)
	}

	slices.SortFunc(out.Matches, func(a, b soundPlanReceiverMatch) int {
		return a.AconiqIndex - b.AconiqIndex
	})

	matchedAconiq := make(map[int]struct{}, len(out.Matches))

	for _, match := range out.Matches {
		matchedAconiq[match.AconiqIndex] = struct{}{}
		out.StrategyCounts[match.Strategy]++
		out.MaxDistanceM = math.Max(out.MaxDistanceM, match.DistanceM)

		if match.DistanceM > tolM {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"receiver %s is %.3f m from the SoundPLAN row it is keyed to, beyond the %.3f m assertion tolerance",
				table.Records[match.AconiqIndex].ID, match.DistanceM, tolM,
			))
		}
	}

	for _, recordIndex := range unkeyed {
		if _, ok := matchedAconiq[recordIndex]; ok {
			continue
		}

		out.UnmatchedAconiq = append(out.UnmatchedAconiq, describeAconiqReceiver(table.Records[recordIndex], soundPlanReceiverKey{}))
	}

	for i, row := range soundPlan {
		if !used[i] {
			out.UnmatchedSoundPlan = append(out.UnmatchedSoundPlan, describeSoundPlanRow(row))
		}
	}

	return out, nil
}

// indexSoundPlanReceiversByKey indexes the reference rows by (ObjID, Floor).
//
// A duplicate key is a hard error, not something to resolve quietly: it means
// the row set spans more than one scenario — exactly what reading every RSPS*
// directory into one pool used to produce — and no answer the matcher could
// give would be the right one.
func indexSoundPlanReceiversByKey(soundPlan []soundplanimport.ReceiverResult) (map[soundPlanReceiverKey]int, error) {
	byKey := make(map[soundPlanReceiverKey]int, len(soundPlan))

	for i, row := range soundPlan {
		key := soundPlanReceiverKey{ObjID: int64(row.ObjID), Floor: int(row.Floor)}
		if _, exists := byKey[key]; exists {
			return nil, domainerrors.New(
				domainerrors.KindValidation, "cli.compare",
				fmt.Sprintf("SoundPLAN receiver results contain %s twice; the result set spans more than one calculation run", describeSoundPlanRow(row)),
				nil,
			)
		}

		byKey[key] = i
	}

	return byKey, nil
}

// matchSoundPlanReceiversByKey runs the keyed pass and returns the indices of
// the records that carry no SoundPLAN key at all, for the coordinate fallback.
//
// It appends matches and unmatched Aconiq receivers to out and marks the rows
// it consumed in used, both of which the caller owns.
func matchSoundPlanReceiversByKey(
	table results.ReceiverTable,
	keys map[string]soundPlanReceiverKey,
	soundPlan []soundplanimport.ReceiverResult,
	byKey map[soundPlanReceiverKey]int,
	used []bool,
	out *soundPlanMatchResult,
) ([]int, error) {
	unkeyed := make([]int, 0, len(table.Records))
	claimedBy := make(map[soundPlanReceiverKey]string, len(table.Records))

	for recordIndex, record := range table.Records {
		key, hasKey := keys[record.ID]
		if !hasKey {
			unkeyed = append(unkeyed, recordIndex)

			continue
		}

		// A SoundPLAN immission point whose floor attributes could not be
		// decoded becomes one receiver at the project default height carrying
		// floor 0, and docs/geojson-schema-v1.md says it has no usable key and
		// will not match. It is failed here rather than left to the coordinate
		// fallback: every row of that point's column shares the immission
		// point's X/Y, so a nearest-neighbour search would pair a guessed
		// height against whichever floor happens to sort first.
		if key.Floor <= 0 {
			out.UnmatchedAconiq = append(out.UnmatchedAconiq, describeAconiqReceiver(record, key))

			continue
		}

		// Two receivers claiming one reference row is the mirror of the
		// duplicate indexSoundPlanReceiversByKey rejects, and it is worse
		// undetected: both would be appended against the same row, so one
		// reference receiver would be counted twice in every aggregate while
		// the report showed no unmatched SoundPLAN row to say so.
		if first, claimed := claimedBy[key]; claimed {
			return nil, domainerrors.New(
				domainerrors.KindValidation, "cli.compare",
				fmt.Sprintf(
					"receivers %s and %s both carry soundplan_obj_id %d floor %d; one reference receiver cannot stand for two",
					first, record.ID, key.ObjID, key.Floor,
				),
				nil,
			)
		}

		claimedBy[key] = record.ID

		soundPlanIndex, found := byKey[key]
		if !found {
			out.UnmatchedAconiq = append(out.UnmatchedAconiq, describeAconiqReceiver(record, key))

			continue
		}

		used[soundPlanIndex] = true

		out.Matches = append(out.Matches, soundPlanReceiverMatch{
			AconiqIndex:    recordIndex,
			SoundPlanIndex: soundPlanIndex,
			Strategy:       matchStrategyKey,
			DistanceM:      soundPlanMatchDistance(record, soundPlan[soundPlanIndex]),
		})
	}

	return unkeyed, nil
}

// matchSoundPlanReceiversByCoordinates pairs the receivers that carry no
// SoundPLAN key with the rows still unused, by mutual nearest neighbour within
// tolM. O(n·m) and order-independent; ties fall to the lower index on both
// sides, so the result is fully determined by the inputs.
func matchSoundPlanReceiversByCoordinates(
	table results.ReceiverTable,
	unkeyed []int,
	soundPlan []soundplanimport.ReceiverResult,
	used []bool,
	tolM float64,
) []soundPlanReceiverMatch {
	if len(unkeyed) == 0 {
		return nil
	}

	nearestRow := make(map[int]int, len(unkeyed))

	for _, recordIndex := range unkeyed {
		if rowIndex, ok := nearestSoundPlanRow(table.Records[recordIndex], soundPlan, used, tolM); ok {
			nearestRow[recordIndex] = rowIndex
		}
	}

	matches := make([]soundPlanReceiverMatch, 0, len(nearestRow))

	for _, recordIndex := range unkeyed {
		rowIndex, ok := nearestRow[recordIndex]
		if !ok {
			continue
		}

		if nearestUnkeyedRecord(soundPlan[rowIndex], table, unkeyed) != recordIndex {
			continue
		}

		matches = append(matches, soundPlanReceiverMatch{
			AconiqIndex:    recordIndex,
			SoundPlanIndex: rowIndex,
			Strategy:       matchStrategyCoordinates,
			DistanceM:      soundPlanMatchDistance(table.Records[recordIndex], soundPlan[rowIndex]),
		})
	}

	return matches
}

func nearestSoundPlanRow(
	record results.ReceiverRecord,
	soundPlan []soundplanimport.ReceiverResult,
	used []bool,
	tolM float64,
) (int, bool) {
	bestIndex := -1
	bestDistance := math.Inf(1)

	for i, candidate := range soundPlan {
		if used[i] || !candidate.HasCoords {
			continue
		}

		distance := math.Hypot(record.X-candidate.X, record.Y-candidate.Y)
		if distance > tolM || distance >= bestDistance {
			continue
		}

		bestIndex = i
		bestDistance = distance
	}

	return bestIndex, bestIndex >= 0
}

func nearestUnkeyedRecord(
	row soundplanimport.ReceiverResult,
	table results.ReceiverTable,
	unkeyed []int,
) int {
	if !row.HasCoords {
		return -1
	}

	bestIndex := -1
	bestDistance := math.Inf(1)

	for _, recordIndex := range unkeyed {
		record := table.Records[recordIndex]

		distance := math.Hypot(record.X-row.X, record.Y-row.Y)
		if distance >= bestDistance {
			continue
		}

		bestIndex = recordIndex
		bestDistance = distance
	}

	return bestIndex
}

// soundPlanMatchDistance is the plan distance between a matched pair. A row
// without coordinates yields 0: there is nothing to assert, and a negative
// sentinel would sort as the best possible agreement.
func soundPlanMatchDistance(record results.ReceiverRecord, row soundplanimport.ReceiverResult) float64 {
	if !row.HasCoords {
		return 0
	}

	return math.Hypot(record.X-row.X, record.Y-row.Y)
}

func describeAconiqReceiver(record results.ReceiverRecord, key soundPlanReceiverKey) string {
	if key.ObjID > 0 {
		return fmt.Sprintf("%s (obj %d floor %d)", record.ID, key.ObjID, key.Floor)
	}

	return record.ID
}

func describeSoundPlanRow(row soundplanimport.ReceiverResult) string {
	name := strings.TrimSpace(row.Name)
	if name == "" {
		name = fmt.Sprintf("rec %d", row.RecNo)
	}

	return fmt.Sprintf("%s (obj %d floor %d)", name, row.ObjID, row.Floor)
}

// soundPlanReceiverKeysFromModel reads each receiver's SoundPLAN identity out
// of the model the run computed.
//
// The identity is read from the feature's properties rather than parsed back
// out of its ID, so the import is the one place that decides what a receiver
// is. It must be handed the *original* model: the raster comparison hands the
// run a temporary copy with thousands of synthetic receivers appended, which
// carry no SoundPLAN identity and are filtered out of this comparison anyway.
//
// A floor of 0 is kept rather than dropped. It is not a usable key, but it is
// still an identity, and matchSoundPlanReceivers needs to tell such a receiver
// apart from one that carries no SoundPLAN identity at all: the first must not
// match, the second falls back to coordinates.
func soundPlanReceiverKeysFromModel(model modelgeojson.Model) map[string]soundPlanReceiverKey {
	keys := make(map[string]soundPlanReceiverKey, len(model.Features))

	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindReceiver {
			continue
		}

		objID, hasObjID := propertyAsInt64(feature.Properties["soundplan_obj_id"])
		floor, hasFloor := propertyAsInt64(feature.Properties["soundplan_floor"])

		if !hasObjID || !hasFloor || objID <= 0 {
			continue
		}

		keys[feature.ID] = soundPlanReceiverKey{ObjID: objID, Floor: int(floor)}
	}

	return keys
}

// propertyAsInt64 reads a GeoJSON property as an integer. A model that came
// back through JSON carries float64 where the import wrote int, so both have
// to be accepted; a non-integral number is not an id and is rejected.
func propertyAsInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if typed != math.Trunc(typed) || math.IsInf(typed, 0) {
			return 0, false
		}

		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}

		return parsed, true
	default:
		return 0, false
	}
}
