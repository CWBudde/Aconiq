package results

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
)

// The receiver-CSV byte contract.
//
// testdata/csv-parity/ is read from *two* trees: this test writes it, and
// frontend/src/model/receiver-csv.parity.test.ts reads it to check that the
// browser builder emits the same bytes as encoding/csv does here. Moving,
// renaming or reshaping anything under testdata/csv-parity/ breaks that test,
// which no Go tool will tell you about — grep frontend/ before you do.
//
// Go's encoding/csv is canonical. This file does not re-implement it; it pins
// what it produces for a table built to hit every clause of
// csv.Writer.fieldNeedsQuotes and every branch of strconv.FormatFloat's 'f'
// rendering, so that the TypeScript mirror has something to be wrong against.
// The contract itself is written out in docs/result-containers-v1.md.
//
// Two goldens, both load-bearing:
//
//   - receiver_table.golden.json is the frontend's *input*. Asserting it here
//     keeps it in step with the Go literal below, so the two targets cannot
//     silently start from different tables.
//   - receiver_table.golden.csv is the frontend's *expectation*, byte for byte.
//
// Regenerate both with `just update-golden`.

// csvParityTable exercises every clause a *valid* ReceiverTable can reach.
//
// What it cannot contain, because ReceiverTable.Validate refuses it: an empty
// id, an empty indicator name, a NaN or infinite value, a negative height, a
// duplicate id. Those states are unreachable from Go and are covered on the
// TypeScript side as unit cases in frontend/src/model/receiver-csv.test.ts,
// which has no such gate in front of it.
func csvParityTable() ReceiverTable {
	negZero := math.Copysign(0, -1)

	// The second indicator carries a comma, so the *header* needs quoting
	// too — a builder that only escapes data rows fails here.
	indicators := []string{"Lden", "Lr,Night"}

	return ReceiverTable{
		IndicatorOrder: indicators,
		Units:          UniformUnits(indicators, "dB(A)"),
		Records: []ReceiverRecord{
			// Plain: nothing needs quoting, nothing needs expanding.
			{
				ID: "R1", X: 100.5, Y: 200.25, HeightM: 4,
				Values: map[string]float64{"Lden": 62.4, "Lr,Night": 55.1},
			},
			// Comma in the id.
			{
				ID: "R,2", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// Double quote in the id: `R"3` must become `"R""3"`.
			{
				ID: `R"3`, X: 0.30000000000000004, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// Leading ASCII space forces quotes.
			{
				ID: " R4", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// Trailing space does *not*: only the first rune is inspected.
			{
				ID: "R5 ", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// Embedded LF, copied verbatim inside the quotes.
			{
				ID: "R6\nX", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// Embedded CR, likewise — UseCRLF is false, so it stays a lone CR.
			{
				ID: "R7\rX", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// The Postgres-COPY sentinel, which encoding/csv special-cases even
			// though it contains no delimiter, quote or newline.
			{
				ID: `\.`, X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// U+00A0 NO-BREAK SPACE: in Go's unicode.IsSpace and in JS `\s`.
			// Written as an escape, here and below, because the whole point
			// of these three records is a character an editor will not show.
			{
				ID: "\u00a0R9", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// U+0085 NEL: in Go's unicode.IsSpace, *not* in JS `\s`. Quoted.
			{
				ID: "\u0085R10", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// U+FEFF BOM: in JS `\s`, *not* in Go's unicode.IsSpace. Unquoted.
			// This is the case that catches a naive /^\s/ in the other
			// direction, where the two sets differ by an over-match.
			{
				ID: "\ufeffR11", X: 1, Y: 2, HeightM: 3,
				Values: map[string]float64{"Lden": 60, "Lr,Night": 50},
			},
			// Numeric spellings JS String() renders differently: -0 (JS says
			// "0"), 1e21 and MaxFloat64 (JS uses an exponent above 1e21), 1e-7
			// and SmallestNonzeroFloat64 (JS uses an exponent below 1e-6).
			// A negative zero height passes Validate: -0 < 0 is false.
			{
				ID: "R12", X: 1e21, Y: 1e-7, HeightM: negZero,
				Values: map[string]float64{
					"Lden":     math.MaxFloat64,
					"Lr,Night": math.SmallestNonzeroFloat64,
				},
			},
		},
	}
}

func TestReceiverTableCSVByteContract(t *testing.T) {
	t.Parallel()

	table := csvParityTable()

	err := table.Validate()
	if err != nil {
		t.Fatalf("fixture table must be valid: %v", err)
	}

	golden.AssertJSONSnapshot(t, "testdata/csv-parity/receiver_table.golden.json", table)

	var buf bytes.Buffer

	err = WriteReceiverTableCSV(&buf, table)
	if err != nil {
		t.Fatalf("write receiver table csv: %v", err)
	}

	golden.AssertBytesSnapshot(t, "testdata/csv-parity/receiver_table.golden.csv", buf.Bytes())
}

// TestReceiverTableCSVFloatSpelling pins strconv.FormatFloat(v, 'f', -1, 64)
// for a curated set of doubles.
//
// Keyed on the IEEE-754 bit pattern rather than on a decimal literal: the
// frontend reconstructs each double through a DataView, so no JS parser sits
// between the fixture and the value under test. A decimal literal would make
// the fixture assert its own round-tripping instead of the spelling.
func TestReceiverTableCSVFloatSpelling(t *testing.T) {
	t.Parallel()

	values := []float64{
		0,
		math.Copysign(0, -1),
		1,
		-1,
		100.5,
		-100.5,
		0.1,
		0.30000000000000004,
		1.0 / 3.0,
		math.Pi,
		62.4,
		55.1,
		// The JS String() presentation boundaries, from both sides.
		1e-6,
		1e-7,
		-1e-7,
		9.999999999999999e-7,
		1e20,
		1e21,
		-1e21,
		1e22,
		// The extremes.
		math.MaxFloat64,
		-math.MaxFloat64,
		math.SmallestNonzeroFloat64,
		-math.SmallestNonzeroFloat64,
		math.MaxFloat32,
		// Values whose shortest representation is not their exact value.
		9007199254740993.0,
		123456789012345678901.0,
		2.2250738585072014e-308,
	}

	type spelling struct {
		Bits string `json:"bits"`
		Want string `json:"want"`
	}

	got := make([]spelling, 0, len(values))
	for _, value := range values {
		got = append(got, spelling{
			Bits: fmt.Sprintf("%016x", math.Float64bits(value)),
			Want: strconv.FormatFloat(value, 'f', -1, 64),
		})
	}

	golden.AssertJSONSnapshot(t, "testdata/csv-parity/float_spelling.golden.json", got)
}
