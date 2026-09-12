package schall03

import (
	"encoding/json"
	"strings"
	"testing"
)

// The wire vocabulary must resolve to the reference row of each table when the
// property is absent. PLAN.md 1.2 records what happens when it does not: an
// omitted "fahrbahn" silently collected +7 dB Schiene and +1 dB Reflexion.
func TestEmptyVocabularyNamesResolveToTheReferenceRow(t *testing.T) {
	t.Parallel()

	fahrbahn, err := ParseFahrbahnart("")
	if err != nil || fahrbahn != FahrbahnartSchwellengleis {
		t.Fatalf("ParseFahrbahnart(\"\") = %v, %v; want Schwellengleis", fahrbahn, err)
	}

	sFahrbahn, err := ParseSFahrbahnart("")
	if err != nil || sFahrbahn != SFahrbahnSchwellengleis {
		t.Fatalf("ParseSFahrbahnart(\"\") = %v, %v; want Schwellengleis", sFahrbahn, err)
	}

	surface, err := ParseSurfaceCond("")
	if err != nil || surface != SurfaceCondNone {
		t.Fatalf("ParseSurfaceCond(\"\") = %v, %v; want none", surface, err)
	}

	wall, err := ParseWallSurface("")
	if err != nil || wall != WallSurfaceHard {
		t.Fatalf("ParseWallSurface(\"\") = %v, %v; want hard", wall, err)
	}
}

func TestVocabularyNamesRoundTripAndRejectUnknownValues(t *testing.T) {
	t.Parallel()

	for i, name := range FahrbahnartNames() {
		got, err := ParseFahrbahnart(name)
		if err != nil || int(got) != i {
			t.Fatalf("ParseFahrbahnart(%q) = %v, %v; want %d", name, got, err, i)
		}
	}

	for i, name := range SFahrbahnartNames() {
		got, err := ParseSFahrbahnart(name)
		if err != nil || int(got) != i {
			t.Fatalf("ParseSFahrbahnart(%q) = %v, %v; want %d", name, got, err, i)
		}
	}

	for i, name := range SurfaceCondNames() {
		got, err := ParseSurfaceCond(name)
		if err != nil || int(got) != i {
			t.Fatalf("ParseSurfaceCond(%q) = %v, %v; want %d", name, got, err, i)
		}
	}

	for i, name := range WallSurfaceNames() {
		got, err := ParseWallSurface(name)
		if err != nil || int(got) != i {
			t.Fatalf("ParseWallSurface(%q) = %v, %v; want %d", name, got, err, i)
		}
	}

	if _, err := ParseFahrbahnart("feste fahrbahn"); err == nil {
		t.Fatal("expected an unknown Fahrbahnart to be rejected rather than defaulted")
	}
}

// Every name ZugartNames advertises must actually build an operation, or the
// error message it appears in would be lying.
func TestZugartNamesAllResolve(t *testing.T) {
	t.Parallel()

	names := ZugartNames()
	if len(names) != len(Zugarten)+len(ZugartStrassenbahn) {
		t.Fatalf("ZugartNames returned %d entries, want %d", len(names), len(Zugarten)+len(ZugartStrassenbahn))
	}

	for _, name := range names {
		op, err := NewTrainOperationFromZugart(name, 1, 1)
		if err != nil {
			t.Fatalf("NewTrainOperationFromZugart(%q): %v", name, err)
		}

		if err := op.Validate(); err != nil {
			t.Fatalf("Zugart %q builds an invalid operation: %v", name, err)
		}
	}
}

// The four enumerations are ordinals of Anlage 2 tables, so the wire format
// carries names.  A file written against the old numbering must fail loudly:
// PLAN.md 1.2 records that it otherwise decoded to a different track type and
// shifted levels by up to +8 dB with nothing detecting it.
func TestVocabularyJSONRoundTrips(t *testing.T) {
	t.Parallel()

	type wire struct {
		Fahrbahn  FahrbahnartType  `json:"fahrbahn"`
		SFahrbahn SFahrbahnartType `json:"s_fahrbahn"`
		Surface   SurfaceCondType  `json:"surface"`
		Wall      WallSurfaceType  `json:"wall"`
	}

	want := wire{
		Fahrbahn:  FahrbahnartBahnuebergang,
		SFahrbahn: SFahrbahnGruenHoch,
		Surface:   SurfaceCondSchienenstegabschirm,
		Wall:      WallSurfaceHighlyAbsorbing,
	}

	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const encoded = `{"fahrbahn":"bahnuebergang","s_fahrbahn":"begruent-hoch",` +
		`"surface":"schienenstegabschirmung","wall":"highly-absorbing"}`

	if string(payload) != encoded {
		t.Fatalf("marshal = %s, want %s", payload, encoded)
	}

	var got wire

	err = json.Unmarshal(payload, &got)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

// Every accepted name must survive a round trip through JSON, so that Marshal
// and Unmarshal stay exact inverses as the tables grow.
func TestVocabularyJSONCoversEveryName(t *testing.T) {
	t.Parallel()

	for _, name := range FahrbahnartNames() {
		var decoded FahrbahnartType

		assertNameRoundTrips(t, name, &decoded, func() any { return decoded })
	}

	for _, name := range SFahrbahnartNames() {
		var decoded SFahrbahnartType

		assertNameRoundTrips(t, name, &decoded, func() any { return decoded })
	}

	for _, name := range SurfaceCondNames() {
		var decoded SurfaceCondType

		assertNameRoundTrips(t, name, &decoded, func() any { return decoded })
	}

	for _, name := range WallSurfaceNames() {
		var decoded WallSurfaceType

		assertNameRoundTrips(t, name, &decoded, func() any { return decoded })
	}
}

// assertNameRoundTrips decodes a quoted name into target and checks that
// re-encoding the decoded value yields the same name.
func assertNameRoundTrips(t *testing.T, name string, target any, encode func() any) {
	t.Helper()

	quoted, err := json.Marshal(name)
	if err != nil {
		t.Fatalf("marshal name %q: %v", name, err)
	}

	err = json.Unmarshal(quoted, target)
	if err != nil {
		t.Fatalf("unmarshal %s: %v", quoted, err)
	}

	payload, err := json.Marshal(encode())
	if err != nil {
		t.Fatalf("re-marshal %q: %v", name, err)
	}

	if string(payload) != string(quoted) {
		t.Fatalf("round trip of %q produced %s", name, payload)
	}
}

// A bare ordinal is the failure PLAN.md 1.2 describes, so it must be rejected
// with an error that names the accepted vocabulary.
func TestVocabularyJSONRejectsBareOrdinals(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		label  string
		target json.Unmarshaler
		want   string
	}{
		{"Fahrbahnart", new(FahrbahnartType), "feste-fahrbahn"},
		{"SFahrbahnart", new(SFahrbahnartType), "strassenbuendig"},
		{"SurfaceCond", new(SurfaceCondType), "bug"},
		{"WallSurface", new(WallSurfaceType), "absorbing"},
	} {
		err := tc.target.UnmarshalJSON([]byte("1"))
		if err == nil {
			t.Fatalf("%s accepted the bare ordinal 1", tc.label)
		}

		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s error %q does not name the vocabulary entry %q", tc.label, err, tc.want)
		}

		if !strings.Contains(err.Error(), "not a wire format") {
			t.Errorf("%s error %q does not explain why ordinals are refused", tc.label, err)
		}
	}
}

// An unknown name must be refused rather than defaulted to the reference row.
func TestVocabularyJSONRejectsUnknownNames(t *testing.T) {
	t.Parallel()

	for _, target := range []json.Unmarshaler{
		new(FahrbahnartType), new(SFahrbahnartType), new(SurfaceCondType), new(WallSurfaceType),
	} {
		err := target.UnmarshalJSON([]byte(`"no-such-row"`))
		if err == nil {
			t.Fatalf("%T accepted an unknown name", target)
		}
	}
}

// An empty string is the documented spelling of "absent", and JSON null is a
// no-op by encoding/json convention; both leave the reference row in place.
func TestVocabularyJSONEmptyStringAndNullKeepTheReferenceRow(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{`""`, "null"} {
		fahrbahn := FahrbahnartBahnuebergang

		err := fahrbahn.UnmarshalJSON([]byte(payload))
		if err != nil {
			t.Fatalf("Fahrbahnart.UnmarshalJSON(%s): %v", payload, err)
		}

		want := FahrbahnartSchwellengleis
		if payload == "null" {
			want = FahrbahnartBahnuebergang
		}

		if fahrbahn != want {
			t.Errorf("Fahrbahnart after %s = %v, want %v", payload, fahrbahn, want)
		}

		wall := WallSurfaceHighlyAbsorbing

		err = wall.UnmarshalJSON([]byte(payload))
		if err != nil {
			t.Fatalf("WallSurface.UnmarshalJSON(%s): %v", payload, err)
		}

		wantWall := WallSurfaceHard
		if payload == "null" {
			wantWall = WallSurfaceHighlyAbsorbing
		}

		if wall != wantWall {
			t.Errorf("WallSurface after %s = %v, want %v", payload, wall, wantWall)
		}
	}
}

// An ordinal that no table row carries must not be encoded as if it were a
// legitimate name.
func TestVocabularyJSONRejectsOutOfRangeOrdinals(t *testing.T) {
	t.Parallel()

	_, err := json.Marshal(FahrbahnartType(len(FahrbahnartNames())))
	if err == nil {
		t.Fatal("an out-of-range Fahrbahnart ordinal was encoded")
	}
}
