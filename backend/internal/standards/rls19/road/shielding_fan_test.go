package road

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The barrier fan may only narrow the candidates the crossing test sees, never
// change what that test finds. These tests shield the same paths with the fan
// and with no index at all — every barrier tested — and require the same
// ShieldingResult to the bit, including which barrier is reported.

// fanComparison shields paths ending at one receiver both ways.
type fanComparison struct {
	t        *testing.T
	barriers []Barrier
	scratch  *pathScratch

	paths, shielded int
}

func (c *fanComparison) check(source geo.Point2D, sourceHeightM float64, receiver geo.Point2D, exclude []string) {
	c.t.Helper()

	exclusion := exclusionByID(c.barriers, exclude)
	want := computeShielding(source, sourceHeightM, receiver, 4, c.barriers, exclusion, nil)
	got := computeShielding(source, sourceHeightM, receiver, 4, c.barriers, exclusion, c.scratch)

	if got != want {
		c.t.Fatalf("source %v, receiver %v, exclude %v: fan %+v, unindexed %+v", source, receiver, exclude, got, want)
	}

	c.paths++

	if want.Shielded {
		c.shielded++
	}
}

func (c *fanComparison) requireShielding() {
	c.t.Helper()

	if c.shielded == 0 || c.shielded == c.paths {
		c.t.Fatalf("%d of %d paths shielded; the scene must produce both kinds", c.shielded, c.paths)
	}

	c.t.Logf("%d paths compared, %d of them shielded", c.paths, c.shielded)
}

// TestFanShieldingMatchesUnindexedOnCity shields rays from Teilstücke and
// from far-off points, as a mirrored path's image source is, into receivers
// in the street, against a facade, inside a footprint and on a corner.
func TestFanShieldingMatchesUnindexedOnCity(t *testing.T) {
	t.Parallel()

	spec := smallCity()
	city := spec.build()

	scene, err := prepareScene(city.sources, city.barriers, city.config(6, ""))
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	receivers := make([]geo.Point2D, 0, len(city.receivers)+3)
	for _, receiver := range city.receivers {
		receivers = append(receivers, receiver.Point)
	}

	receivers = append(
		receivers,
		spec.at(spec.pitchM/2, spec.pitchM/2), // a courtyard, ringed by footprints
		city.buildings[3].Footprint[2],        // on a corner
		city.barriers[0].Geometry[0],          // on a screen's end
	)

	comparison := fanComparison{t: t, barriers: scene.barriers, scratch: scene.newScratch()}
	rng := rand.New(rand.NewPCG(5, 6))
	stride := exhaustiveStride(13)

	for r, receiver := range receivers {
		count := r

		for _, source := range scene.sources {
			for _, seg := range source.segments {
				if count%stride == 0 {
					comparison.check(seg.MidPoint, 0.5, receiver, nil)

					// The same ray, as a mirrored path off a building would
					// arrive: with that building excluded, from a point well
					// beyond the Teilstück.
					building := city.buildings[rng.IntN(len(city.buildings))]
					far := geo.Point2D{
						X: receiver.X + 3*(seg.MidPoint.X-receiver.X),
						Y: receiver.Y + 3*(seg.MidPoint.Y-receiver.Y),
					}
					comparison.check(far, 0.5, receiver, []string{building.ID})
				}

				count++
			}
		}
	}

	comparison.requireShielding()
}

// TestFanShieldingMatchesUnindexedOnRandomScenes plants the cases the fan's
// clearance and wrap handling exist for: a polyline through the receiver, one
// passing within a metre of it, one coiled all the way around it, and rays
// that run along a barrier.
func TestFanShieldingMatchesUnindexedOnRandomScenes(t *testing.T) {
	t.Parallel()

	var total fanComparison

	total.t = t

	seeds := uint64(40)
	if exhaustiveStride(2) == 1 {
		seeds = 400
	}

	for seed := range seeds {
		rng := rand.New(rand.NewPCG(seed, 99))

		origin := geo.Point2D{}
		if seed%2 == 1 {
			origin = geo.Point2D{X: 548_000 + uniform(rng, 0, 1000), Y: 5_802_000 + uniform(rng, 0, 1000)}
		}

		receiver := geo.Point2D{X: origin.X + 150, Y: origin.Y + 150}
		box := func() geo.Point2D {
			return geo.Point2D{X: origin.X + uniform(rng, 0, 300), Y: origin.Y + uniform(rng, 0, 300)}
		}

		var barriers []Barrier

		add := func(height float64, points ...geo.Point2D) {
			barriers = append(barriers, Barrier{ID: fmt.Sprintf("b-%d", len(barriers)), Geometry: points, HeightM: height})
		}

		for range 30 {
			start := box()
			points := []geo.Point2D{start}

			for range 1 + rng.IntN(4) {
				angle := uniform(rng, 0, 2*math.Pi)
				length := uniform(rng, 2, 80)
				last := points[len(points)-1]
				points = append(points, geo.Point2D{X: last.X + length*math.Cos(angle), Y: last.Y + length*math.Sin(angle)})
			}

			add(uniform(rng, 1, 20), points...)
		}

		// Through the receiver, grazing it, and coiled around it.
		add(8, geo.Point2D{X: receiver.X - 20, Y: receiver.Y - 5}, receiver, geo.Point2D{X: receiver.X + 30, Y: receiver.Y + 2})
		add(6, geo.Point2D{X: receiver.X - 40, Y: receiver.Y + 0.4}, geo.Point2D{X: receiver.X + 40, Y: receiver.Y + 0.6})

		if seed%3 == 0 {
			coil := make([]geo.Point2D, 0, 40)

			for k := range 40 {
				angle := float64(k) * 0.2
				radius := 30 + float64(k)
				coil = append(coil, geo.Point2D{X: receiver.X + radius*math.Cos(angle), Y: receiver.Y + radius*math.Sin(angle)})
			}

			add(12, coil...)
		}

		comparison := fanComparison{t: t, barriers: barriers, scratch: &pathScratch{}}

		for range 60 {
			comparison.check(box(), uniform(rng, 0.5, 3), receiver, nil)
		}

		// Rays that run along a barrier edge, from beyond its end, and from
		// a vertex.
		for _, barrier := range barriers[:10] {
			a, b := barrier.Geometry[0], barrier.Geometry[1]
			comparison.check(geo.Point2D{X: 2*b.X - a.X, Y: 2*b.Y - a.Y}, 0.5, receiver, nil)
			comparison.check(a, 0.5, receiver, []string{barrier.ID})
		}

		// Far sources, as image sources are.
		for range 20 {
			angle := uniform(rng, 0, 2*math.Pi)
			distance := uniform(rng, 300, 3000)
			comparison.check(geo.Point2D{
				X: receiver.X + distance*math.Cos(angle),
				Y: receiver.Y + distance*math.Sin(angle),
			}, 0.5, receiver, nil)
		}

		total.paths += comparison.paths
		total.shielded += comparison.shielded
	}

	total.requireShielding()
}

// TestFanIntervalMarksNearAndCoiledBarriersOmni pins the two refusals the
// fan's exactness argument rests on.
func TestFanIntervalMarksNearAndCoiledBarriersOmni(t *testing.T) {
	t.Parallel()

	receiver := geo.Point2D{X: 10, Y: 10}

	cases := []struct {
		name string
		line []geo.Point2D
		omni bool
	}{
		{"far and narrow", []geo.Point2D{{X: 100, Y: 0}, {X: 100, Y: 20}}, false},
		{"through the receiver", []geo.Point2D{{X: 0, Y: 10}, {X: 20, Y: 10}}, true},
		{"within the clearance", []geo.Point2D{{X: 0, Y: 10.5}, {X: 20, Y: 10.5}}, true},
		{"ring around it", []geo.Point2D{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20}, {X: 0, Y: 0}}, true},
		{"three sides of it", []geo.Point2D{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20}}, false},
	}

	for _, tc := range cases {
		_, _, _, ok := fanInterval(receiver, tc.line)
		if ok == tc.omni {
			t.Errorf("%s: interval ok=%v, want omni=%v", tc.name, ok, tc.omni)
		}
	}
}
