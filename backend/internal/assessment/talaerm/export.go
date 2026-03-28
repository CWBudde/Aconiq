package talaerm

import "time"

// SkippedReceiver records a receiver that was excluded from assessment.
type SkippedReceiver struct {
	ReceiverID string `json:"receiver_id"`
	Reason     string `json:"reason"`
}

// ExportEnvelope wraps a complete TA Lärm assessment for serialisation and reporting.
type ExportEnvelope struct {
	Regulation       string               `json:"regulation"`
	Edition          string               `json:"edition"`
	GeneratedAt      time.Time            `json:"generated_at"`
	SourceStandardID string               `json:"source_standard_id"`
	ReceiverCount    int                  `json:"receiver_count"`
	AssessedCount    int                  `json:"assessed_count"`
	ExceedingCount   int                  `json:"exceeding_count"`
	IrrelevantCount  int                  `json:"irrelevant_count"`
	CategoryCounts   map[string]int       `json:"category_counts,omitempty"`
	Results          []ReceiverAssessment `json:"results"`
	Skipped          []SkippedReceiver    `json:"skipped,omitempty"`
}

// BuildExportEnvelope assembles an ExportEnvelope from pre-assessed results and skipped receivers.
func BuildExportEnvelope(results []ReceiverAssessment, skipped []SkippedReceiver, sourceStandardID string, generatedAt time.Time) ExportEnvelope {
	exceedingCount := 0
	irrelevantCount := 0
	categoryCounts := make(map[string]int, len(results))

	for i := range results {
		if results[i].Exceeds {
			exceedingCount++
		}

		if results[i].Irrelevanz.DayIrrelevant && results[i].Irrelevanz.NightIrrelevant {
			irrelevantCount++
		}

		code := AreaCategoryCode(results[i].AreaCategory)
		if code != "" {
			categoryCounts[code]++
		}
	}

	if len(categoryCounts) == 0 {
		categoryCounts = nil
	}

	return ExportEnvelope{
		Regulation:       RegulationName,
		Edition:          RegulationEdition,
		GeneratedAt:      generatedAt.UTC(),
		SourceStandardID: sourceStandardID,
		ReceiverCount:    len(results) + len(skipped),
		AssessedCount:    len(results),
		ExceedingCount:   exceedingCount,
		IrrelevantCount:  irrelevantCount,
		CategoryCounts:   categoryCounts,
		Results:          results,
		Skipped:          skipped,
	}
}
