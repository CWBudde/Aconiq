package soundplanimport

import (
	"path/filepath"
	"testing"
)

func TestParseImmissionTableFile(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	table, err := ParseImmissionTableFile(filepath.Join(dir, "IOTable1.ntd"))
	if err != nil {
		t.Fatalf("ParseImmissionTableFile: %v", err)
	}

	if table.SourceFile != "IOTable1.ntd" {
		t.Fatalf("source file = %q, want IOTable1.ntd", table.SourceFile)
	}

	if len(table.Columns) < 20 {
		t.Fatalf("got %d columns, want at least 20", len(table.Columns))
	}

	first := table.Columns[0]
	if first.Title != "No." || first.ResultFile != "RSPS0011.res" || first.ResultField != "RecNo" {
		t.Fatalf("first column = %+v, want No./RSPS0011.res/RecNo", first)
	}

	foundFormula := false
	foundSecondRun := false
	for _, col := range table.Columns {
		if col.Formula == `if X6=1 then "EG" else Text(X6-1)+".OG";)` {
			foundFormula = true
		}

		if col.ResultFile == "RSPS0021.res" && col.ResultField == "ZB1" {
			foundSecondRun = true
		}
	}

	if !foundFormula {
		t.Fatal("expected floor formula column")
	}

	if !foundSecondRun {
		t.Fatal("expected RSPS0021.res ZB1 column")
	}
}

func TestLoadImmissionTables(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tables, warnings := LoadImmissionTables(dir)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}

	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
}
