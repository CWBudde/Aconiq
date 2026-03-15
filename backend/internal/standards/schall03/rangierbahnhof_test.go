package schall03_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/aconiq/backend/internal/standards/schall03"
)

func assertApproxRbf(t *testing.T, got, want, tol float64, label string) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s: want %g, got %g (tol %g)", label, want, got, tol)
	}
}

func TestPointSourceGl3SingleBand(t *testing.T) {
	t.Parallel()
	// Gleisbremse Zulaufbremse (L_WA=110, ΔL_W,f[0]=-56 at 63 Hz)
	// n_i = 100 events/hour, no K_k corrections
	// L_WA,f,h,i = 110 + (-56) + 10·lg(100) = 110 - 56 + 20 = 74 dB
	data, ok := schall03.Beiblatt3GleisbremsenByType(schall03.GleisbremsZulaufOhneSegmente)
	if !ok {
		t.Fatal("GleisbremsZulaufOhneSegmente not found")
	}

	level := schall03.ComputePointSourceLevel(data, 100.0, 0)
	assertApproxRbf(t, level[0], 74.0, 0.01, "Gl.3 63Hz point source")
}

func TestPointSourceGl3AllBands(t *testing.T) {
	t.Parallel()
	// Hemmschuhauflaufgeräusch (L_WA=95), n_i=50, no K_k
	// Band 0 (63 Hz): 95 + (-41) + 10·lg(50) = 95 - 41 + 16.99 = 70.99
	data := schall03.Beiblatt3HemmschuhauflaufgeraeuschData
	level := schall03.ComputePointSourceLevel(data, 50.0, 0)
	assertApproxRbf(t, level[0], 70.99, 0.05, "Gl.3 Hemmschuh 63Hz")
}

func TestLineSourceGl4(t *testing.T) {
	t.Parallel()
	// Kurvenfahrgeräusch: L_WA=69, ΔL_W',f[4]=-8 (1000 Hz)
	// n_j = 200 events/hour, no K_k
	// L_W'A,f,h,j = 69 + (-8) + 10·lg(200) = 69 - 8 + 23.01 = 84.01
	data := schall03.Beiblatt3Kurvenfahrgeraeusch
	level := schall03.ComputeLineSourceLevel(data, 200.0, 0)
	assertApproxRbf(t, level[4], 84.01, 0.05, "Gl.4 Kurven 1000Hz")
}

func TestLineToPointTeilstueckGl6(t *testing.T) {
	t.Parallel()
	// Gl. 6: L_WA,f,h,kS = L_W'A,f,h + 10·lg(l_kS / l_0)
	// Given L_W'A,f,h = 80 dB, l_kS = 100 m (l_0=1m)
	// L_WA,f,h,kS = 80 + 10·lg(100) = 80 + 20 = 100 dB
	var spectrum schall03.BeiblattSpectrum
	for i := range spectrum {
		spectrum[i] = 80
	}

	result := schall03.LineToPointTeilstueck(spectrum, 100.0)
	for f, v := range result {
		assertApproxRbf(t, v, 100.0, 0.01, fmt.Sprintf("Gl.6 band %d", f))
	}
}

func TestAreaToPointTeilflaecheGl7(t *testing.T) {
	t.Parallel()
	// Gl. 7: L_WA,f,h,kF = L_W''A,f,h + 10·lg(S_kF / S_0)
	// L_W''A,f,h = 75 dB, S_kF = 400 m² (S_0=1 m²)
	// L_WA = 75 + 10·lg(400) = 75 + 26.02 = 101.02 dB
	var spectrum schall03.BeiblattSpectrum
	for i := range spectrum {
		spectrum[i] = 75
	}

	result := schall03.AreaToPointTeilflaeche(spectrum, 400.0)
	for f, v := range result {
		assertApproxRbf(t, v, 101.02, 0.05, fmt.Sprintf("Gl.7 band %d", f))
	}
}

func TestAreaSourceAggregationGl5(t *testing.T) {
	t.Parallel()
	// Simple case: two identical point sources (L_WA=90, all bands flat)
	// q_i,h = 2 (count), S_F = 100 m²
	// Each point contributes 10^(0.1·90) = 10^9 per band
	// Sum = 2 · 10^9
	// Divided by S_F/S_0 = 100 → L_W''A = 10·lg(2·10^9/100) = 10·lg(2·10^7) = 73.01 dB
	var src schall03.BeiblattSpectrum
	for i := range src {
		src[i] = 90
	}

	pointSources := []schall03.AreaPointContrib{{Level: src, Count: 2}}
	result := schall03.ComputeAreaSourceLevel(pointSources, nil, 100.0)
	assertApproxRbf(t, result[0], 73.01, 0.05, "Gl.5 area aggregation band 0")
}
