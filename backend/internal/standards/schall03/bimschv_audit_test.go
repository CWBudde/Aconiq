package schall03_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==========================================================================
// BImSchV conformance audit — regression tests pinning every normative table
// value from BGBl. 2014 I Nr. 61 (Anlage 2 zu § 4 der 16. BImSchV).
// ==========================================================================

// ---------------------------------------------------------------------------
// Table 6 — Geschwindigkeitsfaktor b (BGBl p. 2285)
// ---------------------------------------------------------------------------

func TestTable6_SpeedFactorB_BGBl2285(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    int
		want [8]float64
	}{
		// Row 1: Rollgeräusche (m=1,2,3,4)
		{"m=1 Rollgeraeusch", 1, [8]float64{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"m=2 Rollgeraeusch", 2, [8]float64{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"m=3 Rollgeraeusch", 3, [8]float64{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"m=4 Rollgeraeusch", 4, [8]float64{-5, -5, 0, 10, 25, 25, 25, 25}},
		// Row 2: Aerodynamische Geräusche (m=5,6,7)
		{"m=5 Aerodynamisch", 5, [8]float64{50, 50, 50, 50, 50, 50, 50, 50}},
		{"m=6 Aerodynamisch", 6, [8]float64{50, 50, 50, 50, 50, 50, 50, 50}},
		{"m=7 Aerodynamisch", 7, [8]float64{50, 50, 50, 50, 50, 50, 50, 50}},
		// Row 3: Aggregatgeräusche (m=8,9)
		{"m=8 Aggregat", 8, [8]float64{-10, -10, -10, -10, -10, -10, -10, -10}},
		{"m=9 Aggregat", 9, [8]float64{-10, -10, -10, -10, -10, -10, -10, -10}},
		// Row 4: Antriebsgeräusche (m=10,11)
		{"m=10 Antrieb", 10, [8]float64{20, 20, 20, 20, 20, 20, 20, 20}},
		{"m=11 Antrieb", 11, [8]float64{20, 20, 20, 20, 20, 20, 20, 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := schall03.SpeedFactorBForTeilquelle(tt.m)
			assert.Equal(t, tt.want, [8]float64(got), "speed factor b spectrum mismatch for m=%d", tt.m)
		})
	}

	// Unknown m should return zero spectrum.
	t.Run("m=0 unknown", func(t *testing.T) {
		t.Parallel()

		got := schall03.SpeedFactorBForTeilquelle(0)
		assert.Equal(t, [8]float64{}, [8]float64(got), "unknown m should return zero spectrum")
	})
}

// ---------------------------------------------------------------------------
// Table 9 — Brückenkorrekturen K_Br und K_LM (BGBl p. 2287)
// ---------------------------------------------------------------------------

func TestTable9_BridgeCorrections_BGBl2287(t *testing.T) {
	t.Parallel()

	table := schall03.BridgeCorrectionTable
	require.Len(t, table, 4, "Table 9 must have 4 rows")

	tests := []struct {
		name    string
		idx     int
		wantTyp schall03.BridgeType
		wantKBr float64
		wantKLM float64 // NaN if not applicable
		nanKLM  bool
	}{
		{"steel direct", 0, schall03.BridgeSteelDirect, 12, -6, false},
		{"steel ballast", 1, schall03.BridgeSteelBallast, 6, -3, false},
		{"massive ballast", 2, schall03.BridgeMassiveBallast, 3, -3, false},
		{"massive feste", 3, schall03.BridgeMassiveFeste, 4, math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := table[tt.idx]
			assert.Equal(t, tt.wantTyp, row.Type, "BridgeType mismatch")
			assert.InDelta(t, tt.wantKBr, row.KBr, 0.001, "K_Br mismatch")

			if tt.nanKLM {
				assert.True(t, math.IsNaN(row.KLM), "K_LM should be NaN for type %d", row.Type)
			} else {
				assert.InDelta(t, tt.wantKLM, row.KLM, 0.001, "K_LM mismatch")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Table 11 — Kurvenzuschlag K_L (BGBl p. 2289)
// ---------------------------------------------------------------------------

func TestTable11_CurveNoise_BGBl2289(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		radiusM float64
		wantKL  float64
		wantKLA float64
	}{
		{"r=100 (<300)", 100, 8, -3},
		{"r=299.9 (<300)", 299.9, 8, -3},
		{"r=300 (300..500)", 300, 3, -3},
		{"r=400 (300..500)", 400, 3, -3},
		{"r=499.9 (300..500)", 499.9, 3, -3},
		{"r=500 (>=500)", 500, 0, 0},
		{"r=1000 (>=500)", 1000, 0, 0},
		{"straight (r=0)", 0, 0, 0},
		{"straight (r<0)", -1, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kL, kLA := schall03.CurveNoiseCorrectionForRadius(tt.radiusM)
			assert.InDelta(t, tt.wantKL, kL, 0.001, "K_L mismatch for r=%.1f", tt.radiusM)
			assert.InDelta(t, tt.wantKLA, kLA, 0.001, "K_L,A mismatch for r=%.1f", tt.radiusM)
		})
	}
}

// ---------------------------------------------------------------------------
// Table 17 — Luftabsorptionskoeffizienten (BGBl p. 2294)
// ---------------------------------------------------------------------------

func TestTable17_AirAbsorption_BGBl2294(t *testing.T) {
	t.Parallel()

	want := [8]float64{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
	got := [8]float64(schall03.AirAbsorptionAlpha)
	assert.Equal(t, want, got, "AirAbsorptionAlpha mismatch (dB/1000m, 63..8000 Hz)")
}

// ---------------------------------------------------------------------------
// Table 15 — Strassenbahn c1 Fahrbahnart (BGBl p. 2292)
// ---------------------------------------------------------------------------

func TestTable15_StrassenbahnC1_BGBl2292(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		typ    schall03.SFahrbahnartType
		wantC1 [8]float64
	}{
		{"Strassenbuendig", schall03.SFahrbahnStrassenbuendig, [8]float64{2, 3, 2, 5, 8, 4, 2, 1}},
		{"Gruengleis tief", schall03.SFahrbahnGruenTief, [8]float64{-2, -4, -3, -1, -1, -1, -1, -3}},
		{"Gruengleis hoch", schall03.SFahrbahnGruenHoch, [8]float64{1, -1, -3, -4, -4, -7, -7, -5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entry, ok := schall03.C1StrassenbahnForType(tt.typ)
			require.True(t, ok, "type %d must exist", tt.typ)
			assert.Equal(t, tt.wantC1, [8]float64(entry.C1), "c1 spectrum mismatch")
		})
	}

	// Schwellengleis (reference) should not be found.
	t.Run("Schwellengleis not found", func(t *testing.T) {
		t.Parallel()

		_, ok := schall03.C1StrassenbahnForType(schall03.SFahrbahnSchwellengleis)
		assert.False(t, ok, "Schwellengleis (reference) should not have a c1 entry")
	})
}

// ---------------------------------------------------------------------------
// Table 16 — Strassenbahn Brückenkorrektur (BGBl p. 2292)
// ---------------------------------------------------------------------------

func TestTable16_StrassenbahnBridge_BGBl2292(t *testing.T) {
	t.Parallel()

	table := schall03.BridgeCorrectionStrassenbahnTable
	require.Len(t, table, 5, "Table 16 must have 5 rows")

	tests := []struct {
		name    string
		idx     int
		wantKBr float64
		wantKLM float64
		nanKLM  bool
	}{
		{"row 1 steel direct", 0, 12, -6, false},
		{"row 2 steel ballast", 1, 6, -3, false},
		{"row 3 Rillenschiene", 2, 4, 0, true},
		{"row 4 massive ballast", 3, 3, -3, false},
		{"row 5 massive feste", 4, 4, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := table[tt.idx]
			assert.InDelta(t, tt.wantKBr, row.KBr, 0.001, "K_Br mismatch")

			if tt.nanKLM {
				assert.True(t, math.IsNaN(row.KLM), "K_LM should be NaN for row %d", tt.idx+1)
			} else {
				assert.InDelta(t, tt.wantKLM, row.KLM, 0.001, "K_LM mismatch")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Table 14 — Strassenbahn Geschwindigkeitsfaktor b (BGBl p. 2291)
// ---------------------------------------------------------------------------

func TestTable14_StrassenbahnSpeed_BGBl2291(t *testing.T) {
	t.Parallel()

	// The three speed-factor spectra are package-level vars accessed via
	// the Teilquelle.B pointer in each Strassenbahn FzKategorie.

	// Fz21 m=1 uses bStrassenbahnFahrNiederHoch
	fz21, ok := schall03.LookupFzKategorie(21)
	require.True(t, ok)
	require.NotNil(t, fz21.Teilquellen[0].B, "Fz21 m=1 must have explicit B")
	assert.Equal(t,
		[8]float64{0, 0, -5, 5, 20, 15, 15, 20},
		[8]float64(*fz21.Teilquellen[0].B),
		"bStrassenbahnFahrNiederHoch mismatch (from Fz21 m=1)")

	// Fz21 m=4 (Aggregat) uses bStrassenbahnAggregat
	require.NotNil(t, fz21.Teilquellen[2].B, "Fz21 m=4 must have explicit B")
	assert.Equal(t,
		[8]float64{-10, -10, -10, -10, -10, -10, -10, -10},
		[8]float64(*fz21.Teilquellen[2].B),
		"bStrassenbahnAggregat mismatch (from Fz21 m=4)")

	// Fz23 m=1 uses bStrassenbahnFahrUBahn
	fz23, ok := schall03.LookupFzKategorie(23)
	require.True(t, ok)
	require.NotNil(t, fz23.Teilquellen[0].B, "Fz23 m=1 must have explicit B")
	assert.Equal(t,
		[8]float64{15, 10, 20, 20, 30, 25, 25, 20},
		[8]float64(*fz23.Teilquellen[0].B),
		"bStrassenbahnFahrUBahn mismatch (from Fz23 m=1)")
}

// ---------------------------------------------------------------------------
// Beiblatt 1 — a_A spot-checks for all 10 Fz-Kategorien (BGBl pp. 2306-2311)
// ---------------------------------------------------------------------------

func TestBeiblatt1_AllFzKategorien_AA_BGBl2306(t *testing.T) {
	t.Parallel()

	// Each entry maps Fz number to {m -> expected a_A}.
	type aaCheck struct {
		m    int
		want float64
	}

	tests := []struct {
		fz     int
		checks []aaCheck
	}{
		{1, []aaCheck{
			{1, 62}, {2, 51}, {5, 43}, {6, 46}, {7, 35}, {8, 62}, {9, 54}, {11, 50},
		}},
		{2, []aaCheck{
			{1, 62}, {2, 51}, {6, 29}, {7, 35}, {8, 44},
		}},
		{3, []aaCheck{
			{1, 73}, {2, 62}, {5, 41}, {6, 46}, {7, 45}, {8, 56}, {9, 62}, {11, 53},
		}},
		{4, []aaCheck{
			{1, 72}, {2, 61}, {5, 41}, {6, 47}, {7, 44}, {8, 52}, {9, 59}, {11, 49},
		}},
		{5, []aaCheck{
			{1, 71}, {2, 60}, {5, 43}, {6, 44}, {7, 44}, {8, 48}, {9, 55}, {11, 45},
		}},
		{6, []aaCheck{
			{1, 69}, {2, 58}, {6, 32}, {7, 38}, {8, 47}, {9, 55}, {10, 42}, {11, 57},
		}},
		{7, []aaCheck{
			{1, 66}, {2, 55}, {5, 43}, {6, 49}, {7, 40}, {8, 61}, {9, 54}, {11, 50},
		}},
		{8, []aaCheck{
			{1, 67}, {2, 71}, {6, 40}, {7, 40}, {8, 60}, {10, 47}, {11, 62},
		}},
		{9, []aaCheck{
			{1, 67}, {2, 56}, {6, 29}, {7, 40}, {8, 44},
		}},
		{10, []aaCheck{
			{1, 67}, {2, 58}, {7, 40},
		}},
	}

	for _, tt := range tests {
		t.Run("Fz"+itoa(tt.fz), func(t *testing.T) {
			t.Parallel()

			fz, ok := schall03.LookupFzKategorie(tt.fz)
			require.True(t, ok, "Fz %d must exist", tt.fz)

			// Build lookup by m.
			byM := make(map[int]float64, len(fz.Teilquellen))
			for _, tq := range fz.Teilquellen {
				byM[tq.M] = tq.AA
			}

			for _, c := range tt.checks {
				aa, found := byM[c.m]
				require.True(t, found, "Fz%d must have Teilquelle m=%d", tt.fz, c.m)
				assert.InDelta(t, c.want, aa, 0.001, "Fz%d m=%d: a_A mismatch", tt.fz, c.m)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Beiblatt 2 — Strassenbahn Fz 21-23 full data (BGBl pp. 2311-2312)
// ---------------------------------------------------------------------------

func TestBeiblatt2_StrassenbahnFz_BGBl2311(t *testing.T) {
	t.Parallel()

	type tqCheck struct {
		m      int
		aa     float64
		deltaA [8]float64
	}

	tests := []struct {
		fz     int
		checks []tqCheck
	}{
		{21, []tqCheck{
			{1, 63, [8]float64{-34, -25, -20, -10, -2, -7, -12, -20}},
			{2, 63, [8]float64{-34, -25, -20, -10, -2, -7, -12, -20}},
			{4, 39, [8]float64{-26, -15, -11, -8, -5, -6, -10, -11}},
		}},
		{22, []tqCheck{
			{1, 63, [8]float64{-32, -23, -17, -11, -2, -7, -12, -19}},
			{2, 63, [8]float64{-32, -23, -17, -11, -2, -7, -12, -19}},
			{3, 39, [8]float64{-26, -15, -11, -8, -5, -6, -10, -11}},
		}},
		{23, []tqCheck{
			{1, 60, [8]float64{-34, -25, -13, -9, -4, -6, -10, -17}},
			{2, 60, [8]float64{-34, -25, -13, -9, -4, -6, -10, -17}},
			{3, 39, [8]float64{-26, -15, -11, -8, -5, -6, -10, -11}},
		}},
	}

	for _, tt := range tests {
		t.Run("Fz"+itoa(tt.fz), func(t *testing.T) {
			t.Parallel()

			fz, ok := schall03.LookupFzKategorie(tt.fz)
			require.True(t, ok, "Fz %d must exist", tt.fz)

			byM := make(map[int]schall03.Teilquelle, len(fz.Teilquellen))
			for _, tq := range fz.Teilquellen {
				byM[tq.M] = tq
			}

			for _, c := range tt.checks {
				tq, found := byM[c.m]
				require.True(t, found, "Fz%d must have Teilquelle m=%d", tt.fz, c.m)
				assert.InDelta(t, c.aa, tq.AA, 0.001, "Fz%d m=%d: a_A mismatch", tt.fz, c.m)
				assert.Equal(t, c.deltaA, [8]float64(tq.DeltaA), "Fz%d m=%d: DeltaA mismatch", tt.fz, c.m)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Beiblatt 3 — all 9 Gleisbremse types (BGBl p. 2312)
// ---------------------------------------------------------------------------

func TestBeiblatt3_AllGleisbremsen_BGBl2312(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typ     schall03.GleisbremseType
		iNum    int // table row i-number for documentation
		wantLWA float64
		wantDLW [8]float64
	}{
		{
			"i=2 Zulauf ohne Segmente", schall03.GleisbremsZulaufOhneSegmente, 2,
			110,
			[8]float64{-56, -50, -42, -32, -24, -13, -1, -12},
		},
		{
			"i=3 Talbremse ohne Segmente", schall03.GleisbremsTalbremseOhneSegmente, 3,
			105,
			[8]float64{-56, -50, -42, -32, -24, -13, -1, -12},
		},
		{
			"i=4 Talbremse mit GG", schall03.GleisbremsTalbremseMitGG, 4,
			88,
			[8]float64{-53, -46, -36, -35, -33, -9, -2, -7},
		},
		{
			"i=5 Schalloptimiert", schall03.GleisbremseSchalloptimiert, 5,
			85,
			[8]float64{-28, -23, -18, -13, -9, -6, -4, -9},
		},
		{
			"i=6 Talbremse mit Segmenten", schall03.GleisbremsTalbremsMitSegmenten, 6,
			98,
			[8]float64{-56, -52, -45, -41, -38, -9, -1, -13},
		},
		{
			"i=7 Richtung einseitig Segmente", schall03.GleisbremsRichtungEinseitigSegmente, 7,
			92,
			[8]float64{-56, -52, -45, -41, -38, -9, -1, -13},
		},
		{
			"i=8 Gummiwalkbremse", schall03.GleisbremsGummiwalk, 8,
			83,
			[8]float64{-28, -18, -12, -7, -6, -7, -8, -11},
		},
		{
			"i=9 FEW Talbremse", schall03.GleisbremsFEWTalbremse, 9,
			98,
			[8]float64{-38, -28, -23, -18, -15, -5, -3, -13},
		},
		{
			"i=10 Schraubenbremse", schall03.GleisbremsSchraubenbremse, 10,
			72,
			[8]float64{-29, -21, -9, -10, -8, -4, -9, -13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, ok := schall03.Beiblatt3GleisbremsenByType(tt.typ)
			require.True(t, ok, "Gleisbremse type i=%d must exist", tt.iNum)

			assert.InDelta(t, tt.wantLWA, data.LWA, 0.001, "L_WA mismatch for i=%d", tt.iNum)
			assert.Equal(t, tt.wantDLW, [8]float64(data.DeltaLW), "DeltaLW mismatch for i=%d", tt.iNum)
			assert.Equal(t, schall03.YardSourcePoint, data.SourceShape, "Gleisbremsen must be point sources")
			assert.InDelta(t, 0.0, data.HeightM, 0.001, "Gleisbremsen height must be 0 m")
		})
	}
}

// ---------------------------------------------------------------------------
// Beiblatt 3 — other yard sources (BGBl pp. 2312-2313)
// ---------------------------------------------------------------------------

func TestBeiblatt3_OtherYardSources_BGBl2312(t *testing.T) {
	t.Parallel()

	t.Run("Kurvenfahrgeraeusch", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3Kurvenfahrgeraeusch
		assert.InDelta(t, 69.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-27, -19, -12, -10, -8, -5, -6, -8}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourceLine, d.SourceShape)
	})

	t.Run("RetarderVerzoegerung", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3RetarderVerzoegerungsstrecke
		assert.InDelta(t, 90.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-11, -15, -15, -16, -9, -5, -8, -15}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourcePoint, d.SourceShape)
	})

	t.Run("RetarderBeharrung", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3RetarderBeharrungsstreckeBase
		assert.InDelta(t, 62.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-28, -23, -16, -12, -9, -3, -8, -14}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourceLine, d.SourceShape)
	})

	t.Run("RetarderRangieren", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3RetarderRangierenBase
		assert.InDelta(t, 72.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-30, -26, -18, -12, -9, -3, -6, -13}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourceLine, d.SourceShape)
	})

	t.Run("Hemmschuh", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3HemmschuhauflaufgeraeuschData
		assert.InDelta(t, 95.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-41, -37, -16, -21, -18, -19, -7, -1}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourcePoint, d.SourceShape)
	})

	t.Run("AuflaufstossModern", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3AuflaufstossByTech(true)
		assert.InDelta(t, 78.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-23, -15, -11, -11, -6, -5, -7, -13}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourcePoint, d.SourceShape)
	})

	t.Run("AuflaufstossOlder", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3AuflaufstossByTech(false)
		assert.InDelta(t, 91.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-25, -18, -12, -11, -6, -4, -8, -13}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourcePoint, d.SourceShape)
	})

	t.Run("Anreissen", func(t *testing.T) {
		t.Parallel()

		d := schall03.Beiblatt3AnreissenAbbremsenBase
		assert.InDelta(t, 75.0, d.LWA, 0.001)
		assert.Equal(t, [8]float64{-26, -15, -13, -9, -6, -5, -7, -12}, [8]float64(d.DeltaLW))
		assert.Equal(t, schall03.YardSourceLine, d.SourceShape)
	})
}

// itoa is a minimal int-to-string helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	neg := false
	if n < 0 {
		neg = true
		n = -n
	}

	buf := [20]byte{}
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}
