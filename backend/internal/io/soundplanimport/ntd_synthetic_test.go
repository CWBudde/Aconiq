package soundplanimport

import (
	"path/filepath"
	"strings"
	"testing"
)

// Fixture-free tests for the IOTable*.ntd immission-table reader.
//
// An .ntd file is read as a stream of null-terminated printable runs, so a
// fixture is exactly that: the tokens the parser reacts to, each followed by a
// NUL. Nothing here is copied from the licensed project; the token vocabulary
// ("StandardtextT", the ignored labels, the ".res"/"field)" pair) is the one
// ntd.go itself declares.

// ntdImage joins tokens into an .ntd image. Runs shorter than three printable
// bytes are dropped by the extractor, so a token below that length is padded
// by the caller rather than silently lost here.
func ntdImage(tokens ...string) []byte {
	out := make([]byte, 0, 64)
	for _, token := range tokens {
		out = append(out, token...)
		out = append(out, 0x00)
	}

	return out
}

// TestParseImmissionTableData_Synthetic pins the column extraction: the title
// that precedes a binding, the result-file/field pair, the formula column, and
// the labels the parser must step over.
func TestParseImmissionTableData_Synthetic(t *testing.T) {
	t.Parallel()

	image := ntdImage(
		"Arial",
		"RSPS0011.res", // before StandardtextT: must not become a column
		"RecNo)",
		"StandardtextT",
		"No.",
		"RSPS0011.res",
		"RecNo)",
		"Beschreibung",
		"Tag",
		"Calibri",
		"RSPS0011.res",
		"Formel",
		"ZB1)",
		"Geschoss",
		`if X6=1 then "EG" else Text(X6-1)+".OG";)`,
		"RSPS0021.res",
		"ZB1)",
	)

	table, err := parseImmissionTableData("IOTable1.ntd", image)
	if err != nil {
		t.Fatalf("parseImmissionTableData: %v", err)
	}

	if table.SourceFile != "IOTable1.ntd" {
		t.Errorf("SourceFile = %q", table.SourceFile)
	}

	want := []ImmissionTableColumn{
		{Title: "No.", ResultFile: "RSPS0011.res", ResultField: "RecNo"},
		{Title: "Tag", ResultFile: "RSPS0011.res", ResultField: "ZB1"},
		{Title: "Geschoss", Formula: `if X6=1 then "EG" else Text(X6-1)+".OG";)`},
		{Title: "ZB1", ResultFile: "RSPS0021.res", ResultField: "ZB1"},
	}

	if len(table.Columns) != len(want) {
		t.Fatalf("columns = %+v, want %+v", table.Columns, want)
	}

	for i, expected := range want {
		if table.Columns[i] != expected {
			t.Errorf("columns[%d] = %+v, want %+v", i, table.Columns[i], expected)
		}
	}
}

// TestParseImmissionTableData_Refusals pins the inputs that yield no table at
// all. An .ntd the parser cannot read must say so, because LoadImmissionTables
// turns that into a per-file warning rather than dropping the file in silence.
func TestParseImmissionTableData_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		image []byte
	}{
		{
			name:  "empty file",
			image: nil,
		},
		{
			name:  "no StandardtextT marker, so nothing is active",
			image: ntdImage("No.", "RSPS0011.res", "RecNo)"),
		},
		{
			name:  "a result file with no field token after it",
			image: ntdImage("StandardtextT", "No.", "RSPS0011.res"),
		},
		{
			name:  "a result file followed by something that is not a field",
			image: ntdImage("StandardtextT", "No.", "RSPS0011.res", "RSPS0012.res"),
		},
		{
			name:  "only ignored labels",
			image: ntdImage("StandardtextT", "Formel", "Beschreibung", "Calibri"),
		},
		{
			name:  "runs shorter than the extractor's minimum are not tokens",
			image: ntdImage("StandardtextT", "ab", "x)"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseImmissionTableData("IOTable1.ntd", tc.image)
			if err == nil {
				t.Fatal("got nil error, want a refusal")
			}

			if !strings.Contains(err.Error(), "no immission-table columns found") {
				t.Errorf("error = %q, want it to say no columns were found", err)
			}
		})
	}
}

// TestParseImmissionTableData_TitleHandling pins where a column's title comes
// from, including the case that is easy to read as a bug and is the shipped
// behaviour: a field token is not consumed when its binding is built, so the
// scan reaches it again and files it as the *next* column's pending title.
//
// A file whose columns each carry their own title never shows it. A file whose
// second binding has no title of its own inherits the first binding's field
// name instead of falling back to its own — pinned here so that changing it is
// a deliberate edit with a visible diff.
func TestParseImmissionTableData_TitleHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tokens     []string
		wantTitles []string
	}{
		{
			name:       "the first binding with no title takes its own field name",
			tokens:     []string{"StandardtextT", "RSPS0011.res", "ZB1)"},
			wantTitles: []string{"ZB1"},
		},
		{
			name: "each binding uses the title written before it",
			tokens: []string{
				"StandardtextT",
				"Pegel Tag", "RSPS0011.res", "ZB1)",
				"Pegel Nacht", "RSPS0011.res", "ZB2)",
			},
			wantTitles: []string{"Pegel Tag", "Pegel Nacht"},
		},
		{
			name: "an untitled binding inherits the previous binding's field token",
			tokens: []string{
				"StandardtextT",
				"Pegel Tag", "RSPS0011.res", "ZB1)",
				"RSPS0011.res", "ZB2)",
			},
			wantTitles: []string{"Pegel Tag", "ZB1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			table, err := parseImmissionTableData("IOTable1.ntd", ntdImage(tc.tokens...))
			if err != nil {
				t.Fatalf("parseImmissionTableData: %v", err)
			}

			if len(table.Columns) != len(tc.wantTitles) {
				t.Fatalf("columns = %+v, want %d", table.Columns, len(tc.wantTitles))
			}

			for i, want := range tc.wantTitles {
				if table.Columns[i].Title != want {
					t.Errorf("columns[%d].Title = %q, want %q", i, table.Columns[i].Title, want)
				}
			}
		})
	}
}

// TestLoadImmissionTables_Synthetic pins the directory sweep: only IOTable*.ntd
// is read, results keep the glob's sorted order, and a file that will not parse
// becomes a warning naming it rather than a failure of the whole project load.
func TestLoadImmissionTables_Synthetic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	good := ntdImage("StandardtextT", "No.", "RSPS0011.res", "RecNo)")

	writeFixture(t, dir, "IOTable2.ntd", good)
	writeFixture(t, dir, "IOTable1.ntd", good)
	writeFixture(t, dir, "IOTable3.ntd", []byte("nothing the parser recognises"))
	writeFixture(t, dir, "Other.ntd", good)

	tables, warnings := LoadImmissionTables(dir)

	if len(tables) != 2 {
		t.Fatalf("tables = %+v, want the two readable IOTable files", tables)
	}

	if tables[0].SourceFile != "IOTable1.ntd" || tables[1].SourceFile != "IOTable2.ntd" {
		t.Errorf("table order = %q, %q; want sorted by name", tables[0].SourceFile, tables[1].SourceFile)
	}

	if len(warnings) != 1 || !strings.Contains(warnings[0], "IOTable3.ntd") {
		t.Errorf("warnings = %v, want one naming the unreadable table", warnings)
	}
}

// TestLoadImmissionTables_NoTables pins that a project without immission
// tables is neither an error nor a warning.
func TestLoadImmissionTables_NoTables(t *testing.T) {
	t.Parallel()

	tables, warnings := LoadImmissionTables(t.TempDir())
	if len(tables) != 0 || len(warnings) != 0 {
		t.Errorf("got %d tables and %v warnings, want none of either", len(tables), warnings)
	}
}

// TestParseImmissionTableFile_MissingFile pins the file-level error path.
func TestParseImmissionTableFile_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseImmissionTableFile(filepath.Join(t.TempDir(), "IOTable1.ntd"))
	if err == nil {
		t.Fatal("got nil error for a missing .ntd file")
	}

	if !strings.Contains(err.Error(), "soundplan: read ntd") {
		t.Errorf("error = %q, want it to name the read step", err)
	}
}
