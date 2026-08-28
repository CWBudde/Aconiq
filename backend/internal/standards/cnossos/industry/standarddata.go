package industry

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the preview coefficient sets this scaffold module
// computes with.
//
// The values are this project's own invented preview constants, kept stable so
// results stay reproducible. They are not CNOSSOS-EU coefficients, and no
// coefficient from Directive 2015/996 Annex II exists in this package; see
// docs/conformance/cnossos-umfangserklaerung.md. A digest of invented constants
// is still worth carrying: it answers "could these two runs have produced
// different numbers?", which is a question about the data a module holds and
// not about the authority behind it.
//
// This module declares no coefficient table at all — every constant sits in a
// switch statement or inside a capped expression — so the tables below are
// produced by evaluating those functions over their enumerated domain. A digest
// that covered only declared package-level variables would be empty here, and
// would stay empty while a hand-edited switch changed every number.
//
// Three terms are deliberately absent. The operating-factor term is 10 lg(f), a
// formula over an unbounded domain with no constant of its own; the tonality and
// impulsivity terms pass their input through unchanged; and the area-geometry
// term is a function of the source polygon and the receiver position rather
// than of any enumerable input, so there is no fixed sample that would
// characterise it. The same applies to the 20 lg(d) + 11 divergence: closed-form
// code, not data, and a change to it is a change to the code path, which the
// tool version already identifies.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "preview-industry/model-version", Value: BuiltinModelVersion},
		{Name: "preview-industry/source-category-corrections", Value: sourceCategoryCorrectionTable()},
		{Name: "preview-industry/enclosure-corrections", Value: enclosureCorrectionTable()},
		{Name: "preview-industry/height-correction-samples", Value: heightCorrectionSampleTable()},
		{Name: "preview-industry/propagation-defaults", Value: DefaultPropagationConfig()},
	}}
}

// standardDataSourceTypes lists the source types in the fixed order every table
// below is built in.
func standardDataSourceTypes() []string {
	return []string{SourceTypePoint, SourceTypeArea}
}

// sourceCategoryCorrectionTable evaluates the category correction over every
// allowed source category and source type, since the correction is dispatched
// on both.
func sourceCategoryCorrectionTable() [][]float64 {
	categories := []string{CategoryProcess, CategoryStack, CategoryYard}
	sourceTypes := standardDataSourceTypes()

	rows := make([][]float64, 0, len(categories))
	for _, category := range categories {
		row := make([]float64, 0, len(sourceTypes))
		for _, sourceType := range sourceTypes {
			row = append(row, sourceCategoryCorrection(category, sourceType))
		}

		rows = append(rows, row)
	}

	return rows
}

// enclosureCorrectionTable evaluates the enclosure correction over every
// allowed enclosure state.
func enclosureCorrectionTable() []float64 {
	states := []string{EnclosureOpen, EnclosurePartial, EnclosureEnclosed}

	values := make([]float64, 0, len(states))
	for _, state := range states {
		values = append(values, enclosureCorrection(state))
	}

	return values
}

// heightCorrectionSampleTable samples the source-height correction per source
// type. The sample heights sit below, at and above the height where each branch
// reaches its cap — near 10 m for point sources and near 9 m for area sources —
// so both the divisor inside the logarithm and the cap itself are pinned.
func heightCorrectionSampleTable() [][]float64 {
	heights := []float64{0, 2, 5, 9, 20, 100}
	sourceTypes := standardDataSourceTypes()

	rows := make([][]float64, 0, len(sourceTypes))
	for _, sourceType := range sourceTypes {
		row := make([]float64, 0, len(heights))
		for _, height := range heights {
			row = append(row, heightCorrection(height, sourceType))
		}

		rows = append(rows, row)
	}

	return rows
}
