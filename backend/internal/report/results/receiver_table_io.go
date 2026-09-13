package results

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// LoadReceiverTableJSON reads a receiver table from a JSON file.
func LoadReceiverTableJSON(path string) (ReceiverTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReceiverTable{}, fmt.Errorf("read receiver table json %s: %w", path, err)
	}

	var table ReceiverTable

	err = json.Unmarshal(data, &table)
	if err != nil {
		return ReceiverTable{}, fmt.Errorf("decode receiver table json %s: %w", path, err)
	}

	return table, nil
}

// SaveReceiverTableJSON writes receiver table as JSON.
func SaveReceiverTableJSON(path string, table ReceiverTable) error {
	err := table.Validate()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return fmt.Errorf("create receiver table directory: %w", err)
	}

	payload, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("encode receiver table json: %w", err)
	}

	payload = append(payload, '\n')

	err = os.WriteFile(path, payload, 0o600)
	if err != nil {
		return fmt.Errorf("write receiver table json %s: %w", path, err)
	}

	return nil
}

// SaveReceiverTableCSV writes receiver table as CSV.
//
// The table is validated before the file is created, so an invalid table
// leaves no half-written artifact behind.
func SaveReceiverTableCSV(path string, table ReceiverTable) (err error) {
	err = table.Validate()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return fmt.Errorf("create receiver table directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create receiver table csv %s: %w", path, err)
	}

	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close receiver table csv %s: %w", path, cerr)
		}
	}()

	err = WriteReceiverTableCSV(file, table)
	if err != nil {
		return fmt.Errorf("write receiver table csv %s: %w", path, err)
	}

	return nil
}

// WriteReceiverTableCSV streams the receiver table to w in the canonical CSV
// spelling.
//
// This spelling is the cross-target contract: the browser build has its own
// implementation in frontend/src/model/receiver-csv.ts, and the two must agree
// byte for byte. Whatever encoding/csv does here is what the frontend mirrors,
// which is why the body stays a thin shell around csv.Writer rather than
// hand-rolling the quoting. See docs/result-containers-v1.md, section
// "Receiver table CSV — byte contract".
//
// The table is validated first, so a caller that hands over garbage gets an
// error rather than a partially written stream. Records are written one at a
// time; the whole table is never buffered in memory.
func WriteReceiverTableCSV(w io.Writer, table ReceiverTable) error {
	err := table.Validate()
	if err != nil {
		return err
	}

	writer := csv.NewWriter(w)
	header := make([]string, 0, len(table.IndicatorOrder)+4)
	header = append(header, "id", "x", "y", "height_m")

	header = append(header, table.IndicatorOrder...)

	err = writer.Write(header)
	if err != nil {
		return fmt.Errorf("write receiver table csv header: %w", err)
	}

	row := make([]string, 0, len(table.IndicatorOrder)+4)

	for _, record := range table.Records {
		row = append(
			row[:0],
			record.ID,
			strconv.FormatFloat(record.X, 'f', -1, 64),
			strconv.FormatFloat(record.Y, 'f', -1, 64),
			strconv.FormatFloat(record.HeightM, 'f', -1, 64),
		)
		// Validate has already rejected any record missing an ordered
		// indicator, so the map lookup below always hits. The frontend builder
		// has no such gate and writes an empty field instead — see the comment
		// on buildReceiverTableCSV.
		for _, indicator := range table.IndicatorOrder {
			row = append(row, strconv.FormatFloat(record.Values[indicator], 'f', -1, 64))
		}

		err := writer.Write(row)
		if err != nil {
			return fmt.Errorf("write receiver table csv row for %s: %w", record.ID, err)
		}
	}

	writer.Flush()

	err = writer.Error()
	if err != nil {
		return fmt.Errorf("flush receiver table csv: %w", err)
	}

	return nil
}
