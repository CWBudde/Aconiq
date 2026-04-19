package soundplanimport

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ImmissionTableColumn describes one column in an NTD immission-table definition.
type ImmissionTableColumn struct {
	Title       string
	ResultFile  string
	ResultField string
	Formula     string
}

// ImmissionTable describes one SoundPLAN IOTable*.ntd file.
type ImmissionTable struct {
	SourceFile string
	Columns    []ImmissionTableColumn
}

// ParseImmissionTableFile reads an IOTable*.ntd file and extracts the column
// bindings and formulas of the immission-point table definition.
func ParseImmissionTableFile(path string) (*ImmissionTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read ntd: %w", err)
	}

	return parseImmissionTableData(filepath.Base(path), data)
}

// LoadImmissionTables discovers and parses IOTable*.ntd files in one project directory.
func LoadImmissionTables(projectDir string) ([]ImmissionTable, []string) {
	paths, err := filepath.Glob(filepath.Join(projectDir, "IOTable*.ntd"))
	if err != nil {
		return nil, []string{fmt.Sprintf("soundplan: list ntd: %v", err)}
	}

	sort.Strings(paths)

	tables := make([]ImmissionTable, 0, len(paths))
	warnings := make([]string, 0)
	for _, path := range paths {
		table, parseErr := ParseImmissionTableFile(path)
		if parseErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", filepath.Base(path), parseErr))
			continue
		}

		tables = append(tables, *table)
	}

	return tables, warnings
}

func parseImmissionTableData(sourceFile string, data []byte) (*ImmissionTable, error) {
	tokens := extractPrintableNullTerminatedStrings(data)
	table := &ImmissionTable{
		SourceFile: sourceFile,
		Columns:    make([]ImmissionTableColumn, 0, 16),
	}

	active := false
	pendingTitle := ""

	for i := 0; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])
		if token == "" {
			continue
		}

		if token == "StandardtextT" {
			active = true
			pendingTitle = ""
			continue
		}

		if !active || isIgnoredNTDToken(token) {
			continue
		}

		switch {
		case looksLikeResultFile(token):
			next, ok := nextMeaningfulNTDToken(tokens, i+1)
			if !ok || !isFieldToken(next) {
				continue
			}

			title := pendingTitle
			if title == "" {
				title = trimColumnToken(next)
			}

			table.Columns = append(table.Columns, ImmissionTableColumn{
				Title:       title,
				ResultFile:  token,
				ResultField: trimColumnToken(next),
			})
			pendingTitle = ""

		case looksLikeFormula(token):
			table.Columns = append(table.Columns, ImmissionTableColumn{
				Title:   pendingTitle,
				Formula: token,
			})
			pendingTitle = ""

		case isTitleToken(token):
			pendingTitle = trimColumnToken(token)
		}
	}

	if len(table.Columns) == 0 {
		return nil, fmt.Errorf("soundplan: ntd: no immission-table columns found")
	}

	return table, nil
}

func nextMeaningfulNTDToken(tokens []string, start int) (string, bool) {
	for i := start; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])
		if token == "" || token == "StandardtextT" || isIgnoredNTDToken(token) {
			continue
		}

		return token, true
	}

	return "", false
}

func isIgnoredNTDToken(value string) bool {
	switch strings.TrimSpace(value) {
	case "Formel", "Beschreibung", "Calibri":
		return true
	default:
		return false
	}
}

func looksLikeResultFile(value string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(value)), ".res")
}

func isFieldToken(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasSuffix(value, ")") && !looksLikeResultFile(value)
}

func isTitleToken(value string) bool {
	value = trimColumnToken(value)
	if value == "" {
		return false
	}

	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			return true
		case r >= 'a' && r <= 'z':
			return true
		}
	}

	return false
}

func trimColumnToken(value string) string {
	return strings.TrimSpace(strings.TrimSuffix(value, ")"))
}

func looksLikeFormula(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasSuffix(value, ";)") || strings.HasSuffix(value, ";") || strings.HasPrefix(value, "if ") || strings.HasPrefix(value, "X")
}
