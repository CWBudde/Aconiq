package soundplanimport

import (
	"path/filepath"
	"strings"
	"testing"

	absdb "github.com/cwbudde/go-absolute-database"
)

// Fixture-free tests for the *.abs result-table readers.
//
// The fixtures are real Absolute Database files, written here by the same
// library the parsers read them with, rather than copies of the licensed
// project's tables. That library's CreateTable only has corpus evidence for
// Int32 and Varchar columns, so a synthetic table can carry the identity,
// name and enumeration columns but not the floating-point level columns. What
// these tests therefore pin is the mapping layer: which schema column feeds
// which struct field, what an absent column reads as, and what a NULL reads
// as. The numeric decode itself belongs to the library and has its own tests
// there.

func absIntColumn(name string) absdb.Column {
	return absdb.Column{Name: name, BaseType: absdb.BftInt32, FieldType: absdb.FieldInteger}
}

func absTextColumn(name string) absdb.Column {
	return absdb.Column{Name: name, BaseType: absdb.BftVarchar, FieldType: absdb.FieldString, Size: 64}
}

// writeAbsTable creates an .abs file holding one table with the given columns
// and rows, and returns its path. A nil value in a row is stored as NULL.
func writeAbsTable(t *testing.T, dir string, fileName string, tableName string, columns []absdb.Column, rows [][]any) string {
	t.Helper()

	path := filepath.Join(dir, fileName)

	db, err := absdb.CreateDatabase(path, absdb.CreateDatabaseOptions{})
	if err != nil {
		t.Fatalf("create %s: %v", fileName, err)
	}

	defer func() { _ = db.Close() }()

	err = db.CreateTable(tableName, columns)
	if err != nil {
		t.Fatalf("create table %s: %v", tableName, err)
	}

	if len(rows) == 0 {
		return path
	}

	writer, err := db.OpenTableWriter()
	if err != nil {
		t.Fatalf("open writer for %s: %v", tableName, err)
	}

	for i, row := range rows {
		_, err = writer.Insert(row)
		if err != nil {
			t.Fatalf("insert row %d into %s: %v", i, tableName, err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("commit %s: %v", tableName, err)
	}

	return path
}

// TestParseReceiverResults_Synthetic pins the RREC column mapping, and the two
// things a caller has to be able to trust about a receiver row: HasCoords
// tracks whether the table carried a coordinate at all, and a column the table
// does not have reads as zero rather than as a decode failure.
//
// It also pins an inconsistency that is live in the code: Name is trimmed and
// PointID is not.
func TestParseReceiverResults_Synthetic(t *testing.T) {
	t.Parallel()

	columns := []absdb.Column{
		absIntColumn("RecNo"),
		absIntColumn("Floor"),
		absIntColumn("ObjID"),
		absTextColumn("PointID"),
		absTextColumn("Name"),
		absIntColumn("Usage"),
		absIntColumn("X/m"),
	}

	rows := [][]any{
		{int32(1), int32(1), int32(101), "  IO-1  ", "  Hauptstraße 4  ", int32(3), int32(7)},
		{int32(2), int32(2), int32(101), nil, nil, nil, nil},
	}

	path := writeAbsTable(t, t.TempDir(), "RREC0011.abs", "RREC", columns, rows)

	results, err := ParseReceiverResults(path)
	if err != nil {
		t.Fatalf("ParseReceiverResults: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d rows, want 2", len(results))
	}

	first := results[0]
	if first.RecNo != 1 || first.Floor != 1 || first.ObjID != 101 || first.Usage != 3 {
		t.Errorf("first row identity = %+v", first)
	}

	if first.Name != "Hauptstraße 4" {
		t.Errorf("Name = %q, want it trimmed", first.Name)
	}

	if first.PointID != "  IO-1  " {
		t.Errorf("PointID = %q, want the untrimmed value: readStr is not used for this column", first.PointID)
	}

	if !first.HasCoords {
		t.Error("HasCoords = false for a row whose X/m column holds a value")
	}

	second := results[1]
	if second.RecNo != 2 || second.Name != "" || second.PointID != "" || second.Usage != 0 {
		t.Errorf("second row = %+v, want the NULL columns to read as zero values", second)
	}

	if second.HasCoords {
		t.Error("HasCoords = true for a row whose X/m column is NULL")
	}

	// None of the level columns exists in this table.
	if first.Limit1 != 0 || first.ZB1 != 0 || first.ZB2 != 0 || first.GH != 0 {
		t.Errorf("absent columns read as %+v, want zeroes", first)
	}
}

// TestParseGroupResults_Synthetic pins the RGRP column mapping.
func TestParseGroupResults_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeAbsTable(t, t.TempDir(), "RGRP0011.abs", "RGRP",
		[]absdb.Column{
			absIntColumn("GrpNo"),
			absIntColumn("RecNo"),
			absIntColumn("Floor"),
			absTextColumn("GName"),
		},
		[][]any{
			{int32(1), int32(1), int32(1), "Default railway noise "},
			{int32(2), int32(1), int32(2), nil},
		})

	groups, err := ParseGroupResults(path)
	if err != nil {
		t.Fatalf("ParseGroupResults: %v", err)
	}

	want := []GroupResult{
		{GrpNo: 1, RecNo: 1, Floor: 1, GName: "Default railway noise"},
		{GrpNo: 2, RecNo: 1, Floor: 2},
	}

	if len(groups) != len(want) {
		t.Fatalf("got %d groups, want %d", len(groups), len(want))
	}

	for i, expected := range want {
		if groups[i] != expected {
			t.Errorf("groups[%d] = %+v, want %+v", i, groups[i], expected)
		}
	}
}

// TestParsePartialResults_Synthetic pins the RMPA column mapping, whose 24
// columns are the easiest place in this package for a field to be wired to the
// wrong name.
func TestParsePartialResults_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeAbsTable(t, t.TempDir(), "RMPA0011.abs", "RMPA",
		[]absdb.Column{
			absIntColumn("IDX"),
			absIntColumn("SrcNo"),
			absIntColumn("RecNo"),
			absIntColumn("Floor"),
			absTextColumn("ZBName"),
			absTextColumn("QName"),
			absIntColumn("SrcObjID"),
		},
		[][]any{{int32(4), int32(5), int32(6), int32(7), "Tag", "Gleis 1", int32(8)}})

	partials, err := ParsePartialResults(path)
	if err != nil {
		t.Fatalf("ParsePartialResults: %v", err)
	}

	if len(partials) != 1 {
		t.Fatalf("got %d partials, want 1", len(partials))
	}

	got := partials[0]
	if got.IDX != 4 || got.SrcNo != 5 || got.RecNo != 6 || got.Floor != 7 || got.SrcObjID != 8 {
		t.Errorf("identity columns = %+v", got)
	}

	if got.ZBName != "Tag" || got.QName != "Gleis 1" {
		t.Errorf("names = %q/%q, want Tag/Gleis 1", got.ZBName, got.QName)
	}
}

// TestParseTrainTypes_Synthetic pins the TS03 mapping, including that a table
// without the Kommentar memo column yields an empty comment rather than an
// error.
func TestParseTrainTypes_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeAbsTable(t, t.TempDir(), "TS03.abs", "TS03",
		[]absdb.Column{absIntColumn("ZugArt"), absTextColumn("Name")},
		[][]any{
			{int32(6), " Güterzug "},
			{int32(8), "ICE"},
		})

	types, err := ParseTrainTypes(path)
	if err != nil {
		t.Fatalf("ParseTrainTypes: %v", err)
	}

	want := []TrainType{{ZugArt: 6, Name: "Güterzug"}, {ZugArt: 8, Name: "ICE"}}
	if len(types) != len(want) {
		t.Fatalf("got %d train types, want %d", len(types), len(want))
	}

	for i, expected := range want {
		if types[i] != expected {
			t.Errorf("types[%d] = %+v, want %+v", i, types[i], expected)
		}
	}
}

// TestParseRailEmissions_Synthetic and TestParseTrainEmissions_Synthetic pin
// the two tables the rail-operation derivation joins, including that the
// boolean Max column reads false when the table does not carry it.
func TestParseRailEmissions_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeAbsTable(t, t.TempDir(), "RRAI0011.abs", "RRAI",
		[]absdb.Column{absIntColumn("IDX"), absIntColumn("ObjID"), absTextColumn("Railname")},
		[][]any{{int32(1), int32(42), " Gleis 1 "}})

	emissions, err := ParseRailEmissions(path)
	if err != nil {
		t.Fatalf("ParseRailEmissions: %v", err)
	}

	if len(emissions) != 1 {
		t.Fatalf("got %d rows, want 1", len(emissions))
	}

	if emissions[0].IDX != 1 || emissions[0].ObjID != 42 || emissions[0].Railname != "Gleis 1" {
		t.Errorf("row = %+v", emissions[0])
	}
}

func TestParseTrainEmissions_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeAbsTable(t, t.TempDir(), "RRAD0011.abs", "RRAD",
		[]absdb.Column{absIntColumn("No"), absIntColumn("IDX"), absTextColumn("Trainname")},
		[][]any{{int32(1), int32(1), "ICE"}, {int32(2), int32(1), "Güterzug"}})

	emissions, err := ParseTrainEmissions(path)
	if err != nil {
		t.Fatalf("ParseTrainEmissions: %v", err)
	}

	if len(emissions) != 2 {
		t.Fatalf("got %d rows, want 2", len(emissions))
	}

	if emissions[0].Trainname != "ICE" || emissions[1].Trainname != "Güterzug" {
		t.Errorf("names = %q/%q", emissions[0].Trainname, emissions[1].Trainname)
	}

	if emissions[0].Max {
		t.Error("Max = true although the table carries no Max column")
	}
}

// TestParseAbsTables_Refusals pins that every .abs entry point refuses a file
// that is not an Absolute Database, and names which table it was reading. An
// import that silently returned no rows here would present a project with no
// receivers as a successful comparison.
func TestParseAbsTables_Refusals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notADatabase := writeFixture(t, dir, "garbage.abs", []byte(strings.Repeat("not a database ", 64)))

	tests := []struct {
		name     string
		call     func(path string) error
		wantText string
	}{
		{name: "RREC", call: func(p string) error { _, err := ParseReceiverResults(p); return err }, wantText: "open RREC"},
		{name: "RGRP", call: func(p string) error { _, err := ParseGroupResults(p); return err }, wantText: "open RGRP"},
		{name: "RMPA", call: func(p string) error { _, err := ParsePartialResults(p); return err }, wantText: "open RMPA"},
		{name: "TS03", call: func(p string) error { _, err := ParseTrainTypes(p); return err }, wantText: "open TS03"},
		{name: "RRAI", call: func(p string) error { _, err := ParseRailEmissions(p); return err }, wantText: "open RRAI"},
		{name: "RRAD", call: func(p string) error { _, err := ParseTrainEmissions(p); return err }, wantText: "open RRAD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.call(notADatabase)
			if err == nil {
				t.Fatal("got nil error for a file that is not an Absolute Database")
			}

			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantText)
			}

			err = tc.call(filepath.Join(dir, "absent.abs"))
			if err == nil {
				t.Fatal("got nil error for a missing file")
			}
		})
	}
}

// TestLoadRunResults_Synthetic pins the result-directory sweep: the run suffix
// comes from the directory name, and a table the run did not produce leaves
// its slice empty instead of failing the load.
func TestLoadRunResults_Synthetic(t *testing.T) {
	t.Parallel()

	runDir := makeDir(t, t.TempDir(), "RSPS0011")

	writeAbsTable(t, runDir, "RREC0011.abs", "RREC",
		[]absdb.Column{absIntColumn("RecNo")}, [][]any{{int32(1)}, {int32(2)}})
	writeAbsTable(t, runDir, "RGRP0011.abs", "RGRP",
		[]absdb.Column{absIntColumn("GrpNo")}, [][]any{{int32(9)}})

	results, err := LoadRunResults(runDir)
	if err != nil {
		t.Fatalf("LoadRunResults: %v", err)
	}

	if len(results.Receivers) != 2 || len(results.Groups) != 1 {
		t.Errorf("receivers/groups = %d/%d, want 2/1", len(results.Receivers), len(results.Groups))
	}

	if results.Partials != nil {
		t.Errorf("Partials = %+v, want nil: the run wrote no RMPA table", results.Partials)
	}
}

// TestLoadRunResults_UnreadableTableFails pins that a corrupt table in a
// result directory fails the load rather than producing a half-read run.
func TestLoadRunResults_UnreadableTableFails(t *testing.T) {
	t.Parallel()

	runDir := makeDir(t, t.TempDir(), "RSPS0011")

	writeFixture(t, runDir, "RREC0011.abs", []byte("not a database"))

	_, err := LoadRunResults(runDir)
	if err == nil {
		t.Fatal("got nil error for a corrupt RREC table")
	}
}

// TestLoadRunResults_NoTables pins that a result directory holding none of the
// three tables is an empty result rather than an error, which is what lets a
// grid-map run directory go through the same call.
func TestLoadRunResults_NoTables(t *testing.T) {
	t.Parallel()

	results, err := LoadRunResults(t.TempDir())
	if err != nil {
		t.Fatalf("LoadRunResults: %v", err)
	}

	if len(results.Receivers)+len(results.Groups)+len(results.Partials) != 0 {
		t.Errorf("results = %+v, want nothing", results)
	}
}
