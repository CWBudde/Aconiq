// Package csvimport reads CSV attribute tables and merges them into model features.
package csvimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Resource limits for untrusted CSV attribute tables.
//
// A CSV row costs a few bytes on disk and becomes a Record with one map entry
// per column in memory, so the memory a table occupies follows columns × rows
// and grows much faster than the file describing it: a map entry costs on the
// order of fifty bytes, against two bytes for the comma-separated blank that
// produces it. The numbers below are policy choices — generous for any real
// attribute or traffic table, small enough that a hostile file cannot turn a
// modest download into gigabytes of live heap.
const (
	// maxColumns caps the header width. This is the bound that matters most:
	// the per-row property map is sized from the header before a single value
	// has been read. Real attribute tables have tens of columns.
	maxColumns = 1024

	// maxRecords caps the rows kept. An attribute table with more rows than
	// this describes more features than any model this tool builds.
	maxRecords = 1 << 22

	// maxProperties caps the total property entries across all records, which
	// is the product the memory cost actually follows. Neither maxColumns nor
	// maxRecords bounds it on its own: 1024 columns over 2^22 rows is four
	// billion entries, and both limits would be satisfied.
	maxProperties = 1 << 22
)

// limits is the set of bounds one call to readTable enforces. It exists so the
// bounds can be exercised by a test without building a table large enough to
// hit the real ones, which is the cost the bounds are there to prevent.
type limits struct {
	columns    int
	records    int
	properties int
}

// defaultLimits are the bounds ReadTable applies.
var defaultLimits = limits{
	columns:    maxColumns,
	records:    maxRecords,
	properties: maxProperties,
}

// Record holds one row from a CSV attribute table.
type Record struct {
	FeatureID  string
	Properties map[string]any
}

// header describes the parsed first row of an attribute table.
type header struct {
	// names holds the trimmed column names, indexed as the rows are. Trimming
	// once here rather than once per row keeps the row loop linear in the
	// cells it actually reads.
	names []string
	// fidIndex is the position of the feature_id column.
	fidIndex int
	// propertyCount is how many columns become properties: named, and not the
	// feature_id column. It sizes each row's map.
	propertyCount int
}

// ReadTable reads a CSV attribute table from r.
//
// The first row must be a header. One column must be named "feature_id"
// (case-insensitive). All other columns become typed properties:
// float64 if parseable as a float, bool if parseable as a bool, else string.
// Rows where feature_id is empty or blank are skipped.
func ReadTable(r io.Reader) ([]Record, error) {
	return readTable(r, defaultLimits)
}

func readTable(r io.Reader, lim limits) ([]Record, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	hdr, err := readHeader(csvReader, lim)
	if err != nil {
		return nil, err
	}

	if hdr == nil {
		return nil, nil
	}

	var (
		records    []Record
		properties int
	)

	for {
		row, rowErr := csvReader.Read()
		if rowErr != nil {
			if errors.Is(rowErr, io.EOF) {
				break
			}

			return nil, fmt.Errorf("read csv row: %w", rowErr)
		}

		fid := strings.TrimSpace(row[hdr.fidIndex])
		if fid == "" {
			continue
		}

		if len(records) >= lim.records {
			return nil, fmt.Errorf("csv: more than %d rows", lim.records)
		}

		props := hdr.properties(row)

		// Bound the running total before the next row can add to it, so the
		// limit holds on the memory already retained rather than after the
		// fact.
		properties += len(props)
		if properties > lim.properties {
			return nil, fmt.Errorf("csv: more than %d property values", lim.properties)
		}

		records = append(records, Record{
			FeatureID:  fid,
			Properties: props,
		})
	}

	return records, nil
}

// readHeader reads and validates the first row. It returns a nil header for an
// empty input, which is not an error.
func readHeader(csvReader *csv.Reader, lim limits) (*header, error) {
	names, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil //nolint:nilnil // an empty table is not an error
		}

		return nil, fmt.Errorf("read csv header: %w", err)
	}

	// The width is bounded before it sizes anything: encoding/csv holds the
	// whole header row in memory anyway, but every subsequent row allocates a
	// map from this count.
	if len(names) > lim.columns {
		return nil, fmt.Errorf("csv: header declares %d columns, more than the %d supported", len(names), lim.columns)
	}

	hdr := &header{names: make([]string, len(names)), fidIndex: -1}

	for i, name := range names {
		hdr.names[i] = strings.TrimSpace(name)

		switch {
		case strings.EqualFold(hdr.names[i], "feature_id") && hdr.fidIndex < 0:
			hdr.fidIndex = i
		case hdr.names[i] != "":
			hdr.propertyCount++
		}
	}

	if hdr.fidIndex < 0 {
		return nil, errors.New("csv: no feature_id column found")
	}

	return hdr, nil
}

// properties converts one row into its typed property map. row is guaranteed
// to be as wide as the header: encoding/csv fixes the field count from the
// first record read and rejects any row that disagrees.
func (h *header) properties(row []string) map[string]any {
	props := make(map[string]any, h.propertyCount)

	for i, name := range h.names {
		if i == h.fidIndex || name == "" {
			continue
		}

		props[name] = inferType(row[i])
	}

	return props
}

// inferType attempts to parse value as float64, then bool, then returns the
// original string.
func inferType(value string) any {
	f, fErr := strconv.ParseFloat(value, 64)
	if fErr == nil {
		return f
	}

	b, bErr := strconv.ParseBool(value)
	if bErr == nil {
		return b
	}

	return value
}
