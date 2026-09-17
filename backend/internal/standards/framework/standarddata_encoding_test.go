package framework

import (
	"math"
	"strings"
	"testing"
)

// digestOf is a shorthand for the digest of a single unnamed-by-content table.
func digestOf(t *testing.T, value any) string {
	t.Helper()

	digest, err := StandardData{Tables: []StandardDataTable{{Name: "t", Value: value}}}.
		Digest("unit-test", EvidenceTierTestFixture)
	if err != nil {
		t.Fatalf("digest %#v: %v", value, err)
	}

	return digest.Digest
}

// The digest exists to tell two coefficient sets apart. Each pair here differs
// only in the way named, so a digest that collapsed the difference would let a
// module change its numbers without changing its provenance.
func TestStandardDataDigestSeparatesValuesThatEncodeSimilarly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a, b any
	}{
		{name: "nil slice against empty slice", a: []float64(nil), b: []float64{}},
		{name: "nil map against empty map", a: map[string]float64(nil), b: map[string]float64{}},
		{name: "nil pointer against a pointer to zero", a: (*float64)(nil), b: new(float64)},
		{name: "NaN against positive infinity", a: math.NaN(), b: math.Inf(1)},
		{name: "positive against negative infinity", a: math.Inf(1), b: math.Inf(-1)},
		{name: "NaN against the string NaN", a: math.NaN(), b: "NaN"},
		{name: "number against its decimal string", a: 1.5, b: "1.5"},
		{name: "signed against unsigned", a: int8(-1), b: uint8(255)},
		{name: "reordered slice", a: []float64{1, 2}, b: []float64{2, 1}},
		{name: "nested empty slice against empty slice", a: [][]float64{{}}, b: [][]float64{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if digestOf(t, testCase.a) == digestOf(t, testCase.b) {
				t.Fatalf("%#v and %#v share a digest", testCase.a, testCase.b)
			}
		})
	}
}

// canonicalEncode renders a value by kind, not by Go type, so two numerically
// equal values of different numeric kinds share an encoding: a table that
// changes from []int to []float64 without changing its numbers keeps its
// digest. This pins that limit rather than leaving it to be rediscovered — the
// digest tracks the coefficients, not the Go types that hold them.
func TestStandardDataDigestIgnoresNumericGoKind(t *testing.T) {
	t.Parallel()

	if digestOf(t, []int{1, 2}) != digestOf(t, []float64{1, 2}) {
		t.Fatal("this limit has been closed; update the comment above and the note in docs")
	}

	// Values that differ numerically must still separate, including across
	// kinds.
	if digestOf(t, []int{1, 2}) == digestOf(t, []float64{1, 2.5}) {
		t.Fatal("numerically different tables share a digest")
	}
}

// Arrays encode like slices, but a fixed-size array is never nil, so an empty
// array and a nil slice must not collide.
func TestStandardDataDigestEncodesArrays(t *testing.T) {
	t.Parallel()

	if digestOf(t, [3]float64{1, 2, 3}) != digestOf(t, []float64{1, 2, 3}) {
		t.Fatal("an array and the equivalent slice disagree; the encoding is not purely structural")
	}

	if digestOf(t, [0]float64{}) == digestOf(t, []float64(nil)) {
		t.Fatal("an empty array and a nil slice share a digest")
	}
}

// NaN, +Inf and -Inf are legitimate table entries — RLS-19 Tabelle 4 uses NaN
// for "not applicable" — so they must hash rather than be rejected, and each
// must hash to itself reproducibly. encoding/json would refuse all three, which
// is why canonicalEncode exists at all.
func TestStandardDataDigestHashesNonFiniteFloats(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		first := digestOf(t, []float64{value, 1})

		second := digestOf(t, []float64{value, 1})
		if first != second {
			t.Fatalf("digest of %v is not reproducible: %s vs %s", value, first, second)
		}

		if first == "" {
			t.Fatalf("digest of %v is empty", value)
		}
	}

	// NaN is not comparable to itself, so a naive encoder that compared rather
	// than classified would be caught here.
	firstNaN := digestOf(t, math.NaN())

	secondNaN := digestOf(t, math.NaN())
	if firstNaN != secondNaN {
		t.Fatalf("two NaN tables produced different digests: %s vs %s", firstNaN, secondNaN)
	}
}

// A value kind canonicalEncode cannot represent must fail the digest rather
// than hash to something that ignores it: a silently unhashed field would let a
// module's coefficients change without its digest moving.
func TestStandardDataDigestRefusesUnrepresentableValues(t *testing.T) {
	t.Parallel()

	type withFunc struct {
		Coefficients []float64
		Lookup       func(float64) float64
	}

	cases := []struct {
		name  string
		value any
	}{
		{name: "bare function", value: func() {}},
		{name: "channel", value: make(chan int)},
		{name: "function inside a struct", value: withFunc{Coefficients: []float64{1}, Lookup: math.Abs}},
		{name: "function inside a slice", value: []any{1.0, func() {}}},
		{name: "function inside a map value", value: map[string]any{"a": func() {}}},
		{name: "channel as a map key", value: map[chan int]float64{make(chan int): 1}},
		{name: "function inside an array", value: [2]any{1.0, func() {}}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := StandardData{Tables: []StandardDataTable{{Name: "t", Value: testCase.value}}}.
				Digest("unit-test", EvidenceTierTestFixture)
			if err == nil {
				t.Fatalf("expected %#v to be refused", testCase.value)
			}

			if !strings.Contains(err.Error(), "standard data cannot contain") {
				t.Fatalf("error %q does not say the value is unrepresentable", err)
			}

			// The failing table must be named: a module carrying a dozen
			// tables needs to know which one to fix.
			if !strings.Contains(err.Error(), `table "t"`) {
				t.Fatalf("error %q does not name the offending table", err)
			}
		})
	}
}

// A nil entry inside an otherwise fine table is representable and must not be
// mistaken for a failure.
func TestStandardDataDigestAcceptsNilEntries(t *testing.T) {
	t.Parallel()

	value := map[string]any{
		"present": 1.5,
		"absent":  nil,
		"ptr":     (*float64)(nil),
		"slice":   []float64(nil),
	}

	digest, err := StandardData{Tables: []StandardDataTable{{Name: "t", Value: value}}}.
		Digest("unit-test", EvidenceTierTestFixture)
	if err != nil {
		t.Fatalf("digest with nil entries: %v", err)
	}

	if digest.IsZero() {
		t.Fatal("digest is absent for a table that carries data")
	}
}

// A table whose Value is a bare nil carries no reflect kind at all, which is a
// different path from a typed nil. It must still hash rather than panic: a
// module that declares a table it has not filled in yet is a mistake to
// discover in provenance, not a crash.
func TestStandardDataDigestAcceptsAnUntypedNilTable(t *testing.T) {
	t.Parallel()

	digest, err := StandardData{Tables: []StandardDataTable{{Name: "t", Value: nil}}}.
		Digest("unit-test", EvidenceTierTestFixture)
	if err != nil {
		t.Fatalf("digest of an untyped nil table: %v", err)
	}

	if digest.IsZero() {
		t.Fatal("a declared table produced an absent digest")
	}

	// It must not collide with a table that actually holds something.
	if digest.Tables[0].Digest == digestOf(t, 0.0) {
		t.Fatal("a nil table and a zero-valued table share a digest")
	}
}

// Map iteration order must not reach the digest. A map large enough that Go's
// randomized iteration will differ between the two calls is the direct test of
// docs/policies/determinism.md for this code path.
func TestStandardDataDigestIsIndependentOfMapIterationOrder(t *testing.T) {
	t.Parallel()

	build := func() map[string]float64 {
		m := make(map[string]float64, 64)
		for i := range 64 {
			m[string(rune('a'+i%26))+strings.Repeat("x", i)] = float64(i)
		}

		return m
	}

	want := digestOf(t, build())

	for range 16 {
		if got := digestOf(t, build()); got != want {
			t.Fatalf("digest moved with map iteration order: %s vs %s", got, want)
		}
	}
}

// The recorded algorithm has to match the digest that was actually computed, or
// a later hash change becomes invisible in the artifact.
func TestStandardDataDigestRecordsItsAlgorithmAndPerTableDigests(t *testing.T) {
	t.Parallel()

	data := StandardData{Tables: []StandardDataTable{
		{Name: "surface-corrections", Value: []float64{0, -2, 1.5}},
		{Name: "base-levels", Value: map[string]float64{"pkw": 88.5}},
	}}

	digest, err := data.Digest("rls19-road", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if digest.Algorithm != StandardDataDigestAlgorithm {
		t.Fatalf("Algorithm = %q, want %q", digest.Algorithm, StandardDataDigestAlgorithm)
	}

	if len(digest.Digest) != 64 {
		t.Fatalf("Digest = %q, want 64 hex characters for sha256", digest.Digest)
	}

	// Tables come back sorted by name so the artifact reads the same way twice.
	if len(digest.Tables) != 2 {
		t.Fatalf("recorded %d table digests, want 2", len(digest.Tables))
	}

	if digest.Tables[0].Name != "base-levels" || digest.Tables[1].Name != "surface-corrections" {
		t.Fatalf("table digests are not sorted by name: %v", digest.Tables)
	}

	for _, table := range digest.Tables {
		if len(table.Digest) != 64 {
			t.Fatalf("table %q digest = %q, want 64 hex characters", table.Name, table.Digest)
		}
	}

	if digest.StandardID != "rls19-road" || digest.EvidenceTier != EvidenceTierNormative {
		t.Fatalf("digest identity = %q/%q", digest.StandardID, digest.EvidenceTier)
	}
}

// A table's digest is bound to its name, so renaming a table deliberately moves
// the digest while the values stay put.
func TestStandardDataTableDigestIsBoundToItsName(t *testing.T) {
	t.Parallel()

	value := []float64{1, 2, 3}

	under := func(name string) string {
		digest, err := StandardData{Tables: []StandardDataTable{{Name: name, Value: value}}}.
			Digest("unit-test", EvidenceTierTestFixture)
		if err != nil {
			t.Fatalf("digest under %q: %v", name, err)
		}

		return digest.Tables[0].Digest
	}

	if under("tabelle-4") == under("tabelle-5") {
		t.Fatal("two differently named tables with the same values share a table digest")
	}
}

func TestStandardDataIsEmptyAndDigestIsZero(t *testing.T) {
	t.Parallel()

	var empty StandardData

	if !empty.IsEmpty() {
		t.Fatal("the zero StandardData is not reported as empty")
	}

	if !(StandardData{Tables: []StandardDataTable{}}).IsEmpty() {
		t.Fatal("a StandardData with no tables is not reported as empty")
	}

	digest, err := empty.Digest("dummy-freefield", EvidenceTierTestFixture)
	if err != nil {
		t.Fatalf("digest of an empty StandardData: %v", err)
	}

	if !digest.IsZero() {
		t.Fatalf("expected an absent digest for a module without data, got %#v", digest)
	}

	nonEmpty := StandardData{Tables: []StandardDataTable{{Name: "t", Value: 1.0}}}
	if nonEmpty.IsEmpty() {
		t.Fatal("a StandardData with one table is reported as empty")
	}

	present, err := nonEmpty.Digest("dummy", EvidenceTierTestFixture)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if present.IsZero() {
		t.Fatal("a module with one table produced an absent digest")
	}
}
