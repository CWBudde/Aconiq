package road

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// The RLS-19 Tabelle 6 and Tabelle 7 row identifiers below are carried on the
// wire as names, never as ordinals. Both tables are indexed by the same
// normative dimension — "Parkplatztyp PT" — and a table ordinal is renumbered
// whenever a row moves, so a file written against an older numbering would
// decode to a different row and shift the level by up to 10 dB in silence. See
// PLAN.md 1.2 for the renumbering that made this explicit, and 1.5 for the
// omitted-field half of the same failure.

// ParkingLotType names a row of RLS-19 Tabelle 6, which gives the surcharge
// D_{P,PT} per Parkplatztyp. The standard's own term is Parkplatztyp: the
// dimension is a property of the lot, not of a vehicle.
//
// There is no reference row and no default. Pkw's 0 dB is the Pkw row, not a
// stand-in for "not stated", so ParkingLotNotSpecified is refused by
// ParkingSource.Validate rather than resolved to it.
type ParkingLotType string

const (
	// ParkingLotNotSpecified is the zero value: no Parkplatztyp was stated. It
	// is not a row of Tabelle 6 and is never a computable input.
	ParkingLotNotSpecified ParkingLotType = ""

	ParkingLotPkw        ParkingLotType = "pkw"         // Pkw-Parkplätze:              D_{P,PT} =  0 dB
	ParkingLotMotorrad   ParkingLotType = "motorrad"    // Motorrad-Parkplätze:         D_{P,PT} =  5 dB
	ParkingLotLkwOmnibus ParkingLotType = "lkw-omnibus" // Lkw- und Omnibus-Parkplätze: D_{P,PT} = 10 dB
)

// parkingLotSurcharges is RLS-19 Tabelle 6 in table order: one entry per
// printed row, carrying the row's name and its D_{P,PT}. The computation and
// the standard-data digest both read this slice, so a hand-edited coefficient
// moves the digest instead of hiding in a switch statement.
var parkingLotSurcharges = []struct {
	Type        ParkingLotType
	SurchargeDB float64
}{
	{ParkingLotPkw, 0.0},
	{ParkingLotMotorrad, 5.0},
	{ParkingLotLkwOmnibus, 10.0},
}

// ParkingFacilityType names a row of RLS-19 Tabelle 7, whose standard movement
// rates §3.4.1 admits only "wenn keine geeigneten projektbezogenen
// Untersuchungsergebnisse vorliegen". The standard calls this dimension
// Parkplatztyp too, though its rows are not those of Tabelle 6.
//
// Tabelle 7 carries two rows and no others: an ordinary public car park has no
// standard rate at all and its movements must be stated explicitly.
type ParkingFacilityType string

const (
	// ParkingFacilityNotSpecified is the zero value: no Parkplatztyp was stated.
	ParkingFacilityNotSpecified ParkingFacilityType = ""

	ParkingFacilityPR       ParkingFacilityType = "park-and-ride"   // P+R-Parkplätze
	ParkingFacilityTankRast ParkingFacilityType = "tank-rastanlage" // Tank- und Rastanlagen
)

// parkingMovementRates is RLS-19 Tabelle 7 in table order: movements N per
// space and hour, day (06–22 Uhr) and night (22–06 Uhr).
var parkingMovementRates = []struct {
	Type         ParkingFacilityType
	DayPerHour   float64
	NightPerHour float64
}{
	{ParkingFacilityPR, 0.3, 0.06},
	{ParkingFacilityTankRast, 1.5, 0.8},
}

// TimePeriod selects day or night assessment period.
type TimePeriod int

const (
	TimePeriodDay   TimePeriod = iota // Tag (06–22 Uhr)
	TimePeriodNight                   // Nacht (22–06 Uhr)
)

// silenceDB is the level reported for a period with no acoustic contribution,
// where 10 lg is undefined. It matches the convention used by schall03,
// cnossos/* and bub/road, each of which already names this module as its
// origin; PLAN.md Priority 7 owns unifying the five copies.
const (
	silenceDB = -999.0

	// silenceThresholdDB is the cut-off below which a level is treated as the
	// silence sentinel and dropped from an energy sum.
	silenceThresholdDB = -900.0
)

func normalizeVocabularyName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ParseParkingLotType resolves a Tabelle 6 Parkplatztyp name.
//
// An empty name resolves to ParkingLotNotSpecified rather than erroring, and
// rather than resolving to a reference row the way the Schall 03 vocabulary
// does: Tabelle 6 has no reference row, and the dominant failure is an omitted
// key, which never reaches this function at all. ParkingSource.Validate is the
// single place that refuses the unset value, so an omitted key, an explicit ""
// and a JSON null all fail identically.
func ParseParkingLotType(raw string) (ParkingLotType, error) {
	name := normalizeVocabularyName(raw)
	if name == "" {
		return ParkingLotNotSpecified, nil
	}

	for _, row := range parkingLotSurcharges {
		if string(row.Type) == name {
			return row.Type, nil
		}
	}

	return ParkingLotNotSpecified, fmt.Errorf("unknown Parkplatztyp (Tabelle 6) %q, expected one of %s",
		raw, strings.Join(ParkingLotTypeNames(), ", "))
}

// ParkingLotTypeNames lists the accepted Tabelle 6 Parkplatztyp names in table
// order.
func ParkingLotTypeNames() []string {
	names := make([]string, 0, len(parkingLotSurcharges))
	for _, row := range parkingLotSurcharges {
		names = append(names, string(row.Type))
	}

	return names
}

// ParseParkingFacilityType resolves a Tabelle 7 Parkplatztyp name. Unlike the
// Tabelle 6 vocabulary, an empty name is an error: a facility type is only ever
// stated in order to elect the standard rates, so "not stated" has no meaning
// here and must not resolve to a rate of zero.
func ParseParkingFacilityType(raw string) (ParkingFacilityType, error) {
	name := normalizeVocabularyName(raw)

	for _, row := range parkingMovementRates {
		if string(row.Type) == name {
			return row.Type, nil
		}
	}

	return ParkingFacilityNotSpecified, unknownParkingFacilityError(ParkingFacilityType(raw))
}

// ParkingFacilityTypeNames lists the accepted Tabelle 7 Parkplatztyp names in
// table order.
func ParkingFacilityTypeNames() []string {
	names := make([]string, 0, len(parkingMovementRates))
	for _, row := range parkingMovementRates {
		names = append(names, string(row.Type))
	}

	return names
}

func unknownParkingFacilityError(ft ParkingFacilityType) error {
	return fmt.Errorf("unknown Parkplatztyp (Tabelle 7) %q, expected one of %s",
		string(ft), strings.Join(ParkingFacilityTypeNames(), ", "))
}

// UnmarshalJSON decodes a Tabelle 6 Parkplatztyp from its stable name,
// normalising case and surrounding whitespace. A JSON null is a no-op, per the
// encoding/json convention, and leaves the field unset for Validate to refuse.
//
// A bare number is refused with the reason rather than with encoding/json's
// type error, because a file carrying one was almost certainly written against
// the ordinals this type used to be.
func (t *ParkingLotType) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}

	var name string

	err := json.Unmarshal(data, &name)
	if err != nil {
		return fmt.Errorf(
			"parking_type must be named, got %s: expected one of %s, because the RLS-19 Tabelle 6 row ordinals are renumbered when a row moves and are not a wire format",
			string(data), strings.Join(ParkingLotTypeNames(), ", "),
		)
	}

	value, err := ParseParkingLotType(name)
	if err != nil {
		return err
	}

	*t = value

	return nil
}

// MovementRate returns a pointer to an hourly movement rate, for assignment to
// ParkingSource.MovementsPerSpaceDay/Night. An explicit 0 is a legal rate; only
// a nil pointer means "not stated".
func MovementRate(nPerHour float64) *float64 { return &nPerHour }

// ParkingSource describes an RLS-19 parking-area source (Parkplatz, §3.4).
// The parking lot is approximated as a point source located at Center.
type ParkingSource struct {
	ID string `json:"id"`

	// Center is the centroid of the parking area in plan view.
	// Used as the point-source location for propagation.
	Center geo.Point2D `json:"center"`

	// ElevationM is the absolute Z of the parking surface.
	ElevationM float64 `json:"elevation_m,omitempty"`

	// AreaM2 is the total parking area P [m²] (Stellplatzfläche).
	AreaM2 float64 `json:"area_m2"`

	// NumSpaces is the number of parking spaces n (Anzahl der Stellplätze).
	NumSpaces int `json:"num_spaces"`

	// LotType names the Tabelle 6 Parkplatztyp that sets D_{P,PT}. It is
	// required: the empty value is not a row of the table and there is no
	// default row to fall back to. Carried by name, never as an ordinal, and
	// written without omitempty so a half-built source serialises as an empty
	// string rather than dropping the key.
	LotType ParkingLotType `json:"parking_type"`

	// MovementsPerSpaceDay/Night are the hourly movement rates N per space
	// [h⁻¹]; an arrival and a departure each count as one movement (§3.4.1).
	//
	// nil means "not stated" and is a validation error. An explicit 0 is a
	// legal rate — a lot with no movements in a period — and yields the silence
	// sentinel for that period. DefaultMovementsPerHour supplies the Tabelle 7
	// standard values, which §3.4.1 admits only where no project-specific
	// survey exists.
	MovementsPerSpaceDay   *float64 `json:"movements_per_space_day"`
	MovementsPerSpaceNight *float64 `json:"movements_per_space_night"`
}

// Validate checks a parking source definition.
func (s ParkingSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("parking source id is required")
	}

	if !s.Center.IsFinite() {
		return fmt.Errorf("parking source %q center must be finite", s.ID)
	}

	if !isFinite(s.ElevationM) {
		return fmt.Errorf("parking source %q elevation_m must be finite", s.ID)
	}

	if !isFinite(s.AreaM2) || s.AreaM2 <= 0 {
		return fmt.Errorf("parking source %q area_m2 must be finite and > 0", s.ID)
	}

	if s.NumSpaces <= 0 {
		return fmt.Errorf("parking source %q num_spaces must be > 0", s.ID)
	}

	return s.validateParkingRates()
}

// validateParkingRates checks the two inputs to Eq. 10 that an omission used to
// answer for: the Tabelle 6 Parkplatztyp, which defaulted to Pkw and 0 dB, and
// the movement rates, which passed as 0 and produced the silence sentinel.
func (s ParkingSource) validateParkingRates() error {
	if s.LotType == ParkingLotNotSpecified {
		return fmt.Errorf("parking source %q parking_type is required, expected one of %s",
			s.ID, strings.Join(ParkingLotTypeNames(), ", "))
	}

	if _, ok := parkingLotTypeSurcharge(s.LotType); !ok {
		return fmt.Errorf("parking source %q has unsupported parking_type %q, expected one of %s",
			s.ID, s.LotType, strings.Join(ParkingLotTypeNames(), ", "))
	}

	err := validateMovementRate(s.ID, "movements_per_space_day", s.MovementsPerSpaceDay)
	if err != nil {
		return err
	}

	return validateMovementRate(s.ID, "movements_per_space_night", s.MovementsPerSpaceNight)
}

func validateMovementRate(sourceID, field string, rate *float64) error {
	if rate == nil {
		return fmt.Errorf("parking source %q %s is required; state 0 explicitly for a period with no movements",
			sourceID, field)
	}

	if !isFinite(*rate) || *rate < 0 {
		return fmt.Errorf("parking source %q %s must be finite and >= 0", sourceID, field)
	}

	return nil
}

// ParkingEmissionResult holds the total sound power level L_W per time period.
// This is the point-source total power (not the area-related L_W” from Eq. 10);
// it equals L_W” + 10·lg(P/1m²) = 63 + 10·lg(N·n) + D_{P,PT}.
type ParkingEmissionResult struct {
	LWDay   float64 // total sound power level, day [dB(A)]
	LWNight float64 // total sound power level, night [dB(A)]
}

// ComputeParkingEmission computes total sound power levels per RLS-19 Eq. 10:
//
//	L_W = 63 + 10·lg[N·n] + D_{P,PT}
//
// where N is movements per space per hour, n is the number of spaces, and
// D_{P,PT} is the Parkplatztyp surcharge from Tabelle 6.
//
// An explicitly stated movement rate of zero produces the silence sentinel for
// that period, 10 lg being undefined there. An omitted rate is a validation
// error rather than a silent zero — see ParkingSource.MovementsPerSpaceDay.
func ComputeParkingEmission(source ParkingSource) (ParkingEmissionResult, error) {
	err := source.Validate()
	if err != nil {
		return ParkingEmissionResult{}, err
	}

	// Validate has established that the Parkplatztyp is a row of Tabelle 6 and
	// that both rates are stated.
	dPT, _ := parkingLotTypeSurcharge(source.LotType)
	n := float64(source.NumSpaces)

	lw := func(movPerSpace float64) float64 {
		if movPerSpace <= 0 {
			return silenceDB
		}

		return 63 + 10*math.Log10(movPerSpace*n) + dPT
	}

	return ParkingEmissionResult{
		LWDay:   lw(*source.MovementsPerSpaceDay),
		LWNight: lw(*source.MovementsPerSpaceNight),
	}, nil
}

// parkingLotTypeSurcharge returns the D_{P,PT} correction from Tabelle 6 and
// reports whether the Parkplatztyp names a row of it. An unknown type is not a
// zero surcharge: it is a type that carries no correction at all, and the
// caller must refuse rather than apply the Pkw row to it.
func parkingLotTypeSurcharge(lt ParkingLotType) (float64, bool) {
	for _, row := range parkingLotSurcharges {
		if row.Type == lt {
			return row.SurchargeDB, true
		}
	}

	return 0, false
}

// DefaultMovementsPerHour returns the Tabelle 7 standard hourly movement rate N
// for a Parkplatztyp and period. §3.4.1 admits these values only where no
// suitable project-specific survey results exist.
//
// An unknown or unstated facility type is an error, not a zero: a zero rate
// flows straight into the silence sentinel and would report an occupied car
// park as inaudible.
func DefaultMovementsPerHour(ft ParkingFacilityType, period TimePeriod) (float64, error) {
	for _, row := range parkingMovementRates {
		if row.Type != ft {
			continue
		}

		if period == TimePeriodNight {
			return row.NightPerHour, nil
		}

		return row.DayPerHour, nil
	}

	return 0, unknownParkingFacilityError(ft)
}

// appendParkingContributions adds point-source contributions from all parking
// sources to the day/night level accumulation slices.
//
// Each parking source is treated as a single point source at its Center and
// propagated through the same §3.5 chain a road Teilstück gets: D_div + D_atm,
// then max(D_gr, D_z) per Eq. 11. Source height is 0.5 m above the parking
// surface, as for road sources.
//
// Shielding matters here for the same reason it does on the road path — a lot
// behind a noise barrier is not audible as though the barrier were absent — so
// the barriers and terrain edges are the same effective sets the road sources
// see.
//
// Mirrored paths apply too. Eq. 3 gives the Beurteilungspegel of all
// Parkplatzflächen as a sum over j of L_W”,j + 10·lg[P_j] − D_A,j − D_RV1,j −
// D_RV2,j, and names D_RV1,j the "anzusetzender Reflexionsverlust bei der
// ersten Reflexion für die Parkplatzteilfläche j nach dem Abschnitt 3.6";
// Eq. 1 sums Fahrstreifenteilstücke and Parkplatzteilflächen "jeweils
// einschließlich etwaiger Spiegelschallquellen". A lot therefore reflects on
// exactly the chain a Teilstück does, which is why the same helper serves both.
func appendParkingContributions(
	dayContrib, nightContrib *[]float64,
	parkingSources []ParkingSource,
	receiver geo.Point2D,
	receiverZ float64,
	effectiveBarriers []Barrier,
	cfg PropagationConfig,
) error {
	for _, parking := range parkingSources {
		emission, err := ComputeParkingEmission(parking)
		if err != nil {
			return err
		}

		sourceZ := parking.ElevationM + pointSourceHeightM

		planDist := dist2D(parking.Center, receiver)
		dz := receiverZ - sourceZ
		slantDist := math.Sqrt(planDist*planDist + dz*dz)

		hm := computeMeanHeight(parking.Center, receiver, sourceZ, receiverZ, cfg.Terrain)
		att := computeAttenuation(planDist, slantDist, hm, cfg)

		att = applyShielding(att, parkingShielding(parking, receiver, sourceZ, receiverZ, effectiveBarriers, cfg))

		*dayContrib = append(*dayContrib, emission.LWDay-att.Total)
		*nightContrib = append(*nightContrib, emission.LWNight-att.Total)

		// A silent lot stays silent: ComputeParkingEmission returns the
		// silenceDB sentinel for a zero movement rate, and a mirrored path only
		// subtracts from it, so the contribution stays below silenceThresholdDB
		// and acoustics.EnergySum still drops it.
		appendReflectedContribs(
			dayContrib, nightContrib,
			emission.LWDay, emission.LWNight,
			parking.Center, pointSourceHeightM, sourceZ, receiver, receiverZ,
			effectiveBarriers, cfg,
		)
	}

	return nil
}

// parkingShielding returns the larger of the barrier and terrain-edge insertion
// losses on the path from a parking lot to the receiver.
func parkingShielding(
	parking ParkingSource,
	receiver geo.Point2D,
	sourceZ, receiverZ float64,
	effectiveBarriers []Barrier,
	cfg PropagationConfig,
) float64 {
	barrierLoss := 0.0
	if len(effectiveBarriers) > 0 {
		barrierLoss = ComputeShielding(
			parking.Center, pointSourceHeightM,
			receiver, cfg.ReceiverHeightM,
			effectiveBarriers,
		).InsertionLoss
	}

	terrainLoss := 0.0
	if len(cfg.Terrain) > 0 {
		terrainLoss = computeTerrainEdgeShielding(
			parking.Center, sourceZ,
			receiver, receiverZ,
			cfg.Terrain,
		).InsertionLoss
	}

	return math.Max(barrierLoss, terrainLoss)
}
