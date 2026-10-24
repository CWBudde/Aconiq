package geo

import (
	"errors"
	"fmt"
	"math"
)

// Homography is a projective map of the plane, row-major: a point (x, y)
// goes to ((h0 x + h1 y + h2) / w, (h3 x + h4 y + h5) / w) with
// w = h6 x + h7 y + h8.
//
// FacadeHomography solves the one that takes a photo of a facade onto the
// facade's frame (P18 D1): pixels in, `u` metres along the facade and `z`
// metres above its base out.
type Homography [9]float64

// convexityTolerance is how small a turn at a corner may be, relative to the
// square of the quadrilateral's size, before the corner counts as straight.
const convexityTolerance = 1e-9

// FacadeHomography solves the homography that maps each image point onto the
// facade point at the same index: four pairs, the facade's corners as a
// person clicks them in a photo of it, and where those corners are in the
// facade's frame (docs/research/facade-receivers.md, Windows).
//
// Image points are pixels with y running down, as a photo's do; facade points
// have z running up. Both quadrilaterals must be convex and given in the same
// order round, which after the flip of the y axis means opposite signs of
// their raw signed areas: an image whose points turn the facade's way is a
// mirrored photo, or corners given in another order, and is refused rather
// than solved into a facade seen from behind.
//
// The solve is the direct linear transform with h8 fixed, on points
// normalised to their centroid and a mean distance of √2 (Hartley), by
// Gaussian elimination with partial pivoting: a fixed sequence of float64
// operations, every product rounded on its own (an explicit float64
// conversion stops the compiler fusing it into an FMA on arm64), so the same
// pairs give the same matrix, bit for bit, on every target. The matrix
// is scaled so that w is 1 at the image points' centroid, and so positive
// over the whole quadrilateral.
func FacadeHomography(image, facade [4]Point2D) (Homography, error) {
	for _, set := range []struct {
		name   string
		points [4]Point2D
	}{{"image", image}, {"facade", facade}} {
		for i, p := range set.points {
			if !isFinite(p.X) || !isFinite(p.Y) {
				return Homography{}, fmt.Errorf("%s point %d is not finite", set.name, i)
			}
		}
	}

	imageTurn, ok := convexTurn(image)
	if !ok {
		return Homography{}, errNotConvex("image")
	}

	facadeTurn, ok := convexTurn(facade)
	if !ok {
		return Homography{}, errNotConvex("facade")
	}

	if imageTurn == facadeTurn {
		return Homography{}, errors.New("the image points turn the same way round as the facade's, where a photo's " +
			"y axis runs down: the photo is mirrored, or its corners are not given in the facade's order " +
			"(bottom-left, bottom-right, top-right, top-left)")
	}

	imageNorm, imageScale, imageCentre := normalised(image)
	facadeNorm, facadeScale, facadeCentre := normalised(facade)

	var a [8][9]float64

	for i := range 4 {
		x, y := imageNorm[i].X, imageNorm[i].Y
		u, v := facadeNorm[i].X, facadeNorm[i].Y
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}

	hn, ok := solve8(a)
	if !ok {
		return Homography{}, errors.New("the point pairs determine no homography")
	}

	// Undo the normalisation: H = F⁻¹ · Hn · I, with I taking image points to
	// their normalised form and F facade points to theirs.
	in := Homography{imageScale, 0, -imageScale * imageCentre.X, 0, imageScale, -imageScale * imageCentre.Y, 0, 0, 1}
	outInv := Homography{1 / facadeScale, 0, facadeCentre.X, 0, 1 / facadeScale, facadeCentre.Y, 0, 0, 1}
	h := outInv.times(Homography{hn[0], hn[1], hn[2], hn[3], hn[4], hn[5], hn[6], hn[7], 1}).times(in)

	w := float64(h[6]*imageCentre.X) + float64(h[7]*imageCentre.Y) + h[8]
	if w == 0 || !isFinite(w) {
		return Homography{}, errors.New("the point pairs determine no homography")
	}

	for i := range h {
		h[i] /= w
	}

	for i, p := range image {
		if float64(h[6]*p.X)+float64(h[7]*p.Y)+h[8] <= 0 {
			return Homography{}, fmt.Errorf("image point %d lies beyond the facade plane's horizon in the photo", i)
		}
	}

	return h, nil
}

// Apply maps p, and reports false for a point on the horizon, which maps to
// no finite point.
func (h Homography) Apply(p Point2D) (Point2D, bool) {
	w := h[6]*p.X + h[7]*p.Y + h[8]

	x := (h[0]*p.X + h[1]*p.Y + h[2]) / w
	y := (h[3]*p.X + h[4]*p.Y + h[5]) / w

	if !isFinite(x) || !isFinite(y) {
		return Point2D{}, false
	}

	return Point2D{X: x, Y: y}, true
}

// Inverse is the map back: the facade frame onto the photo. It is the matrix
// inverse, so where h has w > 0 the inverse has too.
func (h Homography) Inverse() (Homography, error) {
	// Each product rounded on its own, as in FacadeHomography.
	minor := func(a, b, c, d float64) float64 { return float64(a*b) - float64(c*d) }
	adj := Homography{
		minor(h[4], h[8], h[5], h[7]), minor(h[2], h[7], h[1], h[8]), minor(h[1], h[5], h[2], h[4]),
		minor(h[5], h[6], h[3], h[8]), minor(h[0], h[8], h[2], h[6]), minor(h[2], h[3], h[0], h[5]),
		minor(h[3], h[7], h[4], h[6]), minor(h[1], h[6], h[0], h[7]), minor(h[0], h[4], h[1], h[3]),
	}

	det := float64(h[0]*adj[0]) + float64(h[1]*adj[3]) + float64(h[2]*adj[6])
	if det == 0 || !isFinite(det) {
		return Homography{}, errors.New("the homography is singular")
	}

	for i := range adj {
		adj[i] /= det
	}

	return adj, nil
}

func (h Homography) times(o Homography) Homography {
	var out Homography

	for r := range 3 {
		for c := range 3 {
			out[3*r+c] = float64(h[3*r]*o[c]) + float64(h[3*r+1]*o[3+c]) + float64(h[3*r+2]*o[6+c])
		}
	}

	return out
}

// PhotoTransform is the D1 evidence a window rectified from a photo carries:
// the four image points a person clicked and the facade-frame points they
// stand for, each as [x, y]. It is what a window's evidence `transform`
// holds, so the same record always solves to the same homography.
type PhotoTransform struct {
	ImagePoints  [][]float64 `json:"image_points"`
	FacadePoints [][]float64 `json:"facade_points"`
}

// Homography checks the transform's shape and solves it through
// FacadeHomography.
func (t PhotoTransform) Homography() (Homography, error) {
	image, err := fourPoints("image_points", t.ImagePoints)
	if err != nil {
		return Homography{}, err
	}

	facade, err := fourPoints("facade_points", t.FacadePoints)
	if err != nil {
		return Homography{}, err
	}

	return FacadeHomography(image, facade)
}

func fourPoints(name string, raw [][]float64) ([4]Point2D, error) {
	var out [4]Point2D

	if len(raw) != len(out) {
		return out, fmt.Errorf("%s must hold 4 points, got %d", name, len(raw))
	}

	for i, p := range raw {
		if len(p) != 2 {
			return out, fmt.Errorf("%s[%d] must be [x, y], got %d numbers", name, i, len(p))
		}

		out[i] = Point2D{X: p[0], Y: p[1]}
	}

	return out, nil
}

func errNotConvex(name string) error {
	return fmt.Errorf("the %s points are not the corners of a convex quadrilateral in the order given: "+
		"two coincide, three lie on one line, or the order crosses over itself", name)
}

// convexTurn reports which way a strictly convex quadrilateral turns at
// every corner (+1 counter-clockwise in its own axes, -1 clockwise), and false
// when it is not one: a corner that turns the other way, or one too straight
// to tell, which is also what two coinciding points give.
func convexTurn(q [4]Point2D) (int, bool) {
	size := 0.0

	for i := range q {
		for j := i + 1; j < len(q); j++ {
			size = math.Max(size, math.Hypot(q[j].X-q[i].X, q[j].Y-q[i].Y))
		}
	}

	limit := convexityTolerance * size * size
	turn := 0

	for i := range q {
		a, b, c := q[i], q[(i+1)%4], q[(i+2)%4]

		cross := (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
		if math.Abs(cross) <= limit {
			return 0, false
		}

		sign := 1
		if cross < 0 {
			sign = -1
		}

		if turn != 0 && sign != turn {
			return 0, false
		}

		turn = sign
	}

	return turn, true
}

// normalised moves the points to their centroid and scales them to a mean
// distance of √2 from it, and returns the scale and the centroid.
func normalised(q [4]Point2D) ([4]Point2D, float64, Point2D) {
	var c Point2D
	for _, p := range q {
		c.X += p.X / 4
		c.Y += p.Y / 4
	}

	mean := 0.0
	for _, p := range q {
		mean += math.Hypot(p.X-c.X, p.Y-c.Y) / 4
	}

	scale := math.Sqrt2 / mean

	var out [4]Point2D
	for i, p := range q {
		out[i] = Point2D{X: (p.X - c.X) * scale, Y: (p.Y - c.Y) * scale}
	}

	return out, scale, c
}

// solve8 solves the 8×8 system whose augmented matrix is a, by Gaussian
// elimination with partial pivoting.
func solve8(a [8][9]float64) ([8]float64, bool) {
	const n = 8

	for col := range n {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[pivot][col]) {
				pivot = r
			}
		}

		if math.Abs(a[pivot][col]) < 1e-12 {
			return [8]float64{}, false
		}

		a[col], a[pivot] = a[pivot], a[col]

		for r := col + 1; r < n; r++ {
			f := a[r][col] / a[col][col]
			for c := col; c <= n; c++ {
				a[r][c] -= float64(f * a[col][c])
			}
		}
	}

	var x [8]float64

	for r := n - 1; r >= 0; r-- {
		s := a[r][n]
		for c := r + 1; c < n; c++ {
			s -= float64(a[r][c] * x[c])
		}

		x[r] = s / a[r][r]
	}

	return x, true
}
