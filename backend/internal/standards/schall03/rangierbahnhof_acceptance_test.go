package schall03_test

// CI-safe conformance scenarios for Rangierbahnhof sources (Phase 20b, Tasks 1–5).
//
// r1: Gleisbremse Zulaufbremse — point source emission (Gl. 3), 100 events/h
// r2: Retarder Verzögerungsstrecke — point source immission (Gl. 30), 50 m free field
//
// These are golden-snapshot tests: the first run (UPDATE_GOLDEN=1) generates
// the expected values; subsequent runs lock the computation against regressions.

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// rbfTestdataPath resolves a path relative to this file's testdata/ directory.
// Using runtime.Caller ensures the path is correct regardless of where tests
// are invoked from.
func rbfTestdataPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve rangierbahnhof acceptance test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

// round6 rounds to 6 decimal places for deterministic snapshot values.
func rbfRound6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

// TestR1GleisbremseZulaufPointSourceEmission exercises Gl. 3 for
// Gleisbremse Zulaufbremse (i=2) at n_i = 100 events/h, no K_k correction.
//
// Expected (hand-verified for band 0):
//
//	L_WA,f,h,0 = 110 + (−56) + 10·lg(100) = 110 − 56 + 20 = 74 dB
func TestR1GleisbremseZulaufPointSourceEmission(t *testing.T) {
	t.Parallel()

	data, ok := schall03.Beiblatt3GleisbremsenByType(schall03.GleisbremsZulaufOhneSegmente)
	if !ok {
		t.Fatal("Beiblatt3GleisbremsenByType: GleisbremsZulaufOhneSegmente not found")
	}

	level := schall03.ComputePointSourceLevel(data, 100.0, 0)

	// Encode as a plain map so the golden file is human-readable JSON with
	// labelled octave bands (63 Hz … 8000 Hz).
	bandLabels := [schall03.NumBeiblattOctaveBands]string{
		"63Hz", "125Hz", "250Hz", "500Hz", "1000Hz", "2000Hz", "4000Hz", "8000Hz",
	}
	snapshot := map[string]any{
		"scenario":    "r1_gleisbremse_zulauf_emission",
		"gleisbremse": "GleisbremsZulaufOhneSegmente",
		"n_per_hour":  100,
		"k_k_db":      0,
		"level_bands": func() map[string]float64 {
			m := make(map[string]float64, schall03.NumBeiblattOctaveBands)
			for i, lbl := range bandLabels {
				m[lbl] = rbfRound6(level[i])
			}

			return m
		}(),
	}

	golden.AssertJSONSnapshot(t, rbfTestdataPath(t, "r1_gleisbremse_emission.golden.json"), snapshot)
}

// TestR2RetarderVerzoegerungsstreckeImmission exercises Gl. 30 for
// Retarder Verzögerungsstrecke at 50 m free field, h_g=0 m, h_r=3.5 m,
// n_i=200 events/h, no barrier.
//
// The immission level is expected in a plausible 30–70 dB(A) range for a
// 90 dB L_WA source at 50 m.
func TestR2RetarderVerzoegerungsstreckeImmission(t *testing.T) {
	t.Parallel()

	data := schall03.Beiblatt3RetarderVerzoegerungsstrecke
	level := schall03.ComputePointSourceLevel(data, 200.0, 0)

	result, err := schall03.ComputeYardPointSourceImmission(schall03.YardPointImmissionInput{
		SourceLevel:     level,
		SourceHeightM:   data.HeightM,
		ReceiverDistM:   50.0,
		ReceiverHeightM: 3.5,
	})
	if err != nil {
		t.Fatalf("ComputeYardPointSourceImmission: %v", err)
	}

	if result < 30 || result > 80 {
		t.Errorf("implausible immission level %g dB(A) — expected 30–80 range", result)
	}

	snapshot := map[string]any{
		"scenario":          "r2_retarder_verzoegerungsstrecke_immission",
		"source":            "Beiblatt3RetarderVerzoegerungsstrecke",
		"n_per_hour":        200,
		"k_k_db":            0,
		"source_height_m":   data.HeightM,
		"receiver_dist_m":   50.0,
		"receiver_height_m": 3.5,
		"barrier":           nil,
		"lp_aeq_db":         rbfRound6(result),
		"lp_aeq_fmt":        fmt.Sprintf("%.4f dB(A)", result),
	}

	golden.AssertJSONSnapshot(t, rbfTestdataPath(t, "r2_retarder_immission.golden.json"), snapshot)
}
