package schall03

import "math"

// Normative correction tables for Strassenbahnen from Schall 03 (Anlage 2 zu
// Paragraph 4 der 16. BImSchV), Nummer 5.
//
// Octave bands: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

// ---------------------------------------------------------------------------
// Table 15: Pegelkorrekturen c1 fuer Fahrbahnarten (Strassenbahnen)
// ---------------------------------------------------------------------------

// SFahrbahnartType identifies a Strassenbahn track surface type for Table 15.
type SFahrbahnartType int

const (
	// SFahrbahnSchwellengleis is the reference track type (Schwellengleis im
	// Schotterbett); no c1 correction is applied.  Intentionally set to a
	// value that does not match any entry in C1StrassenbahnTable.
	SFahrbahnSchwellengleis SFahrbahnartType = iota - 1

	SFahrbahnStrassenbuendig // Strassenbuendiger Bahnkoerper und feste Fahrbahn
	SFahrbahnGruenTief       // Begruentter Bahnkoerper, tief liegende Vegetationsebene
	SFahrbahnGruenHoch       // Begruentter Bahnkoerper, hoch liegende Vegetationsebene
)

// SC1Entry holds the c1 correction for one Strassenbahn Fahrbahnart (Table 15).
// Corrections apply to Teilquellen m=1 and m=2 (Fahrgeraeusche) only.
type SC1Entry struct {
	Type SFahrbahnartType
	Name string
	C1   BeiblattSpectrum
}

// C1StrassenbahnTable contains Table 15 data.
var C1StrassenbahnTable = [3]SC1Entry{
	{
		Type: SFahrbahnStrassenbuendig,
		Name: "Strassenbuendiger Bahnkoerper und feste Fahrbahn",
		C1:   BeiblattSpectrum{2, 3, 2, 5, 8, 4, 2, 1},
	},
	{
		Type: SFahrbahnGruenTief,
		Name: "Begruentter Bahnkoerper, tief liegende Vegetationsebene",
		C1:   BeiblattSpectrum{-2, -4, -3, -1, -1, -1, -1, -3},
	},
	{
		Type: SFahrbahnGruenHoch,
		Name: "Begruentter Bahnkoerper, hoch liegende Vegetationsebene",
		C1:   BeiblattSpectrum{1, -1, -3, -4, -4, -7, -7, -5},
	},
}

// C1StrassenbahnForType returns the SC1Entry for the given SFahrbahnartType.
// Returns (zero, false) for SFahrbahnSchwellengleis or unknown types.
func C1StrassenbahnForType(t SFahrbahnartType) (SC1Entry, bool) {
	for _, e := range C1StrassenbahnTable {
		if e.Type == t {
			return e, true
		}
	}

	return SC1Entry{}, false
}

// sumC1StrassenbahnForTeilquelle returns the c1 correction from Table 15 for
// a given Fahrbahnart, Teilquelle m, and octave band index f.
// Corrections apply to Fahrgeraeusche (m=1, m=2) only; all other m return 0.
func sumC1StrassenbahnForTeilquelle(fahrbahn SFahrbahnartType, m, f int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	entry, ok := C1StrassenbahnForType(fahrbahn)
	if !ok {
		return 0
	}

	return entry.C1[f]
}

// ---------------------------------------------------------------------------
// Table 16: Korrekturen K_Br und K_LM fuer Bruecken (Strassenbahnen)
// ---------------------------------------------------------------------------

// BridgeCorrectionStrassenbahnTable contains the five rows of Table 16.
// Corrections apply to Fahrgeraeusche (m=1, m=2) only.
//
// BridgeCorrectionEntry is reused from tables.go; the Type field holds the
// 1-based table row index (1-5).
var BridgeCorrectionStrassenbahnTable = [5]BridgeCorrectionEntry{
	{
		Type:        1,
		Description: "Bruecke mit staehlernem Ueberbau, Gleise direkt aufgelagert",
		KBr:         12,
		KLM:         -6,
	},
	{
		Type:        2,
		Description: "Bruecke mit staehlernem Ueberbau und Schwellengleis im Schotterbett",
		KBr:         6,
		KLM:         -3,
	},
	{
		Type:        3,
		Description: "Bruecke mit staehlernem oder massivem Ueberbau, Gleise in Strassenfahrbahn eingebettet (Rillenschiene)",
		KBr:         4,
		KLM:         math.NaN(),
	},
	{
		Type:        4,
		Description: "Bruecke mit massiver Fahrbahnplatte oder besonderem staehlernen Ueberbau, Schwellengleis im Schotterbett",
		KBr:         3,
		KLM:         -3,
	},
	{
		Type:        5,
		Description: "Bruecke mit massiver Fahrbahnplatte, Gleise direkt aufgelagert (feste Fahrbahn)",
		KBr:         4,
		KLM:         math.NaN(),
	},
}

// bridgeCorrectionStrassenbahnForTeilquelle returns the K_Br (+K_LM) bridge
// correction from Table 16 for a given bridge type index (1-5) and Teilquelle m.
// Only applies to Fahrgeraeusche (m=1, m=2).
//
//nolint:unused // will be wired into the emission pipeline in Task 4
func bridgeCorrectionStrassenbahnForTeilquelle(bridgeType int, bridgeMitig bool, m int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	if bridgeType < 1 || bridgeType > len(BridgeCorrectionStrassenbahnTable) {
		return 0
	}

	entry := BridgeCorrectionStrassenbahnTable[bridgeType-1]
	correction := entry.KBr

	if bridgeMitig && !math.IsNaN(entry.KLM) {
		correction += entry.KLM
	}

	return correction
}

// curveCorrectionStrassenbahnForTeilquelle returns the Strassenbahn K_L
// correction per Nr. 5.3.2 for a given curve radius and Teilquelle m.
//
// For curves with r < 200 m (without active K_L mitigation), a fixed +4 dB
// is applied to Fahrgeraeusche (m=1, m=2).  Curves with r >= 200 m or
// straight track (r == 0) carry no correction.
func curveCorrectionStrassenbahnForTeilquelle(curveRadiusM float64, m int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	if curveRadiusM > 0 && curveRadiusM < 200 {
		return 4
	}

	return 0
}
