package schall03

// Beiblatt 1 — Datenblatter Eisenbahnen (Strecke)
//
// This file encodes normative acoustic data from Anlage 2 zu Paragraph 4 der
// 16. BImSchV (Schall 03), Beiblatt 1.  The coefficients are amtliches Werk
// per Paragraph 5 UrhG and may be embedded in MIT-licensed code.
//
// All a_A values and octave-band differences (Delta a_f) are given for the
// reference speed v_0 = 100 km/h on Schwellengleis with average surface
// condition.  Octave bands are ordered 63, 125, 250, 500, 1000, 2000, 4000,
// 8000 Hz.

// NumBeiblattOctaveBands is the number of octave bands used in Beiblatt 1.
const NumBeiblattOctaveBands = 8

// BeiblattSpectrum stores one level per Schall 03 octave band (63..8000 Hz).
type BeiblattSpectrum [NumBeiblattOctaveBands]float64

// Teilquelle describes one sub-source of a Fahrzeug-Kategorie.
type Teilquelle struct {
	M          int              // Teilquelle number (1-11)
	SourceType string           // "rolling" | "aerodynamic" | "aggregate" | "drive"
	HeightH    int              // Hoehenbereich: 1 (0m SO), 2 (4m SO), 3 (5m SO)
	HeightM    float64          // actual height above SO in meters
	DeltaA     BeiblattSpectrum // Delta a_f octave-band difference in dB
	AA         float64          // a_A total A-weighted level in dB
	// B overrides the global speed-factor table lookup (Table 6) when non-nil.
	// Strassenbahn Teilquellen set this to the Table 14 value for their Fz class.
	B *BeiblattSpectrum
}

// FzKategorie describes one Fahrzeug-Kategorie from Beiblatt 1.
type FzKategorie struct {
	Fz          int          // 1-10
	Name        string       // e.g., "HGV-Triebkopf"
	NAchs0      int          // reference axle count n_Achs,0
	Teilquellen []Teilquelle // sub-sources
}

// Source-type constants for Teilquelle.SourceType.
const (
	SourceTypeRolling     = "rolling"
	SourceTypeAerodynamic = "aerodynamic"
	SourceTypeAggregate   = "aggregate"
	SourceTypeDrive       = "drive"
)

// FzKategorien contains all 10 Eisenbahn Fahrzeug-Kategorien from Beiblatt 1.
// Default brake variants are chosen as documented per category; alternative
// variants are noted in comments.
var FzKategorien = [10]FzKategorie{
	fzKat1HGVTriebkopf(),
	fzKat2HGVMittelSteuerwagen(),
	fzKat3HGVTriebzug(),
	fzKat4HGVNeigezug(),
	fzKat5ETriebzugSBahn(),
	fzKat6VTriebzug(),
	fzKat7ELok(),
	fzKat8VLok(),
	fzKat9Reisezugwagen(),
	fzKat10Gueterwagen(),
}

// --- Fz-Kategorie 1: HGV-Triebkopf (n_Achs,0 = 4) ---

//nolint:dupl // normative data — structural similarity is inherent
func fzKat1HGVTriebkopf() FzKategorie {
	return FzKategorie{
		Fz:     1,
		Name:   "HGV-Triebkopf",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 62,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 51,
			},
			{
				M: 5, SourceType: SourceTypeAerodynamic, HeightH: 3, HeightM: 5,
				DeltaA: BeiblattSpectrum{-30, -21, -13, -9, -6, -4, -9, -17}, AA: 43,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-28, -21, -12, -9, -6, -4, -9, -17}, AA: 46,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 35,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -10, -5, -5, -8, -15, -26}, AA: 62,
			},
			{
				M: 9, SourceType: SourceTypeAggregate, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-30, -22, -5, -4, -7, -11, -17, -26}, AA: 54,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-32, -24, -5, -4, -8, -12, -18, -29}, AA: 50,
			},
		},
	}
}

// --- Fz-Kategorie 2: HGV-Mittel-/Steuerwagen (n_Achs,0 = 4) ---
// Note: For Thalys-PBKA without Radabsorber, a_A of m=1 and m=2 are each
// increased by 5 dB (m=1: 67, m=2: 56).

//nolint:dupl // normative data — structural similarity is inherent
func fzKat2HGVMittelSteuerwagen() FzKategorie {
	return FzKategorie{
		Fz:     2,
		Name:   "HGV-Mittel-/Steuerwagen",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 62,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 51,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-21, -18, -15, -12, -5, -4, -10, -18}, AA: 29,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 35,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -13, -4, -5, -7, -14, -25}, AA: 44,
			},
		},
	}
}

// --- Fz-Kategorie 3: HGV-Triebzug (n_Achs,0 = 32) ---
// m=6: Zwei-System-Version used as default (a_A=46).
// Ein-System-Version: a_A=44, Drei-System-Version: a_A=47.
// Delta a_f is identical across all system variants.

//nolint:dupl // normative data — structural similarity is inherent
func fzKat3HGVTriebzug() FzKategorie {
	return FzKategorie{
		Fz:     3,
		Name:   "HGV-Triebzug",
		NAchs0: 32,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 73,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 62,
			},
			{
				M: 5, SourceType: SourceTypeAerodynamic, HeightH: 3, HeightM: 5,
				DeltaA: BeiblattSpectrum{-30, -21, -13, -9, -6, -4, -9, -17}, AA: 41,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-27, -21, -12, -8, -5, -5, -11, -19}, AA: 46,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-16, -9, -7, -7, -7, -9, -12, -19}, AA: 45,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -13, -4, -5, -7, -14, -25}, AA: 56,
			},
			{
				M: 9, SourceType: SourceTypeAggregate, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-35, -24, -10, -5, -5, -8, -15, -26}, AA: 62,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-32, -24, -5, -4, -8, -12, -18, -29}, AA: 53,
			},
		},
	}
}

// --- Fz-Kategorie 4: HGV-Neigezug (n_Achs,0 = 28) ---
// Note: For ETR 470 Cisalpino without Radabsorber: a_A of m=1 and m=2 are
// each increased by 5 dB, all other Teilquellen by 2 dB.

//nolint:dupl // normative data — structural similarity is inherent
func fzKat4HGVNeigezug() FzKategorie {
	return FzKategorie{
		Fz:     4,
		Name:   "HGV-Neigezug",
		NAchs0: 28,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 72,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 61,
			},
			{
				M: 5, SourceType: SourceTypeAerodynamic, HeightH: 3, HeightM: 5,
				DeltaA: BeiblattSpectrum{-30, -21, -13, -9, -6, -4, -9, -17}, AA: 41,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-28, -21, -12, -8, -5, -5, -11, -19}, AA: 47,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-16, -9, -7, -7, -9, -12, -19, -19}, AA: 44,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -13, -4, -5, -7, -14, -25}, AA: 52,
			},
			{
				M: 9, SourceType: SourceTypeAggregate, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-35, -24, -10, -5, -5, -8, -15, -26}, AA: 59,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-32, -24, -5, -4, -8, -12, -18, -29}, AA: 49,
			},
		},
	}
}

// --- Fz-Kategorie 5: E-Triebzug und S-Bahn (n_Achs,0 = 10) ---
// Default: WSB (Wellenscheibenbremse) variant for rolling noise.
// RSB (Radscheibenbremse) variant: m=1 a_A=69, m=2 a_A=58.

//nolint:dupl // normative data — structural similarity is inherent
func fzKat5ETriebzugSBahn() FzKategorie {
	return FzKategorie{
		Fz:     5,
		Name:   "E-Triebzug und S-Bahn",
		NAchs0: 10,
		Teilquellen: []Teilquelle{
			// WSB variant (default)
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 71,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 60,
			},
			{
				M: 5, SourceType: SourceTypeAerodynamic, HeightH: 3, HeightM: 5,
				DeltaA: BeiblattSpectrum{-30, -21, -13, -9, -6, -4, -9, -17}, AA: 43,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-29, -22, -11, -7, -5, -5, -12, -20}, AA: 44,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-16, -9, -6, -6, -7, -11, -15, -22}, AA: 44,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -13, -4, -5, -7, -14, -25}, AA: 48,
			},
			{
				M: 9, SourceType: SourceTypeAggregate, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-35, -24, -10, -5, -5, -8, -15, -26}, AA: 55,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-32, -24, -5, -4, -8, -12, -18, -29}, AA: 45,
			},
		},
	}
}

// --- Fz-Kategorie 6: V-Triebzug (n_Achs,0 = 6) ---

//nolint:dupl // normative data — structural similarity is inherent
func fzKat6VTriebzug() FzKategorie {
	return FzKategorie{
		Fz:     6,
		Name:   "V-Triebzug",
		NAchs0: 6,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 69,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 58,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-21, -18, -15, -12, -5, -4, -10, -18}, AA: 32,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-16, -9, -7, -7, -7, -9, -13, -20}, AA: 38,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -13, -4, -5, -7, -14, -25}, AA: 47,
			},
			{
				M: 9, SourceType: SourceTypeAggregate, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-44, -17, -10, -5, -5, -7, -13, -20}, AA: 55,
			},
			{
				M: 10, SourceType: SourceTypeDrive, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-12, -5, -4, -8, -12, -20, -30, -30}, AA: 42,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-25, -16, -9, -5, -5, -8, -12, -20}, AA: 57,
			},
		},
	}
}

// --- Fz-Kategorie 7: Elektrolok / E-Lok (n_Achs,0 = 4) ---
// Default: WSB/RSB variant for rolling noise.
// GG-Bremse variant: m=1 a_A=67, m=2 Delta_a=[-40,-30,-22,-9,-3,-5,-15,-26] a_A=71.

//nolint:dupl // normative data — structural similarity is inherent
func fzKat7ELok() FzKategorie {
	return FzKategorie{
		Fz:     7,
		Name:   "E-Lok",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			// WSB/RSB variant (default)
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 66,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 55,
			},
			{
				M: 5, SourceType: SourceTypeAerodynamic, HeightH: 3, HeightM: 5,
				DeltaA: BeiblattSpectrum{-30, -21, -13, -9, -6, -4, -9, -17}, AA: 43,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-29, -22, -12, -8, -5, -5, -10, -18}, AA: 49,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 40,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-28, -19, -6, -4, -6, -10, -14, -23}, AA: 61,
			},
			{
				M: 9, SourceType: SourceTypeAggregate, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-30, -22, -5, -4, -7, -11, -17, -26}, AA: 54,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-32, -24, -5, -4, -8, -12, -18, -29}, AA: 50,
			},
		},
	}
}

// --- Fz-Kategorie 8: Diesellok / V-Lok (n_Achs,0 = 4) ---
// Rolling noise uses GG-Bremse (only variant specified).
// Fz 8 does not have Teilquelle m=9.

func fzKat8VLok() FzKategorie {
	return FzKategorie{
		Fz:     8,
		Name:   "V-Lok",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 67,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-40, -30, -22, -9, -3, -5, -15, -26}, AA: 71,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-24, -20, -14, -13, -6, -4, -7, -14}, AA: 40,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 40,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-44, -17, -10, -5, -5, -7, -13, -20}, AA: 60,
			},
			{
				M: 10, SourceType: SourceTypeDrive, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-12, -5, -4, -8, -12, -20, -30, -30}, AA: 47,
			},
			{
				M: 11, SourceType: SourceTypeDrive, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-25, -16, -9, -5, -5, -8, -12, -20}, AA: 62,
			},
		},
	}
}

// --- Fz-Kategorie 9: Reisezugwagen (n_Achs,0 = 4) ---
// Default: WSB (Wellenscheibenbremse) variant for rolling noise.
// GG-Bremse variant: m=1 a_A=67, m=2 Delta_a=[-40,-30,-22,-9,-3,-5,-15,-26] a_A=71.

//nolint:dupl // normative data — structural similarity is inherent
func fzKat9Reisezugwagen() FzKategorie {
	return FzKategorie{
		Fz:     9,
		Name:   "Reisezugwagen",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			// WSB variant (default)
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 67,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 56,
			},
			{
				M: 6, SourceType: SourceTypeAerodynamic, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-21, -18, -15, -12, -5, -4, -10, -18}, AA: 29,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 40,
			},
			{
				M: 8, SourceType: SourceTypeAggregate, HeightH: 2, HeightM: 4,
				DeltaA: BeiblattSpectrum{-35, -24, -13, -4, -5, -7, -14, -25}, AA: 44,
			},
		},
	}
}

// --- Fz-Kategorie 10: Gueterwagen (n_Achs,0 = 4) ---
// Default: VK-Bremse (Verbundstoff-Klotzbremse) for rolling noise (m=1,2).
// By 2030 all Gueterwagen are expected to have VK-Bremse per the standard.
//
// Alternative rolling noise variants (a_A only, Delta_a identical where noted):
//   GG-Bremse:  m=1 a_A=67, m=2 Delta_a=[-40,-30,-22,-9,-3,-5,-15,-26] a_A=71
//   WSB:        m=1 a_A=67, m=2 a_A=56
//   RSB (RoLa): m=1 a_A=67, m=2 a_A=61
//
// Kesselwagen variants (m=3,4 at height 4m):
//   GG-Bremse:  m=3 a_A=57, m=4 a_A=61
//   VK-Bremse:  m=3 a_A=57, m=4 a_A=48
//   WSB:        m=3 a_A=57, m=4 a_A=46

func fzKat10Gueterwagen() FzKategorie {
	return FzKategorie{
		Fz:     10,
		Name:   "Gueterwagen",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			// VK-Bremse variant (default)
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 67,
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 58,
			},
			{
				M: 7, SourceType: SourceTypeAerodynamic, HeightH: 1, HeightM: 0,
				DeltaA: BeiblattSpectrum{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 40,
			},
		},
	}
}

// ZugartEntry describes one Zugart from Table 4 of Schall 03.
type ZugartEntry struct {
	Name        string
	MaxSpeedKPH float64
	Composition []FzCount
}

// FzCount pairs a Fahrzeug-Kategorie number with a unit count.
type FzCount struct {
	Fz    int `json:"fz"`
	Count int `json:"count"`
}

// Zugarten contains all 19 Eisenbahn Zugarten from Table 4.
var Zugarten = [19]ZugartEntry{
	{Name: "ICE-1-Zug", MaxSpeedKPH: 250, Composition: []FzCount{{1, 2}, {2, 12}}},
	{Name: "ICE-2-Halbzug", MaxSpeedKPH: 250, Composition: []FzCount{{1, 1}, {2, 7}}},
	{Name: "ICE-2-Vollzug", MaxSpeedKPH: 250, Composition: []FzCount{{1, 2}, {2, 14}}},
	{Name: "ICE-3-Halbzug", MaxSpeedKPH: 300, Composition: []FzCount{{3, 1}}},
	{Name: "ICE-3-Vollzug", MaxSpeedKPH: 300, Composition: []FzCount{{3, 2}}},
	{Name: "ICE-T", MaxSpeedKPH: 230, Composition: []FzCount{{4, 1}}},
	{Name: "Thalys-PBKA-Halbzug", MaxSpeedKPH: 300, Composition: []FzCount{{1, 2}, {2, 5}}},
	{Name: "Thalys-PBKA-Vollzug", MaxSpeedKPH: 300, Composition: []FzCount{{1, 4}, {2, 10}}},
	{Name: "ETR-470-Cisalpino", MaxSpeedKPH: 200, Composition: []FzCount{{4, 1}}},
	{Name: "IC-Zug-E-Lok", MaxSpeedKPH: 200, Composition: []FzCount{{7, 1}, {9, 12}}},
	{Name: "IC-Zug-V-Lok", MaxSpeedKPH: 160, Composition: []FzCount{{8, 1}, {9, 12}}},
	{Name: "Nahverkehrszug-E-Lok", MaxSpeedKPH: 160, Composition: []FzCount{{7, 1}, {9, 5}}},
	{Name: "Nahverkehrszug-V-Lok", MaxSpeedKPH: 140, Composition: []FzCount{{8, 1}, {9, 5}}},
	{Name: "Nahverkehrszug-ET", MaxSpeedKPH: 140, Composition: []FzCount{{5, 1}}},
	{Name: "Nahverkehrszug-VT", MaxSpeedKPH: 120, Composition: []FzCount{{6, 1}}},
	{Name: "IC3", MaxSpeedKPH: 180, Composition: []FzCount{{6, 1}}},
	{Name: "S-Bahn", MaxSpeedKPH: 120, Composition: []FzCount{{5, 1}}},
	{Name: "Gueterzug-E-Lok", MaxSpeedKPH: 100, Composition: []FzCount{{7, 1}, {10, 24}}},
	{Name: "Gueterzug-V-Lok", MaxSpeedKPH: 100, Composition: []FzCount{{8, 1}, {10, 24}}},
}

// Kesselwagen sub-source data for Fz 10 Gueterwagen at height 4m.
// These are used when a fraction of Gueterwagen are Kesselwagen (default 20%).
// The brake variant affects only the a_A of Teilquelle m=4.

// KesselwagenTeilquellen returns the Kesselwagen-specific Teilquellen (m=3,4)
// for the given brake variant. Valid variants: "GG", "VK", "WSB".
// Delta_a values are identical across brake variants; only a_A differs for m=4.
var KesselwagenTeilquellen = struct {
	M3Schiene BeiblattSpectrum
	M3AA      float64
	M4Rad     BeiblattSpectrum
	M4AAGG    float64
	M4AAVK    float64
	M4AAWSB   float64
}{
	M3Schiene: BeiblattSpectrum{-29, -20, -19, -6, -5, -5, -17, -26},
	M3AA:      57,
	M4Rad:     BeiblattSpectrum{-28, -19, -18, -5, -4, -7, -17, -26},
	M4AAGG:    61,
	M4AAVK:    48,
	M4AAWSB:   46,
}
