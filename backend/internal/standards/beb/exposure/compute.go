package exposure

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
)

type preparedBuilding struct {
	building           BuildingUnit
	centroid           geo.Point2D
	candidateReceivers []geo.PointReceiver
}

type levelIndicators struct {
	Lden   float64
	Lnight float64
}

type occupancyEstimate struct {
	Floors    float64
	Dwellings float64
	Persons   float64
}

var (
	defaultLdenBandEdges   = []float64{55, 60, 65, 70, 75}
	defaultLnightBandEdges = []float64{50, 55, 60, 65, 70}
)

// ComputeOutputs computes building exposure results and aggregate totals.
func ComputeOutputs(buildings []BuildingUnit, roads []road.RoadSource, cfg ExposureConfig, propagation road.PropagationConfig, receiverHeightM float64) ([]BuildingExposureOutput, Summary, error) {
	if len(buildings) == 0 {
		return nil, Summary{}, errors.New("at least one building is required")
	}

	if len(roads) == 0 {
		return nil, Summary{}, errors.New("at least one road source is required")
	}

	err := cfg.Validate()
	if err != nil {
		return nil, Summary{}, err
	}

	prepared, receivers, err := prepareBuildings(buildings, receiverHeightM, cfg.FacadeEvaluationMode)
	if err != nil {
		return nil, Summary{}, err
	}

	roadOutputs, err := road.ComputeReceiverOutputs(receivers, roads, propagation)
	if err != nil {
		return nil, Summary{}, err
	}

	levelByID := make(map[string]levelIndicators, len(roadOutputs))
	for _, output := range roadOutputs {
		levelByID[output.Receiver.ID] = levelIndicators{
			Lden:   output.Indicators.Lden,
			Lnight: output.Indicators.Lnight,
		}
	}

	return finalizeOutputs(prepared, levelByID, cfg)
}

// ComputeOutputsFromAircraft computes BEB outputs from BUF aircraft receiver levels.
func ComputeOutputsFromAircraft(buildings []BuildingUnit, aircraftSources []bufaircraft.AircraftSource, cfg ExposureConfig, propagation bufaircraft.PropagationConfig, receiverHeightM float64) ([]BuildingExposureOutput, Summary, error) {
	if len(buildings) == 0 {
		return nil, Summary{}, errors.New("at least one building is required")
	}

	if len(aircraftSources) == 0 {
		return nil, Summary{}, errors.New("at least one aircraft source is required")
	}

	err := cfg.Validate()
	if err != nil {
		return nil, Summary{}, err
	}

	prepared, receivers, err := prepareBuildings(buildings, receiverHeightM, cfg.FacadeEvaluationMode)
	if err != nil {
		return nil, Summary{}, err
	}

	aircraftOutputs, err := bufaircraft.ComputeReceiverOutputs(receivers, aircraftSources, propagation)
	if err != nil {
		return nil, Summary{}, err
	}

	levelByID := make(map[string]levelIndicators, len(aircraftOutputs))
	for _, output := range aircraftOutputs {
		levelByID[output.Receiver.ID] = levelIndicators{
			Lden:   output.Indicators.Lden,
			Lnight: output.Indicators.Lnight,
		}
	}

	return finalizeOutputs(prepared, levelByID, cfg)
}

func prepareBuildings(buildings []BuildingUnit, receiverHeightM float64, facadeEvaluationMode string) ([]preparedBuilding, []geo.PointReceiver, error) {
	receivers := make([]geo.PointReceiver, 0, len(buildings)*5)
	prepared := make([]preparedBuilding, 0, len(buildings))

	for _, building := range buildings {
		err := building.Validate()
		if err != nil {
			return nil, nil, err
		}

		centroid, err := representativePoint(building.Footprint)
		if err != nil {
			return nil, nil, fmt.Errorf("building %q representative point: %w", building.ID, err)
		}

		candidateReceivers := []geo.PointReceiver{{
			ID:      buildingReceiverID(building.ID, "centroid"),
			Point:   centroid,
			HeightM: receiverHeightM,
		}}

		if facadeEvaluationMode == FacadeEvaluationMaxFacade {
			candidateReceivers = append(candidateReceivers, facadeReceivers(building.ID, building.Footprint, receiverHeightM)...)
		}

		prepared = append(prepared, preparedBuilding{
			building:           building,
			centroid:           centroid,
			candidateReceivers: candidateReceivers,
		})
		receivers = append(receivers, candidateReceivers...)
	}

	return prepared, receivers, nil
}

func finalizeOutputs(prepared []preparedBuilding, levelByID map[string]levelIndicators, cfg ExposureConfig) ([]BuildingExposureOutput, Summary, error) {
	outputs := make([]BuildingExposureOutput, 0, len(prepared))
	summary := Summary{
		BuildingCount:           len(prepared),
		ThresholdLdenDB:         cfg.ThresholdLdenDB,
		ThresholdLnightDB:       cfg.ThresholdLnightDB,
		OccupancyMode:           cfg.OccupancyMode,
		FacadeEvaluationMode:    cfg.FacadeEvaluationMode,
		UpstreamMappingStandard: cfg.UpstreamMappingStandard,
		LdenBands:               defaultExposureBands(defaultLdenBandEdges),
		LnightBands:             defaultExposureBands(defaultLnightBandEdges),
	}

	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].building.ID < prepared[j].building.ID
	})

	for _, item := range prepared {
		selectedReceiver, levels, err := selectBuildingLevels(item, levelByID, cfg.FacadeEvaluationMode)
		if err != nil {
			return nil, Summary{}, err
		}

		occupancy := evaluateOccupancy(item.building, cfg)
		affectedDwellingsLden := thresholdExposure(levels.Lden, cfg.ThresholdLdenDB, occupancy.Dwellings)
		affectedPersonsLden := thresholdExposure(levels.Lden, cfg.ThresholdLdenDB, occupancy.Persons)
		affectedDwellingsLnight := thresholdExposure(levels.Lnight, cfg.ThresholdLnightDB, occupancy.Dwellings)
		affectedPersonsLnight := thresholdExposure(levels.Lnight, cfg.ThresholdLnightDB, occupancy.Persons)

		outputs = append(outputs, BuildingExposureOutput{
			Building:               item.building,
			RepresentativeReceiver: selectedReceiver,
			Indicators: BuildingIndicators{
				Lden:                    levels.Lden,
				Lnight:                  levels.Lnight,
				EstimatedDwellings:      occupancy.Dwellings,
				EstimatedPersons:        occupancy.Persons,
				AffectedDwellingsLden:   affectedDwellingsLden,
				AffectedPersonsLden:     affectedPersonsLden,
				AffectedDwellingsLnight: affectedDwellingsLnight,
				AffectedPersonsLnight:   affectedPersonsLnight,
			},
		})

		summary.EstimatedDwellings += occupancy.Dwellings
		summary.EstimatedPersons += occupancy.Persons
		summary.AffectedDwellingsLden += affectedDwellingsLden
		summary.AffectedPersonsLden += affectedPersonsLden
		summary.AffectedDwellingsLnight += affectedDwellingsLnight
		summary.AffectedPersonsLnight += affectedPersonsLnight
		addExposureToBands(summary.LdenBands, levels.Lden, occupancy.Dwellings, occupancy.Persons)
		addExposureToBands(summary.LnightBands, levels.Lnight, occupancy.Dwellings, occupancy.Persons)
	}

	return outputs, summary, nil
}

func defaultExposureBands(edges []float64) []ExposureBandSummary {
	bands := make([]ExposureBandSummary, 0, len(edges))
	for i, lower := range edges {
		band := ExposureBandSummary{
			Label:   fmt.Sprintf("%.0f-%.0f", lower, lower+4),
			LowerDB: lower,
		}

		if i == len(edges)-1 {
			band.Label = fmt.Sprintf("%.0f+", lower)
		} else {
			upper := edges[i+1]
			band.UpperDBExclusive = &upper
		}

		bands = append(bands, band)
	}

	return bands
}

func addExposureToBands(bands []ExposureBandSummary, levelDB float64, dwellings float64, persons float64) {
	for i := range bands {
		band := &bands[i]
		if levelDB < band.LowerDB {
			continue
		}

		if band.UpperDBExclusive != nil && levelDB >= *band.UpperDBExclusive {
			continue
		}

		band.EstimatedDwellings += dwellings
		band.EstimatedPersons += persons

		return
	}
}

func evaluateOccupancy(building BuildingUnit, cfg ExposureConfig) occupancyEstimate {
	floors := estimateFloors(building, cfg)
	dwellings := estimateDwellings(building, floors, cfg)
	persons := estimatePersons(building, dwellings, cfg)

	return occupancyEstimate{
		Floors:    floors,
		Dwellings: dwellings,
		Persons:   persons,
	}
}

func estimateFloors(building BuildingUnit, cfg ExposureConfig) float64 {
	if building.FloorCount != nil {
		return math.Max(0, *building.FloorCount)
	}

	floors := math.Ceil(building.HeightM / cfg.FloorHeightM)
	if floors < 1 {
		floors = 1
	}

	return floors
}

func estimateDwellings(building BuildingUnit, floors float64, cfg ExposureConfig) float64 {
	if cfg.OccupancyMode == OccupancyModePreferFeatureOverrides && building.EstimatedDwellings != nil {
		return *building.EstimatedDwellings
	}

	return floors * cfg.DwellingsPerFloor
}

func estimatePersons(building BuildingUnit, estimatedDwellings float64, cfg ExposureConfig) float64 {
	if cfg.OccupancyMode == OccupancyModePreferFeatureOverrides && building.EstimatedPersons != nil {
		return *building.EstimatedPersons
	}

	return estimatedDwellings * cfg.PersonsPerDwelling
}

func thresholdExposure(levelDB float64, thresholdDB float64, total float64) float64 {
	if levelDB >= thresholdDB {
		return total
	}

	return 0
}

func selectBuildingLevels(item preparedBuilding, levelByID map[string]levelIndicators, facadeEvaluationMode string) (geo.PointReceiver, levelIndicators, error) {
	centroidReceiver := item.candidateReceivers[0]

	centroidLevels, ok := levelByID[centroidReceiver.ID]
	if !ok {
		return geo.PointReceiver{}, levelIndicators{}, fmt.Errorf("missing upstream levels for building %q", item.building.ID)
	}

	if facadeEvaluationMode != FacadeEvaluationMaxFacade || len(item.candidateReceivers) == 1 {
		return centroidReceiver, centroidLevels, nil
	}

	selectedReceiver := centroidReceiver
	selectedLevels := centroidLevels
	maxLden := centroidLevels.Lden
	maxLnight := centroidLevels.Lnight

	for _, receiver := range item.candidateReceivers[1:] {
		levels, ok := levelByID[receiver.ID]
		if !ok {
			return geo.PointReceiver{}, levelIndicators{}, fmt.Errorf("missing upstream levels for building %q receiver %q", item.building.ID, receiver.ID)
		}

		if levels.Lden > maxLden {
			maxLden = levels.Lden
			selectedReceiver = receiver
			selectedLevels.Lden = levels.Lden
		}

		if levels.Lnight > maxLnight {
			maxLnight = levels.Lnight
			selectedLevels.Lnight = levels.Lnight
		}
	}

	return selectedReceiver, selectedLevels, nil
}

func buildingReceiverID(buildingID string, suffix string) string {
	return buildingID + ":" + suffix
}

func facadeReceivers(buildingID string, footprint [][]geo.Point2D, receiverHeightM float64) []geo.PointReceiver {
	if len(footprint) == 0 || len(footprint[0]) < 2 {
		return nil
	}

	ring := footprint[0]

	receivers := make([]geo.PointReceiver, 0, len(ring)-1)
	for i := range len(ring) - 1 {
		midpoint := geo.Point2D{
			X: (ring[i].X + ring[i+1].X) / 2,
			Y: (ring[i].Y + ring[i+1].Y) / 2,
		}
		receivers = append(receivers, geo.PointReceiver{
			ID:      buildingReceiverID(buildingID, fmt.Sprintf("facade-%02d", i+1)),
			Point:   midpoint,
			HeightM: receiverHeightM,
		})
	}

	return receivers
}

func representativePoint(footprint [][]geo.Point2D) (geo.Point2D, error) {
	if len(footprint) == 0 || len(footprint[0]) < 4 {
		return geo.Point2D{}, errors.New("footprint exterior ring is required")
	}

	centroid, ok := polygonCentroid(footprint[0])
	if !ok {
		bbox, bboxOK := geo.BBoxFromPolygon(footprint)
		if !bboxOK {
			return geo.Point2D{}, errors.New("derive footprint bbox")
		}

		centroid = geo.Point2D{
			X: (bbox.MinX + bbox.MaxX) / 2,
			Y: (bbox.MinY + bbox.MaxY) / 2,
		}
	}

	if geo.PointInPolygon(centroid, footprint) {
		return centroid, nil
	}

	bbox, ok := geo.BBoxFromPolygon(footprint)
	if !ok {
		return geo.Point2D{}, errors.New("derive footprint bbox")
	}

	center := geo.Point2D{
		X: (bbox.MinX + bbox.MaxX) / 2,
		Y: (bbox.MinY + bbox.MaxY) / 2,
	}
	if geo.PointInPolygon(center, footprint) {
		return center, nil
	}

	return footprint[0][0], nil
}

func polygonCentroid(ring []geo.Point2D) (geo.Point2D, bool) {
	if len(ring) < 4 {
		return geo.Point2D{}, false
	}

	doubleArea := 0.0
	cx := 0.0
	cy := 0.0

	for i := range len(ring) - 1 {
		cross := ring[i].X*ring[i+1].Y - ring[i+1].X*ring[i].Y
		doubleArea += cross
		cx += (ring[i].X + ring[i+1].X) * cross
		cy += (ring[i].Y + ring[i+1].Y) * cross
	}

	if math.Abs(doubleArea) < 1e-12 {
		return geo.Point2D{}, false
	}

	factor := 1.0 / (3.0 * doubleArea)

	point := geo.Point2D{X: cx * factor, Y: cy * factor}
	if !point.IsFinite() {
		return geo.Point2D{}, false
	}

	return point, true
}
