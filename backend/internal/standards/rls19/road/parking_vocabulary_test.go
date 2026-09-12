package road

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// Tests for the RLS-19 §3.4 wire format: the Tabelle 6 and Tabelle 7
// Parkplatztypen carried as names, and the movement rates that must be stated
// rather than defaulted. See PLAN.md 1.5.

func validParkingSource() ParkingSource {
	return ParkingSource{
		ID:                     "p1",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 1000,
		NumSpaces:              50,
		LotType:                ParkingLotPkw,
		MovementsPerSpaceDay:   MovementRate(0.3),
		MovementsPerSpaceNight: MovementRate(0.06),
	}
}

// TestParkingSourceValidateRejectsUnspecifiedLotType is the first half of the
// defect: vehicle_type was an ordinal with omitempty, so an omitted
// Parkplatztyp decoded as Pkw and applied D_P,PT = 0 dB where the model may
// have meant the +5 or +10 dB row.
func TestParkingSourceValidateRejectsUnspecifiedLotType(t *testing.T) {
	t.Parallel()

	source := validParkingSource()
	source.LotType = ParkingLotNotSpecified

	err := source.Validate()
	if err == nil {
		t.Fatal("expected an error for an unstated parking_type")
	}

	if !strings.Contains(err.Error(), "parking_type is required") {
		t.Errorf("error %q should say parking_type is required", err)
	}

	for _, name := range ParkingLotTypeNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not offer %q", err, name)
		}
	}
}

func TestParkingSourceValidateRejectsUnknownLotType(t *testing.T) {
	t.Parallel()

	source := validParkingSource()
	source.LotType = ParkingLotType("lastwagen")

	err := source.Validate()
	if err == nil {
		t.Fatal("expected an error for an unknown parking_type")
	}

	if !strings.Contains(err.Error(), `"lastwagen"`) {
		t.Errorf("error %q should quote the offending name", err)
	}
}

// TestParkingSourceValidateRejectsUnsetMovements is the second half: an
// omitted rate passed as 0 and produced the silence sentinel, reporting an
// occupied car park as inaudible.
func TestParkingSourceValidateRejectsUnsetMovements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		muts  func(*ParkingSource)
		field string
	}{
		{
			name:  "day unset",
			muts:  func(s *ParkingSource) { s.MovementsPerSpaceDay = nil },
			field: "movements_per_space_day",
		},
		{
			name:  "night unset",
			muts:  func(s *ParkingSource) { s.MovementsPerSpaceNight = nil },
			field: "movements_per_space_night",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := validParkingSource()
			tt.muts(&source)

			err := source.Validate()
			if err == nil {
				t.Fatalf("expected an error for an unstated %s", tt.field)
			}

			if !strings.Contains(err.Error(), tt.field) {
				t.Errorf("error %q should name %s", err, tt.field)
			}

			// The message has to say how to express a genuinely empty period,
			// or the obvious reading is that zero is forbidden.
			if !strings.Contains(err.Error(), "state 0 explicitly") {
				t.Errorf("error %q should point at the explicit zero", err)
			}
		})
	}
}

// TestParkingSourceValidateAcceptsExplicitZeroMovements guards the half of the
// rule that must stay legal: a lot with no movements in a period is a real
// model, and only an *omission* is an error.
func TestParkingSourceValidateAcceptsExplicitZeroMovements(t *testing.T) {
	t.Parallel()

	source := validParkingSource()
	source.MovementsPerSpaceDay = MovementRate(0)
	source.MovementsPerSpaceNight = MovementRate(0)

	if err := source.Validate(); err != nil {
		t.Fatalf("an explicit zero rate is legal: %v", err)
	}
}

func TestComputeParkingEmissionExplicitZeroIsSilent(t *testing.T) {
	t.Parallel()

	source := validParkingSource()
	source.MovementsPerSpaceNight = MovementRate(0)

	result, err := ComputeParkingEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LWNight != silenceDB {
		t.Errorf("LWNight = %g, want the silence sentinel %g", result.LWNight, silenceDB)
	}

	if result.LWDay <= silenceThresholdDB {
		t.Errorf("LWDay = %g, want a finite level", result.LWDay)
	}
}

func TestParkingVocabularyNamesRoundTrip(t *testing.T) {
	t.Parallel()

	for _, name := range ParkingLotTypeNames() {
		parsed, err := ParseParkingLotType(name)
		if err != nil {
			t.Errorf("ParseParkingLotType(%q): %v", name, err)

			continue
		}

		if string(parsed) != name {
			t.Errorf("ParseParkingLotType(%q) = %q, want a round trip", name, parsed)
		}
	}

	for _, name := range ParkingFacilityTypeNames() {
		parsed, err := ParseParkingFacilityType(name)
		if err != nil {
			t.Errorf("ParseParkingFacilityType(%q): %v", name, err)

			continue
		}

		if string(parsed) != name {
			t.Errorf("ParseParkingFacilityType(%q) = %q, want a round trip", name, parsed)
		}
	}
}

// TestParseParkingLotTypeEmptyIsUnset pins the deliberate asymmetry with the
// Schall 03 vocabulary: Tabelle 6 has no reference row, so "" resolves to the
// unset value and Validate is the single place that refuses it.
func TestParseParkingLotTypeEmptyIsUnset(t *testing.T) {
	t.Parallel()

	parsed, err := ParseParkingLotType("")
	if err != nil {
		t.Fatalf("an empty name must not error here: %v", err)
	}

	if parsed != ParkingLotNotSpecified {
		t.Errorf("ParseParkingLotType(\"\") = %q, want the unset value", parsed)
	}

	// A facility type, by contrast, is only ever stated in order to elect the
	// Tabelle 7 rates, so "not stated" has no meaning and must not resolve.
	if _, err := ParseParkingFacilityType(""); err == nil {
		t.Error("an empty Tabelle 7 Parkplatztyp must be an error")
	}
}

// TestParkingLotTypeJSONRejectsBareOrdinals is the direct analogue of the
// Schall 03 wire-format guard: these names are Tabelle 6 row identifiers, and a
// file carrying the ordinals this type used to be must be refused rather than
// silently decoded to a different row.
func TestParkingLotTypeJSONRejectsBareOrdinals(t *testing.T) {
	t.Parallel()

	var source ParkingSource

	err := json.Unmarshal([]byte(`{"id":"p1","parking_type":2}`), &source)
	if err == nil {
		t.Fatal("a bare ordinal must not decode")
	}

	for _, want := range []string{"lkw-omnibus", "not a wire format"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestParkingSourceJSONRoundTrip(t *testing.T) {
	t.Parallel()

	source := validParkingSource()
	source.LotType = ParkingLotLkwOmnibus

	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(encoded), `"parking_type":"lkw-omnibus"`) {
		t.Errorf("encoded %s should carry the Tabelle 6 name", encoded)
	}

	var decoded ParkingSource

	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.LotType != source.LotType {
		t.Errorf("parking_type round trip: got %q, want %q", decoded.LotType, source.LotType)
	}

	if decoded.MovementsPerSpaceDay == nil || *decoded.MovementsPerSpaceDay != *source.MovementsPerSpaceDay {
		t.Errorf("movements_per_space_day did not round trip: %v", decoded.MovementsPerSpaceDay)
	}
}

// TestParkingSourceJSONOmissionsReachValidate pins the convergence the format
// is built around: an omitted key, an explicit empty string and a JSON null all
// decode without complaint and are refused in one place, with one message.
func TestParkingSourceJSONOmissionsReachValidate(t *testing.T) {
	t.Parallel()

	payloads := []string{
		`{"id":"p1","area_m2":1,"num_spaces":1,"movements_per_space_day":1,"movements_per_space_night":1}`,
		`{"id":"p1","area_m2":1,"num_spaces":1,"parking_type":"","movements_per_space_day":1,"movements_per_space_night":1}`,
		`{"id":"p1","area_m2":1,"num_spaces":1,"parking_type":null,"movements_per_space_day":1,"movements_per_space_night":1}`,
	}

	for _, payload := range payloads {
		var source ParkingSource

		err := json.Unmarshal([]byte(payload), &source)
		if err != nil {
			t.Errorf("unmarshal %s: %v", payload, err)

			continue
		}

		err = source.Validate()
		if err == nil || !strings.Contains(err.Error(), "parking_type is required") {
			t.Errorf("payload %s: want the parking_type refusal, got %v", payload, err)
		}
	}

	// An omitted movement rate must survive decoding as nil rather than as 0.
	var source ParkingSource

	err := json.Unmarshal([]byte(`{"id":"p1","area_m2":1,"num_spaces":1,"parking_type":"pkw"}`), &source)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if source.MovementsPerSpaceDay != nil {
		t.Errorf("an omitted rate decoded as %v, want nil", *source.MovementsPerSpaceDay)
	}

	if err := source.Validate(); err == nil || !strings.Contains(err.Error(), "movements_per_space_day") {
		t.Errorf("want the movements_per_space_day refusal, got %v", err)
	}
}

func TestParkingLotTypeJSONNormalizesName(t *testing.T) {
	t.Parallel()

	var source ParkingSource

	err := json.Unmarshal([]byte(`{"id":"p1","parking_type":"  LKW-Omnibus "}`), &source)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if source.LotType != ParkingLotLkwOmnibus {
		t.Errorf("parking_type = %q, want %q", source.LotType, ParkingLotLkwOmnibus)
	}
}

// TestParkingContributionIsShielded is the assertion that would have caught the
// missing D_z: a lot behind a barrier must be quieter than the same lot in the
// open. Before this, appendParkingContributions applied D_div + D_atm + D_gr
// only, so a barrier did nothing at all for a Parkplatz.
func TestParkingContributionIsShielded(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.ParkingSources = []ParkingSource{{
		ID:                     "lot",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 1000,
		NumSpaces:              100,
		LotType:                ParkingLotLkwOmnibus,
		MovementsPerSpaceDay:   MovementRate(1.5),
		MovementsPerSpaceNight: MovementRate(0.8),
	}}

	receiver := geo.Point2D{X: 0, Y: 60}

	open, err := ComputeReceiverLevels(receiver, nil, nil, cfg)
	if err != nil {
		t.Fatalf("unshielded: %v", err)
	}

	// A tall wall across the line of sight, halfway between lot and receiver.
	barriers := []Barrier{{
		ID:       "wall",
		HeightM:  8,
		Geometry: []geo.Point2D{{X: -50, Y: 30}, {X: 50, Y: 30}},
	}}

	shielded, err := ComputeReceiverLevels(receiver, nil, barriers, cfg)
	if err != nil {
		t.Fatalf("shielded: %v", err)
	}

	if !(shielded.LrDay < open.LrDay) {
		t.Errorf("a barrier must lower the parking contribution: open %.3f dB, shielded %.3f dB",
			open.LrDay, shielded.LrDay)
	}

	if !(shielded.LrNight < open.LrNight) {
		t.Errorf("night: open %.3f dB, shielded %.3f dB", open.LrNight, shielded.LrNight)
	}
}

func TestComputeReceiverLevelsRequiresAnySource(t *testing.T) {
	t.Parallel()

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 10}, nil, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected an error when neither a road nor a parking source is given")
	}
}
