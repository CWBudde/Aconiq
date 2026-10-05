package cli

import (
	"math"
	"slices"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// compareReceiverTablesInput bundles everything one receiver comparison needs.
type compareReceiverTablesInput struct {
	table           results.ReceiverTable
	keys            map[string]soundPlanReceiverKey
	soundPlan       []soundplanimport.ReceiverResult
	toleranceDB     float64
	soundPlanSource string
	resultRun       soundPlanResultRunSelection
	runID           string
	standardID      string
	standardVersion string
	standardProfile string
}

func compareSoundPlanReceiverTables(input compareReceiverTablesInput) (soundPlanCompareReport, error) {
	matched, err := matchSoundPlanReceivers(input.table, input.keys, input.soundPlan, defaultReceiverMatchTolM)
	if err != nil {
		return soundPlanCompareReport{}, err
	}

	records := make([]soundPlanReceiverComparisonRecord, 0, len(matched.Matches))
	dayAbs := make([]float64, 0, len(matched.Matches))
	nightAbs := make([]float64, 0, len(matched.Matches))

	for _, match := range matched.Matches {
		record := input.table.Records[match.AconiqIndex]

		dayValue, ok := record.Values[schall03.IndicatorLrDay]
		if !ok {
			return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "receiver table missing LrDay", nil)
		}

		nightValue, ok := record.Values[schall03.IndicatorLrNight]
		if !ok {
			return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "receiver table missing LrNight", nil)
		}

		row := input.soundPlan[match.SoundPlanIndex]
		deltaDay := dayValue - row.ZB1
		deltaNight := nightValue - row.ZB2

		dayAbs = append(dayAbs, math.Abs(deltaDay))
		nightAbs = append(nightAbs, math.Abs(deltaNight))

		records = append(records, soundPlanReceiverComparisonRecord{
			AconiqID:       record.ID,
			SoundPlanRecNo: row.RecNo,
			SoundPlanObjID: int64(row.ObjID),
			SoundPlanFloor: row.Floor,
			SoundPlanName:  row.Name,
			MatchStrategy:  match.Strategy,
			X:              record.X,
			Y:              record.Y,
			DistanceM:      match.DistanceM,
			AconiqLrDay:    dayValue,
			SoundPlanZB1:   row.ZB1,
			DeltaDayDB:     deltaDay,
			AconiqLrNight:  nightValue,
			SoundPlanZB2:   row.ZB2,
			DeltaNightDB:   deltaNight,
		})
	}

	stats := map[string]compareIndicatorStats{
		schall03.IndicatorLrDay:   buildCompareIndicatorStats(dayAbs, input.toleranceDB),
		schall03.IndicatorLrNight: buildCompareIndicatorStats(nightAbs, input.toleranceDB),
	}

	warnings := append(append([]string(nil), input.resultRun.Warnings...), matched.Warnings...)

	return soundPlanCompareReport{
		Command:                      commandNameCompare,
		StandardID:                   input.standardID,
		StandardVersion:              input.standardVersion,
		StandardProfile:              input.standardProfile,
		RunID:                        input.runID,
		SoundPlanSource:              input.soundPlanSource,
		SoundPlanResultRun:           input.resultRun.Dir,
		SoundPlanResultRunCandidates: append([]string(nil), input.resultRun.Candidates...),
		SoundPlanResultRunSelection:  input.resultRun.Selection,
		ReceiverMatchTolM:            defaultReceiverMatchTolM,
		ToleranceDB:                  input.toleranceDB,
		MatchedReceiverCount:         len(records),
		MatchStrategyCounts:          matched.StrategyCounts,
		MaxMatchDistanceM:            matched.MaxDistanceM,
		UnmatchedAconiqCount:         len(matched.UnmatchedAconiq),
		UnmatchedSPCount:             len(matched.UnmatchedSoundPlan),
		UnmatchedAconiq:              matched.UnmatchedAconiq,
		UnmatchedSoundPlan:           matched.UnmatchedSoundPlan,
		StatsScope:                   statsScopeMatchedOnly,
		Warnings:                     warnings,
		Stats:                        stats,
		Records:                      records,
	}, nil
}

func buildCompareIndicatorStats(absDeltas []float64, toleranceDB float64) compareIndicatorStats {
	if len(absDeltas) == 0 {
		return compareIndicatorStats{}
	}

	sorted := append([]float64(nil), absDeltas...)
	slices.Sort(sorted)

	sum := 0.0
	exceeding := 0

	for _, value := range sorted {
		sum += value
		if value > toleranceDB {
			exceeding++
		}
	}

	p95Index := max(int(math.Ceil(0.95*float64(len(sorted))))-1, 0)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}

	return compareIndicatorStats{
		MeanAbsDeltaDB:     sum / float64(len(sorted)),
		MaxAbsDeltaDB:      sorted[len(sorted)-1],
		P95AbsDeltaDB:      sorted[p95Index],
		ToleranceExceeding: exceeding,
		Count:              len(sorted),
	}
}
