package soundplanimport

import (
	"encoding/binary"
	"math"
	"strconv"
	"testing"
	"time"
)

// gmCellBytes encodes one 13-byte grid-map cell record that decodes cleanly.
func gmCellBytes() []byte {
	rec := make([]byte, 13)
	binary.LittleEndian.PutUint32(rec[0:], math.Float32bits(150))
	binary.LittleEndian.PutUint32(rec[4:], math.Float32bits(50))
	binary.LittleEndian.PutUint32(rec[8:], math.Float32bits(40))
	rec[12] = 1

	return rec
}

// decodeGridMapRows matched points_total against the length of every span
// suffix by re-summing the tail for each candidate start, which is quadratic
// in a span count the .GM payload controls. A payload of alternating cell and
// separator records produces one span per pair.
func TestDecodeGridMapRows_SpanMatchingIsLinear(t *testing.T) {
	const spans = 200_000

	cell := gmCellBytes()
	payload := make([]byte, 0, spans*26)

	for range spans {
		payload = append(payload, cell...)
		payload = append(payload, make([]byte, 13)...) // separator: flag byte 0
	}

	start := time.Now()

	// One more point than any span suffix can add up to, so the search runs to
	// the end without matching.
	_, err := decodeGridMapRows(payload, spans+1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected no span suffix to match points_total")
	}

	// The quadratic version needs ~2*10^10 additions for this input.
	if elapsed > 20*time.Second {
		t.Fatalf("span matching took %s; the linear suffix-sum path did not run", elapsed)
	}
}

// The suffix-sum rewrite must select exactly the span the old tail-summing
// loop selected.
func TestDecodeGridMapRows_MatchesExpectedSpan(t *testing.T) {
	cell := gmCellBytes()

	payload := make([]byte, 0, 3*26)
	// Three spans of one cell each.
	for range 3 {
		payload = append(payload, cell...)
		payload = append(payload, make([]byte, 13)...)
	}

	rows, err := decodeGridMapRows(payload, 2)
	if err != nil {
		t.Fatalf("decode rows: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the last 2 spans", len(rows))
	}
}

// buildRailOperationSummary deduplicated train names with a linear scan per
// train, which is quadratic in the number of train rows a project file
// declares.
func TestBuildRailOperationSummary_NameDedupeIsLinear(t *testing.T) {
	const trains = 100_000

	linked := make([]TrainEmission, 0, trains)
	for i := range trains {
		linked = append(linked, TrainEmission{
			IDX:       1,
			Trainname: "train-" + strconv.Itoa(i),
			NDay:      1,
			NNight:    1,
			Speed:     100,
		})
	}

	start := time.Now()
	summary := buildRailOperationSummary(RailEmission{IDX: 1, ObjID: 1}, linked, nil, 16, 8)
	elapsed := time.Since(start)

	if len(summary.TrainNames) != trains {
		t.Fatalf("TrainNames = %d, want %d", len(summary.TrainNames), trains)
	}

	if elapsed > 20*time.Second {
		t.Fatalf("summary build took %s; the name set did not replace the linear scan", elapsed)
	}
}

// Deduplication must keep first-seen order, as the linear scan did.
func TestBuildRailOperationSummary_NameDedupePreservesOrder(t *testing.T) {
	linked := []TrainEmission{
		{IDX: 1, Trainname: "ICE", NDay: 4, Speed: 250},
		{IDX: 1, Trainname: " ", NDay: 1, Speed: 80},
		{IDX: 1, Trainname: "RE", NDay: 2, Speed: 160},
		{IDX: 1, Trainname: "ICE", NDay: 1, Speed: 250},
		{IDX: 1, Trainname: "GZ", NNight: 3, Speed: 100},
		{IDX: 1, Trainname: "RE", NDay: 1, Speed: 160},
	}

	summary := buildRailOperationSummary(RailEmission{IDX: 1, ObjID: 7}, linked, nil, 16, 8)

	want := []string{"ICE", "RE", "GZ"}
	if len(summary.TrainNames) != len(want) {
		t.Fatalf("TrainNames = %v, want %v", summary.TrainNames, want)
	}

	for i, name := range want {
		if summary.TrainNames[i] != name {
			t.Fatalf("TrainNames = %v, want %v", summary.TrainNames, want)
		}
	}
}

func FuzzDecodeGridMapRows(f *testing.F) {
	cell := gmCellBytes()

	f.Add(append(append([]byte{}, cell...), make([]byte, 13)...), 1)
	f.Add(append(append(append([]byte{}, cell...), cell...), make([]byte, 13)...), 2)
	f.Add(make([]byte, 13), 1)
	f.Add([]byte{}, 0)

	f.Fuzz(func(_ *testing.T, payload []byte, pointsTotal int) {
		// Any error is acceptable; panics and runaway allocations are not.
		_, _ = decodeGridMapRows(payload, pointsTotal)
	})
}
