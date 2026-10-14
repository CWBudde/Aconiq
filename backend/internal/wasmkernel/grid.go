package wasmkernel

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/aconiq/backend/internal/geo"
)

// ReceiverGrid is an automatic receiver grid as browser mode sends it: where
// cell (0,0) sits, how far apart the cells are, how many there are, and the
// height every receiver stands at — instead of one JSON object per receiver.
//
// A 10 000-receiver run used to serialise ~875 KB of receivers on the main
// thread and hand the whole list to every Worker of the pool; this is six
// numbers. The grid it names is the one `buildReceiverGrid` used to build in
// TypeScript, receiver for receiver, and Receivers is held to that:
//
//   - Width and Height travel as integers. The kernel never derives them from
//     an extent, because the frontend's gridShape is the one authority on the
//     cell count — the run dialog quotes it back to the user before the run.
//   - Coordinates are origin + index×resolution, each product and each sum
//     rounded on its own, as JavaScript rounds them; see Receivers.
//   - Ids are the browser's R0001… scheme; see gridReceiverID.
//
// It is not geo.GridReceiverSet, which `aconiq run` uses: that one steps by
// accumulation and names receivers differently, and the two targets' grids
// are documented to differ in exactly that.
type ReceiverGrid struct {
	OriginX     float64 `json:"origin_x"`
	OriginY     float64 `json:"origin_y"`
	ResolutionM float64 `json:"resolution_m"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	HeightM     float64 `json:"height_m"`
}

// Validate refuses what cannot be a grid: an empty or overflowing cell count,
// a resolution that is not a positive finite distance, or a non-finite origin
// or height.
func (g ReceiverGrid) Validate() error {
	if g.Width < 1 || g.Height < 1 {
		return fmt.Errorf("grid: width and height must be at least 1, got %d×%d", g.Width, g.Height)
	}

	if g.Width > math.MaxInt/g.Height {
		return fmt.Errorf("grid: %d×%d receivers overflow the receiver count", g.Width, g.Height)
	}

	// Before Receivers allocates anything: a descriptor is six numbers, so a
	// careless one costs nothing to send and a billion receivers to expand.
	// The cap and the wording are `aconiq run`'s, so both targets refuse the
	// same grids.
	if count := g.Count(); count > geo.MaxGridReceivers {
		return fmt.Errorf("receiver grid too large (%d > %d)", count, geo.MaxGridReceivers)
	}

	if math.IsNaN(g.ResolutionM) || math.IsInf(g.ResolutionM, 0) || g.ResolutionM <= 0 {
		return fmt.Errorf("grid: resolution_m must be a positive finite distance, got %v", g.ResolutionM)
	}

	for _, value := range []float64{g.OriginX, g.OriginY, g.HeightM} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("grid: origin_x, origin_y and height_m must be finite")
		}
	}

	return nil
}

// Count is the number of receivers the grid names.
func (g ReceiverGrid) Count() int { return g.Width * g.Height }

// Receivers expands the grid row-major from the south-west corner, as
// `buildReceiverGrid` did: row by row, column by column, the sequence number
// counting from 1.
//
// Each coordinate is origin + float64(index×resolution). The inner conversion
// is load-bearing, not decoration: Go may contract a multiply-add into one
// fused instruction, and does on arm64, which rounds once where JavaScript
// rounds twice — so without it a receiver could sit one ulp away from the one
// the browser's own grid named, and its level would be computed somewhere
// else. TestGridReceiversAreUnfused holds a witness where the two differ.
func (g ReceiverGrid) Receivers() []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, 0, g.Count())
	seq := 1

	for row := range g.Height {
		y := g.OriginY + float64(float64(row)*g.ResolutionM)

		for col := range g.Width {
			x := g.OriginX + float64(float64(col)*g.ResolutionM)

			receivers = append(receivers, geo.PointReceiver{
				ID:      gridReceiverID(seq),
				Point:   geo.Point2D{X: x, Y: y},
				HeightM: g.HeightM,
			})
			seq++
		}
	}

	return receivers
}

// gridReceiverID is `R${String(seq).padStart(4, "0")}`: the sequence number
// zero-padded to at least four digits and never truncated, so the 10 000th
// receiver is R10000 on both sides. The ids reach output_hash, so a different
// spelling would be a different run.
func gridReceiverID(seq int) string {
	digits := strconv.Itoa(seq)
	for len(digits) < 4 {
		digits = "0" + digits
	}

	return "R" + digits
}
