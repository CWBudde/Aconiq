package schall03

// Beiblatt 2 — Datenblatter Strassenbahnen (Strecke)
//
// This file encodes normative acoustic data from Anlage 2 zu Paragraph 4 der
// 16. BImSchV (Schall 03), Beiblatt 2.  The coefficients are amtliches Werk
// per Paragraph 5 UrhG and may be embedded in MIT-licensed code.
//
// Reference speed v_0 = 100 km/h on Schwellengleis im Schotterbett.
// Octave bands: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

// FzKategorienStrassenbahn contains the three Strassenbahn Fahrzeug-Kategorien
// from Beiblatt 2 (Fz 21-23).
var FzKategorienStrassenbahn = [3]FzKategorie{
	fzKat21NiederflurET(),
	fzKat22HochflurET(),
	fzKat23UBahn(),
}

// Speed-factor spectra from Table 14.
var (
	bStrassenbahnFahrNiederHoch = BeiblattSpectrum{0, 0, -5, 5, 20, 15, 15, 20}
	bStrassenbahnFahrUBahn      = BeiblattSpectrum{15, 10, 20, 20, 30, 25, 25, 20}
	bStrassenbahnAggregat       = BeiblattSpectrum{-10, -10, -10, -10, -10, -10, -10, -10}
)

// --- Fz-Kategorie 21: Strassenbahn-Niederflurfahrzeuge (n_Achs,0 = 8) ---

func fzKat21NiederflurET() FzKategorie {
	return FzKategorie{
		Fz:     21,
		Name:   "Strassenbahn-Niederflurfahrzeuge",
		NAchs0: 8,
		Teilquellen: []Teilquelle{
			{
				M:          1,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -20, -10, -2, -7, -12, -20},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				M:          2,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -20, -10, -2, -7, -12, -20},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				// m=4: Aggregat on roof (Hoehe 4 m).  For vehicles with
				// Klimaanlage, a_A rises to 47 dB; the baseline (without
				// climate) is 39 dB as given in Beiblatt 2.
				M:          4,
				SourceType: SourceTypeAggregate,
				HeightH:    2,
				HeightM:    4,
				DeltaA:     BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11},
				AA:         39,
				B:          &bStrassenbahnAggregat,
			},
		},
	}
}

// --- Fz-Kategorie 22: Strassenbahn-Hochflurfahrzeuge (n_Achs,0 = 8) ---

func fzKat22HochflurET() FzKategorie {
	return FzKategorie{
		Fz:     22,
		Name:   "Strassenbahn-Hochflurfahrzeuge",
		NAchs0: 8,
		Teilquellen: []Teilquelle{
			{
				M:          1,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-32, -23, -17, -11, -2, -7, -12, -19},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				M:          2,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-32, -23, -17, -11, -2, -7, -12, -19},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				// m=3: Aggregat under floor (Hoehe 0 m).
				M:          3,
				SourceType: SourceTypeAggregate,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11},
				AA:         39,
				B:          &bStrassenbahnAggregat,
			},
		},
	}
}

// --- Fz-Kategorie 23: U-Bahn-Fahrzeuge (n_Achs,0 = 8) ---

func fzKat23UBahn() FzKategorie {
	return FzKategorie{
		Fz:     23,
		Name:   "U-Bahn-Fahrzeuge",
		NAchs0: 8,
		Teilquellen: []Teilquelle{
			{
				M:          1,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -13, -9, -4, -6, -10, -17},
				AA:         60,
				B:          &bStrassenbahnFahrUBahn,
			},
			{
				M:          2,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -13, -9, -4, -6, -10, -17},
				AA:         60,
				B:          &bStrassenbahnFahrUBahn,
			},
			{
				// m=3: Aggregat under floor (Hoehe 0 m).
				M:          3,
				SourceType: SourceTypeAggregate,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11},
				AA:         39,
				B:          &bStrassenbahnAggregat,
			},
		},
	}
}

// LookupFzKategorie returns the FzKategorie for the given Fz number,
// searching both FzKategorien (Eisenbahn, 1-10) and
// FzKategorienStrassenbahn (21-23).
func LookupFzKategorie(fz int) (FzKategorie, bool) {
	for _, k := range FzKategorien {
		if k.Fz == fz {
			return k, true
		}
	}

	for _, k := range FzKategorienStrassenbahn {
		if k.Fz == fz {
			return k, true
		}
	}

	return FzKategorie{}, false
}

// IsStrassenbahnFz reports whether fz is a Strassenbahn vehicle category (21-23).
func IsStrassenbahnFz(fz int) bool {
	return fz >= 21 && fz <= 23
}

// ZugartStrassenbahn lists the three basic Strassenbahn Zugarten.
// Unlike Eisenbahn, the normative standard does not prescribe train compositions
// for Strassenbahnen; operators supply their own data.  These entries are
// convenience defaults for common vehicle types.
var ZugartStrassenbahn = [3]ZugartEntry{
	{Name: "Niederflur-ET", MaxSpeedKPH: 80, Composition: []FzCount{{Fz: 21, Count: 1}}},
	{Name: "Hochflur-ET", MaxSpeedKPH: 80, Composition: []FzCount{{Fz: 22, Count: 1}}},
	{Name: "Gelenktriebwagen", MaxSpeedKPH: 70, Composition: []FzCount{{Fz: 21, Count: 1}}},
}
