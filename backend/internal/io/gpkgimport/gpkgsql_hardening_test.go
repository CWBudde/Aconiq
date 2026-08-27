package gpkgimport

import (
	"context"
	"os"
	"strings"
	"testing"
)

// A GeoPackage is an untrusted SQLite file, so the table names inside its
// gpkg_contents / gpkg_geometry_columns metadata are attacker-controlled and
// must never reach a query unvalidated.
func TestQuoteIdentifier_RejectsInjection(t *testing.T) {
	hostile := []string{
		`noise_features"; DROP TABLE gpkg_contents; --`,
		`noise_features; ATTACH DATABASE '/tmp/evil.db' AS evil`,
		`noise_features'`,
		`noise_features)`,
		"noise_features\nUNION SELECT 1",
		"noise_features\x00",
		`(SELECT 1)`,
		`1noise`,
		``,
		strings.Repeat("a", maxIdentifierLength+1),
	}

	for _, name := range hostile {
		_, err := quoteIdentifier(name)
		if err == nil {
			t.Errorf("quoteIdentifier(%q) accepted a hostile table name", name)
		}
	}
}

func TestQuoteIdentifier_AcceptsRealLayerNames(t *testing.T) {
	cases := map[string]string{
		"noise_features": `"noise_features"`,
		"_private":       `"_private"`,
		"Gebäude":        `"Gebäude"`,
		"layer 1":        `"layer 1"`,
		"roads-2024.v2":  `"roads-2024.v2"`,
	}

	for name, want := range cases {
		got, err := quoteIdentifier(name)
		if err != nil {
			t.Errorf("quoteIdentifier(%q): %v", name, err)

			continue
		}

		if got != want {
			t.Errorf("quoteIdentifier(%q) = %s, want %s", name, got, want)
		}
	}
}

func TestReadLayer_RejectsHostileLayerName(t *testing.T) {
	path := createTestGPKG(t)

	_, err := ReadLayer(path, `noise_features"; DROP TABLE gpkg_contents; --`)
	if err == nil {
		t.Fatal("expected ReadLayer to refuse a hostile layer name")
	}

	if !strings.Contains(err.Error(), "refusing table name") {
		t.Fatalf("error %q does not report the refused table name", err)
	}
}

// The handle used to be opened READWRITE|CREATE, so a malicious file could be
// modified (and a successful injection could write to it). It must now be
// read-only and immutable.
func TestOpenReadOnly_RefusesWrites(t *testing.T) {
	path := createTestGPKG(t)

	db, err := openReadOnly(path)
	if err != nil {
		t.Fatalf("openReadOnly: %v", err)
	}

	defer db.Close()

	_, err = db.ExecContext(context.Background(), `CREATE TABLE injected (x INTEGER)`)
	if err == nil {
		t.Fatal("expected the read-only handle to refuse a CREATE TABLE")
	}

	// Reads must still work.
	var count int

	err = db.QueryRowContext(context.Background(), `SELECT count(*) FROM gpkg_contents`).Scan(&count)
	if err != nil {
		t.Fatalf("read through the read-only handle: %v", err)
	}

	if count != 1 {
		t.Fatalf("gpkg_contents rows = %d, want 1", count)
	}
}

// A path that does not exist must fail rather than being created, which is
// what the previous READWRITE|CREATE handle would have done.
func TestOpenReadOnly_DoesNotCreateMissingFiles(t *testing.T) {
	path := t.TempDir() + "/absent.gpkg"

	db, err := openReadOnly(path)
	if err != nil {
		t.Fatalf("openReadOnly: %v", err)
	}

	defer db.Close()

	err = db.PingContext(context.Background())
	if err == nil {
		t.Fatal("expected opening a missing database to fail")
	}

	_, statErr := os.Stat(path)
	if statErr == nil {
		t.Fatal("the missing database file was created on disk")
	}
}

func TestReadOnlyDSN_EscapesQueryDelimiters(t *testing.T) {
	dsn, err := readOnlyDSN("/tmp/a?b#c.gpkg")
	if err != nil {
		t.Fatalf("readOnlyDSN: %v", err)
	}

	if !strings.HasSuffix(dsn, "?mode=ro&immutable=1") {
		t.Fatalf("dsn %q does not end with the read-only parameters", dsn)
	}

	if strings.Count(dsn, "?") != 1 || strings.Contains(dsn, "#") {
		t.Fatalf("dsn %q leaks unescaped URI delimiters from the file name", dsn)
	}
}
