package road

import (
	"fmt"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The spatial index is only allowed to make the walk shorter, never different.
// These tests run the same scene twice — once with the index, once with the
// scratch withheld so that every obstacle is tested — and require the two to
// agree to the bit. A prune that is merely "close" fails here, which is the
// point: docs/policies/determinism.md does not have a tolerance in it.

// indexTestSources is a short kinked road, so that the Teilstücke sit at
// different angles to the obstacles rather than all at one.
func indexTestSources() []RoadSource {
	return []RoadSource{{
		ID: "road",
		Centerline: []geo.Point2D{
			{X: -40, Y: 0}, {X: 0, Y: 6}, {X: 40, Y: -4}, {X: 90, Y: 3},
		},
		SurfaceType: SurfaceSMA,
		LaneCount:   2,
		Speeds:      SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50},
		TrafficDay: TrafficInput{
			PkwPerHour: 800, Lkw1PerHour: 40, Lkw2PerHour: 30, KradPerHour: 10,
		},
		TrafficNight: TrafficInput{
			PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 8, KradPerHour: 2,
		},
	}}
}

// indexTestBuildings builds two facing rows of footprints, which is the shape
// that makes second-order reflection matter: a ray can leave one row, bounce
// off the other and come back.
//
// Two of them deliberately carry the same ID. A building becomes a Barrier
// through asBarrier, which copies that ID straight across, and nothing
// upstream promises it is unique — so an index keyed by ID would quietly
// index one of these two and lose the other.
func indexTestBuildings() []Building {
	var buildings []Building

	for i := range 8 {
		x := float64(i)*14 - 30

		buildings = append(
			buildings,
			Building{
				ID:        fmt.Sprintf("north-%d", i),
				Footprint: []geo.Point2D{{X: x, Y: 22}, {X: x + 9, Y: 22}, {X: x + 9, Y: 30}, {X: x, Y: 30}},
				HeightM:   11,
			},
			Building{
				ID:        fmt.Sprintf("south-%d", i),
				Footprint: []geo.Point2D{{X: x + 3, Y: -18}, {X: x + 12, Y: -18}, {X: x + 12, Y: -11}, {X: x + 3, Y: -11}},
				HeightM:   7,
			},
		)
	}

	return append(
		buildings,
		Building{
			ID:        "twin",
			Footprint: []geo.Point2D{{X: 60, Y: 18}, {X: 66, Y: 18}, {X: 66, Y: 24}, {X: 60, Y: 24}},
			HeightM:   9,
		},
		Building{
			ID:        "twin", // same ID on purpose — see the doc comment
			Footprint: []geo.Point2D{{X: 72, Y: 18}, {X: 78, Y: 18}, {X: 78, Y: 24}, {X: 72, Y: 24}},
			HeightM:   9,
		},
	)
}

// indexTestBarriers includes a co-located pair: two screens crossed by the
// same ray at the same distance and standing the same height. Which of them
// the Gummibandmethode reports as the effective barrier is settled by slice
// order alone, because geo.RayCrossings sorts stably — so a candidate list in
// index order gives one answer and a candidate list in cell-walk order gives
// the other.
func indexTestBarriers() []Barrier {
	return []Barrier{
		{ID: "screen-near", Geometry: []geo.Point2D{{X: -60, Y: 14}, {X: 120, Y: 14}}, HeightM: 5},
		{ID: "twin-a", Geometry: []geo.Point2D{{X: -10, Y: 17}, {X: 10, Y: 17}}, HeightM: 6},
		{ID: "twin-b", Geometry: []geo.Point2D{{X: -70, Y: 17}, {X: 130, Y: 17}}, HeightM: 6},
	}
}

func indexTestConfig() PropagationConfig {
	cfg := DefaultPropagationConfig()
	cfg.SegmentLengthM = 4
	cfg.Buildings = indexTestBuildings()

	return cfg
}

func indexTestReceivers() []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, 0, 28)

	for iy := range 4 {
		for ix := range 7 {
			receivers = append(receivers, geo.PointReceiver{
				ID:      fmt.Sprintf("r-%d-%d", ix, iy),
				Point:   geo.Point2D{X: float64(ix)*17 - 25, Y: 34 + float64(iy)*9},
				HeightM: 4 + float64(iy),
			})
		}
	}

	return receivers
}

// TestIndexedWalkMatchesUnindexedWalk is the determinism guard on the whole
// stack: the same scene, walked with and without the spatial index, must come
// out identical to the last bit.
//
// It guards every reuse the scratch carries, not only the index cursors. The
// indexed side runs the receivers off one scratch, so its contribution slices
// arrive at each receiver holding the previous receiver's numbers below the
// length they are truncated to; the nil side allocates them fresh. A reuse
// that let a stale entry through — a missed [:0], a write-back of the wrong
// slice — shows up here as a level that does not match.
func TestIndexedWalkMatchesUnindexedWalk(t *testing.T) {
	t.Parallel()

	sources := indexTestSources()
	barriers := indexTestBarriers()
	cfg := indexTestConfig()
	receivers := indexTestReceivers()

	indexed, err := ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatalf("indexed walk failed: %v", err)
	}

	scene, err := prepareScene(sources, barriers, cfg)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if scene.barrierGrid == nil || scene.reflectors.grid == nil {
		t.Fatal("this scene is supposed to exercise both indices")
	}

	for i, receiver := range receivers {
		receiverCfg := cfg
		receiverCfg.ReceiverHeightM = receiver.HeightM

		// A nil scratch is the unindexed walk: no cursor, so every barrier
		// and every wall pair is tested.
		want, err := scene.receiverLevelsOn(receiver.Point, receiverCfg, nil)
		if err != nil {
			t.Fatalf("unindexed walk failed at %q: %v", receiver.ID, err)
		}

		got := indexed[i].Indicators
		if got.LrDay != want.LrDay || got.LrNight != want.LrNight {
			t.Fatalf("receiver %q: indexed (%v, %v) != unindexed (%v, %v)",
				receiver.ID, got.LrDay, got.LrNight, want.LrDay, want.LrNight)
		}
	}
}

// TestIndexedShieldingKeepsEquidistantOrder pins the tie-break hazard on its
// own, where it is visible: the reported barrier, not just the level.
func TestIndexedShieldingKeepsEquidistantOrder(t *testing.T) {
	t.Parallel()

	barriers := indexTestBarriers()

	scene, err := prepareScene(indexTestSources(), barriers, indexTestConfig())
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	scratch := scene.newScratch()

	for ix := range 21 {
		source := geo.Point2D{X: float64(ix)*5 - 50, Y: 0}
		receiver := geo.Point2D{X: float64(ix)*5 - 50, Y: 40}

		want := ComputeShielding(source, 0.5, receiver, 4, scene.barriers)
		got := computeShielding(source, 0.5, receiver, 4, scene.barriers, nil, scratch)

		if got != want {
			t.Fatalf("source x=%v: indexed %+v != unindexed %+v", source.X, got, want)
		}
	}
}

// TestIndexedReflectionsMatchUnindexed compares the reflected paths
// themselves, in order, so that a prune that dropped or reordered one is
// caught here rather than as a level that happens to round the same.
func TestIndexedReflectionsMatchUnindexed(t *testing.T) {
	t.Parallel()

	scene, err := prepareScene(indexTestSources(), indexTestBarriers(), indexTestConfig())
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	scratch := scene.newScratch()
	total := 0

	for _, seg := range scene.sources[0].segments {
		for iy := range 4 {
			receiver := geo.Point2D{X: 12, Y: 34 + float64(iy)*9}

			want := scene.reflectors.appendPaths(nil, seg.MidPoint, seg.MidZ+0.5, receiver, 4, nil)
			got := scene.reflectors.appendPaths(nil, seg.MidPoint, seg.MidZ+0.5, receiver, 4, scratch)

			if len(got) != len(want) {
				t.Fatalf("segment at %v, receiver %v: %d paths indexed, %d unindexed",
					seg.MidPoint, receiver, len(got), len(want))
			}

			for i := range want {
				if got[i].planDistM != want[i].planDistM ||
					got[i].slantDistM != want[i].slantDistM ||
					got[i].lossDB != want[i].lossDB ||
					got[i].imagePoint != want[i].imagePoint {
					t.Fatalf("path %d differs: indexed %+v, unindexed %+v", i, got[i], want[i])
				}
			}

			total += len(want)
		}
	}

	if total == 0 {
		t.Fatal("this scene is supposed to produce reflected paths")
	}
}
