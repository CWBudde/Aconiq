package iso9613

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the ISO 9613-2 planning-track scaffold.
	StandardID = "iso9613"

	IndicatorLpAeq = "LpAeq"
)

const (
	SourceTypePoint = "point"
)

const (
	MeteorologyDownwind = "downwind-or-moderate-inversion"
)

// PointSource describes the narrow first-scope ISO 9613-2 source contract.
// The initial scaffold intentionally limits the first delivery target to point sources.
type PointSource struct {
	ID                      string      `json:"id"`
	Point                   geo.Point2D `json:"point"`
	SourceHeightM           float64     `json:"source_height_m"`
	SoundPowerLevelDB       float64     `json:"sound_power_level_db"`
	OctaveBandLevels        *BandLevels `json:"octave_band_levels,omitempty"`
	DirectivityCorrectionDB float64     `json:"directivity_correction_db,omitempty"`
	TonalityCorrectionDB    float64     `json:"tonality_correction_db,omitempty"`
	ImpulsivityCorrectionDB float64     `json:"impulsivity_correction_db,omitempty"`
}

// Validate checks one point-source payload.
func (s PointSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("iso9613 point source id is required")
	}

	if !s.Point.IsFinite() {
		return fmt.Errorf("iso9613 point source %q point is not finite", s.ID)
	}

	if math.IsNaN(s.SourceHeightM) || math.IsInf(s.SourceHeightM, 0) || s.SourceHeightM < 0 {
		return fmt.Errorf("iso9613 point source %q source_height_m must be finite and >= 0", s.ID)
	}

	if math.IsNaN(s.SoundPowerLevelDB) || math.IsInf(s.SoundPowerLevelDB, 0) {
		return fmt.Errorf("iso9613 point source %q sound_power_level_db must be finite", s.ID)
	}

	for name, value := range map[string]float64{
		"directivity_correction_db": s.DirectivityCorrectionDB,
		"tonality_correction_db":    s.TonalityCorrectionDB,
		"impulsivity_correction_db": s.ImpulsivityCorrectionDB,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("iso9613 point source %q %s must be finite", s.ID, name)
		}
	}

	return nil
}

// Receiver describes one ISO 9613-2 receiver input.
type Receiver struct {
	ID      string      `json:"id"`
	Point   geo.Point2D `json:"point"`
	HeightM float64     `json:"height_m"`
}

// Validate checks one receiver payload.
func (r Receiver) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("iso9613 receiver id is required")
	}

	if !r.Point.IsFinite() {
		return fmt.Errorf("iso9613 receiver %q point is not finite", r.ID)
	}

	if math.IsNaN(r.HeightM) || math.IsInf(r.HeightM, 0) || r.HeightM < 0 {
		return fmt.Errorf("iso9613 receiver %q height_m must be finite and >= 0", r.ID)
	}

	return nil
}

// GroundZone records a future ISO 9613-2 ground-category area using a normalized ground factor.
type GroundZone struct {
	ID           string          `json:"id"`
	Polygon      [][]geo.Point2D `json:"polygon"`
	GroundFactor float64         `json:"ground_factor"`
}

// Validate checks one ground-zone payload.
func (g GroundZone) Validate() error {
	if strings.TrimSpace(g.ID) == "" {
		return errors.New("iso9613 ground zone id is required")
	}

	if len(g.Polygon) == 0 {
		return fmt.Errorf("iso9613 ground zone %q polygon must contain at least one ring", g.ID)
	}

	for ringIndex, ring := range g.Polygon {
		if len(ring) < 4 {
			return fmt.Errorf("iso9613 ground zone %q polygon ring[%d] must contain at least 4 points", g.ID, ringIndex)
		}

		for pointIndex, point := range ring {
			if !point.IsFinite() {
				return fmt.Errorf("iso9613 ground zone %q polygon ring[%d] point[%d] is not finite", g.ID, ringIndex, pointIndex)
			}
		}
	}

	if math.IsNaN(g.GroundFactor) || math.IsInf(g.GroundFactor, 0) || g.GroundFactor < 0 || g.GroundFactor > 1 {
		return fmt.Errorf("iso9613 ground zone %q ground_factor must be finite and within [0,1]", g.ID)
	}

	return nil
}

// Meteorology captures the narrow favorable-propagation assumption used by ISO 9613-2.
type Meteorology struct {
	Assumption              string  `json:"assumption"`
	TemperatureC            float64 `json:"temperature_c"`
	RelativeHumidityPercent float64 `json:"relative_humidity_percent"`
}

// Validate checks the meteorological assumptions payload.
func (m Meteorology) Validate() error {
	if strings.TrimSpace(m.Assumption) != MeteorologyDownwind {
		return fmt.Errorf("iso9613 meteorology assumption must be %q", MeteorologyDownwind)
	}

	if math.IsNaN(m.TemperatureC) || math.IsInf(m.TemperatureC, 0) {
		return errors.New("iso9613 meteorology temperature_c must be finite")
	}

	if math.IsNaN(m.RelativeHumidityPercent) || math.IsInf(m.RelativeHumidityPercent, 0) || m.RelativeHumidityPercent < 0 || m.RelativeHumidityPercent > 100 {
		return errors.New("iso9613 meteorology relative_humidity_percent must be finite and within [0,100]")
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for the initial ISO 9613-2 scaffold.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	maxGroundFactor := 1.0
	maxHumidity := 100.0

	return framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "ISO 9613-2 industry scaffold for point-source planning runs; metadata and compliance boundary are defined, compute chain remains pending.",
		DefaultVersion: "1996-scaffold",
		Versions: []framework.Version{
			{
				Name:           "1996-scaffold",
				DefaultProfile: "point-source",
				Profiles: []framework.Profile{
					{
						Name:                 "point-source",
						SupportedSourceTypes: []string{SourceTypePoint},
						SupportedIndicators:  []string{IndicatorLpAeq},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{Name: "iso9613_source_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minZero, Description: "Default source height for imported ISO 9613-2 point sources"},
								{Name: "iso9613_sound_power_level_db", Kind: framework.ParameterKindFloat, DefaultValue: "100", Description: "Reference sound power level for imported ISO 9613-2 point sources"},
								{Name: "iso9613_directivity_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Directivity correction applied at the source"},
								{Name: "iso9613_tonality_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Tonality correction applied at the reporting boundary"},
								{Name: "iso9613_impulsivity_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Impulsivity correction applied at the reporting boundary"},
								{Name: "ground_factor", Kind: framework.ParameterKindFloat, DefaultValue: "0.5", Min: &minZero, Max: &maxGroundFactor, Description: "Normalized ground factor G for the initial homogeneous-ground scaffold"},
								{Name: "air_temperature_c", Kind: framework.ParameterKindFloat, DefaultValue: "10", Description: "Air temperature used for atmospheric absorption inputs"},
								{Name: "relative_humidity_percent", Kind: framework.ParameterKindFloat, DefaultValue: "70", Min: &minZero, Max: &maxHumidity, Description: "Relative humidity used for atmospheric absorption inputs"},
								{Name: "meteorology_assumption", Kind: framework.ParameterKindString, DefaultValue: MeteorologyDownwind, Enum: []string{MeteorologyDownwind}, Description: "Favorable propagation assumption for ISO 9613-2 engineering calculations"},
								{Name: "barrier_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Placeholder attenuation term until the explicit ISO barrier chain is implemented"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "1", Min: &minPositive, Description: "Minimum source-receiver distance for stable propagation calculations"},
							},
						},
					},
				},
			},
		},
	}
}
