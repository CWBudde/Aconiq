package cli

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/report/results"
)

// The tests in this file carry no fixture.
//
// Everything that asserted matching behaviour used to sit behind a t.Skip that
// fires whenever the licensed SoundPLAN project is absent — which is every CI
// run. The matcher could therefore be arbitrarily wrong and CI stayed green.
// The algorithm is exercised here against hand-built tables instead, so the
// properties that matter hold in CI: the key decides the pairing, the order of
// the inputs does not, and nothing is ever paired by position.

// spRow builds a SoundPLAN receiver row.
func spRow(recNo int32, objID int32, floor int32, name string, x, y, zb1, zb2 float64) soundplanimport.ReceiverResult {
	return soundplanimport.ReceiverResult{
		RecNo: recNo, ObjID: objID, Floor: floor, Name: name,
		X: x, Y: y, ZB1: zb1, ZB2: zb2, HasCoords: true,
	}
}

// aconiqRecord builds one row of an Aconiq receiver table.
func aconiqRecord(id string, x, y, day, night float64) results.ReceiverRecord {
	return results.ReceiverRecord{
		ID: id, X: x, Y: y,
		Values: map[string]float64{"LrDay": day, "LrNight": night},
	}
}

func keyedTable() (results.ReceiverTable, map[string]soundPlanReceiverKey, []soundplanimport.ReceiverResult) {
	table := results.ReceiverTable{
		IndicatorOrder: []string{"LrDay", "LrNight"},
		Records: []results.ReceiverRecord{
			aconiqRecord("soundplan-receiver-101-f1", 10, 10, 70, 65),
			aconiqRecord("soundplan-receiver-101-f2", 10, 10, 72, 67),
			aconiqRecord("soundplan-receiver-202-f1", 90, 90, 60, 55),
		},
	}

	keys := map[string]soundPlanReceiverKey{
		"soundplan-receiver-101-f1": {ObjID: 101, Floor: 1},
		"soundplan-receiver-101-f2": {ObjID: 101, Floor: 2},
		"soundplan-receiver-202-f1": {ObjID: 202, Floor: 1},
	}

	// Deliberately in a different order from the Aconiq table, and with the
	// floors of one point interleaved, so list position cannot produce the
	// right answer by accident.
	rows := []soundplanimport.ReceiverResult{
		spRow(7, 202, 1, "Talweg 86", 90, 90, 50, 45),
		spRow(3, 101, 2, "Hauptstraße 4", 10, 10, 52, 47),
		spRow(3, 101, 1, "Hauptstraße 4", 10, 10, 51, 46),
	}

	return table, keys, rows
}

func TestMatchSoundPlanReceiversByKey(t *testing.T) {
	t.Parallel()

	table, keys, rows := keyedTable()

	matched, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if want := map[string]int{matchStrategyKey: 3}; !maps.Equal(matched.StrategyCounts, want) {
		t.Fatalf("strategy counts = %v, want %v", matched.StrategyCounts, want)
	}

	if len(matched.UnmatchedAconiq) != 0 || len(matched.UnmatchedSoundPlan) != 0 {
		t.Fatalf("unmatched = %v / %v, want none", matched.UnmatchedAconiq, matched.UnmatchedSoundPlan)
	}

	wantRowIndex := map[string]int{
		"soundplan-receiver-101-f1": 2,
		"soundplan-receiver-101-f2": 1,
		"soundplan-receiver-202-f1": 0,
	}

	for _, match := range matched.Matches {
		id := table.Records[match.AconiqIndex].ID
		if got := match.SoundPlanIndex; got != wantRowIndex[id] {
			t.Fatalf("%s matched row %d, want %d", id, got, wantRowIndex[id])
		}

		if match.DistanceM != 0 {
			t.Fatalf("%s distance_m = %v, want 0", id, match.DistanceM)
		}
	}
}

// TestMatchSoundPlanReceiversIsOrderIndependent is the property the greedy
// first-come matcher violated: which pairs come out must depend on the inputs
// only, never on the order they arrive in.
func TestMatchSoundPlanReceiversIsOrderIndependent(t *testing.T) {
	t.Parallel()

	table, keys, rows := keyedTable()

	baseline, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	reversedRows := slices.Clone(rows)
	slices.Reverse(reversedRows)

	// Reversing the reference rows must not change the result at all: the
	// matches are reported in Aconiq table order and identify their row by
	// value, so the encoded output is byte-identical.
	reversed, err := matchSoundPlanReceivers(table, keys, reversedRows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers (reversed rows): %v", err)
	}

	if got, want := encodeMatchPairs(t, table, reversedRows, reversed), encodeMatchPairs(t, table, rows, baseline); got != want {
		t.Fatalf("reversing the SoundPLAN rows changed the result:\n got %s\nwant %s", got, want)
	}

	reversedTable := results.ReceiverTable{
		IndicatorOrder: table.IndicatorOrder,
		Records:        slices.Clone(table.Records),
	}
	slices.Reverse(reversedTable.Records)

	// Reversing the Aconiq table reverses the order the matches are reported
	// in, so the pairs are compared as a set.
	flipped, err := matchSoundPlanReceivers(reversedTable, keys, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers (reversed table): %v", err)
	}

	got := sortedMatchPairs(reversedTable, rows, flipped)
	want := sortedMatchPairs(table, rows, baseline)

	if !slices.Equal(got, want) {
		t.Fatalf("reversing the Aconiq table changed the pairs:\n got %v\nwant %v", got, want)
	}
}

// TestMatchSoundPlanReceiversRefusesPositionalFallback pins the deletion of the
// `ordinal` strategy. Two receiver sets with no key and no shared geometry
// produce no matches, not a pairing by list position.
func TestMatchSoundPlanReceiversRefusesPositionalFallback(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{Records: []results.ReceiverRecord{
		aconiqRecord("r1", 0, 0, 70, 65),
		aconiqRecord("r2", 1, 1, 71, 66),
	}}

	rows := []soundplanimport.ReceiverResult{
		spRow(1, 901, 1, "far away", 5000, 5000, 50, 45),
		spRow(2, 902, 1, "also far", 6000, 6000, 51, 46),
	}

	matched, err := matchSoundPlanReceivers(table, map[string]soundPlanReceiverKey{}, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(matched.Matches) != 0 {
		t.Fatalf("matched %d pairs, want 0 — a receiver with no key and no neighbour is unmatched", len(matched.Matches))
	}

	if len(matched.UnmatchedAconiq) != 2 || len(matched.UnmatchedSoundPlan) != 2 {
		t.Fatalf("unmatched = %v / %v, want both sides fully enumerated", matched.UnmatchedAconiq, matched.UnmatchedSoundPlan)
	}
}

// TestMatchSoundPlanReceiversKeyMissFallsThroughToUnmatched checks that a key
// no reference row carries is an unmatched receiver rather than an excuse to
// look for something else.
func TestMatchSoundPlanReceiversKeyMissFallsThroughToUnmatched(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{Records: []results.ReceiverRecord{
		aconiqRecord("soundplan-receiver-101-f9", 10, 10, 70, 65),
	}}
	keys := map[string]soundPlanReceiverKey{"soundplan-receiver-101-f9": {ObjID: 101, Floor: 9}}
	rows := []soundplanimport.ReceiverResult{spRow(1, 101, 1, "Hauptstraße 4", 10, 10, 50, 45)}

	matched, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(matched.Matches) != 0 {
		t.Fatalf("matched %d pairs, want 0 — the row sits at the same coordinates but is a different floor", len(matched.Matches))
	}

	if len(matched.UnmatchedAconiq) != 1 || len(matched.UnmatchedSoundPlan) != 1 {
		t.Fatalf("unmatched = %v / %v, want one on each side", matched.UnmatchedAconiq, matched.UnmatchedSoundPlan)
	}
}

// TestMatchSoundPlanReceiversRejectsDuplicateKeys pins the union-of-scenarios
// defect: the same (ObjID, Floor) appearing twice means the row set spans more
// than one calculation run, and no matcher may resolve that quietly.
func TestMatchSoundPlanReceiversRejectsDuplicateKeys(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{Records: []results.ReceiverRecord{aconiqRecord("r1", 10, 10, 70, 65)}}
	keys := map[string]soundPlanReceiverKey{"r1": {ObjID: 101, Floor: 1}}
	rows := []soundplanimport.ReceiverResult{
		spRow(1, 101, 1, "Hauptstraße 4", 10, 10, 54.61, 58.38),
		spRow(1, 101, 1, "Hauptstraße 4", 10, 10, 46.56, 50.31),
	}

	if _, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM); err == nil {
		t.Fatal("expected a validation error for duplicate SoundPLAN receiver keys")
	}
}

// TestMatchSoundPlanReceiversCoordinateFallbackIsMutual covers the hand-built
// receiver set: no key at all, so coordinates decide — but only when each side
// is the other's nearest, which is what removes the order dependence.
func TestMatchSoundPlanReceiversCoordinateFallbackIsMutual(t *testing.T) {
	t.Parallel()

	// r1 and r2 both sit within tolerance of row A. Greedy matching pairs
	// whichever comes first; mutual nearest-neighbour pairs r1, which is the
	// closer of the two, and leaves r2 unmatched.
	table := results.ReceiverTable{Records: []results.ReceiverRecord{
		aconiqRecord("r1", 0.05, 0, 70, 65),
		aconiqRecord("r2", 0.30, 0, 71, 66),
	}}
	rows := []soundplanimport.ReceiverResult{spRow(1, 0, 0, "A", 0, 0, 50, 45)}

	matched, err := matchSoundPlanReceivers(table, map[string]soundPlanReceiverKey{}, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(matched.Matches) != 1 {
		t.Fatalf("matched %d pairs, want 1", len(matched.Matches))
	}

	if got := table.Records[matched.Matches[0].AconiqIndex].ID; got != "r1" {
		t.Fatalf("matched %q, want r1 — the nearer of the two candidates", got)
	}

	if matched.StrategyCounts[matchStrategyCoordinates] != 1 {
		t.Fatalf("strategy counts = %v, want one coordinate match", matched.StrategyCounts)
	}

	reversedTable := results.ReceiverTable{Records: []results.ReceiverRecord{table.Records[1], table.Records[0]}}

	flipped, err := matchSoundPlanReceivers(reversedTable, map[string]soundPlanReceiverKey{}, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers (reversed): %v", err)
	}

	if len(flipped.Matches) != 1 {
		t.Fatalf("matched %d pairs after reversing, want 1", len(flipped.Matches))
	}

	if got := reversedTable.Records[flipped.Matches[0].AconiqIndex].ID; got != "r1" {
		t.Fatalf("reversing the table matched %q, want r1", got)
	}
}

// TestMatchSoundPlanReceiversCoordinateToleranceIsASearchBound is the other
// half of the tolerance's double role: on a keyed match it is an assertion, but
// for a receiver with no key it is the only thing standing between "nearest"
// and "nearest of things hundreds of metres away".
func TestMatchSoundPlanReceiversCoordinateToleranceIsASearchBound(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{Records: []results.ReceiverRecord{aconiqRecord("r1", 0.2, 0, 70, 65)}}
	rows := []soundplanimport.ReceiverResult{spRow(1, 0, 0, "A", 0, 0, 50, 45)}

	within, err := matchSoundPlanReceivers(table, map[string]soundPlanReceiverKey{}, rows, 0.5)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(within.Matches) != 1 {
		t.Fatalf("matched %d pairs at 0.5 m, want 1", len(within.Matches))
	}

	beyond, err := matchSoundPlanReceivers(table, map[string]soundPlanReceiverKey{}, rows, 0.1)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(beyond.Matches) != 0 {
		t.Fatalf("matched %d pairs at 0.1 m, want 0", len(beyond.Matches))
	}
}

// TestMatchSoundPlanReceiversWarnsBeyondTolerance checks that the coordinate
// tolerance is an assertion on a keyed match, not a filter: the pair stands,
// the distance is recorded, and the comparison says so.
func TestMatchSoundPlanReceiversWarnsBeyondTolerance(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{Records: []results.ReceiverRecord{aconiqRecord("r1", 0, 0, 70, 65)}}
	keys := map[string]soundPlanReceiverKey{"r1": {ObjID: 101, Floor: 1}}
	rows := []soundplanimport.ReceiverResult{spRow(1, 101, 1, "Hauptstraße 4", 3, 4, 50, 45)}

	matched, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(matched.Matches) != 1 {
		t.Fatalf("matched %d pairs, want 1 — the key still decides", len(matched.Matches))
	}

	if matched.MaxDistanceM != 5 {
		t.Fatalf("max distance = %v, want 5", matched.MaxDistanceM)
	}

	if len(matched.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one naming the receiver", matched.Warnings)
	}
}

func TestSoundPlanReceiverKeysFromModel(t *testing.T) {
	t.Parallel()

	model := modelgeojson.Model{Features: []modelgeojson.Feature{
		{ID: "r-int", Kind: modelgeojson.FeatureKindReceiver, Properties: map[string]any{"soundplan_obj_id": 101, "soundplan_floor": 2}},
		// A model that came back through JSON carries float64.
		{ID: "r-float", Kind: modelgeojson.FeatureKindReceiver, Properties: map[string]any{"soundplan_obj_id": float64(202), "soundplan_floor": float64(1)}},
		{ID: "r-none", Kind: modelgeojson.FeatureKindReceiver, Properties: map[string]any{}},
		// Object id 0 means the binary layout was not recognised; it is not an
		// identity and must not become a key.
		{ID: "r-zero", Kind: modelgeojson.FeatureKindReceiver, Properties: map[string]any{"soundplan_obj_id": 0, "soundplan_floor": 0}},
		{ID: "b", Kind: modelgeojson.FeatureKindBuilding, Properties: map[string]any{"soundplan_obj_id": 303, "soundplan_floor": 1}},
	}}

	keys := soundPlanReceiverKeysFromModel(model)

	want := map[string]soundPlanReceiverKey{
		"r-int":   {ObjID: 101, Floor: 2},
		"r-float": {ObjID: 202, Floor: 1},
	}

	if !maps.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
}

// TestSelectSoundPlanReceiverResultDir covers the other half of the defect: two
// runs of the same receivers that differ only in the geometry they used.
func TestSelectSoundPlanReceiverResultDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFakeSoundPlanRun(t, root, "RSPS0011", `"GeoObjs.geo" "GeoRail.geo"`)
	writeFakeSoundPlanRun(t, root, "RSPS0021", `"GeoObjs.geo" "GeoWand.geo" "GeoRail.geo"`)

	withBarrier, err := selectSoundPlanReceiverResultDir(root, "", true)
	if err != nil {
		t.Fatalf("selectSoundPlanReceiverResultDir: %v", err)
	}

	if withBarrier.Dir != "RSPS0021" || withBarrier.Selection != resultRunSelectionGeometry {
		t.Fatalf("with barriers: dir = %q (%s), want RSPS0021 (%s)", withBarrier.Dir, withBarrier.Selection, resultRunSelectionGeometry)
	}

	if !slices.Equal(withBarrier.Candidates, []string{"RSPS0011", "RSPS0021"}) {
		t.Fatalf("candidates = %v, want both runs", withBarrier.Candidates)
	}

	withoutBarrier, err := selectSoundPlanReceiverResultDir(root, "", false)
	if err != nil {
		t.Fatalf("selectSoundPlanReceiverResultDir: %v", err)
	}

	if withoutBarrier.Dir != "RSPS0011" || withoutBarrier.Selection != resultRunSelectionGeometry {
		t.Fatalf("without barriers: dir = %q (%s), want RSPS0011", withoutBarrier.Dir, withoutBarrier.Selection)
	}

	explicit, err := selectSoundPlanReceiverResultDir(root, "RSPS0011", true)
	if err != nil {
		t.Fatalf("selectSoundPlanReceiverResultDir (explicit): %v", err)
	}

	if explicit.Dir != "RSPS0011" || explicit.Selection != resultRunSelectionExplicit {
		t.Fatalf("explicit: dir = %q (%s), want RSPS0011 (%s)", explicit.Dir, explicit.Selection, resultRunSelectionExplicit)
	}

	if _, err := selectSoundPlanReceiverResultDir(root, "RSPS9999", true); err == nil {
		t.Fatal("expected an error for a result run that does not exist")
	}
}

// TestSelectSoundPlanReceiverResultDirAmbiguous checks that a choice nothing
// discriminates is recorded as one, rather than made silently.
func TestSelectSoundPlanReceiverResultDirAmbiguous(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFakeSoundPlanRun(t, root, "RSPS0011", `"GeoObjs.geo"`)
	writeFakeSoundPlanRun(t, root, "RSPS0021", `"GeoObjs.geo"`)

	selection, err := selectSoundPlanReceiverResultDir(root, "", false)
	if err != nil {
		t.Fatalf("selectSoundPlanReceiverResultDir: %v", err)
	}

	if selection.Dir != "RSPS0021" || selection.Selection != resultRunSelectionAmbiguous {
		t.Fatalf("dir = %q (%s), want RSPS0021 (%s)", selection.Dir, selection.Selection, resultRunSelectionAmbiguous)
	}

	if len(selection.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one saying the choice was arbitrary", selection.Warnings)
	}
}

func TestSelectSoundPlanReceiverResultDirWithoutCandidates(t *testing.T) {
	t.Parallel()

	if _, err := selectSoundPlanReceiverResultDir(t.TempDir(), "", false); err == nil {
		t.Fatal("expected an error when no RSPS result directory carries a receiver table")
	}
}

// writeFakeSoundPlanRun lays out the minimum a result run needs to be
// discovered: a directory with an RREC table, and a .res naming the geometry.
func writeFakeSoundPlanRun(t *testing.T, root string, name string, runData string) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	suffix := compareExtractRunSuffix(name)
	if err := os.WriteFile(filepath.Join(dir, "RREC"+suffix+".abs"), []byte("not parsed here"), 0o600); err != nil {
		t.Fatalf("write RREC: %v", err)
	}

	res := "[General]\nResultSubFolder=" + name + "\nRunData=\"" + runData + "\"\n"
	if err := os.WriteFile(filepath.Join(root, name+".res"), []byte(res), 0o600); err != nil {
		t.Fatalf("write res: %v", err)
	}
}

// encodeMatchPairs renders a match result as stable JSON so two runs can be
// compared byte for byte.
func encodeMatchPairs(t *testing.T, table results.ReceiverTable, rows []soundplanimport.ReceiverResult, matched soundPlanMatchResult) string {
	t.Helper()

	type pair struct {
		Aconiq    string  `json:"aconiq"`
		RecNo     int32   `json:"rec_no"`
		ObjID     int32   `json:"obj_id"`
		Floor     int32   `json:"floor"`
		Strategy  string  `json:"strategy"`
		DistanceM float64 `json:"distance_m"`
	}

	pairs := make([]pair, 0, len(matched.Matches))
	for _, match := range matched.Matches {
		row := rows[match.SoundPlanIndex]
		pairs = append(pairs, pair{
			Aconiq:    table.Records[match.AconiqIndex].ID,
			RecNo:     row.RecNo,
			ObjID:     row.ObjID,
			Floor:     row.Floor,
			Strategy:  match.Strategy,
			DistanceM: match.DistanceM,
		})
	}

	payload, err := json.Marshal(struct {
		Pairs              []pair   `json:"pairs"`
		UnmatchedAconiq    []string `json:"unmatched_aconiq"`
		UnmatchedSoundPlan []string `json:"unmatched_soundplan"`
	}{pairs, matched.UnmatchedAconiq, matched.UnmatchedSoundPlan})
	if err != nil {
		t.Fatalf("encode match pairs: %v", err)
	}

	return string(payload)
}

// sortedMatchPairs renders the pairs as a sorted set, for comparisons where
// the reporting order is legitimately expected to differ.
func sortedMatchPairs(table results.ReceiverTable, rows []soundplanimport.ReceiverResult, matched soundPlanMatchResult) []string {
	pairs := make([]string, 0, len(matched.Matches))
	for _, match := range matched.Matches {
		pairs = append(pairs, table.Records[match.AconiqIndex].ID+" -> "+describeSoundPlanRow(rows[match.SoundPlanIndex]))
	}

	slices.Sort(pairs)

	return pairs
}

// TestMatchSoundPlanReceiversRefusesFloorZero pins the contract
// docs/geojson-schema-v1.md states for an immission point the import could not
// expand: the receiver it becomes has no usable key and does not match.
//
// Both ways it could match are covered. The reference set carries a floor-zero
// row, so the key would otherwise be an exact hit; and every row sits at the
// receiver's own coordinates, which is where they really are — each floor of a
// column shares its immission point's X/Y — so a fallback to coordinates would
// pair the guessed height with whichever floor sorted first.
func TestMatchSoundPlanReceiversRefusesFloorZero(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{
		IndicatorOrder: []string{"LrDay", "LrNight"},
		Records: []results.ReceiverRecord{
			aconiqRecord("soundplan-receiver-101-f0", 10, 10, 70, 65),
		},
	}
	keys := map[string]soundPlanReceiverKey{
		"soundplan-receiver-101-f0": {ObjID: 101, Floor: 0},
	}
	rows := []soundplanimport.ReceiverResult{
		spRow(1, 101, 0, "Hauptstraße 4", 10, 10, 48, 43),
		spRow(2, 101, 1, "Hauptstraße 4", 10, 10, 50, 45),
	}

	matched, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM)
	if err != nil {
		t.Fatalf("matchSoundPlanReceivers: %v", err)
	}

	if len(matched.Matches) != 0 {
		t.Fatalf("matched %d pairs, want 0 — a guessed-height receiver has no floor to match", len(matched.Matches))
	}

	if len(matched.UnmatchedAconiq) != 1 {
		t.Fatalf("unmatched Aconiq = %v, want the floor-zero receiver", matched.UnmatchedAconiq)
	}

	if len(matched.UnmatchedSoundPlan) != 2 {
		t.Fatalf("unmatched SoundPLAN = %v, want both reference rows", matched.UnmatchedSoundPlan)
	}
}

// TestMatchSoundPlanReceiversRefusesDuplicateAconiqKeys is the mirror of the
// duplicate indexSoundPlanReceiversByKey rejects. Undetected, both receivers
// would be appended against the one reference row, so it would be counted
// twice in every aggregate while the report named no unmatched SoundPLAN row.
func TestMatchSoundPlanReceiversRefusesDuplicateAconiqKeys(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{
		IndicatorOrder: []string{"LrDay", "LrNight"},
		Records: []results.ReceiverRecord{
			aconiqRecord("receiver-a", 10, 10, 70, 65),
			aconiqRecord("receiver-b", 10, 10, 72, 67),
		},
	}
	keys := map[string]soundPlanReceiverKey{
		"receiver-a": {ObjID: 101, Floor: 1},
		"receiver-b": {ObjID: 101, Floor: 1},
	}
	rows := []soundplanimport.ReceiverResult{spRow(1, 101, 1, "Hauptstraße 4", 10, 10, 50, 45)}

	_, err := matchSoundPlanReceivers(table, keys, rows, defaultReceiverMatchTolM)
	if err == nil {
		t.Fatal("expected an error for two receivers claiming one reference row")
	}

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindValidation {
		t.Fatalf("error = %v, want a %s AppError", err, domainerrors.KindValidation)
	}
}

// TestModelHasBarriers pins where the reference run's geometry is read from.
// It used to come from the import report's counts, which record what the
// import produced rather than what the comparison computes: a model edited
// since, or a different one named on --model, picked the reference scenario on
// a stale count and compared against the wrong side of a noise barrier.
func TestModelHasBarriers(t *testing.T) {
	t.Parallel()

	withBarrier := modelgeojson.Model{Features: []modelgeojson.Feature{
		{ID: "s1", Kind: modelgeojson.FeatureKindSource},
		{ID: "w1", Kind: modelgeojson.FeatureKindBarrier},
	}}
	if !modelHasBarriers(withBarrier) {
		t.Fatal("modelHasBarriers = false, want true")
	}

	withoutBarrier := modelgeojson.Model{Features: []modelgeojson.Feature{
		{ID: "s1", Kind: modelgeojson.FeatureKindSource},
		{ID: "b1", Kind: modelgeojson.FeatureKindBuilding},
	}}
	if modelHasBarriers(withoutBarrier) {
		t.Fatal("modelHasBarriers = true, want false")
	}

	if modelHasBarriers(modelgeojson.Model{}) {
		t.Fatal("modelHasBarriers on an empty model = true, want false")
	}
}
