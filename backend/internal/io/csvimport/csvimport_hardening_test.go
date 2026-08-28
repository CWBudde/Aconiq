package csvimport

import (
	"strings"
	"testing"
)

// A CSV attribute table is untrusted input. Every row allocates one map entry
// per named column, at roughly fifty bytes an entry against the two bytes of
// comma-separated blank that produce it, so the live heap a table occupies is
// columns × rows and grows far faster than the file that describes it. Each of
// the three bounds below is checked before the allocation it governs is made.

// The header width sizes every subsequent row's property map, before a single
// value has been read.
func TestReadTable_RejectsTooManyColumns(t *testing.T) {
	header := "feature_id" + strings.Repeat(",c", maxColumns)

	_, err := ReadTable(strings.NewReader(header + "\n"))
	if err == nil {
		t.Fatal("expected an error for a header wider than the column limit")
	}

	if !strings.Contains(err.Error(), "columns") {
		t.Fatalf("error %q does not report the column count", err)
	}
}

// A header exactly at the limit must still be accepted.
func TestReadTable_AcceptsHeaderAtColumnLimit(t *testing.T) {
	var b strings.Builder

	b.WriteString("feature_id")

	for i := range maxColumns - 1 {
		b.WriteString(",c")
		b.WriteString(strings.Repeat("x", 1+i%3))
	}

	_, err := ReadTable(strings.NewReader(b.String() + "\n"))
	if err != nil {
		t.Fatalf("header of exactly %d columns rejected: %v", maxColumns, err)
	}
}

func TestReadTable_RejectsTooManyRows(t *testing.T) {
	input := "feature_id,name\nf1,a\nf2,b\nf3,c\n"

	_, err := readTable(strings.NewReader(input), limits{columns: maxColumns, records: 2, properties: maxProperties})
	if err == nil {
		t.Fatal("expected an error for a table longer than the row limit")
	}

	if !strings.Contains(err.Error(), "rows") {
		t.Fatalf("error %q does not report the row count", err)
	}
}

// Neither the column limit nor the row limit bounds the product, which is what
// the memory cost actually follows: 1024 columns over 2^22 rows satisfies both
// and is four billion map entries.
func TestReadTable_RejectsTooManyProperties(t *testing.T) {
	input := "feature_id,a,b,c\nf1,1,2,3\nf2,4,5,6\n"

	_, err := readTable(strings.NewReader(input), limits{columns: maxColumns, records: maxRecords, properties: 4})
	if err == nil {
		t.Fatal("expected an error for a table over the property limit")
	}

	if !strings.Contains(err.Error(), "property values") {
		t.Fatalf("error %q does not report the property count", err)
	}
}

// A table within every bound must read unchanged.
func TestReadTable_AcceptsTableWithinLimits(t *testing.T) {
	input := "feature_id,a,b\nf1,1,2\nf2,3,4\n"

	records, err := readTable(strings.NewReader(input), limits{columns: 3, records: 2, properties: 4})
	if err != nil {
		t.Fatalf("readTable: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("read %d records, want 2", len(records))
	}

	if records[1].Properties["b"] != float64(4) {
		t.Fatalf("records[1].Properties[\"b\"] = %v, want 4", records[1].Properties["b"])
	}
}

// Rows with a blank feature_id are skipped, so they must not consume either the
// row budget or the property budget.
func TestReadTable_SkippedRowsDoNotConsumeBudget(t *testing.T) {
	input := "feature_id,a\n,1\n   ,2\nf1,3\n"

	records, err := readTable(strings.NewReader(input), limits{columns: 2, records: 1, properties: 1})
	if err != nil {
		t.Fatalf("readTable: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("read %d records, want 1", len(records))
	}
}

// Unnamed columns produce no property, so a header padded with blank names must
// not size the row maps as though it did.
func TestReadTable_BlankColumnNamesProduceNoProperties(t *testing.T) {
	input := "feature_id,,,\nf1,1,2,3\n"

	records, err := readTable(strings.NewReader(input), limits{columns: 4, records: 1, properties: 0})
	if err != nil {
		t.Fatalf("readTable: %v", err)
	}

	if len(records[0].Properties) != 0 {
		t.Fatalf("Properties = %v, want none", records[0].Properties)
	}
}

func FuzzReadTable(f *testing.F) {
	f.Add("feature_id,name,count\nf1,road A,42\nf2,road B,7\n")
	f.Add("feature_id,num,flag,label\nf1,3.14,true,hello\n")
	f.Add("feature_id,name\nf1,road A\n,road B\n   ,road C\nf4,road D\n")
	f.Add("id,name\nf1,road A\n")
	f.Add("feature_id,\"quoted,field\"\nf1,\"a\"\"b\"\n")
	f.Add("feature_id" + strings.Repeat(",c", 64) + "\n" + strings.Repeat("f1,x", 65) + "\n")
	f.Add("\n\n\n")
	f.Add("")

	f.Fuzz(func(_ *testing.T, data string) {
		// Any error is acceptable; panics and runaway allocations are not.
		_, _ = ReadTable(strings.NewReader(data))
	})
}
