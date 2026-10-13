package road

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
)

// The city levels are pinned end to end, to the bit, from the source-side
// reflection search: testdata/city-levels.golden.json was written by the code
// before the receiver-side search and the barrier fan existed, and every
// level here must still come out of it unchanged. Where the property tests
// compare the searches path by path, this compares what a user reads.

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

	golden.AssertJSONSnapshot(t, filepath.Join("testdata", "city-levels.golden.json"), levels)
}

// TestFullCityLevelsMatchTheSourceSideSearch is the same on the benchmark
// scene. It is opt-in, because the race detector multiplies its ten seconds.
func TestFullCityLevelsMatchTheSourceSideSearch(t *testing.T) {
	t.Parallel()

	if os.Getenv("ACONIQ_EXHAUSTIVE_REFLECTION_CHECK") != "1" {
		t.Skip("set ACONIQ_EXHAUSTIVE_REFLECTION_CHECK=1 to compute the full city")
	}

	levels := cityLevels(t, "full", fullCity(), 1, 1, SegmentLengthDistanceScaled)

	golden.AssertJSONSnapshot(t, filepath.Join("testdata", "city-levels-full.golden.json"), levels)
}
