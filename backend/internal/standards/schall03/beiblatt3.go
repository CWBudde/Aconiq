package schall03

// Beiblatt 3 — Datenblätter Rangier- und Umschlagbahnhöfe
//
// This file encodes normative acoustic data from Anlage 2 zu Paragraph 4 der
// 16. BImSchV (Schall 03), Beiblatt 3.  The coefficients are amtliches Werk
// per Paragraph 5 UrhG and may be embedded in MIT-licensed code.
//
// Source: docs/bimsch16_anl2_neu-1.pdf, pages 44–45.
// Octave bands: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

import "math"

// YardSourceShape identifies whether a Beiblatt 3 source is a point or line.
type YardSourceShape int

const (
	YardSourcePoint YardSourceShape = iota // Einzelschallquelle (Punktschallquelle)
	YardSourceLine                         // Linienschallquelle
)

// YardSourceData holds the normative acoustic data for one Beiblatt 3 source type.
type YardSourceData struct {
	// LWA is the A-weighted total sound power level L_WA (or L_W'A for line
	// sources) at the reference condition, in dB.  For Retarder sources, this
	// is the base value before the +10·lg(n_ret) term.
	LWA float64
	// DeltaLW contains the octave-band deviations ΔL_W,f (point) or
	// ΔL_W',f (line) from L_WA, for 63..8000 Hz.
	DeltaLW BeiblattSpectrum
	// HeightM is the source height h_s above SO/FO in metres.
	HeightM float64
	// SourceShape indicates whether this is a point or line source.
	SourceShape YardSourceShape
}

// GleisbremseType enumerates the Gleisbremse variants from Beiblatt 3.
type GleisbremseType int

const (
	GleisbremsZulaufOhneSegmente        GleisbremseType = iota // i=2
	GleisbremsTalbremseOhneSegmente                            // i=3
	GleisbremsTalbremseMitGG                                   // i=4
	GleisbremseSchalloptimiert                                 // i=5
	GleisbremsTalbremsMitSegmenten                             // i=6
	GleisbremsRichtungEinseitigSegmente                        // i=7
	GleisbremsGummiwalk                                        // i=8
	GleisbremsFEWTalbremse                                     // i=9
	GleisbremsSchraubenbremse                                  // i=10
)

// Beiblatt3GleisbremsenByType returns the normative data for the requested
// Gleisbremse variant.  All Gleisbremsengeräusche are Punktschallquellen at
// Quellhöhe 0 m (h=1).  The second return value is false if t is not a valid
// GleisbremseType constant.
func Beiblatt3GleisbremsenByType(t GleisbremseType) (YardSourceData, bool) {
	if t < 0 || int(t) >= len(gleisbremsTable) {
		return YardSourceData{}, false
	}

	return gleisbremsTable[t], true
}

var gleisbremsTable = [9]YardSourceData{
	// i=2: Zulaufbremse, beidseitig ohne Segmente
	{
		LWA: 110, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-56, -50, -42, -32, -24, -13, -1, -12},
	},
	// i=3: Talbremse, TW beidseitig ohne Segmente
	{
		LWA: 105, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-56, -50, -42, -32, -24, -13, -1, -12},
	},
	// i=4: Talbremse, TW beidseitig mit GG-Segmenten
	{
		LWA: 88, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-53, -46, -36, -35, -33, -9, -2, -7},
	},
	// i=5: Tal-/Richtungsgleisbremse, schalloptimiert
	{
		LWA: 85, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-28, -23, -18, -13, -9, -6, -4, -9},
	},
	// i=6: Talbremse, TW beidseitig mit Segmenten
	// ΔL_W,f values: 500 Hz band value cross-referenced from PDF p. 44.
	{
		LWA: 98, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-52, -45, -41, -38, -9, -1, -13, -13},
	},
	// i=7: Richtungsgleisbremse, TWE einseitig mit Segmenten
	{
		LWA: 92, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-56, -52, -45, -41, -38, -9, -1, -13},
	},
	// i=8: Gummiwalkbremse
	{
		LWA: 83, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-57, -52, -45, -41, -38, -9, -7, -11},
	},
	// i=9: FEW Talbremse
	{
		LWA: 98, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-38, -28, -23, -18, -15, -5, -3, -13},
	},
	// i=10: Schraubenbremse (L_WA for 1 element of ~1.2 m length)
	{
		LWA: 72, HeightM: 0, SourceShape: YardSourcePoint,
		DeltaLW: BeiblattSpectrum{-29, -21, -9, -21, -10, -8, -4, -13},
	},
}

// Beiblatt3Kurvenfahrgeraeusch is the normative data for Kurvenfahrgeräusch
// (curve squeal noise, r ≤ 300 m) per Beiblatt 3.  This is a Linienschallquelle.
var Beiblatt3Kurvenfahrgeraeusch = YardSourceData{
	LWA:         69,
	HeightM:     0,
	SourceShape: YardSourceLine,
	DeltaLW:     BeiblattSpectrum{-27, -19, -12, -10, -8, -5, -6, -8},
}

// Beiblatt3RetarderVerzoegerungsstrecke is the Retardergeräusch for a
// Verzögerungsstrecke; Punktschallquelle at 0 m, L_WA=90 dB.
var Beiblatt3RetarderVerzoegerungsstrecke = YardSourceData{
	LWA:         90,
	HeightM:     0,
	SourceShape: YardSourcePoint,
	DeltaLW:     BeiblattSpectrum{-11, -15, -15, -16, -9, -5, -8, -15},
}

// Beiblatt3RetarderBeharrungsstreckeBase is the spectral data for a retarder
// on a Beharrungsstrecke (line source).  The base L_WA = 62 dB is before the
// +10·lg(n_ret) term.  Use Beiblatt3RetarderBeharrungsstreckeLevel to get the
// effective L_WA for a given retarder density n_ret (retarders per metre of track).
var Beiblatt3RetarderBeharrungsstreckeBase = YardSourceData{
	LWA:         62,
	HeightM:     0,
	SourceShape: YardSourceLine,
	DeltaLW:     BeiblattSpectrum{-28, -23, -16, -12, -9, -3, -8, -14},
}

// Beiblatt3RetarderBeharrungsstreckeLevel returns the effective L_WA for the
// given retarder density nRetPerM (retarders per laufenden Meter of track).
func Beiblatt3RetarderBeharrungsstreckeLevel(nRetPerM float64) float64 {
	return 62 + 10*math.Log10(nRetPerM)
}

// Beiblatt3RetarderRangierenBase is for Rangierfahrten auf Beharrungsstrecke
// (line source).  L_WA = 72 + 10·lg(n_ret).
var Beiblatt3RetarderRangierenBase = YardSourceData{
	LWA:         72,
	HeightM:     0,
	SourceShape: YardSourceLine,
	DeltaLW:     BeiblattSpectrum{-30, -26, -18, -12, -9, -3, -6, -13},
}

// Beiblatt3RetarderRangierenLevel returns effective L_WA for rangieren on
// Beharrungsstrecke for given retarder density nRetPerM.
func Beiblatt3RetarderRangierenLevel(nRetPerM float64) float64 {
	return 72 + 10*math.Log10(nRetPerM)
}

// Beiblatt3HemmschuhauflaufgeraeuschData is the Hemmschuhauflaufgeräusch;
// Punktschallquelle at 0 m, L_WA = 95 dB.
var Beiblatt3HemmschuhauflaufgeraeuschData = YardSourceData{
	LWA:         95,
	HeightM:     0,
	SourceShape: YardSourcePoint,
	DeltaLW:     BeiblattSpectrum{-41, -37, -16, -21, -18, -19, -7, -1},
}

// Beiblatt3AuflaufstossByTech returns the Auflaufstoßgeräusch data.
// Pass modernTech=true for a fully-automated (vollautomatische) yard (L_WA=78),
// false for yards without modern equipment (L_WA=91).
// This is a Punktschallquelle at 1.5 m above SO/FO.
func Beiblatt3AuflaufstossByTech(modernTech bool) YardSourceData {
	if modernTech {
		return YardSourceData{
			LWA:         78,
			HeightM:     1.5,
			SourceShape: YardSourcePoint,
			DeltaLW:     BeiblattSpectrum{-23, -15, -11, -11, -6, -5, -7, -13},
		}
	}

	return YardSourceData{
		LWA:         91,
		HeightM:     1.5,
		SourceShape: YardSourcePoint,
		DeltaLW:     BeiblattSpectrum{-25, -18, -12, -11, -6, -4, -8, -13},
	}
}

// Beiblatt3AnreissenAbbremsenBase is the Geräusch beim Anreißen und Abbremsen
// lose gekoppelter Wagen.  Linienschallquelle at 1.5 m.
// L_WA = 75 dB (reference: 20-wagon Rangiergruppe, 400 m length).
var Beiblatt3AnreissenAbbremsenBase = YardSourceData{
	LWA:         75,
	HeightM:     1.5,
	SourceShape: YardSourceLine,
	DeltaLW:     BeiblattSpectrum{-26, -15, -13, -9, -6, -5, -7, -12},
}
