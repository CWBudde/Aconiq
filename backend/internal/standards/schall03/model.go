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
	TrainClassPassenger = "passenger"
	TrainClassFreight   = "freight"
	TrainClassMixed     = "mixed"
)

const (
	TrackTypeBallasted = "ballasted"
	TrackTypeSlab      = "slab"
)

const (
	TrackFormMainline = "mainline"
	TrackFormStation  = "station"
	TrackFormSwitches = "switches"
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

var allowedTrainClasses = map[string]struct{}{
	TrainClassPassenger: {},
	TrainClassFreight:   {},
	TrainClassMixed:     {},
}

var allowedTrackTypes = map[string]struct{}{
	TrackTypeBallasted: {},
	TrackTypeSlab:      {},
}

var allowedTrackForms = map[string]struct{}{
	TrackFormMainline: {},
	TrackFormStation:  {},
	TrackFormSwitches: {},
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
	TrackForm           string  `json:"track_form"`
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

	if _, ok := allowedTrackForms[strings.TrimSpace(i.TrackForm)]; !ok {
		return fmt.Errorf("rail source %q has unsupported track_form %q", sourceID, i.TrackForm)
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
	TrainClass      string             `json:"train_class"`
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

	if _, ok := allowedTrainClasses[strings.TrimSpace(s.TrainClass)]; !ok {
		return fmt.Errorf("rail source %q has unsupported train_class %q", s.ID, s.TrainClass)
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

// TrainOperation describes one train type operating on a track segment.
type TrainOperation struct {
	TrainType          string    // Zugart name (e.g. "ICE-1-Zug") or "custom"
	FzComposition      []FzCount // vehicle category composition
	SpeedKPH           float64   // operating speed in km/h
	TrainsPerHourDay   float64   // trains per hour, day period
	TrainsPerHourNight float64   // trains per hour, night period
}

// NewTrainOperationFromZugart creates a TrainOperation from a Zugart name
// (Table 4 in Beiblatt 1) with day/night trains per hour.
// It looks up the Zugart in the Zugarten slice from beiblatt1.go,
// decomposes it into FzComposition, and sets the default speed from the Zugart.
func NewTrainOperationFromZugart(zugartName string, trainsPerHourDay, trainsPerHourNight float64) (*TrainOperation, error) {
	for _, z := range Zugarten {
		if z.Name == zugartName {
			comp := make([]FzCount, len(z.Composition))
			copy(comp, z.Composition)

			return &TrainOperation{
				TrainType:          z.Name,
				FzComposition:      comp,
				SpeedKPH:           z.MaxSpeedKPH,
				TrainsPerHourDay:   trainsPerHourDay,
				TrainsPerHourNight: trainsPerHourNight,
			}, nil
		}
	}

	return nil, fmt.Errorf("unknown Zugart %q", zugartName)
}

// Validate checks a TrainOperation for consistency.
func (op TrainOperation) Validate() error {
	if math.IsNaN(op.SpeedKPH) || math.IsInf(op.SpeedKPH, 0) || op.SpeedKPH <= 0 {
		return errors.New("TrainOperation: SpeedKPH must be finite and > 0")
	}

	if len(op.FzComposition) == 0 {
		return errors.New("TrainOperation: at least one FzCount entry required")
	}

	for i, fc := range op.FzComposition {
		if fc.Fz < 1 || fc.Fz > 10 {
			return fmt.Errorf("TrainOperation: FzComposition[%d].Fz=%d is out of range 1-10", i, fc.Fz)
		}

		if fc.Count < 0 {
			return fmt.Errorf("TrainOperation: FzComposition[%d].Count must be >= 0", i)
		}
	}

	if math.IsNaN(op.TrainsPerHourDay) || math.IsInf(op.TrainsPerHourDay, 0) || op.TrainsPerHourDay < 0 {
		return errors.New("TrainOperation: TrainsPerHourDay must be finite and >= 0")
	}

	if math.IsNaN(op.TrainsPerHourNight) || math.IsInf(op.TrainsPerHourNight, 0) || op.TrainsPerHourNight < 0 {
		return errors.New("TrainOperation: TrainsPerHourNight must be finite and >= 0")
	}

	return nil
}

// resolveEffectiveSpeed determines the effective speed per Nr. 4.3:
//   - v = min(streckeMax, fahrzeugMax)
//   - v >= 50 km/h (minimum for free-field Eisenbahn Strecke)
//   - v >= 70 km/h if isStation (Haltestelle minimum)
func resolveEffectiveSpeed(streckeMax, fahrzeugMax float64, isStation bool) float64 {
	v := math.Min(streckeMax, fahrzeugMax)

	minSpeed := 50.0
	if isStation {
		minSpeed = 70.0
	}

	return math.Max(v, minSpeed)
}

// TrackSegment describes one normative track segment for emission computation.
type TrackSegment struct {
	ID              string
	TrackCenterline []geo.Point2D
	ElevationM      float64
	Fahrbahn        FahrbahnartType // from tables.go constants
	Surface         SurfaceCondType // from emission_v2.go constants
	BridgeType      int             // 0=none, 1-4 per Table 9
	BridgeMitig     bool            // K_LM noise reduction measures
	CurveRadiusM    float64         // 0 = straight
	IsStation       bool            // for speed min 70 km/h rule
	StreckeMaxKPH   float64         // track speed limit
	Operations      []TrainOperation
}

// Validate checks a TrackSegment for consistency.
func (seg TrackSegment) Validate() error {
	if strings.TrimSpace(seg.ID) == "" {
		return errors.New("TrackSegment: ID is required")
	}

	err := seg.validateGeometry()
	if err != nil {
		return err
	}

	err = seg.validateInfrastructure()
	if err != nil {
		return err
	}

	return seg.validateOperations()
}

func (seg TrackSegment) validateGeometry() error {
	if len(seg.TrackCenterline) < 2 {
		return fmt.Errorf("TrackSegment %q: track_centerline must contain at least 2 points", seg.ID)
	}

	for i, pt := range seg.TrackCenterline {
		if !pt.IsFinite() {
			return fmt.Errorf("TrackSegment %q: track_centerline point[%d] is not finite", seg.ID, i)
		}
	}

	if math.IsNaN(seg.ElevationM) || math.IsInf(seg.ElevationM, 0) {
		return fmt.Errorf("TrackSegment %q: ElevationM must be finite", seg.ID)
	}

	return nil
}

func (seg TrackSegment) validateInfrastructure() error {
	if seg.BridgeType < 0 || seg.BridgeType > 4 {
		return fmt.Errorf("TrackSegment %q: BridgeType must be 0-4", seg.ID)
	}

	if math.IsNaN(seg.CurveRadiusM) || math.IsInf(seg.CurveRadiusM, 0) || seg.CurveRadiusM < 0 {
		return fmt.Errorf("TrackSegment %q: CurveRadiusM must be finite and >= 0", seg.ID)
	}

	if math.IsNaN(seg.StreckeMaxKPH) || math.IsInf(seg.StreckeMaxKPH, 0) || seg.StreckeMaxKPH <= 0 {
		return fmt.Errorf("TrackSegment %q: StreckeMaxKPH must be finite and > 0", seg.ID)
	}

	return nil
}

func (seg TrackSegment) validateOperations() error {
	if len(seg.Operations) == 0 {
		return fmt.Errorf("TrackSegment %q: at least one TrainOperation required", seg.ID)
	}

	for i, op := range seg.Operations {
		err := op.Validate()
		if err != nil {
			return fmt.Errorf("TrackSegment %q: Operations[%d]: %w", seg.ID, i, err)
		}
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
// planning-track baseline with preview coefficients routed through a data-pack
// shaped boundary.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "Schall 03 planning-track rail baseline with typed inputs, octave-band handling, deterministic line integration, explicit compliance-boundary metadata, and a data-pack shaped coefficient boundary.",
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
								{Name: "rail_train_class", Kind: framework.ParameterKindString, DefaultValue: TrainClassMixed, Enum: []string{TrainClassPassenger, TrainClassFreight, TrainClassMixed}, Description: "Default train class placeholder for Schall 03 source mapping"},
								{Name: "rail_traction_type", Kind: framework.ParameterKindString, DefaultValue: TractionElectric, Enum: []string{TractionElectric, TractionDiesel, TractionMixed}, Description: "Default traction type for imported Schall 03 rail sources"},
								{Name: "rail_track_type", Kind: framework.ParameterKindString, DefaultValue: TrackTypeBallasted, Enum: []string{TrackTypeBallasted, TrackTypeSlab}, Description: "Default track construction type for imported rail sources"},
								{Name: "rail_track_form", Kind: framework.ParameterKindString, DefaultValue: TrackFormMainline, Enum: []string{TrackFormMainline, TrackFormStation, TrackFormSwitches}, Description: "Default track-form placeholder for future Schall 03 source mapping"},
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
