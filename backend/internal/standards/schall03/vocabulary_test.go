package schall03

import "testing"

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
