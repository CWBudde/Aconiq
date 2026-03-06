package schall03

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the Schall 03 planning-track module.
	StandardID = "schall03"

	IndicatorLrDay   = "LrDay"
	IndicatorLrNight = "LrNight"
)

const (
	TractionElectric = "electric"
	TractionDiesel   = "diesel"
	TractionMixed    = "mixed"
)

const (
	TrackTypeBallasted = "ballasted"
	TrackTypeSlab      = "slab"
)

const (
	RoughnessStandard = "standard"
	RoughnessLowNoise = "low-noise"
	RoughnessRough    = "rough"
)

var allowedTractionTypes = map[string]struct{}{
	TractionElectric: {},
	TractionDiesel:   {},
	TractionMixed:    {},
}

var allowedTrackTypes = map[string]struct{}{
	TrackTypeBallasted: {},
	TrackTypeSlab:      {},
}

var allowedRoughnessClasses = map[string]struct{}{
	RoughnessStandard: {},
	RoughnessLowNoise: {},
	RoughnessRough:    {},
}

// OctaveBand is one Schall 03 octave-band center frequency.
type OctaveBand int

const (
	OctaveBand63Hz   OctaveBand = 63
	OctaveBand125Hz  OctaveBand = 125
	OctaveBand250Hz  OctaveBand = 250
	OctaveBand500Hz  OctaveBand = 500
	OctaveBand1000Hz OctaveBand = 1000
	OctaveBand2000Hz OctaveBand = 2000
	OctaveBand4000Hz OctaveBand = 4000
	OctaveBand8000Hz OctaveBand = 8000
)

var octaveBandOrder = [...]OctaveBand{
	OctaveBand63Hz,
	OctaveBand125Hz,
	OctaveBand250Hz,
	OctaveBand500Hz,
	OctaveBand1000Hz,
	OctaveBand2000Hz,
	OctaveBand4000Hz,
	OctaveBand8000Hz,
}

// OctaveBands returns the canonical Schall 03 octave-band order.
func OctaveBands() []OctaveBand {
	return append([]OctaveBand(nil), octaveBandOrder[:]...)
}

// OctaveSpectrum stores one level per Schall 03 octave band in canonical order.
type OctaveSpectrum [8]float64

// Validate checks all octave-band levels for finite values.
func (s OctaveSpectrum) Validate(name string) error {
	for i, level := range s {
		if math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("%s octave band %d Hz must be finite", name, octaveBandOrder[i])
		}
	}

	return nil
}

// EnergeticTotal returns the energetic sum across all octave bands.
func (s OctaveSpectrum) EnergeticTotal() float64 {
	return EnergeticSumLevels(s[:]...)
}

// EnergeticSumLevels adds dB levels in deterministic input order.
func EnergeticSumLevels(levels ...float64) float64 {
	if len(levels) == 0 {
		return math.Inf(-1)
	}

	sum := 0.0
	hasFinite := false

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 1) {
			return math.NaN()
		}

		if math.IsInf(level, -1) {
			continue
		}

		sum += math.Pow(10, level/10)
		hasFinite = true
	}

	if !hasFinite || sum <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(sum)
}

// SumSpectra sums multiple spectra band-by-band using canonical band order.
func SumSpectra(spectra []OctaveSpectrum) OctaveSpectrum {
	var out OctaveSpectrum
	for bandIdx := range out {
		levels := make([]float64, 0, len(spectra))
		for _, spectrum := range spectra {
			levels = append(levels, spectrum[bandIdx])
		}

		out[bandIdx] = EnergeticSumLevels(levels...)
	}

	return out
}

// TrafficPeriod stores train count information for one planning period.
type TrafficPeriod struct {
	TrainsPerHour float64 `json:"trains_per_hour"`
}

// RailInfrastructure collects source metadata that later maps into the
// Schall 03 emission and propagation chain without embedding normative tables.
type RailInfrastructure struct {
	TractionType        string  `json:"traction_type"`
	TrackType           string  `json:"track_type"`
	TrackRoughnessClass string  `json:"track_roughness_class"`
	OnBridge            bool    `json:"on_bridge,omitempty"`
	CurveRadiusM        float64 `json:"curve_radius_m,omitempty"`
}

// Validate checks one infrastructure payload.
func (i RailInfrastructure) Validate(sourceID string) error {
	if _, ok := allowedTractionTypes[strings.TrimSpace(i.TractionType)]; !ok {
		return fmt.Errorf("rail source %q has unsupported traction_type %q", sourceID, i.TractionType)
	}

	if _, ok := allowedTrackTypes[strings.TrimSpace(i.TrackType)]; !ok {
		return fmt.Errorf("rail source %q has unsupported track_type %q", sourceID, i.TrackType)
	}

	if _, ok := allowedRoughnessClasses[strings.TrimSpace(i.TrackRoughnessClass)]; !ok {
		return fmt.Errorf("rail source %q has unsupported track_roughness_class %q", sourceID, i.TrackRoughnessClass)
	}

	if math.IsNaN(i.CurveRadiusM) || math.IsInf(i.CurveRadiusM, 0) || i.CurveRadiusM < 0 {
		return fmt.Errorf("rail source %q curve_radius_m must be finite and >= 0", sourceID)
	}

	return nil
}

// RailSource describes one Schall 03 rail source segment.
type RailSource struct {
	ID              string             `json:"id"`
	TrackCenterline []geo.Point2D      `json:"track_centerline"`
	ElevationM      float64            `json:"elevation_m,omitempty"`
	AverageSpeedKPH float64            `json:"average_speed_kph"`
	Infrastructure  RailInfrastructure `json:"infrastructure"`
	TrafficDay      TrafficPeriod      `json:"traffic_day"`
	TrafficNight    TrafficPeriod      `json:"traffic_night"`
}

// Validate checks one Schall 03 rail source payload.
func (s RailSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("rail source id is required")
	}

	if len(s.TrackCenterline) < 2 {
		return fmt.Errorf("rail source %q track_centerline must contain at least 2 points", s.ID)
	}

	for i, point := range s.TrackCenterline {
		if !point.IsFinite() {
			return fmt.Errorf("rail source %q track_centerline point[%d] is not finite", s.ID, i)
		}
	}

	if math.IsNaN(s.ElevationM) || math.IsInf(s.ElevationM, 0) {
		return fmt.Errorf("rail source %q elevation_m must be finite", s.ID)
	}

	if math.IsNaN(s.AverageSpeedKPH) || math.IsInf(s.AverageSpeedKPH, 0) || s.AverageSpeedKPH <= 0 {
		return fmt.Errorf("rail source %q average_speed_kph must be finite and > 0", s.ID)
	}

	err := s.Infrastructure.Validate(s.ID)
	if err != nil {
		return err
	}

	err = validateTrafficPeriod(s.ID, "day", s.TrafficDay)
	if err != nil {
		return err
	}

	err = validateTrafficPeriod(s.ID, "night", s.TrafficNight)
	if err != nil {
		return err
	}

	return nil
}

func sourceSegmentLengthM(centerline []geo.Point2D) float64 {
	total := 0.0
	for i := range len(centerline) - 1 {
		total += geo.Distance(centerline[i], centerline[i+1])
	}

	return total
}

func validateTrafficPeriod(sourceID string, period string, traffic TrafficPeriod) error {
	if math.IsNaN(traffic.TrainsPerHour) || math.IsInf(traffic.TrainsPerHour, 0) || traffic.TrainsPerHour < 0 {
		return fmt.Errorf("rail source %q traffic_%s trains_per_hour must be finite and >= 0", sourceID, period)
	}

	return nil
}

// ReceiverInput describes one planning receiver location.
type ReceiverInput struct {
	ID      string      `json:"id"`
	Point   geo.Point2D `json:"point"`
	HeightM float64     `json:"height_m"`
}

// Validate checks one receiver payload.
func (r ReceiverInput) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("receiver id is required")
	}

	if !r.Point.IsFinite() {
		return fmt.Errorf("receiver %q point must be finite", r.ID)
	}

	if math.IsNaN(r.HeightM) || math.IsInf(r.HeightM, 0) || r.HeightM < 0 {
		return fmt.Errorf("receiver %q height_m must be finite and >= 0", r.ID)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for the Schall 03
// planning-track baseline preview.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "Schall 03 planning-track rail baseline preview with typed inputs, octave-band handling, deterministic line integration, and explicit compliance-boundary metadata.",
		DefaultVersion: "phase18-baseline-preview",
		Versions: []framework.Version{
			{
				Name:           "phase18-baseline-preview",
				DefaultProfile: "rail-planning-preview",
				Profiles: []framework.Profile{
					{
						Name:                 "rail-planning-preview",
						SupportedSourceTypes: []string{"line"},
						SupportedIndicators:  []string{IndicatorLrDay, IndicatorLrNight},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters for the future Schall 03 run/export path"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{Name: "rail_traction_type", Kind: framework.ParameterKindString, DefaultValue: TractionElectric, Enum: []string{TractionElectric, TractionDiesel, TractionMixed}, Description: "Default traction type for imported Schall 03 rail sources"},
								{Name: "rail_track_type", Kind: framework.ParameterKindString, DefaultValue: TrackTypeBallasted, Enum: []string{TrackTypeBallasted, TrackTypeSlab}, Description: "Default track construction type for imported rail sources"},
								{Name: "rail_track_roughness_class", Kind: framework.ParameterKindString, DefaultValue: RoughnessStandard, Enum: []string{RoughnessStandard, RoughnessLowNoise, RoughnessRough}, Description: "Default roughness class for imported rail sources"},
								{Name: "rail_average_train_speed_kph", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minPositive, Description: "Default train speed for imported rail sources"},
								{Name: "rail_curve_radius_m", Kind: framework.ParameterKindFloat, DefaultValue: "500", Min: &minZero, Description: "Default curve radius for imported rail sources"},
								{Name: "rail_on_bridge", Kind: framework.ParameterKindBool, DefaultValue: "false", Description: "Default bridge flag for imported rail sources"},
								{Name: "traffic_day_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "8", Min: &minZero, Description: "Default day trains per hour for imported rail sources"},
								{Name: "traffic_night_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Default night trains per hour for imported rail sources"},
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Baseline air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.2", Min: &minZero, Description: "Baseline ground attenuation term"},
								{Name: "slab_track_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Additional correction for slab track sections"},
								{Name: "bridge_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "2", Min: &minZero, Description: "Additional correction for bridge sections"},
								{Name: "curve_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Maximum correction for tight-curve sections"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum source-receiver distance for the future propagation chain"},
							},
						},
					},
				},
			},
		},
	}
}
