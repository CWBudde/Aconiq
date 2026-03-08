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

// Record holds one row from a CSV attribute table.
type Record struct {
	FeatureID  string
	Properties map[string]any
}

// ReadTable reads a CSV attribute table from r.
//
// The first row must be a header. One column must be named "feature_id"
// (case-insensitive). All other columns become typed properties:
// float64 if parseable as a float, bool if parseable as a bool, else string.
// Rows where feature_id is empty or blank are skipped.
func ReadTable(r io.Reader) ([]Record, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	headers, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}

		return nil, fmt.Errorf("read csv header: %w", err)
	}

	fidIndex := -1

	for i, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h), "feature_id") {
			fidIndex = i

			break
		}
	}

	if fidIndex < 0 {
		return nil, errors.New("csv: no feature_id column found")
	}

	var records []Record

	for {
		row, rowErr := csvReader.Read()
		if rowErr != nil {
			if errors.Is(rowErr, io.EOF) {
				break
			}

			return nil, fmt.Errorf("read csv row: %w", rowErr)
		}

		fid := strings.TrimSpace(row[fidIndex])
		if fid == "" {
			continue
		}

		props := make(map[string]any, len(headers)-1)

		for i, h := range headers {
			if i == fidIndex {
				continue
			}

			colName := strings.TrimSpace(h)
			if colName == "" {
				continue
			}

			props[colName] = inferType(row[i])
		}

		records = append(records, Record{
			FeatureID:  fid,
			Properties: props,
		})
	}

	return records, nil
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
