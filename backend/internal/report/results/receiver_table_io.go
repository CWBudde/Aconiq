package results

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// SaveReceiverTableJSON writes receiver table as JSON.
func SaveReceiverTableJSON(path string, table ReceiverTable) error {
	if err := table.Validate(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create receiver table directory: %w", err)
	}

	payload, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("encode receiver table json: %w", err)
	}

	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write receiver table json %s: %w", path, err)
	}

	return nil
}

// SaveReceiverTableCSV writes receiver table as CSV.
func SaveReceiverTableCSV(path string, table ReceiverTable) error {
	if err := table.Validate(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create receiver table directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create receiver table csv %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	header := []string{"id", "x", "y", "height_m"}

	header = append(header, table.IndicatorOrder...)
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write receiver table csv header: %w", err)
	}

	for _, record := range table.Records {
		row := []string{
			record.ID,
			strconv.FormatFloat(record.X, 'f', -1, 64),
			strconv.FormatFloat(record.Y, 'f', -1, 64),
			strconv.FormatFloat(record.HeightM, 'f', -1, 64),
		}
		for _, indicator := range table.IndicatorOrder {
			row = append(row, strconv.FormatFloat(record.Values[indicator], 'f', -1, 64))
		}

		err := writer.Write(row)
		if err != nil {
			return fmt.Errorf("write receiver table csv row for %s: %w", record.ID, err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush receiver table csv: %w", err)
	}

	return nil
}
