package cli

import (
	"errors"
	"fmt"
	"math"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// The RLS-19 Parkplatz vocabulary (§3.4). Namespaced like the schall03_*
// properties and, for the same reason, carrying its enumerations as names: both
// Parkplatztyp enumerations are table row identifiers, and a row ordinal moves
// when the table does.
const (
	propRLS19ParkingNumSpaces      = "rls19_parking_num_spaces"
	propRLS19ParkingType           = "rls19_parking_type"
	propRLS19ParkingFacilityType   = "rls19_parking_facility_type"
	propRLS19ParkingMovementsDay   = "rls19_parking_movements_per_space_day"
	propRLS19ParkingMovementsNight = "rls19_parking_movements_per_space_night"
)

const rls19ParkingScope = "cli.extractRLS19ParkingSources"

// extractRLS19ParkingSources extracts RLS-19 §3.4 Parkplatz sources from the
// normalized model. A Parkplatz is an `area` source feature with a single
// Polygon: the polygon supplies both the Stellplatzfläche P and the centroid
// the lot is propagated from, so neither has to be asserted by hand.
//
// Sources are returned in model feature order, so extraction is deterministic
// regardless of worker count. The second return value is every polygon vertex,
// which the receiver grid needs: a lot is an extended footprint, and padding a
// grid around its centroid alone would put the whole grid inside the source.
func extractRLS19ParkingSources(model modelgeojson.Model) ([]rls19road.ParkingSource, []geo.Point2D, error) {
	sources := make([]rls19road.ParkingSource, 0)
	extent := make([]geo.Point2D, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		if strings.ToLower(strings.TrimSpace(feature.SourceType)) != modelgeojson.SourceTypeArea {
			continue
		}

		polygon, err := rls19ParkingPolygon(feature)
		if err != nil {
			return nil, nil, rls19ParkingFeatureError(feature, err)
		}

		source, err := buildRLS19ParkingSource(feature, featureIndex, polygon)
		if err != nil {
			return nil, nil, rls19ParkingFeatureError(feature, err)
		}

		sources = append(sources, source)

		for _, ring := range polygon {
			extent = append(extent, ring...)
		}
	}

	return sources, extent, nil
}

// rls19ParkingPolygon resolves the one polygon a Parkplatz feature must carry.
//
// A MultiPolygon is refused rather than merged or repeated: the number of
// Stellplätze n cannot be split across parts, and duplicating the lot once per
// part would multiply the radiated energy. §3.4 asks for a Parkplatz to be
// divided into Teilflächen, and each Teilfläche is its own feature with its own
// n — which is a modelling decision, not one an importer may take.
func rls19ParkingPolygon(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
	polygons, err := polygonsFromFeature(feature, rls19road.StandardID)
	if err != nil {
		return nil, fmt.Errorf("parking geometry: %w", err)
	}

	if len(polygons) != 1 {
		return nil, fmt.Errorf(
			"a parking source must be a single Polygon, got %d parts; model each Teilfläche (§3.4, Bild 10) as its own feature with its own %s",
			len(polygons), propRLS19ParkingNumSpaces,
		)
	}

	return polygons[0], nil
}

// buildRLS19ParkingSource reads one feature's parking properties.
//
// The reads are straight-line rather than driven by the propertyOverride table
// the neighbouring extractors use: that machinery seeds a field from the run
// options and then lets a property override it, and it carries no "was it
// present" signal. A Parkplatz has no run-wide defaults to seed — deliberately,
// because a run-wide movement rate would reintroduce the omission this
// vocabulary exists to refuse — so each property is required or optional on its
// own terms and must say which when it is missing.
func buildRLS19ParkingSource(
	feature modelgeojson.Feature,
	featureIndex int,
	polygon [][]geo.Point2D,
) (rls19road.ParkingSource, error) {
	center, ok := geo.PolygonCentroid(polygon)
	if !ok {
		return rls19road.ParkingSource{}, errors.New("parking polygon encloses no area, so it has no centroid to propagate from")
	}

	numSpaces, err := rls19ParkingNumSpaces(feature)
	if err != nil {
		return rls19road.ParkingSource{}, err
	}

	lotType, err := rls19ParkingLotType(feature)
	if err != nil {
		return rls19road.ParkingSource{}, err
	}

	day, night, err := rls19ParkingMovementRates(feature)
	if err != nil {
		return rls19road.ParkingSource{}, err
	}

	elevationM, _, err := propertyFloat(feature.Properties, "elevation_m")
	if err != nil {
		return rls19road.ParkingSource{}, err
	}

	source := rls19road.ParkingSource{
		ID:                     rls19ParkingSourceID(feature, featureIndex),
		Center:                 center,
		ElevationM:             elevationM,
		AreaM2:                 geo.PolygonArea(polygon),
		NumSpaces:              numSpaces,
		LotType:                lotType,
		MovementsPerSpaceDay:   day,
		MovementsPerSpaceNight: night,
	}

	err = source.Validate()
	if err != nil {
		return rls19road.ParkingSource{}, fmt.Errorf("parking source is not valid: %w", err)
	}

	return source, nil
}

func rls19ParkingSourceID(feature modelgeojson.Feature, featureIndex int) string {
	id := strings.TrimSpace(feature.ID)
	if id != "" {
		return id
	}

	return fmt.Sprintf("rls19-parking-%03d", featureIndex)
}

func rls19ParkingNumSpaces(feature modelgeojson.Feature) (int, error) {
	value, ok, err := propertyFloat(feature.Properties, propRLS19ParkingNumSpaces)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, fmt.Errorf("property %q is required: the number of Stellplätze n has no default", propRLS19ParkingNumSpaces)
	}

	if value < 1 || math.Trunc(value) != value {
		return 0, fmt.Errorf("property %q must be an integer >= 1", propRLS19ParkingNumSpaces)
	}

	return int(value), nil
}

func rls19ParkingLotType(feature modelgeojson.Feature) (rls19road.ParkingLotType, error) {
	value, ok, err := propertyString(feature.Properties, propRLS19ParkingType)
	if err != nil {
		return rls19road.ParkingLotNotSpecified, err
	}

	if !ok {
		return rls19road.ParkingLotNotSpecified, fmt.Errorf(
			"property %q is required, expected one of %s; it selects the Tabelle 6 row and has no default",
			propRLS19ParkingType, strings.Join(rls19road.ParkingLotTypeNames(), ", "),
		)
	}

	lotType, err := rls19road.ParseParkingLotType(value)
	if err != nil {
		return rls19road.ParkingLotNotSpecified, fmt.Errorf("property %q: %w", propRLS19ParkingType, err)
	}

	return lotType, nil
}

// rls19ParkingMovementRates resolves the two movement rates N.
//
// A stated facility type seeds both from Tabelle 7 and an explicit rate then
// overrides its own period, which is the precedence shape surface_type already
// uses. With no facility type there is nothing to seed from — Tabelle 7 carries
// only P+R and Tank-/Rastanlagen, so an ordinary public car park has no
// standard rate — and both rates are required. §3.4.1 admits the standard
// values only where no project-specific survey exists, so stating the rate is
// the primary case and electing the table is the deliberate exception.
func rls19ParkingMovementRates(feature modelgeojson.Feature) (*float64, *float64, error) {
	day, night, err := rls19ParkingFacilityDefaults(feature)
	if err != nil {
		return nil, nil, err
	}

	day, err = rls19ParkingMovementOverride(feature, propRLS19ParkingMovementsDay, day)
	if err != nil {
		return nil, nil, err
	}

	night, err = rls19ParkingMovementOverride(feature, propRLS19ParkingMovementsNight, night)
	if err != nil {
		return nil, nil, err
	}

	return day, night, nil
}

func rls19ParkingFacilityDefaults(feature modelgeojson.Feature) (*float64, *float64, error) {
	value, ok, err := propertyString(feature.Properties, propRLS19ParkingFacilityType)
	if err != nil {
		return nil, nil, err
	}

	if !ok {
		return nil, nil, nil
	}

	facility, err := rls19road.ParseParkingFacilityType(value)
	if err != nil {
		return nil, nil, fmt.Errorf("property %q: %w", propRLS19ParkingFacilityType, err)
	}

	day, err := rls19road.DefaultMovementsPerHour(facility, rls19road.TimePeriodDay)
	if err != nil {
		return nil, nil, fmt.Errorf("property %q: %w", propRLS19ParkingFacilityType, err)
	}

	night, err := rls19road.DefaultMovementsPerHour(facility, rls19road.TimePeriodNight)
	if err != nil {
		return nil, nil, fmt.Errorf("property %q: %w", propRLS19ParkingFacilityType, err)
	}

	return rls19road.MovementRate(day), rls19road.MovementRate(night), nil
}

func rls19ParkingMovementOverride(feature modelgeojson.Feature, key string, seeded *float64) (*float64, error) {
	value, ok, err := propertyFloat(feature.Properties, key)
	if err != nil {
		return nil, err
	}

	if ok {
		return rls19road.MovementRate(value), nil
	}

	if seeded == nil {
		return nil, fmt.Errorf(
			"property %q is required unless %q states a Tabelle 7 Parkplatztyp; state 0 explicitly for a period with no movements",
			key, propRLS19ParkingFacilityType,
		)
	}

	return seeded, nil
}

func rls19ParkingFeatureError(feature modelgeojson.Feature, err error) error {
	return domainerrors.New(domainerrors.KindValidation, rls19ParkingScope, fmt.Sprintf("feature %q", feature.ID), err)
}
