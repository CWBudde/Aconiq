package schall03

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/numeric"
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
//
// The reduction is compensated (numeric.CompensatedSum) because the caller's
// term count is not bounded: lineSourceSpectrumAtReceiver feeds it one level
// per integration subsegment, so a long line source produces thousands of terms
// spanning many orders of magnitude.
//
// It is deliberately not acoustics.EnergySum. This module works in -Inf for
// silence internally and converts to the -999 sentinel only at its own output
// boundary (finiteOrSilence in compute.go), and it returns NaN on a +Inf term
// rather than skipping it, so that an impossible level cannot be silently
// dropped mid-spectrum. Adopting the shared contract would change both
// behaviours and so would change numbers; that is a decision to take on its own
// — see PLAN.md Priority 7.
func EnergeticSumLevels(levels ...float64) float64 {
	if len(levels) == 0 {
		return math.Inf(-1)
	}

	var sum numeric.CompensatedSum

	hasFinite := false

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 1) {
			return math.NaN()
		}

		if math.IsInf(level, -1) {
			continue
		}

		sum.Add(math.Pow(10, level/10))

		hasFinite = true
	}

	if !hasFinite || sum.Sum() <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(sum.Sum())
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
	var total numeric.CompensatedSum

	for i := range len(centerline) - 1 {
		total.Add(geo.Distance(centerline[i], centerline[i+1]))
	}

	return total.Sum()
}

func validateTrafficPeriod(sourceID string, period string, traffic TrafficPeriod) error {
	if math.IsNaN(traffic.TrainsPerHour) || math.IsInf(traffic.TrainsPerHour, 0) || traffic.TrainsPerHour < 0 {
		return fmt.Errorf("rail source %q traffic_%s trains_per_hour must be finite and >= 0", sourceID, period)
	}

	return nil
}

// TrainOperation describes one train type operating on a track segment.
type TrainOperation struct {
	TrainType          string    `json:"train_type"`            // Zugart name or "custom"
	FzComposition      []FzCount `json:"fz_composition"`        // vehicle category composition
	SpeedKPH           float64   `json:"speed_kph"`             // operating speed in km/h
	TrainsPerHourDay   float64   `json:"trains_per_hour_day"`   // trains per hour, day period
	TrainsPerHourNight float64   `json:"trains_per_hour_night"` // trains per hour, night period
}

// NewTrainOperationFromZugart creates a TrainOperation from a Zugart name.
// It searches first the Eisenbahn Zugarten (Table 4 in Beiblatt 1) and then
// the Straßenbahn Zugarten (Beiblatt 2), decomposes the entry into
// FzComposition, and sets the default speed from the Zugart.
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

	for _, z := range ZugartStrassenbahn {
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

	fzMap := buildFzMap()

	for i, fc := range op.FzComposition {
		if _, ok := fzMap[fc.Fz]; !ok {
			return fmt.Errorf("TrainOperation: FzComposition[%d].Fz=%d is not a valid Fz-Kategorie", i, fc.Fz)
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

// resolveEffectiveSpeed determines the effective speed v_Fz.
//
// Nr. 4.3 (Eisenbahnen): the starting point is the vehicle-borne maximum speed;
// where the permitted track speed is lower, that one is used.  Inside
// Personenbahnhöfen (innerhalb der Einfahrsignale) and at Haltepunkten bzw.
// Haltestellen (Bahnsteiglänge zuzüglich auf jeder Seite 100 m) the free-line
// speed applies, but at least 70 km/h.  Nr. 4.3 prescribes **no** substitute
// speed below that, so a slow Eisenbahn line is computed at its real speed.
//
// Nr. 5.3.2 (Straßenbahnen) is the section that carries a 50 km/h substitute
// speed, together with its "dauerhaft v ≤ 30 km/h" exception.  Both are applied
// by ComputeStreckeEmission, which can only do so while it still sees the real
// track speed — so nothing is clamped here for Straßenbahn segments, and the
// Eisenbahn-only 70 km/h station floor does not reach them either.
func resolveEffectiveSpeed(streckeMax, fahrzeugMax float64, isStation, isStrassenbahn bool) float64 {
	v := math.Min(streckeMax, fahrzeugMax)

	if isStrassenbahn {
		return v
	}

	if isStation {
		return math.Max(v, 70)
	}

	return v
}

// TrackSegment describes one normative track segment for emission computation.
type TrackSegment struct {
	ID                 string           `json:"id"`
	TrackCenterline    []geo.Point2D    `json:"track_centerline"`
	ElevationM         float64          `json:"elevation_m"`
	Fahrbahn           FahrbahnartType  `json:"fahrbahn"`                        // from tables.go constants; Eisenbahn only
	SFahrbahn          SFahrbahnartType `json:"s_fahrbahn"`                      // from tables_strassenbahn.go constants; Strassenbahn only
	Surface            SurfaceCondType  `json:"surface"`                         // from emission_v2.go constants
	BridgeType         int              `json:"bridge_type"`                     // 0=none, 1-4 per Table 9 (Eisenbahn) or Table 16 (Strassenbahn)
	BridgeMitig        bool             `json:"bridge_mitig,omitempty"`          // K_LM noise reduction
	CurveRadiusM       float64          `json:"curve_radius_m"`                  // 0 = straight
	IsStation          bool             `json:"is_station,omitempty"`            // speed min 70 km/h rule
	StreckeMaxKPH      float64          `json:"strecke_max_kph"`                 // track speed limit
	WaterBodyFractionW float64          `json:"water_body_fraction_w,omitempty"` // Gl. 16: fraction of source–receiver path over water, 0–1
	PermanentlySlow    bool             `json:"permanently_slow,omitempty"`      // Nr. 5.3.2: Straßenbahn section permanently at ≤ 30 km/h
	Features           []TrackFeature   `json:"features,omitempty"`              // Nr. 5.3.2: Weichen, Kreuzungen and Haltestellen the substitute speed is scoped to
	Operations         []TrainOperation `json:"operations"`
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

	err = seg.validateTrackEnums()
	if err != nil {
		return err
	}

	err = seg.validateTrackFeatures()
	if err != nil {
		return err
	}

	return seg.validateOperations()
}

// validateTrackFeatures checks the Nr. 5.3.2 track features and their one
// contradiction with the permanently-slow exception.
func (seg TrackSegment) validateTrackFeatures() error {
	for i, feature := range seg.Features {
		err := feature.Validate()
		if err != nil {
			return fmt.Errorf("TrackSegment %q: features[%d]: %w", seg.ID, i, err)
		}
	}

	// Nr. 5.3.2 grants the "dauerhaft v ≤ 30 km/h" exception to sections that
	// carry no Weichen, Kreuzungen or Haltestellen, so a segment claiming both
	// is describing two different stretches of track and has to be split by the
	// caller.
	if seg.PermanentlySlow && len(seg.Features) > 0 {
		return fmt.Errorf(
			"TrackSegment %q: permanently_slow excludes Weichen, Kreuzungen and Haltestellen, but %d feature(s) are declared",
			seg.ID, len(seg.Features),
		)
	}

	return nil
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

	if math.IsNaN(seg.WaterBodyFractionW) || math.IsInf(seg.WaterBodyFractionW, 0) ||
		seg.WaterBodyFractionW < 0 || seg.WaterBodyFractionW > 1 {
		return fmt.Errorf("TrackSegment %q: WaterBodyFractionW must be in [0, 1]", seg.ID)
	}

	return nil
}

// validateTrackEnums range-checks the three enum-valued track properties.
//
// The JSON wire format names them (vocabulary.go), so a decoded segment is
// always in range.  A segment built in Go code is not, and an out-of-range
// ordinal falls through every table lookup to "no correction" — which is the
// same silence PLAN.md 1.2 describes for a misread ordinal, only reached from
// the other side.
func (seg TrackSegment) validateTrackEnums() error {
	if int(seg.Fahrbahn) < 0 || int(seg.Fahrbahn) >= len(fahrbahnartNames) {
		return fmt.Errorf("TrackSegment %q: Fahrbahn %d is out of range, expected one of %s",
			seg.ID, int(seg.Fahrbahn), strings.Join(FahrbahnartNames(), ", "))
	}

	if int(seg.SFahrbahn) < 0 || int(seg.SFahrbahn) >= len(sFahrbahnartNames) {
		return fmt.Errorf("TrackSegment %q: SFahrbahn %d is out of range, expected one of %s",
			seg.ID, int(seg.SFahrbahn), strings.Join(SFahrbahnartNames(), ", "))
	}

	if int(seg.Surface) < 0 || int(seg.Surface) >= len(surfaceCondNames) {
		return fmt.Errorf("TrackSegment %q: Surface %d is out of range, expected one of %s",
			seg.ID, int(seg.Surface), strings.Join(SurfaceCondNames(), ", "))
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

	// TerrainZ is the absolute elevation of the ground under the receiver [m],
	// the datum HeightM is measured from.  Anlage 2's ground model is flat, so
	// this one plane also carries the source end of every path: a track's
	// elevation_m is an absolute Z, and TerrainZ is what turns it into the
	// height above ground that Gl. 9, Gl. 14 and Gl. 15 ask for.
	//
	// Zero means "ground at sea level", which is also the reading a scene that
	// never mentions terrain gets — and for such a scene elevation_m is itself
	// a height above ground, so the two readings agree.
	TerrainZ float64 `json:"terrain_z_m,omitempty"`
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

	// TerrainZ may be negative — ground below sea level is a real place — but
	// it must be a number, because every vertical term is measured from it.
	if math.IsNaN(r.TerrainZ) || math.IsInf(r.TerrainZ, 0) {
		return fmt.Errorf("receiver %q terrain_z_m must be finite", r.ID)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for the Schall 03
// planning-track baseline with preview coefficients routed through a data-pack
// shaped boundary.
func Descriptor() framework.StandardDescriptor {
	return framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "Schall 03 planning-track rail baseline with typed inputs, octave-band handling, deterministic line integration, explicit compliance-boundary metadata, and a data-pack shaped coefficient boundary.",
		EvidenceTier:   framework.EvidenceTierNormative,
		DefaultVersion: "2014-anlage2",
		Versions: []framework.Version{
			{
				Name:           "2014-anlage2",
				DefaultProfile: "rail-planning-preview",
				Profiles: []framework.Profile{
					{
						Name:                 "rail-planning-preview",
						SupportedSourceTypes: []string{"line"},
						SupportedIndicators:  []string{IndicatorLrDay, IndicatorLrNight},
						ParameterSchema:      parameterSchema(),
					},
				},
			},
		},
	}
}
