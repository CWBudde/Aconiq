package road

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aconiq/backend/internal/jsonio"
	"github.com/aconiq/backend/internal/qa/golden"
)

// The city levels are pinned end to end, to the bit, from the source-side
// reflection search: testdata/city-levels.golden.json was written by the code
// before the receiver-side search and the barrier fan existed, and every
// level here must still come out of it unchanged. Where the property tests
// compare the searches path by path, this compares what a user reads.
//
// It has been rewritten once, deliberately, when the acceptance test both
// searches share changed: a specular point a rounding past a facade end now
// counts, and a two-bounce path through a corner does not (see
// reflectionEndToleranceM). The rewrite ran on amd64, with the property tests
// holding the two searches to the same paths under the new test.

// assertSourceSideGolden compares levels against a golden that the source-side
// search wrote, and refuses to let `just update-golden` rewrite it.
//
// These goldens are evidence only for as long as their provenance holds: they
// were generated from origin/main's code, before the receiver-side search
// existed, and the claim that the new search changes no number rests on that.
// UPDATE_GOLDEN rewrites a snapshot from whatever the code produces now, so a
// routine regeneration would silently replace the old search's numbers with
// the new search's, and the comparison would become a tautology. Rewriting
// them therefore takes a second, deliberate variable,
// ACONIQ_REWRITE_SOURCE_SIDE_GOLDENS=1, and should only ever be done from a
// checkout whose RLS-19 levels are known to be right by other means — a change
// to the standard's arithmetic, reviewed as such.
func assertSourceSideGolden(t *testing.T, name string, levels []cityLevel) {
	t.Helper()

	path := filepath.Join("testdata", name)

	// Before the rewrite as well: a golden rewritten off amd64 would hold
	// that machine's bits, and every amd64 job would then fail against it.
	if golden.SkipAMD64Bits(t) {
		return
	}

	if golden.UpdateEnabled() && os.Getenv("ACONIQ_REWRITE_SOURCE_SIDE_GOLDENS") == "1" {
		golden.AssertJSONSnapshot(t, path, levels)

		return
	}

	got, err := jsonio.Marshal(levels)
	if err != nil {
		t.Fatalf("marshal levels: %v", err)
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("levels differ from %s, which the source-side search wrote "+
			"(UPDATE_GOLDEN alone does not rewrite it; see assertSourceSideGolden)\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// cityLevel is one receiver's levels, in decimal for a reader and as the
// float64's bits, which is what the comparison is made on.
type cityLevel struct {
	Receiver     string `json:"receiver"`
	LrDay        string `json:"lr_day"`
	LrNight      string `json:"lr_night"`
	LrDayBits    string `json:"lr_day_bits"`
	LrNightBits  string `json:"lr_night_bits"`
	SegmentMode  string `json:"segment_mode"`
	SegmentLenM  string `json:"segment_length_m"`
	CitySizeName string `json:"city"`
}

func cityLevels(t *testing.T, name string, spec citySpec, receivers int, segmentLengthM float64, mode SegmentLengthMode) []cityLevel {
	t.Helper()

	city := spec.build()

	outputs, err := ComputeReceiverOutputs(city.receivers[:receivers], city.sources, city.barriers, city.config(segmentLengthM, mode))
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}

	levels := make([]cityLevel, 0, len(outputs))

	for _, output := range outputs {
		day, night := output.Indicators.LrDay, output.Indicators.LrNight
		levels = append(levels, cityLevel{
			Receiver:     output.Receiver.ID,
			LrDay:        strconv.FormatFloat(day, 'f', 6, 64),
			LrNight:      strconv.FormatFloat(night, 'f', 6, 64),
			LrDayBits:    strconv.FormatUint(math.Float64bits(day), 16),
			LrNightBits:  strconv.FormatUint(math.Float64bits(night), 16),
			SegmentMode:  string(mode.resolved()),
			SegmentLenM:  strconv.FormatFloat(segmentLengthM, 'f', -1, 64),
			CitySizeName: name,
		})
	}

	return levels
}

func TestCityLevelsMatchTheSourceSideSearch(t *testing.T) {
	t.Parallel()

	levels := cityLevels(t, "small", smallCity(), 2, 6, SegmentLengthFixed)
	levels = append(levels, cityLevels(t, "small", smallCity(), 2, 1, SegmentLengthDistanceScaled)...)

	assertSourceSideGolden(t, "city-levels.golden.json", levels)
}

// TestFullCityLevelsMatchTheSourceSideSearch is the same on the benchmark
// scene. It is opt-in, because the race detector multiplies its ten seconds.
func TestFullCityLevelsMatchTheSourceSideSearch(t *testing.T) {
	t.Parallel()

	if os.Getenv("ACONIQ_EXHAUSTIVE_REFLECTION_CHECK") != "1" {
		t.Skip("set ACONIQ_EXHAUSTIVE_REFLECTION_CHECK=1 to compute the full city")
	}

	levels := cityLevels(t, "full", fullCity(), 1, 1, SegmentLengthDistanceScaled)

	assertSourceSideGolden(t, "city-levels-full.golden.json", levels)
}
