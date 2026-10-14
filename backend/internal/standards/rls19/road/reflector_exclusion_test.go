package road

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// dropExcludedBarriersByID is the exclusion as it was written before the
// reflector classes: a string compare of every candidate barrier's ID against
// the IDs of the reflectors the path bounced off. The class compare must drop
// exactly the barriers this drops, in the same order.
func dropExcludedBarriersByID(candidates []int, barriers []Barrier, exclude []string) []int {
	if len(exclude) == 0 {
		return candidates
	}

	kept := candidates[:0]

	for _, index := range candidates {
		if !slices.Contains(exclude, barriers[index].ID) {
			kept = append(kept, index)
		}
	}

	return kept
}

// exclusionByID builds the exclusion of the barriers carrying one of ids, for
// a test that names reflectors by ID rather than through a reflectorField.
func exclusionByID(barriers []Barrier, ids []string) reflectorExclusion {
	exclusion := reflectorExclusion{barrierClass: make([]int32, len(barriers)), classes: [2]int32{0, 1}, count: len(ids)}

	for i := range barriers {
		exclusion.barrierClass[i] = int32(slices.Index(ids, barriers[i].ID))
	}

	return exclusion
}

// TestReflectorClassesDropWhatIDsDrop draws barrier and reflector IDs from one
// small pool, so that IDs repeat across barriers, across reflectors and
// between the two, some reflector IDs name no barrier and some barrier IDs no
// reflector, and the empty ID is among them. Every one- and two-bounce
// exclusion a wall pair can produce must keep the same candidates both ways.
func TestReflectorClassesDropWhatIDsDrop(t *testing.T) {
	t.Parallel()

	for seed := range uint64(50) {
		rng := rand.New(rand.NewPCG(seed, 0xc1a55))

		pool := make([]string, 3+rng.IntN(8))
		for i := range pool {
			pool[i] = fmt.Sprintf("b%d", i)
		}

		pool[0] = ""

		barriers := make([]Barrier, 4+rng.IntN(20))
		for i := range barriers {
			barriers[i] = Barrier{
				ID:       pool[rng.IntN(len(pool))],
				Geometry: []geo.Point2D{{X: float64(i), Y: 0}, {X: float64(i), Y: 5}},
				HeightM:  3,
			}
		}

		reflectors := make([]Reflector, 2+rng.IntN(10))
		for i := range reflectors {
			reflectors[i] = Reflector{
				ID: pool[rng.IntN(len(pool))],
				Geometry: []geo.Point2D{
					{X: float64(i), Y: 10}, {X: float64(i) + 1, Y: 10}, {X: float64(i) + 1, Y: 12},
				},
				HeightM: 8,
			}
		}

		field := newReflectorField(reflectors)
		field.bindBarriers(barriers)

		all := appendAllIndices(nil, len(barriers))

		for i := range field.walls {
			w1 := &field.walls[i]

			for j := -1; j < len(field.walls); j++ {
				path := reflectedPath{reflectorClasses: [2]int32{w1.ownerClass, 0}, bounces: 1}
				ids := []string{w1.ownerID}

				if j >= 0 {
					w2 := &field.walls[j]
					path.reflectorClasses[1] = w2.ownerClass
					path.bounces = 2

					ids = append(ids, w2.ownerID)
				}

				got := dropExcludedBarriers(slices.Clone(all), field.exclusion(&path))
				want := dropExcludedBarriersByID(slices.Clone(all), barriers, ids)

				if !slices.Equal(got, want) {
					t.Fatalf("seed %d, reflectors %q: kept %v, by ID %v", seed, ids, got, want)
				}
			}
		}
	}
}

// TestReflectorClassesAreTheOwnersIdentity holds the classes to what the IDs
// meant: two walls share a class exactly when they share an owner ID.
func TestReflectorClassesAreTheOwnersIdentity(t *testing.T) {
	t.Parallel()

	scene := newRandomReflectorScene(7)
	field := newReflectorField(scene.reflectors)

	for i := range field.walls {
		for j := range field.walls {
			sameID := field.walls[i].ownerID == field.walls[j].ownerID
			sameClass := field.walls[i].ownerClass == field.walls[j].ownerClass

			if sameID != sameClass {
				t.Fatalf("walls %d (%q, class %d) and %d (%q, class %d)",
					i, field.walls[i].ownerID, field.walls[i].ownerClass,
					j, field.walls[j].ownerID, field.walls[j].ownerClass)
			}
		}
	}
}
