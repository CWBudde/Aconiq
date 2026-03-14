package schall03

import (
	"math"
	"slices"
)

// Normative correction tables from Schall 03 (Anlage 2 zu Paragraph 4 der
// 16. BImSchV), sections 4.3 through 6.3.
//
// Octave bands are ordered 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

// ---------------------------------------------------------------------------
// Table 6: Geschwindigkeitsfaktor b fuer Eisenbahnen
// ---------------------------------------------------------------------------

// SpeedFactorEntry holds one row of Table 6, mapping a set of Teilquellen to
// their octave-band speed exponent b.  The returned spectrum is used in the
// formula: correction = b * lg(v/v0).
type SpeedFactorEntry struct {
	Description string
	Teilquellen []int            // which m values this applies to
	B           BeiblattSpectrum // speed factor b per octave band
}

// SpeedFactorBTable contains the four rows of Table 6.
var SpeedFactorBTable = [4]SpeedFactorEntry{
	{
		Description: "Rollgeraeusche",
		Teilquellen: []int{1, 2, 3, 4},
		B:           BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25},
	},
	{
		Description: "Aerodynamische Geraeusche",
		Teilquellen: []int{5, 6, 7},
		B:           BeiblattSpectrum{50, 50, 50, 50, 50, 50, 50, 50},
	},
	{
		Description: "Aggregatgeraeusche",
		Teilquellen: []int{8, 9},
		B:           BeiblattSpectrum{-10, -10, -10, -10, -10, -10, -10, -10},
	},
	{
		Description: "Antriebsgeraeusche",
		Teilquellen: []int{10, 11},
		B:           BeiblattSpectrum{20, 20, 20, 20, 20, 20, 20, 20},
	},
}

// SpeedFactorBForTeilquelle returns the speed factor spectrum for a given
// Teilquelle number m.  Returns a zero spectrum if m is not found.
func SpeedFactorBForTeilquelle(m int) BeiblattSpectrum {
	for _, entry := range SpeedFactorBTable {
		if slices.Contains(entry.Teilquellen, m) {
			return entry.B
		}
	}

	return BeiblattSpectrum{}
}

// ---------------------------------------------------------------------------
// Table 7: Pegelkorrekturen c1 fuer Fahrbahnarten
// ---------------------------------------------------------------------------

// FahrbahnartType identifies a track surface type for Table 7.
type FahrbahnartType int

const (
	FahrbahnartFesteFahrbahn            FahrbahnartType = iota // Feste Fahrbahn
	FahrbahnartFesteFahrbahnMitAbsorber                        // Feste Fahrbahn mit Absorber
	FahrbahnartBahnuebergang                                   // Bahnuebergang
)

// C1Entry holds a single correction component of Table 7.
type C1Entry struct {
	Effect      string           // "schiene" or "reflexion"
	Teilquellen []int            // which m values this correction applies to
	C1          BeiblattSpectrum // correction in dB per octave band
}

// C1FahrbahnartEntry holds all correction components for one Fahrbahnart.
type C1FahrbahnartEntry struct {
	Name        string
	Type        FahrbahnartType
	Corrections []C1Entry
}

// C1FahrbahnartTable contains Table 7 data.
var C1FahrbahnartTable = [3]C1FahrbahnartEntry{
	{
		Name: "Feste Fahrbahn",
		Type: FahrbahnartFesteFahrbahn,
		Corrections: []C1Entry{
			{
				Effect:      "schiene",
				Teilquellen: []int{1, 2},
				C1:          BeiblattSpectrum{0, 0, 0, 7, 3, 0, 0, 0},
			},
			{
				Effect:      "reflexion",
				Teilquellen: []int{1, 2, 7, 9, 11},
				C1:          BeiblattSpectrum{1, 1, 1, 1, 1, 1, 1, 1},
			},
		},
	},
	{
		Name: "Feste Fahrbahn mit Absorber",
		Type: FahrbahnartFesteFahrbahnMitAbsorber,
		Corrections: []C1Entry{
			{
				Effect:      "schiene",
				Teilquellen: []int{1, 2},
				C1:          BeiblattSpectrum{0, 0, 0, 7, 3, 0, 0, 0},
			},
			{
				Effect:      "reflexion",
				Teilquellen: []int{1, 2, 7, 9, 11},
				C1:          BeiblattSpectrum{0, 0, 0, -2, -2, -3, 0, 0},
			},
		},
	},
	{
		Name: "Bahnuebergang",
		Type: FahrbahnartBahnuebergang,
		Corrections: []C1Entry{
			{
				Effect:      "schiene",
				Teilquellen: []int{1, 2},
				C1:          BeiblattSpectrum{0, 0, 0, 8, 4, 0, 0, 0},
			},
			{
				Effect:      "reflexion",
				Teilquellen: []int{1, 2, 7, 9, 11},
				C1:          BeiblattSpectrum{1, 1, 1, 1, 1, 1, 1, 1},
			},
		},
	},
}

// ---------------------------------------------------------------------------
// Table 8: Pegelkorrekturen c2 fuer Fahrflaechenzustand
// ---------------------------------------------------------------------------

// C2MeasureType identifies a surface condition measure for Table 8.
type C2MeasureType int

const (
	C2BuG                     C2MeasureType = iota // besonders ueberwachtes Gleis
	C2Schienenstegdaempfer                         // Schienenstegdaempfer
	C2Schienenstegabschirmung                      // Schienenstegabschirmung
)

// C2Entry holds one correction row from Table 8.
type C2Entry struct {
	Measure     C2MeasureType
	Name        string
	Teilquellen []int            // which m values this correction applies to
	C2          BeiblattSpectrum // correction in dB per octave band
}

// C2SurfaceConditionTable contains all rows from Table 8.
var C2SurfaceConditionTable = []C2Entry{
	{
		Measure:     C2BuG,
		Name:        "besonders ueberwachtes Gleis (bueG)",
		Teilquellen: []int{1, 3},
		C2:          BeiblattSpectrum{0, 0, 0, -4, -5, -5, -4, 0},
	},
	{
		Measure:     C2Schienenstegdaempfer,
		Name:        "Schienenstegdaempfer (m=1,3)",
		Teilquellen: []int{1, 3},
		C2:          BeiblattSpectrum{0, 0, 0, -2, -3, -3, 0, 0},
	},
	{
		Measure:     C2Schienenstegdaempfer,
		Name:        "Schienenstegdaempfer (m=2,4)",
		Teilquellen: []int{2, 4},
		C2:          BeiblattSpectrum{0, 0, 0, -1, -3, -2, 0, 0},
	},
	{
		Measure:     C2Schienenstegabschirmung,
		Name:        "Schienenstegabschirmung (m=1)",
		Teilquellen: []int{1},
		C2:          BeiblattSpectrum{0, 0, 0, -3, -4, -5, 0, 0},
	},
}

// ---------------------------------------------------------------------------
// Table 9: Korrekturen K_Br und K_LM fuer Bruecken
// ---------------------------------------------------------------------------

// BridgeType identifies a bridge construction type per Table 9.
type BridgeType int

const (
	BridgeSteelDirect    BridgeType = iota + 1 // steel, direct rail mount
	BridgeSteelBallast                         // steel, ballast bed
	BridgeMassiveBallast                       // massive/special steel, ballast
	BridgeMassiveFeste                         // massive, feste Fahrbahn
)

// BridgeCorrectionEntry holds one row of Table 9.
type BridgeCorrectionEntry struct {
	Type        BridgeType
	Description string
	KBr         float64 // bridge correction K_Br in dB
	KLM         float64 // noise-reduction measure correction K_LM in dB (NaN if not applicable)
}

// BridgeCorrectionTable contains all 4 rows of Table 9.
var BridgeCorrectionTable = [4]BridgeCorrectionEntry{
	{
		Type:        BridgeSteelDirect,
		Description: "Bruecke mit staehlernem Ueberbau, Gleise direkt aufgelagert",
		KBr:         12,
		KLM:         -6,
	},
	{
		Type:        BridgeSteelBallast,
		Description: "Bruecke mit staehlernem Ueberbau und Schwellengleis im Schotterbett",
		KBr:         6,
		KLM:         -3,
	},
	{
		Type:        BridgeMassiveBallast,
		Description: "Bruecke mit massiver Fahrbahnplatte oder besonderem staehlernen Ueberbau und Schwellengleis im Schotterbett",
		KBr:         3,
		KLM:         -3,
	},
	{
		Type:        BridgeMassiveFeste,
		Description: "Bruecke mit fester Fahrbahn",
		KBr:         4,
		KLM:         math.NaN(),
	},
}

// ---------------------------------------------------------------------------
// Table 11: Pegelkorrekturen K_L fuer die Auffaelligkeit von Geraeuschen
// (Eisenbahnstrecke scope only — curve noise)
// ---------------------------------------------------------------------------

// CurveNoiseEntry holds one row of Table 11 for Eisenbahnstrecken.
type CurveNoiseEntry struct {
	Description string
	MinRadiusM  float64 // inclusive lower bound (0 for first row)
	MaxRadiusM  float64 // exclusive upper bound (math.Inf(1) for last row)
	KL          float64 // K_L correction in dB
	KLA         float64 // K_L,A tonal penalty reduction in dB
}

// CurveNoiseCorrectionTable contains the Eisenbahnstrecke rows of Table 11.
var CurveNoiseCorrectionTable = [3]CurveNoiseEntry{
	{
		Description: "Kurvenradius < 300 m",
		MinRadiusM:  0,
		MaxRadiusM:  300,
		KL:          8,
		KLA:         -3,
	},
	{
		Description: "Kurvenradius 300 m bis < 500 m",
		MinRadiusM:  300,
		MaxRadiusM:  500,
		KL:          3,
		KLA:         -3,
	},
	{
		Description: "Kurvenradius >= 500 m",
		MinRadiusM:  500,
		MaxRadiusM:  math.Inf(1),
		KL:          0,
		KLA:         0,
	},
}

// CurveNoiseCorrectionForRadius returns (K_L, K_LA) for a given curve radius.
// If radiusM <= 0 (straight track), returns (0, 0).
func CurveNoiseCorrectionForRadius(radiusM float64) (kL, kLA float64) {
	if radiusM <= 0 {
		return 0, 0
	}

	for _, entry := range CurveNoiseCorrectionTable {
		if radiusM >= entry.MinRadiusM && radiusM < entry.MaxRadiusM {
			return entry.KL, entry.KLA
		}
	}

	return 0, 0
}

// ---------------------------------------------------------------------------
// Table 17: Absorptionskoeffizienten der Luft fuer Oktavbaender
// ---------------------------------------------------------------------------

// AirAbsorptionAlpha contains the air absorption coefficients alpha in
// dB per 1000 m, per octave band (Table 17).  Based on ISO 9613-2 for
// 10 degrees C and 70% relative humidity.
var AirAbsorptionAlpha = BeiblattSpectrum{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
