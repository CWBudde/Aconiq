package road

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// ParkingVehicleType classifies the vehicle category for a parking facility.
// Determines the D_{P,PT} surcharge from RLS-19 Tabelle 6.
type ParkingVehicleType int

const (
	ParkingPkw        ParkingVehicleType = iota // Pkw:          D_{P,PT} = 0 dB
	ParkingMotorrad                             // Motorrad:     D_{P,PT} = 5 dB
	ParkingLkwOmnibus                           // Lkw/Omnibus:  D_{P,PT} = 10 dB
)

// ParkingFacilityType identifies a parking facility category used to look up
// standard hourly movement rates from RLS-19 Tabelle 7.
type ParkingFacilityType int

const (
	ParkingFacilityPR       ParkingFacilityType = iota // Park-and-Ride
	ParkingFacilityTankRast                            // Tankstelle/Rastanlage
)

// TimePeriod selects day or night assessment period.
type TimePeriod int

const (
	TimePeriodDay   TimePeriod = iota // Tag (06–22 Uhr)
	TimePeriodNight                   // Nacht (22–06 Uhr)
)

// ParkingSource describes an RLS-19 parking-area source (Parkplatz, §3.4).
// The parking lot is approximated as a point source located at Center.
type ParkingSource struct {
	ID string `json:"id"`

	// Center is the centroid of the parking area in plan view.
	// Used as the point-source location for propagation.
	Center geo.Point2D `json:"center"`

	// ElevationM is the absolute Z of the parking surface.
	ElevationM float64 `json:"elevation_m,omitempty"`

	// AreaM2 is the total parking area P [m²] (Stellplatzfläche).
	AreaM2 float64 `json:"area_m2"`

	// NumSpaces is the number of parking spaces n (Anzahl der Stellplätze).
	NumSpaces int `json:"num_spaces"`

	// VehicleType determines the D_{P,PT} surcharge (Tabelle 6).
	VehicleType ParkingVehicleType `json:"vehicle_type,omitempty"`

	// MovementsPerSpaceDay/Night are the hourly movement rates N per space [h⁻¹].
	// Use DefaultMovementsPerHour to obtain Tabelle 7 standard values.
	MovementsPerSpaceDay   float64 `json:"movements_per_space_day"`
	MovementsPerSpaceNight float64 `json:"movements_per_space_night"`
}

// Validate checks a parking source definition.
func (s ParkingSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("parking source id is required")
	}

	if !s.Center.IsFinite() {
		return fmt.Errorf("parking source %q center must be finite", s.ID)
	}

	if !isFinite(s.ElevationM) {
		return fmt.Errorf("parking source %q elevation_m must be finite", s.ID)
	}

	if !isFinite(s.AreaM2) || s.AreaM2 <= 0 {
		return fmt.Errorf("parking source %q area_m2 must be finite and > 0", s.ID)
	}

	if s.NumSpaces <= 0 {
		return fmt.Errorf("parking source %q num_spaces must be > 0", s.ID)
	}

	if !isFinite(s.MovementsPerSpaceDay) || s.MovementsPerSpaceDay < 0 {
		return fmt.Errorf("parking source %q movements_per_space_day must be finite and >= 0", s.ID)
	}

	if !isFinite(s.MovementsPerSpaceNight) || s.MovementsPerSpaceNight < 0 {
		return fmt.Errorf("parking source %q movements_per_space_night must be finite and >= 0", s.ID)
	}

	return nil
}

// ParkingEmissionResult holds the total sound power level L_W per time period.
// This is the point-source total power (not the area-related L_W” from Eq. 10);
// it equals L_W” + 10·lg(P/1m²) = 63 + 10·lg(N·n) + D_{P,PT}.
type ParkingEmissionResult struct {
	LWDay   float64 // total sound power level, day [dB(A)]
	LWNight float64 // total sound power level, night [dB(A)]
}

// ComputeParkingEmission computes total sound power levels per RLS-19 Eq. 10:
//
//	L_W = 63 + 10·lg[N·n] + D_{P,PT}
//
// where N is movements per space per hour, n is number of spaces, and D_{P,PT}
// is the vehicle-type surcharge from Tabelle 6.
// Zero movement rate produces a sentinel level of −999 dB (silence).
func ComputeParkingEmission(source ParkingSource) (ParkingEmissionResult, error) {
	err := source.Validate()
	if err != nil {
		return ParkingEmissionResult{}, err
	}

	dPT := parkingVehicleTypeSurcharge(source.VehicleType)
	n := float64(source.NumSpaces)

	lw := func(movPerSpace float64) float64 {
		if movPerSpace <= 0 {
			return -999.0
		}

		return 63 + 10*math.Log10(movPerSpace*n) + dPT
	}

	return ParkingEmissionResult{
		LWDay:   lw(source.MovementsPerSpaceDay),
		LWNight: lw(source.MovementsPerSpaceNight),
	}, nil
}

// parkingVehicleTypeSurcharge returns the D_{P,PT} correction from Tabelle 6.
func parkingVehicleTypeSurcharge(vt ParkingVehicleType) float64 {
	switch vt {
	case ParkingMotorrad:
		return 5.0
	case ParkingLkwOmnibus:
		return 10.0
	default: // ParkingPkw
		return 0.0
	}
}

// DefaultMovementsPerHour returns the Tabelle 7 standard hourly movement rate N
// for a given parking facility type and assessment period.
func DefaultMovementsPerHour(ft ParkingFacilityType, period TimePeriod) float64 {
	switch ft {
	case ParkingFacilityPR:
		if period == TimePeriodNight {
			return 0.06
		}

		return 0.3
	case ParkingFacilityTankRast:
		if period == TimePeriodNight {
			return 0.8
		}

		return 1.5
	default:
		return 0
	}
}

// appendParkingContributions adds point-source contributions from all parking
// sources to the day/night level accumulation slices.
//
// Each parking source is treated as a single point source at its Center,
// propagated via the standard D_div + D_atm + D_gr attenuation chain.
// Source height is 0.5 m above the parking surface (same as road sources).
func appendParkingContributions(
	dayContrib, nightContrib *[]float64,
	parkingSources []ParkingSource,
	receiver geo.Point2D,
	receiverZ float64,
	cfg PropagationConfig,
) error {
	const sourceHeightM = 0.5

	for _, parking := range parkingSources {
		emission, err := ComputeParkingEmission(parking)
		if err != nil {
			return err
		}

		sourceZ := parking.ElevationM + sourceHeightM

		planDist := dist2D(parking.Center, receiver)
		dz := receiverZ - sourceZ
		slantDist := math.Sqrt(planDist*planDist + dz*dz)

		hm := computeMeanHeight(parking.Center, receiver, sourceZ, receiverZ, cfg.Terrain)
		att := computeAttenuation(planDist, slantDist, hm, cfg)

		*dayContrib = append(*dayContrib, emission.LWDay-att.Total)
		*nightContrib = append(*nightContrib, emission.LWNight-att.Total)
	}

	return nil
}
