package exposure

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the BEB exposure module entry in the standards registry.
	StandardID = "beb-exposure"

	IndicatorLden                    = "Lden"
	IndicatorLnight                  = "Lnight"
	IndicatorEstimatedDwellings      = "estimated_dwellings"
	IndicatorEstimatedPersons        = "estimated_persons"
	IndicatorAffectedDwellingsLden   = "affected_dwellings_lden"
	IndicatorAffectedPersonsLden     = "affected_persons_lden"
	IndicatorAffectedDwellingsLnight = "affected_dwellings_lnight"
	IndicatorAffectedPersonsLnight   = "affected_persons_lnight"
)

const (
	UsageResidential = "residential"
)

const (
	OccupancyModePreferFeatureOverrides = "prefer_feature_overrides"
	OccupancyModeHeightDerived          = "height_derived"
)

const (
	FacadeEvaluationCentroid  = "centroid"
	FacadeEvaluationMaxFacade = "max_facade"
)

const (
	UpstreamStandardBUBRoad     = road.StandardID
	UpstreamStandardBUFAircraft = bufaircraft.StandardID
)

var allowedUsageTypes = map[string]struct{}{
	UsageResidential: {},
}

var allowedOccupancyModes = map[string]struct{}{
	OccupancyModePreferFeatureOverrides: {},
	OccupancyModeHeightDerived:          {},
}

var allowedFacadeEvaluationModes = map[string]struct{}{
	FacadeEvaluationCentroid:  {},
	FacadeEvaluationMaxFacade: {},
}

// BuildingUnit describes one BEB exposure aggregation unit.
type BuildingUnit struct {
	ID                 string          `json:"id"`
	UsageType          string          `json:"usage_type"`
	HeightM            float64         `json:"height_m"`
	FloorCount         *float64        `json:"floor_count,omitempty"`
	EstimatedDwellings *float64        `json:"estimated_dwellings,omitempty"`
	EstimatedPersons   *float64        `json:"estimated_persons,omitempty"`
	Footprint          [][]geo.Point2D `json:"footprint"`
}

// ExposureConfig stores building occupancy and threshold assumptions.
type ExposureConfig struct {
	FloorHeightM            float64
	DwellingsPerFloor       float64
	PersonsPerDwelling      float64
	ThresholdLdenDB         float64
	ThresholdLnightDB       float64
	OccupancyMode           string
	FacadeEvaluationMode    string
	UpstreamMappingStandard string
}

// BuildingIndicators stores derived building-level outputs.
type BuildingIndicators struct {
	Lden                    float64
	Lnight                  float64
	EstimatedDwellings      float64
	EstimatedPersons        float64
	AffectedDwellingsLden   float64
	AffectedPersonsLden     float64
	AffectedDwellingsLnight float64
	AffectedPersonsLnight   float64
}

// BuildingExposureOutput stores one building exposure result.
type BuildingExposureOutput struct {
	Building               BuildingUnit
	RepresentativeReceiver geo.PointReceiver
	Indicators             BuildingIndicators
}

// Summary stores aggregate BEB totals.
type Summary struct {
	BuildingCount           int     `json:"building_count"`
	EstimatedDwellings      float64 `json:"estimated_dwellings"`
	EstimatedPersons        float64 `json:"estimated_persons"`
	AffectedDwellingsLden   float64 `json:"affected_dwellings_lden"`
	AffectedPersonsLden     float64 `json:"affected_persons_lden"`
	AffectedDwellingsLnight float64 `json:"affected_dwellings_lnight"`
	AffectedPersonsLnight   float64 `json:"affected_persons_lnight"`
	ThresholdLdenDB         float64 `json:"threshold_lden_db"`
	ThresholdLnightDB       float64 `json:"threshold_lnight_db"`
	OccupancyMode           string  `json:"occupancy_mode"`
	FacadeEvaluationMode    string  `json:"facade_evaluation_mode"`
	UpstreamMappingStandard string  `json:"upstream_mapping_standard"`
}

// Validate validates one building unit payload.
func (b BuildingUnit) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return errors.New("building id is required")
	}

	if _, ok := allowedUsageTypes[strings.TrimSpace(b.UsageType)]; !ok {
		return fmt.Errorf("building %q has unsupported usage_type %q", b.ID, b.UsageType)
	}

	if math.IsNaN(b.HeightM) || math.IsInf(b.HeightM, 0) || b.HeightM <= 0 {
		return fmt.Errorf("building %q height_m must be finite and > 0", b.ID)
	}

	if b.EstimatedDwellings != nil {
		if math.IsNaN(*b.EstimatedDwellings) || math.IsInf(*b.EstimatedDwellings, 0) || *b.EstimatedDwellings < 0 {
			return fmt.Errorf("building %q estimated_dwellings must be finite and >= 0", b.ID)
		}
	}

	if b.FloorCount != nil {
		if math.IsNaN(*b.FloorCount) || math.IsInf(*b.FloorCount, 0) || *b.FloorCount < 0 {
			return fmt.Errorf("building %q floor_count must be finite and >= 0", b.ID)
		}
	}

	if b.EstimatedPersons != nil {
		if math.IsNaN(*b.EstimatedPersons) || math.IsInf(*b.EstimatedPersons, 0) || *b.EstimatedPersons < 0 {
			return fmt.Errorf("building %q estimated_persons must be finite and >= 0", b.ID)
		}
	}

	if len(b.Footprint) == 0 {
		return fmt.Errorf("building %q footprint is required", b.ID)
	}

	for ringIndex, ring := range b.Footprint {
		if len(ring) < 4 {
			return fmt.Errorf("building %q footprint ring[%d] must contain at least 4 points", b.ID, ringIndex)
		}

		for pointIndex, point := range ring {
			if !point.IsFinite() {
				return fmt.Errorf("building %q footprint ring[%d] point[%d] is not finite", b.ID, ringIndex, pointIndex)
			}
		}
	}

	return nil
}

// Validate validates one exposure configuration.
func (c ExposureConfig) Validate() error {
	if math.IsNaN(c.FloorHeightM) || math.IsInf(c.FloorHeightM, 0) || c.FloorHeightM <= 0 {
		return errors.New("floor_height_m must be finite and > 0")
	}

	if math.IsNaN(c.DwellingsPerFloor) || math.IsInf(c.DwellingsPerFloor, 0) || c.DwellingsPerFloor < 0 {
		return errors.New("dwellings_per_floor must be finite and >= 0")
	}

	if math.IsNaN(c.PersonsPerDwelling) || math.IsInf(c.PersonsPerDwelling, 0) || c.PersonsPerDwelling < 0 {
		return errors.New("persons_per_dwelling must be finite and >= 0")
	}

	if math.IsNaN(c.ThresholdLdenDB) || math.IsInf(c.ThresholdLdenDB, 0) {
		return errors.New("threshold_lden_db must be finite")
	}

	if math.IsNaN(c.ThresholdLnightDB) || math.IsInf(c.ThresholdLnightDB, 0) {
		return errors.New("threshold_lnight_db must be finite")
	}

	if _, ok := allowedOccupancyModes[strings.TrimSpace(c.OccupancyMode)]; !ok {
		return fmt.Errorf("occupancy_mode must be one of %q, %q", OccupancyModePreferFeatureOverrides, OccupancyModeHeightDerived)
	}

	if _, ok := allowedFacadeEvaluationModes[strings.TrimSpace(c.FacadeEvaluationMode)]; !ok {
		return fmt.Errorf("facade_evaluation_mode must be one of %q, %q", FacadeEvaluationCentroid, FacadeEvaluationMaxFacade)
	}

	switch strings.TrimSpace(c.UpstreamMappingStandard) {
	case UpstreamStandardBUBRoad, UpstreamStandardBUFAircraft:
	default:
		return fmt.Errorf("upstream_mapping_standard must be one of %q, %q", UpstreamStandardBUBRoad, UpstreamStandardBUFAircraft)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for BEB exposure.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	minBuildingHeight := 0.1
	maxOne := 1.0

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "BEB exposure baseline using building footprints plus BUB road or BUF aircraft mapping levels for affected persons and dwellings.",
		DefaultVersion: "2021-preview",
		Versions: []framework.Version{
			{
				Name:           "2021-preview",
				DefaultProfile: "building-exposure",
				Profiles: []framework.Profile{
					{
						Name:                 "building-exposure",
						SupportedSourceTypes: []string{"line"},
						SupportedIndicators: []string{
							IndicatorLden,
							IndicatorLnight,
							IndicatorEstimatedDwellings,
							IndicatorEstimatedPersons,
							IndicatorAffectedDwellingsLden,
							IndicatorAffectedPersonsLden,
							IndicatorAffectedDwellingsLnight,
							IndicatorAffectedPersonsLnight,
						},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "upstream_mapping_standard", Kind: framework.ParameterKindString, DefaultValue: UpstreamStandardBUBRoad, Enum: []string{UpstreamStandardBUBRoad, UpstreamStandardBUFAircraft}, Description: "Upstream mapping standard contract used for level derivation"},
								{Name: "building_usage_type", Kind: framework.ParameterKindString, DefaultValue: UsageResidential, Enum: []string{UsageResidential}, Description: "Default building usage for imported building polygons"},
								{Name: "minimum_building_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minBuildingHeight, Description: "Minimum building height fallback when deriving occupancy is needed"},
								{Name: "floor_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Floor height used to estimate dwelling count from building height"},
								{Name: "dwellings_per_floor", Kind: framework.ParameterKindFloat, DefaultValue: "1", Min: &minZero, Description: "Estimated dwellings per floor"},
								{Name: "persons_per_dwelling", Kind: framework.ParameterKindFloat, DefaultValue: "2.2", Min: &minZero, Description: "Estimated persons per dwelling"},
								{Name: "threshold_lden_db", Kind: framework.ParameterKindFloat, DefaultValue: "55", Description: "Lden threshold for affected dwellings/persons"},
								{Name: "threshold_lnight_db", Kind: framework.ParameterKindFloat, DefaultValue: "50", Description: "Lnight threshold for affected dwellings/persons"},
								{Name: "occupancy_mode", Kind: framework.ParameterKindString, DefaultValue: OccupancyModePreferFeatureOverrides, Enum: []string{OccupancyModePreferFeatureOverrides, OccupancyModeHeightDerived}, Description: "How explicit building occupancy overrides are interpreted"},
								{Name: "facade_evaluation_mode", Kind: framework.ParameterKindString, DefaultValue: FacadeEvaluationCentroid, Enum: []string{FacadeEvaluationCentroid, FacadeEvaluationMaxFacade}, Description: "How building representative levels are selected from candidate facade receivers"},
								{Name: "facade_receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Representative receiver height for building exposure evaluation"},
								{Name: "road_surface_type", Kind: framework.ParameterKindString, DefaultValue: road.SurfaceDenseAsphalt, Enum: []string{road.SurfaceDenseAsphalt, road.SurfacePorousAsphalt, road.SurfaceConcrete, road.SurfaceCobblestone}, Description: "Default surface type for imported BUB road sources"},
								{Name: "road_function_class", Kind: framework.ParameterKindString, DefaultValue: road.FunctionUrbanMain, Enum: []string{road.FunctionUrbanMain, road.FunctionUrbanLocal, road.FunctionRuralMain}, Description: "Default road function class for imported BUB road sources"},
								{Name: "road_speed_kph", Kind: framework.ParameterKindFloat, DefaultValue: "60", Min: &minPositive, Description: "Default speed for imported BUB road sources"},
								{Name: "road_gradient_percent", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Default gradient for imported BUB road sources"},
								{Name: "road_junction_type", Kind: framework.ParameterKindString, DefaultValue: road.JunctionNone, Enum: []string{road.JunctionNone, road.JunctionTrafficLight, road.JunctionRoundabout}, Description: "Default junction context for imported BUB road sources"},
								{Name: "road_junction_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Distance to the nearest influencing junction for imported BUB road sources"},
								{Name: "road_temperature_c", Kind: framework.ParameterKindFloat, DefaultValue: "15", Description: "Reference temperature used for imported BUB road sources"},
								{Name: "road_studded_tyre_share", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Max: &maxOne, Description: "Share of light vehicles using studded tyres for imported BUB road sources"},
								{Name: "traffic_day_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "900", Min: &minZero, Description: "Day light vehicles per hour"},
								{Name: "traffic_day_medium_vph", Kind: framework.ParameterKindFloat, DefaultValue: "120", Min: &minZero, Description: "Day medium vehicles per hour"},
								{Name: "traffic_day_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "90", Min: &minZero, Description: "Day heavy vehicles per hour"},
								{Name: "traffic_day_ptw_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Day powered two-wheelers per hour"},
								{Name: "traffic_evening_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "500", Min: &minZero, Description: "Evening light vehicles per hour"},
								{Name: "traffic_evening_medium_vph", Kind: framework.ParameterKindFloat, DefaultValue: "60", Min: &minZero, Description: "Evening medium vehicles per hour"},
								{Name: "traffic_evening_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "45", Min: &minZero, Description: "Evening heavy vehicles per hour"},
								{Name: "traffic_evening_ptw_vph", Kind: framework.ParameterKindFloat, DefaultValue: "15", Min: &minZero, Description: "Evening powered two-wheelers per hour"},
								{Name: "traffic_night_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "250", Min: &minZero, Description: "Night light vehicles per hour"},
								{Name: "traffic_night_medium_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Night medium vehicles per hour"},
								{Name: "traffic_night_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Night heavy vehicles per hour"},
								{Name: "traffic_night_ptw_vph", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minZero, Description: "Night powered two-wheelers per hour"},
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.2", Min: &minZero, Description: "Ground attenuation term"},
								{Name: "urban_canyon_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Urban canyon mapping adjustment"},
								{Name: "intersection_density_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Intersection density mapping adjustment"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
								{Name: "airport_id", Kind: framework.ParameterKindString, DefaultValue: "DE-APT", Description: "Airport identifier for imported BUF aircraft sources"},
								{Name: "runway_id", Kind: framework.ParameterKindString, DefaultValue: "RWY", Description: "Runway identifier for imported BUF aircraft sources"},
								{Name: "aircraft_operation_type", Kind: framework.ParameterKindString, DefaultValue: bufaircraft.OperationDeparture, Enum: []string{bufaircraft.OperationDeparture, bufaircraft.OperationArrival}, Description: "Operation type for imported BUF aircraft sources"},
								{Name: "aircraft_class", Kind: framework.ParameterKindString, DefaultValue: bufaircraft.AircraftClassNarrow, Enum: []string{bufaircraft.AircraftClassRegional, bufaircraft.AircraftClassNarrow, bufaircraft.AircraftClassWide, bufaircraft.AircraftClassCargo}, Description: "Aircraft class for imported BUF aircraft sources"},
								{Name: "aircraft_procedure_type", Kind: framework.ParameterKindString, DefaultValue: bufaircraft.ProcedureStandardSID, Enum: []string{bufaircraft.ProcedureStandardSID, bufaircraft.ProcedureStandardSTAR, bufaircraft.ProcedureContinuousDescent}, Description: "Procedure type for imported BUF aircraft sources"},
								{Name: "aircraft_thrust_mode", Kind: framework.ParameterKindString, DefaultValue: bufaircraft.ThrustTakeoff, Enum: []string{bufaircraft.ThrustTakeoff, bufaircraft.ThrustReduced, bufaircraft.ThrustIdle}, Description: "Thrust mode for imported BUF aircraft sources"},
								{Name: "reference_power_level_db", Kind: framework.ParameterKindFloat, DefaultValue: "110", Description: "Reference sound power level for imported BUF aircraft sources"},
								{Name: "engine_state_factor", Kind: framework.ParameterKindFloat, DefaultValue: "1.0", Min: &minPositive, Description: "Engine state multiplier for imported BUF aircraft sources"},
								{Name: "bank_angle_deg", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Bank angle used for BUF aircraft directivity adjustment"},
								{Name: "lateral_offset_m", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Lateral procedure offset for imported BUF aircraft sources"},
								{Name: "track_start_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Start altitude of imported BUF flight tracks"},
								{Name: "track_end_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "250", Min: &minZero, Description: "End altitude of imported BUF flight tracks"},
								{Name: "movement_day_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "12", Min: &minZero, Description: "Day aircraft movements per hour"},
								{Name: "movement_evening_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "6", Min: &minZero, Description: "Evening aircraft movements per hour"},
								{Name: "movement_night_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "2", Min: &minZero, Description: "Night aircraft movements per hour"},
								{Name: "lateral_directivity_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.0", Description: "Lateral directivity adjustment for BUF aircraft"},
								{Name: "approach_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Arrival correction term for BUF aircraft"},
								{Name: "climb_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "2.5", Min: &minZero, Description: "Departure climb correction term for BUF aircraft"},
								{Name: "min_slant_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minPositive, Description: "Minimum slant propagation distance for BUF aircraft"},
							},
						},
					},
				},
			},
		},
	}
}
