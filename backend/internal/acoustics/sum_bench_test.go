package acoustics_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/acoustics"
)

// The term counts an RLS-19 road run actually produces. A receiver's
// contribution slice holds one entry per Teilstück plus one per valid
// Spiegelschallquelle, so an open-field 2 km road at the default 1 m split is
// a few thousand terms and a building-dense scene is a few hundred thousand.
var benchTermCounts = []int{8, 64, 2000, 20000}

func benchLevels(n int) []float64 {
	levels := make([]float64, n)
	for i := range levels {
		// A plausible spread rather than a constant: divergence alone puts
		// ~34 dB between the nearest and furthest Teilstück of a 2 km road,
		// and math.Pow's cost is not input-independent.
		levels[i] = 35 + 34*float64(i%997)/997
	}

	return levels
}

func BenchmarkEnergySum(b *testing.B) {
	for _, n := range benchTermCounts {
		b.Run(termName(n), func(b *testing.B) {
			levels := benchLevels(n)

			b.ReportAllocs()

			var sink float64

			for b.Loop() {
				sink = acoustics.EnergySum(levels)
			}

			runtimeSink = sink
		})
	}
}

// BenchmarkLevelToEnergy isolates the single operation that dominates
// EnergySum, so the Pow-versus-Exp question is answered without the loop and
// the guards around it.
//
// math.Pow(10, x) has no base-10 fast path: it is Frexp/Modf/Log/Exp/Sqrt/Ldexp.
// math.Exp(x * Ln10/10) is one Exp. The two differ by about one ulp, which is
// why this is a measurement here and a deliberate, declared numerics change
// elsewhere — never a quiet substitution.
func BenchmarkLevelToEnergy(b *testing.B) {
	const ln10Over10 = math.Ln10 / 10

	levels := benchLevels(2000)

	b.Run("pow", func(b *testing.B) {
		var total float64

		for b.Loop() {
			total = 0
			for _, level := range levels {
				total += math.Pow(10, level/10)
			}
		}

		runtimeSink = total
	})

	b.Run("exp", func(b *testing.B) {
		var total float64

		for b.Loop() {
			total = 0
			for _, level := range levels {
				total += math.Exp(level * ln10Over10)
			}
		}

		runtimeSink = total
	})
}

// runtimeSink keeps the compiler from eliminating the sums above.
var runtimeSink float64

func termName(n int) string {
	switch n {
	case 8:
		return "terms=8"
	case 64:
		return "terms=64"
	case 2000:
		return "terms=2000"
	default:
		return "terms=20000"
	}
}
