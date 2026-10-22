package results

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
)

// A facade receiver row names its building, facade and storey (PLAN P18 B3).
// The fields are optional and come as a set; a table without them keeps the
// JSON and CSV bytes it had before they existed, which the existing
// receiver_table.golden.* fixtures pin.

func intPtr(v int) *int { return &v }

func facadeRecord(id string) ReceiverRecord {
	return ReceiverRecord{
		ID: id, X: 500012.05, Y: 5800019.95, HeightM: 3,
		Values:        map[string]float64{"LrDay": 61.2},
		PlacementRule: "rls19-facade",
		Facade: &ReceiverFacade{
			BuildingRef: "osm-way-1",
			Part:        0,
			Ring:        0,
			Edge:        3,
			From:        [2]float64{500010, 5800020},
			To:          [2]float64{500018.5, 5800022.5},
		},
		Storey: intPtr(0),
	}
}

func singleRecordTable(record ReceiverRecord) ReceiverTable {
	indicators := []string{"LrDay"}

	return ReceiverTable{
		IndicatorOrder: indicators,
		Units:          UniformUnits(indicators, UnitDecibel),
		Records:        []ReceiverRecord{record},
	}
}

func TestReceiverTableValidateAcceptsFacadeRecord(t *testing.T) {
	t.Parallel()

	err := singleRecordTable(facadeRecord("F1")).Validate()
	if err != nil {
		t.Fatalf("facade record must be valid: %v", err)
	}
}

func TestReceiverTableValidateRefusesBadFacadeAttributes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*ReceiverRecord)
		want   string
	}{
		{"facade without storey", func(r *ReceiverRecord) { r.Storey = nil }, "facade attributes"},
		{"facade without rule", func(r *ReceiverRecord) { r.PlacementRule = "" }, "facade attributes"},
		{"storey alone", func(r *ReceiverRecord) { r.Facade, r.PlacementRule = nil, "" }, "facade attributes"},
		{"rule alone", func(r *ReceiverRecord) { r.Facade, r.Storey = nil, nil }, "facade attributes"},
		{"empty building ref", func(r *ReceiverRecord) { r.Facade.BuildingRef = "" }, "building_ref"},
		{"negative part", func(r *ReceiverRecord) { r.Facade.Part = -1 }, "part"},
		{"negative ring", func(r *ReceiverRecord) { r.Facade.Ring = -1 }, "ring"},
		{"negative edge", func(r *ReceiverRecord) { r.Facade.Edge = -1 }, "edge"},
		{"negative storey", func(r *ReceiverRecord) { r.Storey = intPtr(-1) }, "storey"},
		{"non-finite from", func(r *ReceiverRecord) { r.Facade.From[0] = math.NaN() }, "from"},
		{"non-finite to", func(r *ReceiverRecord) { r.Facade.To[1] = math.Inf(1) }, "to"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			record := facadeRecord("F1")
			tc.mutate(&record)

			err := singleRecordTable(record).Validate()
			if err == nil {
				t.Fatal("expected a validation error")
			}

			if !strings.Contains(err.Error(), `"F1"`) {
				t.Fatalf("error %q does not name the receiver", err)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestReceiverRecordWithoutFacadeMarshalsWithoutFacadeKeys(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(ReceiverRecord{ID: "R1", X: 1, Y: 2, HeightM: 4, Values: map[string]float64{"L": 1}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const want = `{"id":"R1","x":1,"y":2,"height_m":4,"values":{"L":1}}`
	if string(payload) != want {
		t.Fatalf("plain record = %s, want %s", payload, want)
	}
}

func TestLoadReceiverTableJSONRoundTripsFacadeAttributes(t *testing.T) {
	t.Parallel()

	table := csvParityFacadeTable()
	path := filepath.Join(t.TempDir(), "receivers.json")

	err := SaveReceiverTableJSON(path, table)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadReceiverTableJSON(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !reflect.DeepEqual(loaded.Records, table.Records) {
		t.Fatalf("records did not round-trip:\n got %#v\nwant %#v", loaded.Records, table.Records)
	}
}

func TestReceiverTableCSVPlainTableHasNoFacadeColumns(t *testing.T) {
	t.Parallel()

	record := facadeRecord("R1")
	record.Facade, record.Storey, record.PlacementRule = nil, nil, ""

	var buf bytes.Buffer

	err := WriteReceiverTableCSV(&buf, singleRecordTable(record))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	const want = "id,x,y,height_m,LrDay\nR1,500012.05,5800019.95,3,61.2\n"
	if buf.String() != want {
		t.Fatalf("plain csv = %q, want %q", buf.String(), want)
	}
}

// csvParityFacadeTable is the facade half of the byte contract: a table that
// mixes a plain receiver with facade receivers, as a custom table may.
//
// The plain row comes first, so a writer that decides on the facade columns
// from the first record rather than from all of them fails here. The first
// building_ref carries a comma and a quote, so the new fields go through the
// same quoting as the id. The second facade row has non-zero part, ring, edge
// and storey, so a writer that swaps two of the integer columns fails too.
// Coordinates and ids are synthetic.
func csvParityFacadeTable() ReceiverTable {
	indicators := []string{"LrDay", "LrNight"}

	return ReceiverTable{
		IndicatorOrder: indicators,
		Units:          UniformUnits(indicators, "dB(A)"),
		Records: []ReceiverRecord{
			{
				ID: "R1", X: 500030, Y: 5800040, HeightM: 4,
				Values: map[string]float64{"LrDay": 58.3, "LrNight": 49.9},
			},
			{
				ID: "F1", X: 500012.05, Y: 5800019.95, HeightM: 3,
				Values:        map[string]float64{"LrDay": 61.2, "LrNight": 52.4},
				PlacementRule: "rls19-facade",
				Facade: &ReceiverFacade{
					BuildingRef: `osm-way-1,"a"`,
					Part:        0,
					Ring:        0,
					Edge:        3,
					From:        [2]float64{500010, 5800020},
					To:          [2]float64{500018.5, 5800022.5},
				},
				Storey: intPtr(0),
			},
			{
				ID: "F2", X: 500101.25, Y: 5800202.75, HeightM: 12,
				Values:        map[string]float64{"LrDay": 55, "LrNight": 47.05},
				PlacementRule: "schall03-facade",
				Facade: &ReceiverFacade{
					BuildingRef: "osm-way-2",
					Part:        1,
					Ring:        2,
					Edge:        5,
					From:        [2]float64{500100, 5800200},
					To:          [2]float64{500102.5, 5800205.5},
				},
				Storey: intPtr(3),
			},
		},
	}
}

func TestReceiverTableCSVFacadeByteContract(t *testing.T) {
	t.Parallel()

	table := csvParityFacadeTable()

	err := table.Validate()
	if err != nil {
		t.Fatalf("fixture table must be valid: %v", err)
	}

	golden.AssertJSONSnapshot(t, "testdata/csv-parity/receiver_table_facade.golden.json", table)

	var buf bytes.Buffer

	err = WriteReceiverTableCSV(&buf, table)
	if err != nil {
		t.Fatalf("write receiver table csv: %v", err)
	}

	golden.AssertBytesSnapshot(t, "testdata/csv-parity/receiver_table_facade.golden.csv", buf.Bytes())
}
