package soundplanimport

import (
	"path/filepath"
	"testing"
)

// interopDir is the path to the SoundPlan sample project.
const interopDir = "../../../../interoperability/Schienenprojekt - Schall 03"

func TestParseTrainTypes(t *testing.T) {
	path := filepath.Join(interopDir, "TS03.abs")

	types, err := ParseTrainTypes(path)
	if err != nil {
		t.Fatalf("ParseTrainTypes: %v", err)
	}

	if len(types) < 15 {
		t.Fatalf("got %d train types, want at least 15", len(types))
	}

	// Verify first train type has expected fields.
	tt := types[0]
	if tt.ZugArt != 1 {
		t.Errorf("first ZugArt = %d, want 1", tt.ZugArt)
	}

	if tt.Name == "" {
		t.Error("first train type Name is empty")
	}

	t.Logf("TS03: %d train types", len(types))

	for i, tt := range types {
		t.Logf("  [%2d] ID=%d Name=%-30q SBA=%.1f Vmax=%.0f LZug=%.0f DFz=%.1f DAo=%.1f",
			i, tt.ZugArt, tt.Name, tt.SBA, tt.Vmax, tt.LZug, tt.DFz, tt.DAo)
	}
}

func TestParseGroupResults(t *testing.T) {
	path := filepath.Join(interopDir, "RSPS0011", "RGRP0011.abs")

	groups, err := ParseGroupResults(path)
	if err != nil {
		t.Fatalf("ParseGroupResults: %v", err)
	}

	if len(groups) == 0 {
		t.Fatal("expected at least 1 group result")
	}

	// RGRP should contain noise levels for receivers.
	for i, g := range groups {
		if i >= 10 {
			break
		}

		t.Logf("  [%2d] GrpNo=%d RecNo=%d Floor=%d GName=%-30q ZB1=%.1f ZB2=%.1f",
			i, g.GrpNo, g.RecNo, g.Floor, g.GName, g.ZB1, g.ZB2)
	}

	// Verify levels are in a reasonable dB range.
	for _, g := range groups {
		if g.ZB1 < 20 || g.ZB1 > 120 {
			t.Errorf("unreasonable ZB1=%.1f for RecNo=%d Floor=%d", g.ZB1, g.RecNo, g.Floor)
		}

		if g.ZB2 < 20 || g.ZB2 > 120 {
			t.Errorf("unreasonable ZB2=%.1f for RecNo=%d Floor=%d", g.ZB2, g.RecNo, g.Floor)
		}
	}

	t.Logf("RGRP: %d group results", len(groups))
}

func TestParseReceiverResults(t *testing.T) {
	path := filepath.Join(interopDir, "RSPS0011", "RREC0011.abs")

	recs, err := ParseReceiverResults(path)
	if err != nil {
		t.Fatalf("ParseReceiverResults: %v", err)
	}

	if len(recs) == 0 {
		t.Fatal("expected at least 1 receiver result")
	}

	// Verify receiver names are populated.
	namedCount := 0

	for _, r := range recs {
		if r.Name != "" {
			namedCount++
		}
	}

	if namedCount == 0 {
		t.Error("no receivers have names")
	}

	t.Logf("RREC: %d receivers (%d with names)", len(recs), namedCount)

	for i, r := range recs {
		if i >= 10 {
			break
		}

		t.Logf("  [%2d] RecNo=%d Floor=%d ObjID=%d Name=%q Usage=%d HasCoords=%v",
			i, r.RecNo, r.Floor, r.ObjID, r.Name, r.Usage, r.HasCoords)
	}
}

func TestParsePartialResults(t *testing.T) {
	path := filepath.Join(interopDir, "RSPS0011", "RMPA0011.abs")

	parts, err := ParsePartialResults(path)
	if err != nil {
		t.Fatalf("ParsePartialResults: %v", err)
	}

	if len(parts) == 0 {
		t.Fatal("expected at least 1 partial result")
	}

	// Verify source names are present.
	namedCount := 0

	for _, p := range parts {
		if p.QName != "" {
			namedCount++
		}
	}

	t.Logf("RMPA: %d partial results (%d with source names)", len(parts), namedCount)

	for i, p := range parts {
		if i >= 5 {
			break
		}

		t.Logf("  [%2d] IDX=%d SrcNo=%d RecNo=%d Floor=%d ZBName=%q QName=%q Lr=%.1f",
			i, p.IDX, p.SrcNo, p.RecNo, p.Floor, p.ZBName, p.QName, p.Lr)
	}
}

func TestLoadRunResults(t *testing.T) {
	resultDir := filepath.Join(interopDir, "RSPS0011")

	results, err := LoadRunResults(resultDir)
	if err != nil {
		t.Fatalf("LoadRunResults: %v", err)
	}

	if len(results.Receivers) == 0 {
		t.Error("no receivers loaded")
	}

	if len(results.Groups) == 0 {
		t.Error("no group results loaded")
	}

	if len(results.Partials) == 0 {
		t.Error("no partial results loaded")
	}

	t.Logf("Run results: %d receivers, %d groups, %d partials",
		len(results.Receivers), len(results.Groups), len(results.Partials))
}

func TestExtractRunSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"RSPS0011", "0011"},
		{"RRLK0022", "0022"},
		{"RSPS0021", "0021"},
		{"abc123", "123"},
		{"0011", "0011"},
	}

	for _, tt := range tests {
		got := extractRunSuffix(tt.input)
		if got != tt.want {
			t.Errorf("extractRunSuffix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseGridMapRailEmissions(t *testing.T) {
	// RRLK directories contain grid-map calculation results.
	// They use the same RRAI/RRAD format but for grid calculations.
	path := filepath.Join(interopDir, "RRLK0022", "RRAI0022.abs")

	emissions, err := ParseRailEmissions(path)
	if err != nil {
		t.Fatalf("ParseRailEmissions: %v", err)
	}

	t.Logf("RRAI (grid map): %d rail emissions", len(emissions))

	for i, e := range emissions {
		if i >= 5 {
			break
		}

		t.Logf("  [%2d] IDX=%d ObjID=%d Railname=%q LmEDay=%.1f LmENight=%.1f",
			i, e.IDX, e.ObjID, e.Railname, e.LmEDay, e.LmENight)
	}
}
