package road

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/aconiq/backend/internal/geo"
)

// The city scene is a synthetic stand-in for the building-dense OSM extracts
// the reflection search is slow on, generated rather than imported so that no
// third-party data reaches the repository (docs/policies/data-handling.md).
//
// It is built to have the proportions that make the search expensive, not to
// look like any real place: a street grid of perimeter blocks around
// courtyards, so that facades face each other across every street and every
// courtyard; row buildings with ~20 m of frontage and ten facade segments
// each, which is what an OSM footprint averages; a two-carriageway road on
// every street plus a ring road out in the open, so that both near and
// distant Teilstücke are on the clock. Buildings in a block share party
// walls, which puts collinear and coincident wall segments into the scene for
// free.
//
// The grid is turned off the coordinate axes, because axis-aligned walls are
// the spatial index's best case and no real city is one. And the whole scene
// sits at UTM magnitudes, because the rounding a prune has to survive is a
// function of the coordinates' size and not of their differences.
//
// Everything is drawn from one seeded PCG stream in a fixed order, so the same
// spec yields the same scene, to the bit, on every machine.

// citySpec sizes one synthetic city.
type citySpec struct {
	blocksPerSide    int     // perimeter blocks along each side of the square grid
	pitchM           float64 // street centreline spacing
	streetM          float64 // building line to building line across a street
	buildingsPerSide int     // row buildings along each side of a block
	ringOffsetM      float64 // ring road distance outside the built area; 0 for none
	receiverSide     int     // the receiver patch is receiverSide × receiverSide
	barrierCount     int     // free-standing noise screens along the streets
	seed             uint64
}

// cityOrigin puts the grid at UTM-zone magnitudes (roughly where a German
// city sits in EPSG:25832), for the reason given above.
var cityOrigin = geo.Point2D{X: 548_000, Y: 5_802_000}

// cityRotationRad turns the street grid off the axes.
const cityRotationRad = 17 * math.Pi / 180

// fullCity is the benchmark scene: eight by eight blocks at a 100 m pitch,
// ~800 buildings with ~8 000 facade segments, and ~35 km of road — close to
// the 875 buildings, 8 083 facade segments and 44 km of the OSM extract that
// motivated it.
func fullCity() citySpec {
	return citySpec{
		blocksPerSide:    8,
		pitchM:           100,
		streetM:          20,
		buildingsPerSide: 3,
		ringOffsetM:      250,
		receiverSide:     4,
		barrierCount:     6,
		seed:             19,
	}
}

// smallCity is the same generator at a size a unit test can afford.
func smallCity() citySpec {
	return citySpec{
		blocksPerSide:    3,
		pitchM:           90,
		streetM:          18,
		buildingsPerSide: 2,
		ringOffsetM:      120,
		receiverSide:     2,
		barrierCount:     2,
		seed:             7,
	}
}

type cityScene struct {
	sources   []RoadSource
	barriers  []Barrier
	buildings []Building
	receivers []geo.PointReceiver
}

func (spec citySpec) build() cityScene {
	rng := rand.New(rand.NewPCG(spec.seed, 0x5eed))

	scene := cityScene{}
	scene.buildings = spec.buildingsFrom(rng)
	scene.sources = spec.roadsFrom(rng)
	scene.barriers = spec.barriersFrom(rng)
	scene.receivers = spec.receiverPatch()

	return scene
}

// config is the propagation configuration the scene is computed under.
func (s cityScene) config(segmentLengthM float64, mode SegmentLengthMode) PropagationConfig {
	cfg := DefaultPropagationConfig()
	cfg.SegmentLengthM = segmentLengthM
	cfg.SegmentLengthMode = mode
	cfg.Buildings = s.buildings

	return cfg
}

// at maps grid-local metres to model coordinates: rotated, then shifted to
// the origin.
func (spec citySpec) at(x, y float64) geo.Point2D {
	sin, cos := math.Sincos(cityRotationRad)

	return geo.Point2D{
		X: cityOrigin.X + x*cos - y*sin,
		Y: cityOrigin.Y + x*sin + y*cos,
	}
}

func uniform(rng *rand.Rand, lo, hi float64) float64 {
	return lo + rng.Float64()*(hi-lo)
}

// blockSide is one side of a perimeter block in a frame of its own: along runs
// the length of the side, across from the building line (0) into the block.
type blockSide struct {
	name     string
	from, to float64
	depth    float64
	toWorld  func(along, across float64) geo.Point2D
}

// buildingsFrom lines every block with row buildings around an open
// courtyard, and stands an outbuilding in about half of the courtyards. The
// north and south rows run the full width of the block and the east and west
// rows fit between them, so the perimeter is closed and neighbouring buildings
// share a wall line.
func (spec citySpec) buildingsFrom(rng *rand.Rand) []Building {
	var buildings []Building

	half := spec.streetM / 2

	for by := range spec.blocksPerSide {
		for bx := range spec.blocksPerSide {
			x0 := float64(bx)*spec.pitchM + half
			x1 := float64(bx+1)*spec.pitchM - half
			y0 := float64(by)*spec.pitchM + half
			y1 := float64(by+1)*spec.pitchM - half

			depthS := uniform(rng, 10, 14)
			depthN := uniform(rng, 10, 14)
			depthW := uniform(rng, 10, 14)
			depthE := uniform(rng, 10, 14)

			sides := []blockSide{
				{"s", x0, x1, depthS, func(a, c float64) geo.Point2D { return spec.at(a, y0+c) }},
				{"n", x0, x1, depthN, func(a, c float64) geo.Point2D { return spec.at(a, y1-c) }},
				{"w", y0 + depthS, y1 - depthN, depthW, func(a, c float64) geo.Point2D { return spec.at(x0+c, a) }},
				{"e", y0 + depthS, y1 - depthN, depthE, func(a, c float64) geo.Point2D { return spec.at(x1-c, a) }},
			}

			for _, side := range sides {
				buildings = spec.appendRow(rng, buildings, fmt.Sprintf("b-%d-%d-%s", bx, by, side.name), side)
			}

			if rng.IntN(2) == 0 {
				cx := uniform(rng, x0+depthW+6, x1-depthE-18)
				cy := uniform(rng, y0+depthS+6, y1-depthN-14)
				buildings = append(buildings, Building{
					ID: fmt.Sprintf("b-%d-%d-yard", bx, by),
					Footprint: []geo.Point2D{
						spec.at(cx, cy), spec.at(cx+12, cy), spec.at(cx+12, cy+8), spec.at(cx, cy+8),
					},
					HeightM: uniform(rng, 3, 7),
				})
			}
		}
	}

	return buildings
}

// appendRow splits one block side into buildingsPerSide row buildings at
// jittered party-wall positions.
func (spec citySpec) appendRow(rng *rand.Rand, dst []Building, prefix string, side blockSide) []Building {
	n := spec.buildingsPerSide
	width := (side.to - side.from) / float64(n)

	cuts := make([]float64, n+1)
	cuts[0], cuts[n] = side.from, side.to

	for i := 1; i < n; i++ {
		cuts[i] = side.from + float64(i)*width + uniform(rng, -0.2, 0.2)*width
	}

	for i := range n {
		dst = append(dst, Building{
			ID:        fmt.Sprintf("%s%d", prefix, i),
			Footprint: jogFootprint(rng, cuts[i], cuts[i+1], side.depth, side.toWorld),
			HeightM:   uniform(rng, 6, 20),
		})
	}

	return dst
}

// jogFootprint is a rectangle from along=a0 to a1, across=0 to depth, with a
// step in the street facade and a notch in the courtyard facade — ten
// vertices, so ten facade segments.
func jogFootprint(
	rng *rand.Rand,
	a0, a1, depth float64,
	toWorld func(along, across float64) geo.Point2D,
) []geo.Point2D {
	mid := uniform(rng, a0+0.3*(a1-a0), a0+0.7*(a1-a0))
	jog := uniform(rng, 0.4, 1.6)

	n0 := uniform(rng, a0+0.2*(a1-a0), a0+0.4*(a1-a0))
	n1 := uniform(rng, a0+0.6*(a1-a0), a0+0.8*(a1-a0))
	notch := uniform(rng, 1, 3)

	local := [][2]float64{
		{a0, 0},
		{mid, 0},
		{mid, jog},
		{a1, jog},
		{a1, depth},
		{n1, depth},
		{n1, depth - notch},
		{n0, depth - notch},
		{n0, depth},
		{a0, depth},
	}

	footprint := make([]geo.Point2D, 0, len(local))
	for _, p := range local {
		footprint = append(footprint, toWorld(p[0], p[1]))
	}

	return footprint
}

// roadsFrom places two carriageways, 3 m either side of the centreline, on
// every street between two crossings, then a ring road out in the open with
// one approach from each side.
func (spec citySpec) roadsFrom(rng *rand.Rand) []RoadSource {
	var sources []RoadSource

	n := spec.blocksPerSide

	add := func(id string, centerline []geo.Point2D) {
		pkw := uniform(rng, 150, 1200)
		sources = append(sources, RoadSource{
			ID:          id,
			Centerline:  centerline,
			SurfaceType: SurfaceSMA,
			LaneCount:   1,
			Speeds:      SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50},
			TrafficDay: TrafficInput{
				PkwPerHour: pkw, Lkw1PerHour: pkw * 0.04, Lkw2PerHour: pkw * 0.03, KradPerHour: pkw * 0.01,
			},
			TrafficNight: TrafficInput{
				PkwPerHour: pkw * 0.2, Lkw1PerHour: pkw * 0.01, Lkw2PerHour: pkw * 0.01, KradPerHour: pkw * 0.002,
			},
		})
	}

	for line := 0; line <= n; line++ {
		for piece := range n {
			along0 := float64(piece) * spec.pitchM
			along1 := float64(piece+1) * spec.pitchM
			across := float64(line) * spec.pitchM

			for _, lane := range []float64{-3, 3} {
				add(fmt.Sprintf("road-h-%d-%d-%v", line, piece, lane),
					[]geo.Point2D{spec.at(along0, across+lane), spec.at(along1, across+lane)})
				add(fmt.Sprintf("road-v-%d-%d-%v", line, piece, lane),
					[]geo.Point2D{spec.at(across+lane, along0), spec.at(across+lane, along1)})
			}
		}
	}

	if spec.ringOffsetM > 0 {
		lo := -spec.ringOffsetM
		hi := float64(n)*spec.pitchM + spec.ringOffsetM
		mid := float64(n/2) * spec.pitchM

		add("ring", []geo.Point2D{
			spec.at(lo, lo), spec.at(hi, lo), spec.at(hi, hi), spec.at(lo, hi), spec.at(lo, lo),
		})
		add("approach-s", []geo.Point2D{spec.at(mid, lo), spec.at(mid, 0)})
		add("approach-n", []geo.Point2D{spec.at(mid, float64(n)*spec.pitchM), spec.at(mid, hi)})
		add("approach-w", []geo.Point2D{spec.at(lo, mid), spec.at(0, mid)})
		add("approach-e", []geo.Point2D{spec.at(float64(n)*spec.pitchM, mid), spec.at(hi, mid)})
	}

	return sources
}

// barriersFrom stands a few low screens along street edges, between the road
// and the building line.
func (spec citySpec) barriersFrom(rng *rand.Rand) []Barrier {
	barriers := make([]Barrier, 0, spec.barrierCount)

	offset := spec.streetM/2 - 2

	for i := range spec.barrierCount {
		line := float64(1+rng.IntN(spec.blocksPerSide-1)) * spec.pitchM
		start := uniform(rng, 0, float64(spec.blocksPerSide-1)*spec.pitchM)
		length := uniform(rng, 40, 120)

		side := offset
		if rng.IntN(2) == 0 {
			side = -offset
		}

		var geometry []geo.Point2D
		if i%2 == 0 {
			geometry = []geo.Point2D{spec.at(start, line+side), spec.at(start+length, line+side)}
		} else {
			geometry = []geo.Point2D{spec.at(line+side, start), spec.at(line+side, start+length)}
		}

		barriers = append(barriers, Barrier{
			ID:       fmt.Sprintf("screen-%d", i),
			Geometry: geometry,
			HeightM:  uniform(rng, 2.5, 4.5),
		})
	}

	return barriers
}

// receiverPatch is a 10 m grid next to the central crossing. It reaches from
// the carriageway across the building line, so the patch has receivers in the
// open street, against a facade and — like an auto grid — inside a footprint.
func (spec citySpec) receiverPatch() []geo.PointReceiver {
	centre := float64(spec.blocksPerSide/2) * spec.pitchM

	receivers := make([]geo.PointReceiver, 0, spec.receiverSide*spec.receiverSide)

	for iy := range spec.receiverSide {
		for ix := range spec.receiverSide {
			receivers = append(receivers, geo.PointReceiver{
				ID:      fmt.Sprintf("r-%d-%d", ix, iy),
				Point:   spec.at(centre+5+float64(ix)*10, centre+5+float64(iy)*10),
				HeightM: 4,
			})
		}
	}

	return receivers
}
