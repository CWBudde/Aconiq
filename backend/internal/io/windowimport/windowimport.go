// Package windowimport reads a window list: the windows of a building's
// facades, each a rectangle in its facade's frame (P18 C1,
// facade-receivers.md, The facade frame).
//
// A window list names a building by feature id or "<street> <number>", the
// facade by its canonical part, ring and edge, and the window by the u of its
// left edge, the height of its sill, its width and its height, all in metres.
// It comes as CSV or as a JSON array of objects with the same names; only the
// JSON form carries evidence. Turning a row into a model feature needs the
// model, and is modelgeojson.WindowPlacer's.
package windowimport

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// The formats a window list comes in.
const (
	FormatCSV  = "csv"
	FormatJSON = "json"
)

// Resource limits for an untrusted window list. A real building has tens of
// windows and a study a few hundred buildings; maxRows is the facade receiver
// set's own cap, since every window can become a point.
const (
	maxBytes = 32 << 20
	maxRows  = 100_000
)

// Row is one window as the list states it. Row is its 1-based position in
// the list, for the messages that name it.
type Row struct {
	Row          int
	ID           string
	Building     string
	Part         int
	Ring         int
	Edge         int
	UM           float64
	SillZM       float64
	WidthM       float64
	HeightM      float64
	Sensitive    bool
	ReviewStatus string
	Evidence     []any
}

// FormatOf is the format a path's extension names, or "" for neither.
func FormatOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return FormatCSV
	case ".json":
		return FormatJSON
	default:
		return ""
	}
}

// ReadFile reads the window list at path, in the format its extension names.
func ReadFile(path string) ([]Row, error) {
	format := FormatOf(path)
	if format == "" {
		return nil, fmt.Errorf("window list %s: the extension must be .csv or .json", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open window list: %w", err)
	}

	defer func() { _ = f.Close() }()

	return Read(f, format)
}

// Read reads a window list in the given format. It refuses the whole list on
// the first row it cannot read, naming the row.
func Read(r io.Reader, format string) ([]Row, error) {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}

	var (
		rows []Row
		err  error
	)

	switch format {
	case FormatCSV:
		rows, err = readCSV(limited)
	case FormatJSON:
		rows, err = readJSON(limited)
	default:
		return nil, fmt.Errorf("unknown window list format %q: want %s or %s", format, FormatCSV, FormatJSON)
	}

	if limited.N <= 0 {
		return nil, fmt.Errorf("window list is larger than %d bytes", maxBytes)
	}

	if err != nil {
		return nil, err
	}

	return rows, checkRows(rows)
}

// checkRows applies what both formats require of every row.
func checkRows(rows []Row) error {
	if len(rows) == 0 {
		return errors.New("window list has no window")
	}

	seen := make(map[string]int, len(rows))

	for _, row := range rows {
		switch {
		case row.ID == "":
			return fmt.Errorf("window list row %d: id is required", row.Row)
		case row.Building == "":
			return fmt.Errorf("window list row %d: building is required", row.Row)
		case row.Part < 0 || row.Ring < 0 || row.Edge < 0:
			return fmt.Errorf("window list row %d: part, ring and edge must be >= 0", row.Row)
		}

		if first, dup := seen[row.ID]; dup {
			return fmt.Errorf("window list row %d: window %q is listed twice (rows %d and %d)", row.Row, row.ID, first, row.Row)
		}

		seen[row.ID] = row.Row
	}

	return nil
}

// The column names of a window list, shared by both formats.
const (
	columnID           = "id"
	columnBuilding     = "building"
	columnPart         = "part"
	columnRing         = "ring"
	columnEdge         = "edge"
	columnUM           = "u_m"
	columnSillZM       = "sill_z_m"
	columnWidthM       = "width_m"
	columnHeightM      = "height_m"
	columnSensitive    = "sensitive"
	columnReviewStatus = "review_status"
	columnEvidence     = "evidence"
)

// The CSV header names: the required ones, and the optional ones, which
// default to their zero value.
var (
	requiredColumns = []string{columnID, columnBuilding, columnEdge, columnUM, columnSillZM, columnWidthM, columnHeightM}
	optionalColumns = []string{columnPart, columnRing, columnSensitive, columnReviewStatus}
)

func readCSV(r io.Reader) ([]Row, error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read window list header: %w", err)
	}

	index := make(map[string]int, len(header))

	for i, name := range header {
		name = strings.TrimSpace(strings.TrimPrefix(name, "\uFEFF"))

		switch {
		case name == columnEvidence:
			return nil, errors.New("window list column evidence is accepted in a JSON window list only")
		case !slices.Contains(requiredColumns, name) && !slices.Contains(optionalColumns, name):
			return nil, fmt.Errorf("window list column %q is not one of %s",
				name, strings.Join(slices.Concat(requiredColumns, optionalColumns), ", "))
		}

		index[name] = i
	}

	for _, name := range requiredColumns {
		if _, ok := index[name]; !ok {
			return nil, fmt.Errorf("window list has no %s column", name)
		}
	}

	var rows []Row

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}

		if err != nil {
			return nil, fmt.Errorf("read window list: %w", err)
		}

		if len(rows) == maxRows {
			return nil, fmt.Errorf("window list has more than %d windows", maxRows)
		}

		row, err := csvRow(record, index, len(rows)+1)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}
}

func csvRow(record []string, index map[string]int, n int) (Row, error) {
	cell := func(name string) string {
		i, ok := index[name]
		if !ok {
			return ""
		}

		return strings.TrimSpace(record[i])
	}

	row := Row{Row: n, ID: cell(columnID), Building: cell(columnBuilding), ReviewStatus: cell(columnReviewStatus)}

	for _, field := range []struct {
		name     string
		dst      *int
		optional bool
	}{{columnPart, &row.Part, true}, {columnRing, &row.Ring, true}, {columnEdge, &row.Edge, false}} {
		text := cell(field.name)
		if text == "" && field.optional {
			continue
		}

		v, err := strconv.Atoi(text)
		if err != nil {
			return Row{}, fmt.Errorf("window list row %d: %s %q is not an integer", n, field.name, text)
		}

		*field.dst = v
	}

	for _, field := range []struct {
		name string
		dst  *float64
	}{{columnUM, &row.UM}, {columnSillZM, &row.SillZM}, {columnWidthM, &row.WidthM}, {columnHeightM, &row.HeightM}} {
		text := cell(field.name)

		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return Row{}, fmt.Errorf("window list row %d: %s %q is not a number", n, field.name, text)
		}

		*field.dst = v
	}

	if text := cell(columnSensitive); text != "" {
		sensitive, err := strconv.ParseBool(text)
		if err != nil {
			return Row{}, fmt.Errorf("window list row %d: sensitive %q is neither true nor false", n, text)
		}

		row.Sensitive = sensitive
	}

	return row, nil
}

// jsonRow is one JSON window. The required numbers are pointers, so a missing
// one is told from a zero.
type jsonRow struct {
	ID           string   `json:"id"`
	Building     string   `json:"building"`
	Part         int      `json:"part"`
	Ring         int      `json:"ring"`
	Edge         *int     `json:"edge"`
	UM           *float64 `json:"u_m"`
	SillZM       *float64 `json:"sill_z_m"`
	WidthM       *float64 `json:"width_m"`
	HeightM      *float64 `json:"height_m"`
	Sensitive    bool     `json:"sensitive"`
	ReviewStatus string   `json:"review_status"`
	Evidence     []any    `json:"evidence"`
}

func readJSON(r io.Reader) ([]Row, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("read window list: %w", err)
	}

	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return nil, errors.New("window list must be a JSON array of window objects")
	}

	var rows []Row

	for decoder.More() {
		if len(rows) == maxRows {
			return nil, fmt.Errorf("window list has more than %d windows", maxRows)
		}

		n := len(rows) + 1

		var raw jsonRow

		err := decoder.Decode(&raw)
		if err != nil {
			return nil, fmt.Errorf("window list row %d: %w", n, err)
		}

		row, err := raw.row(n)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	// The closing bracket, then nothing.
	_, err = decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("read window list: %w", err)
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("window list has data after its closing ]")
	}

	return rows, nil
}

func (raw jsonRow) row(n int) (Row, error) {
	for _, member := range []struct {
		name    string
		present bool
	}{
		{columnEdge, raw.Edge != nil},
		{columnUM, raw.UM != nil},
		{columnSillZM, raw.SillZM != nil},
		{columnWidthM, raw.WidthM != nil},
		{columnHeightM, raw.HeightM != nil},
	} {
		if !member.present {
			return Row{}, fmt.Errorf("window list row %d: %s is required", n, member.name)
		}
	}

	for i, entry := range raw.Evidence {
		if _, ok := entry.(map[string]any); !ok {
			return Row{}, fmt.Errorf("window list row %d: evidence[%d] must be an object", n, i)
		}
	}

	return Row{
		Row:          n,
		ID:           strings.TrimSpace(raw.ID),
		Building:     strings.TrimSpace(raw.Building),
		Part:         raw.Part,
		Ring:         raw.Ring,
		Edge:         *raw.Edge,
		UM:           *raw.UM,
		SillZM:       *raw.SillZM,
		WidthM:       *raw.WidthM,
		HeightM:      *raw.HeightM,
		Sensitive:    raw.Sensitive,
		ReviewStatus: strings.TrimSpace(raw.ReviewStatus),
		Evidence:     raw.Evidence,
	}, nil
}
