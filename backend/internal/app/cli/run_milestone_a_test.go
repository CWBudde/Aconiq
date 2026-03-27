package cli

import (
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestExtractCnossosRoadSourcesUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "road-1",
        "kind": "source",
        "source_type": "line",
        "road_category": "urban_local",
        "road_surface_type": "concrete",
        "road_speed_kph": 42,
        "road_gradient_percent": 3,
        "road_junction_type": "traffic_light",
        "road_junction_distance_m": 25,
        "road_temperature_c": 5,
        "road_studded_tyre_share": 0.15,
        "traffic_day_light_vph": 111,
        "traffic_day_medium_vph": 21,
        "traffic_day_heavy_vph": 12,
        "traffic_evening_light_vph": 88,
        "traffic_evening_medium_vph": 13,
        "traffic_evening_heavy_vph": 9,
        "traffic_night_light_vph": 55,
        "traffic_night_medium_vph": 8,
        "traffic_night_heavy_vph": 6,
        "traffic_day_ptw_vph": 7,
        "traffic_evening_ptw_vph": 4,
        "traffic_night_ptw_vph": 1
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    }
  ]
}`)

	options := cnossosRoadRunOptions{
		RoadCategory:            cnossosroad.CategoryUrbanMajor,
		SurfaceType:             cnossosroad.SurfaceDenseAsphalt,
		SpeedKPH:                70,
		GradientPercent:         0,
		JunctionType:            cnossosroad.JunctionNone,
		JunctionDistanceM:       0,
		TemperatureC:            20,
		StuddedTyreShare:        0,
		TrafficDayLightVPH:      900,
		TrafficDayMediumVPH:     120,
		TrafficDayHeavyVPH:      90,
		TrafficEveningLightVPH:  500,
		TrafficEveningMediumVPH: 60,
		TrafficEveningHeavyVPH:  45,
		TrafficNightLightVPH:    250,
		TrafficNightMediumVPH:   30,
		TrafficNightHeavyVPH:    30,
		TrafficDayPTWVPH:        40,
		TrafficEveningPTWVPH:    20,
		TrafficNightPTWVPH:      5,
	}

	sources, err := extractCnossosRoadSources(model, options, []string{"line"})
	if err != nil {
		t.Fatalf("extract cnossos road sources: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	source := sources[0]
	if source.RoadCategory != cnossosroad.CategoryUrbanLocal || source.SurfaceType != cnossosroad.SurfaceConcrete || source.SpeedKPH != 42 || source.GradientPercent != 3 {
		t.Fatalf("unexpected road overrides: %#v", source)
	}

	if source.JunctionType != cnossosroad.JunctionTrafficLight || source.JunctionDistanceM != 25 || source.TemperatureC != 5 || source.StuddedTyreShare != 0.15 {
		t.Fatalf("unexpected road context overrides: %#v", source)
	}

	if source.TrafficDay.LightVehiclesPerHour != 111 || source.TrafficDay.MediumVehiclesPerHour != 21 || source.TrafficNight.HeavyVehiclesPerHour != 6 || source.TrafficDay.PoweredTwoWheelersPerHour != 7 {
		t.Fatalf("unexpected traffic overrides: %#v", source)
	}
}

func TestExtractBUBRoadSourcesUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "road-1",
        "kind": "source",
        "source_type": "line",
        "road_surface_type": "cobblestone",
        "road_function_class": "rural_main",
        "road_speed_kph": 35,
        "road_junction_type": "traffic_light",
        "road_junction_distance_m": 25,
        "road_temperature_c": 3,
        "road_studded_tyre_share": 0.2,
        "traffic_day_medium_vph": 33,
        "traffic_evening_ptw_vph": 7
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    }
  ]
}`)

	options := bubRoadRunOptions{
		SurfaceType:          bubroad.SurfaceDenseAsphalt,
		RoadFunctionClass:    bubroad.FunctionUrbanMain,
		SpeedKPH:             60,
		JunctionType:         bubroad.JunctionNone,
		JunctionDistanceM:    0,
		TemperatureC:         15,
		StuddedTyreShare:     0,
		TrafficDayMediumVPH:  10,
		TrafficEveningPTWVPH: 2,
	}

	sources, err := extractBUBRoadSources(model, options, []string{"line"})
	if err != nil {
		t.Fatalf("extract bub road sources: %v", err)
	}

	source := sources[0]
	if source.SurfaceType != bubroad.SurfaceCobblestone || source.RoadFunctionClass != bubroad.FunctionRuralMain || source.SpeedKPH != 35 {
		t.Fatalf("unexpected BUB road overrides: %#v", source)
	}

	if source.JunctionType != bubroad.JunctionTrafficLight || source.JunctionDistanceM != 25 || source.TemperatureC != 3 || source.StuddedTyreShare != 0.2 {
		t.Fatalf("unexpected BUB road context overrides: %#v", source)
	}

	if source.TrafficDay.MediumVehiclesPerHour != 33 || source.TrafficEvening.PoweredTwoWheelersPerHour != 7 {
		t.Fatalf("unexpected BUB road traffic overrides: %#v", source)
	}
}

func TestExtractCnossosRailSourcesUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rail-1",
        "kind": "source",
        "source_type": "line",
        "rail_traction_type": "diesel",
        "rail_track_type": "slab",
        "rail_track_roughness_class": "rough",
        "rail_average_train_speed_kph": 90,
        "rail_braking_share": 0.35,
        "rail_curve_radius_m": 280,
        "rail_on_bridge": true,
        "traffic_day_trains_per_hour": 11,
        "traffic_evening_trains_per_hour": 7,
        "traffic_night_trains_per_hour": 5
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    }
  ]
}`)

	options := cnossosRailRunOptions{
		TractionType:                cnossosrail.TractionElectric,
		TrackType:                   cnossosrail.TrackTypeBallasted,
		TrackRoughnessClass:         cnossosrail.RoughnessStandard,
		AverageTrainSpeedKPH:        120,
		BrakingShare:                0.1,
		CurveRadiusM:                500,
		OnBridge:                    false,
		TrafficDayTrainsPerHour:     8,
		TrafficEveningTrainsPerHour: 4,
		TrafficNightTrainsPerHour:   3,
	}

	sources, err := extractCnossosRailSources(model, options, []string{"line"})
	if err != nil {
		t.Fatalf("extract cnossos rail sources: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	source := sources[0]
	if source.TractionType != cnossosrail.TractionDiesel || source.TrackType != cnossosrail.TrackTypeSlab || source.TrackRoughnessClass != cnossosrail.RoughnessRough {
		t.Fatalf("unexpected rail overrides: %#v", source)
	}

	if source.AverageTrainSpeedKPH != 90 || source.BrakingShare != 0.35 || source.CurveRadiusM != 280 || !source.OnBridge {
		t.Fatalf("unexpected rail source context overrides: %#v", source)
	}

	if source.TrafficDay.TrainsPerHour != 11 || source.TrafficEvening.TrainsPerHour != 7 || source.TrafficNight.TrainsPerHour != 5 {
		t.Fatalf("unexpected rail traffic overrides: %#v", source)
	}
}

func TestExtractCnossosIndustrySourcesUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "stack-1",
        "kind": "source",
        "source_type": "point",
        "industry_source_category": "stack",
        "industry_enclosure_state": "partial",
        "industry_source_height_m": 18,
        "industry_sound_power_level_db": 101,
        "industry_tonality_correction_db": 2,
        "industry_impulsivity_correction_db": 1,
        "operation_day_factor": 0.8,
        "operation_evening_factor": 0.5,
        "operation_night_factor": 0.2
      },
      "geometry": {"type": "Point", "coordinates": [0,0]}
    }
  ]
}`)

	options := cnossosIndustryRunOptions{
		SourceCategory:          cnossosindustry.CategoryProcess,
		EnclosureState:          cnossosindustry.EnclosureOpen,
		SourceHeightM:           5,
		SoundPowerLevelDB:       96,
		TonalityCorrectionDB:    0,
		ImpulsivityCorrectionDB: 0,
		OperationDayFactor:      1,
		OperationEveningFactor:  0.7,
		OperationNightFactor:    0.4,
	}

	sources, err := extractCnossosIndustrySources(model, options, []string{"point", "area"})
	if err != nil {
		t.Fatalf("extract cnossos industry sources: %v", err)
	}

	source := sources[0]
	if source.SourceCategory != cnossosindustry.CategoryStack || source.EnclosureState != cnossosindustry.EnclosurePartial {
		t.Fatalf("unexpected industry category overrides: %#v", source)
	}

	if source.SourceHeightM != 18 || source.SoundPowerLevelDB != 101 || source.OperationNight.OperatingFactor != 0.2 {
		t.Fatalf("unexpected industry overrides: %#v", source)
	}
}

func TestExtractBEBBuildingsUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "building-1",
        "kind": "building",
        "height_m": 12,
        "building_usage_type": "residential",
        "estimated_dwellings": 6,
        "estimated_persons": 11,
        "floor_count": 4
      },
      "geometry": {
        "type": "Polygon",
        "coordinates": [[[0,0],[20,0],[20,10],[0,10],[0,0]]]
      }
    }
  ]
}`)

	options := bebExposureRunOptions{
		BuildingUsageType:      bebexposure.UsageResidential,
		MinimumBuildingHeightM: 3,
	}

	buildings, err := extractBEBBuildings(model, options)
	if err != nil {
		t.Fatalf("extract beb buildings: %v", err)
	}

	if len(buildings) != 1 {
		t.Fatalf("expected 1 building, got %d", len(buildings))
	}

	building := buildings[0]
	if building.HeightM != 12 {
		t.Fatalf("unexpected building height: %#v", building)
	}

	if building.FloorCount == nil || *building.FloorCount != 4 {
		t.Fatalf("expected floor_count override, got %#v", building)
	}

	if building.EstimatedDwellings == nil || *building.EstimatedDwellings != 6 || building.EstimatedPersons == nil || *building.EstimatedPersons != 11 {
		t.Fatalf("expected occupancy overrides, got %#v", building)
	}
}

func TestExtractAircraftSourcesUseFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "flight-1",
        "kind": "source",
        "source_type": "line",
        "airport_id": "TXL",
        "runway_id": "08",
        "aircraft_operation_type": "arrival",
        "aircraft_class": "cargo",
        "aircraft_procedure_type": "continuous_descent",
        "aircraft_thrust_mode": "idle",
        "reference_power_level_db": 115,
        "engine_state_factor": 1.2,
        "bank_angle_deg": 4,
        "lateral_offset_m": 180,
        "track_start_height_m": 20,
        "track_end_height_m": 220,
        "movement_day_per_hour": 7,
        "movement_evening_per_hour": 3,
        "movement_night_per_hour": 1
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    }
  ]
}`)

	cnossosOptions := cnossosAircraftRunOptions{
		AirportID:              "APT",
		RunwayID:               "RWY",
		OperationType:          cnossosaircraft.OperationDeparture,
		AircraftClass:          cnossosaircraft.AircraftClassNarrow,
		ProcedureType:          cnossosaircraft.ProcedureStandardSID,
		ThrustMode:             cnossosaircraft.ThrustTakeoff,
		ReferencePowerLevelDB:  108,
		EngineStateFactor:      1,
		BankAngleDeg:           0,
		LateralOffsetM:         0,
		TrackStartHeightM:      30,
		TrackEndHeightM:        300,
		MovementDayPerHour:     10,
		MovementEveningPerHour: 5,
		MovementNightPerHour:   2,
	}

	cnossosSources, err := extractCnossosAircraftSources(model, cnossosOptions, []string{"line"})
	if err != nil {
		t.Fatalf("extract cnossos aircraft sources: %v", err)
	}

	cnossosSource := cnossosSources[0]
	if cnossosSource.Airport.AirportID != "TXL" || cnossosSource.OperationType != cnossosaircraft.OperationArrival || cnossosSource.AircraftClass != cnossosaircraft.AircraftClassCargo {
		t.Fatalf("unexpected cnossos aircraft overrides: %#v", cnossosSource)
	}

	if cnossosSource.ProcedureType != cnossosaircraft.ProcedureContinuousDescent || cnossosSource.ThrustMode != cnossosaircraft.ThrustIdle || cnossosSource.LateralOffsetM != 180 {
		t.Fatalf("unexpected cnossos aircraft context overrides: %#v", cnossosSource)
	}

	if cnossosSource.FlightTrack[0].Z != 20 || cnossosSource.FlightTrack[len(cnossosSource.FlightTrack)-1].Z != 220 {
		t.Fatalf("expected per-feature track heights, got %#v", cnossosSource.FlightTrack)
	}

	bufOptions := bufAircraftRunOptions{
		AirportID:              "DE-APT",
		RunwayID:               "RWY",
		OperationType:          bufaircraft.OperationDeparture,
		AircraftClass:          bufaircraft.AircraftClassNarrow,
		ProcedureType:          bufaircraft.ProcedureStandardSID,
		ThrustMode:             bufaircraft.ThrustTakeoff,
		ReferencePowerLevelDB:  110,
		EngineStateFactor:      1,
		BankAngleDeg:           0,
		LateralOffsetM:         0,
		TrackStartHeightM:      20,
		TrackEndHeightM:        250,
		MovementDayPerHour:     12,
		MovementEveningPerHour: 6,
		MovementNightPerHour:   2,
	}

	bufSources, err := extractBUFAircraftSources(model, bufOptions, []string{"line"})
	if err != nil {
		t.Fatalf("extract buf aircraft sources: %v", err)
	}

	bufSource := bufSources[0]
	if bufSource.Airport.RunwayID != "08" || bufSource.ReferencePowerLevelDB != 115 || bufSource.MovementNight.MovementsPerHour != 1 {
		t.Fatalf("unexpected BUF aircraft overrides: %#v", bufSource)
	}

	if bufSource.ProcedureType != bufaircraft.ProcedureContinuousDescent || bufSource.ThrustMode != bufaircraft.ThrustIdle || bufSource.LateralOffsetM != 180 {
		t.Fatalf("unexpected BUF aircraft context overrides: %#v", bufSource)
	}
}

func TestExtractSchall03SourcesUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rail-1",
        "kind": "source",
        "source_type": "line",
        "rail_train_class": "freight",
        "rail_traction_type": "diesel",
        "rail_track_type": "slab",
        "rail_track_form": "switches",
        "rail_track_roughness_class": "rough",
        "rail_average_train_speed_kph": 90,
        "rail_curve_radius_m": 280,
        "rail_on_bridge": true,
        "traffic_day_trains_per_hour": 11,
        "traffic_night_trains_per_hour": 5,
        "elevation_m": 2
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    }
  ]
}`)

	options := schall03RunOptions{
		TrainClass:           schall03.TrainClassMixed,
		TractionType:         schall03.TractionElectric,
		TrackType:            schall03.TrackTypeBallasted,
		TrackForm:            schall03.TrackFormMainline,
		TrackRoughnessClass:  schall03.RoughnessStandard,
		AverageTrainSpeedKPH: 120,
		CurveRadiusM:         500,
		OnBridge:             false,
		TrafficDayTrainsPH:   8,
		TrafficNightTrainsPH: 3,
	}

	sources, err := extractSchall03Sources(model, options, []string{"line"})
	if err != nil {
		t.Fatalf("extract schall03 sources: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	source := sources[0]
	if source.Infrastructure.TractionType != schall03.TractionDiesel || source.Infrastructure.TrackType != schall03.TrackTypeSlab || source.Infrastructure.TrackRoughnessClass != schall03.RoughnessRough {
		t.Fatalf("unexpected source infrastructure: %#v", source.Infrastructure)
	}

	if source.TrainClass != schall03.TrainClassFreight || source.Infrastructure.TrackForm != schall03.TrackFormSwitches {
		t.Fatalf("unexpected source classification: %#v", source)
	}

	if source.AverageSpeedKPH != 90 || source.Infrastructure.CurveRadiusM != 280 || !source.Infrastructure.OnBridge || source.ElevationM != 2 {
		t.Fatalf("unexpected source overrides: %#v", source)
	}

	if source.TrafficDay.TrainsPerHour != 11 || source.TrafficNight.TrainsPerHour != 5 {
		t.Fatalf("unexpected traffic overrides: %#v", source)
	}
}

func TestExtractRLS19RoadSourcesUsesFeatureProperties(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rls19-rd-1",
        "kind": "source",
        "source_type": "line",
        "surface_type": "OPA",
        "speed_pkw_kph": 40,
        "speed_lkw1_kph": 40,
        "speed_lkw2_kph": 40,
        "speed_krad_kph": 40,
        "gradient_percent": 3,
        "junction_type": "signalized",
        "junction_distance_m": 30,
        "building_height_m": 12,
        "street_width_m": 15,
        "traffic_day_pkw": 500,
        "traffic_day_lkw1": 20,
        "traffic_day_lkw2": 30,
        "traffic_day_krad": 5,
        "traffic_night_pkw": 100,
        "traffic_night_lkw1": 5,
        "traffic_night_lkw2": 10,
        "traffic_night_krad": 1
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    },
    {
      "type": "Feature",
      "properties": {
        "id": "rls19-rd-2",
        "kind": "source",
        "source_type": "line"
      },
      "geometry": {"type": "LineString", "coordinates": [[0,50],[100,50]]}
    }
  ]
}`)

	options := rls19RoadRunOptions{
		SurfaceType:      string(rls19road.SurfaceSMA),
		SpeedPkwKPH:      100,
		SpeedLkw1KPH:     100,
		SpeedLkw2KPH:     80,
		SpeedKradKPH:     100,
		GradientPercent:  0,
		TrafficDayPkw:    900,
		TrafficDayLkw1:   40,
		TrafficDayLkw2:   60,
		TrafficDayKrad:   10,
		TrafficNightPkw:  200,
		TrafficNightLkw1: 10,
		TrafficNightLkw2: 20,
		TrafficNightKrad: 2,
		SegmentLengthM:   1,
		MinDistanceM:     3,
	}

	sources, overrideCount, err := extractRLS19RoadSources(model, options, []string{"line"})
	if err != nil {
		t.Fatalf("extract rls19 road sources: %v", err)
	}

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}

	// Only the first feature has per-source acoustic overrides.
	if overrideCount != 1 {
		t.Fatalf("expected 1 source with feature overrides, got %d", overrideCount)
	}

	// Sources must appear in model feature order (deterministic ordering).
	if sources[0].ID != "rls19-rd-1" || sources[1].ID != "rls19-rd-2" {
		t.Fatalf("unexpected source order: %v, %v", sources[0].ID, sources[1].ID)
	}

	s := sources[0]

	// Per-source acoustic overrides must take precedence over run-wide defaults.
	if s.SurfaceType != rls19road.SurfaceOPA {
		t.Fatalf("expected OPA surface type, got %q", s.SurfaceType)
	}

	if s.Speeds.PkwKPH != 40 || s.Speeds.Lkw1KPH != 40 || s.Speeds.Lkw2KPH != 40 || s.Speeds.KradKPH != 40 {
		t.Fatalf("unexpected per-source speed overrides: %+v", s.Speeds)
	}

	if s.GradientPercent != 3 {
		t.Fatalf("expected gradient 3, got %g", s.GradientPercent)
	}

	if s.JunctionType != rls19road.JunctionSignalized || s.JunctionDistanceM != 30 {
		t.Fatalf("unexpected junction overrides: type=%v dist=%g", s.JunctionType, s.JunctionDistanceM)
	}

	if s.BuildingHeightM != 12 || s.StreetWidthM != 15 {
		t.Fatalf("expected building_height_m=12 street_width_m=15, got h=%g w=%g", s.BuildingHeightM, s.StreetWidthM)
	}

	if s.TrafficDay.PkwPerHour != 500 || s.TrafficDay.Lkw1PerHour != 20 || s.TrafficDay.Lkw2PerHour != 30 || s.TrafficDay.KradPerHour != 5 {
		t.Fatalf("unexpected day traffic overrides: %+v", s.TrafficDay)
	}

	if s.TrafficNight.PkwPerHour != 100 || s.TrafficNight.Lkw1PerHour != 5 || s.TrafficNight.Lkw2PerHour != 10 || s.TrafficNight.KradPerHour != 1 {
		t.Fatalf("unexpected night traffic overrides: %+v", s.TrafficNight)
	}

	// Second source uses run-wide defaults.
	s2 := sources[1]
	if s2.SurfaceType != rls19road.SurfaceSMA {
		t.Fatalf("expected SMA (run-wide default) for second source, got %q", s2.SurfaceType)
	}

	if s2.TrafficDay.PkwPerHour != 900 {
		t.Fatalf("expected run-wide day pkw 900 for second source, got %g", s2.TrafficDay.PkwPerHour)
	}
}

func TestExtractRLS19RoadSourcesParsesLaneCount(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rls19-rd-lanes",
        "kind": "source",
        "source_type": "line",
        "lanes": 2
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    }
  ]
}`)

	sources, _, err := extractRLS19RoadSources(model, rls19RoadRunOptions{
		SurfaceType:      string(rls19road.SurfaceSMA),
		SpeedPkwKPH:      100,
		SpeedLkw1KPH:     100,
		SpeedLkw2KPH:     80,
		SpeedKradKPH:     100,
		TrafficDayPkw:    900,
		TrafficDayLkw1:   40,
		TrafficDayLkw2:   60,
		TrafficDayKrad:   10,
		TrafficNightPkw:  200,
		TrafficNightLkw1: 10,
		TrafficNightLkw2: 20,
		TrafficNightKrad: 2,
		SegmentLengthM:   1,
		MinDistanceM:     3,
	}, []string{"line"})
	if err != nil {
		t.Fatalf("extract rls19 road sources: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	if sources[0].LaneCount != 2 {
		t.Fatalf("expected lane_count=2, got %d", sources[0].LaneCount)
	}
}

func mustNormalizeModel(t *testing.T, payload string) modelgeojson.Model {
	t.Helper()

	model, err := modelgeojson.Normalize([]byte(payload), "EPSG:25832", "test.geojson")
	if err != nil {
		t.Fatalf("normalize model: %v", err)
	}

	return model
}
