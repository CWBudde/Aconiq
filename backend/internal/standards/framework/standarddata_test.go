package framework

import (
	"math"
	"strings"
	"testing"
)

type coefficientRow struct {
	Name    string
	Values  [3]float64
	hidden  float64
	Pointer *float64
}

func sampleData() StandardData {
	hidden := 1.5

	return StandardData{Tables: []StandardDataTable{
		{Name: "b/second", Value: map[string]float64{"alpha": 1, "beta": 2, "gamma": math.NaN()}},
		{Name: "a/first", Value: []coefficientRow{
			{Name: "row-1", Values: [3]float64{1, 2, 3}, hidden: hidden, Pointer: &hidden},
			{Name: "row-2", Values: [3]float64{4, 5, 6}},
		}},
	}}
}

func TestStandardDataDigestIsStableAcrossRepeatedComputation(t *testing.T) {
	// A map with several keys is hashed here on purpose: if the encoder ever
	// leaked Go's randomized map iteration order into the digest, repeating the
	// computation would eventually disagree with itself.
	first, err := sampleData().Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if first.IsZero() {
		t.Fatal("expected a non-empty digest")
	}

	for range 64 {
		again, err := sampleData().Digest("demo", EvidenceTierNormative)
		if err != nil {
			t.Fatalf("digest: %v", err)
		}

		if again.Digest != first.Digest {
			t.Fatalf("digest is not deterministic: %s != %s", again.Digest, first.Digest)
		}
	}
}

func TestStandardDataDigestIgnoresTableOrder(t *testing.T) {
	forward, err := sampleData().Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	reversed := sampleData()
	reversed.Tables[0], reversed.Tables[1] = reversed.Tables[1], reversed.Tables[0]

	backward, err := reversed.Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if forward.Digest != backward.Digest {
		t.Fatalf("digest depends on table order: %s != %s", forward.Digest, backward.Digest)
	}

	if len(forward.Tables) != 2 || forward.Tables[0].Name != "a/first" || forward.Tables[1].Name != "b/second" {
		t.Fatalf("tables are not sorted by name: %#v", forward.Tables)
	}
}

func TestStandardDataDigestCoversTierAndStandardID(t *testing.T) {
	normative, err := sampleData().Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	// Re-tiering a module changes what its numbers mean, so it must change the
	// digest even though not one coefficient moved.
	scaffold, err := sampleData().Digest("demo", EvidenceTierScaffold)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if normative.Digest == scaffold.Digest {
		t.Fatal("evidence tier does not participate in the digest")
	}

	other, err := sampleData().Digest("other", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if normative.Digest == other.Digest {
		t.Fatal("standard id does not participate in the digest")
	}

	// The per-table digests are scoped to the table, not to the module, so two
	// modules sharing a table can be compared table by table.
	if normative.Tables[0].Digest != scaffold.Tables[0].Digest {
		t.Fatal("per-table digests must not depend on the module tier")
	}
}

func TestStandardDataDigestSeesUnexportedAndPointerFields(t *testing.T) {
	base, err := sampleData().Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	changedHidden := sampleData()
	rows, _ := changedHidden.Tables[1].Value.([]coefficientRow)
	rows[0].hidden = 99
	changedHidden.Tables[1].Value = rows

	hiddenDigest, err := changedHidden.Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if hiddenDigest.Digest == base.Digest {
		t.Fatal("an unexported coefficient field was not hashed")
	}

	changedPointer := sampleData()
	rows, _ = changedPointer.Tables[1].Value.([]coefficientRow)
	replacement := 42.0
	rows[0].Pointer = &replacement
	changedPointer.Tables[1].Value = rows

	pointerDigest, err := changedPointer.Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if pointerDigest.Digest == base.Digest {
		t.Fatal("a pointed-to coefficient value was not hashed")
	}
}

func TestStandardDataDigestNormalizesNegativeZero(t *testing.T) {
	positive, err := StandardData{Tables: []StandardDataTable{{Name: "t", Value: 0.0}}}.
		Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	negative, err := StandardData{Tables: []StandardDataTable{{Name: "t", Value: math.Copysign(0, -1)}}}.
		Digest("demo", EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if positive.Digest != negative.Digest {
		t.Fatal("the sign of zero must not move the digest")
	}
}

func TestStandardDataDigestRejectsBadTables(t *testing.T) {
	cases := []struct {
		name string
		data StandardData
		id   string
		want string
	}{
		{
			name: "empty table name",
			data: StandardData{Tables: []StandardDataTable{{Name: "  ", Value: 1}}},
			id:   "demo",
			want: "table name is required",
		},
		{
			name: "duplicate table name",
			data: StandardData{Tables: []StandardDataTable{{Name: "t", Value: 1}, {Name: "t", Value: 2}}},
			id:   "demo",
			want: "is duplicated",
		},
		{
			name: "unhashable value",
			data: StandardData{Tables: []StandardDataTable{{Name: "t", Value: func() {}}}},
			id:   "demo",
			want: "cannot contain func values",
		},
		{
			name: "missing standard id",
			data: sampleData(),
			id:   " ",
			want: "standard id is required",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.data.Digest(testCase.id, EvidenceTierNormative)
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), testCase.want)
			}
		})
	}
}

func TestStandardDataDigestIsAbsentForModulesWithoutData(t *testing.T) {
	empty := StandardData{}
	if !empty.IsEmpty() {
		t.Fatal("expected the zero value to be empty")
	}

	digest, err := empty.Digest("dummy-freefield", EvidenceTierTestFixture)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if !digest.IsZero() {
		t.Fatalf("expected no digest for a module carrying no data, got %#v", digest)
	}
}
