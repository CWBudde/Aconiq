package schall03

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

const maxIntegrationStepM = 10.0

// PropagationConfig defines the baseline Schall 03 preview attenuation chain.
type PropagationConfig struct {
	AirAbsorptionDBPerKM  float64
	GroundAttenuationDB   float64
	SlabTrackCorrectionDB float64
	BridgeCorrectionDB    float64
	CurveCorrectionDB     float64
	MinDistanceM          float64
}

// DefaultPropagationConfig returns default baseline propagation terms.
func DefaultPropagationConfig() PropagationConfig {
	return BuiltinDataPack().Propagation.DefaultConfig
}

func (cfg PropagationConfig) Validate() error {
	for _, item := range propagationParams() {
		value := item.configValue(cfg)
		if math.IsNaN(value) || math.IsInf(value, 0) || value < item.configMin {
			return errors.New(item.definition.Name + " must be finite and >= 0")
		}
	}

	return nil
}

func attenuation(distanceM float64, bandIdx int, cfg PropagationConfig, pack DataPack) float64 {
	d := distanceM
	if d < cfg.MinDistanceM {
		d = cfg.MinDistanceM
	}

	geometric := 20*math.Log10(d) + 11.0
	air := cfg.AirAbsorptionDBPerKM * pack.Propagation.AirAbsorptionBandFactor[bandIdx] * (d / 1000.0)

	return geometric + air + cfg.GroundAttenuationDB
}

func sourceAdjustment(source RailSource, cfg PropagationConfig) float64 {
	adjustment := 0.0
	if source.Infrastructure.TrackType == TrackTypeSlab {
		adjustment += cfg.SlabTrackCorrectionDB
	}

	if source.Infrastructure.OnBridge {
		adjustment += cfg.BridgeCorrectionDB
	}

	if source.Infrastructure.CurveRadiusM > 0 && source.Infrastructure.CurveRadiusM < 500 {
		severity := (500 - source.Infrastructure.CurveRadiusM) / 500
		adjustment += severity * cfg.CurveCorrectionDB
	}

	return adjustment
}

func lineSourceSpectrumAtReceiver(sourceSpectrum OctaveSpectrum, receiver geo.Point2D, source RailSource, cfg PropagationConfig, pack DataPack) OctaveSpectrum {
	var bandContribs [8][]float64

	adjustment := sourceAdjustment(source, cfg)

	for i := range len(source.TrackCenterline) - 1 {
		a := source.TrackCenterline[i]
		b := source.TrackCenterline[i+1]

		length := geo.Distance(a, b)
		if math.IsNaN(length) || math.IsInf(length, 0) || length <= 0 {
			continue
		}

		subsegments := max(int(math.Ceil(length/maxIntegrationStepM)), 1)
		stepLength := length / float64(subsegments)

		for j := range subsegments {
			fraction := (float64(j) + 0.5) / float64(subsegments)
			point := geo.Point2D{
				X: a.X + (b.X-a.X)*fraction,
				Y: a.Y + (b.Y-a.Y)*fraction,
			}
			distance := geo.Distance(receiver, point)

			for bandIdx := range sourceSpectrum {
				a := attenuation(distance, bandIdx, cfg, pack)
				level := sourceSpectrum[bandIdx] + 10*math.Log10(stepLength) - a + adjustment
				bandContribs[bandIdx] = append(bandContribs[bandIdx], level)
			}
		}
	}

	var result OctaveSpectrum
	for bandIdx := range result {
		result[bandIdx] = EnergeticSumLevels(bandContribs[bandIdx]...)
	}

	return result
}

// BeiblattOctaveBandFrequencies are the octave-band centre frequencies (Hz)
// in the canonical order 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.
// Used by the barrier diffraction module.
var BeiblattOctaveBandFrequencies = [NumBeiblattOctaveBands]float64{
	63, 125, 250, 500, 1000, 2000, 4000, 8000,
}

// ---------------------------------------------------------------------------
// Normative propagation functions (Anlage 2 zu §4 der 16. BImSchV)
// ---------------------------------------------------------------------------

// adiv computes geometric divergence per Gl. 11.
//
//	A_div = 10·lg(4π·d²/d₀²) with d₀ = 1 m.
func adiv(d float64) float64 {
	return 10.0 * math.Log10(4.0*math.Pi*d*d)
}

// aatm computes air absorption per Gl. 12 for a single octave band.
//
//	A_atm = α·d/1000
func aatm(alpha, d float64) float64 {
	return alpha * d / 1000.0
}

// groundReferenceLengthM is the reference length d₀ = 1 m defined with Gl. 11
// ("d₀ = 1 m  Bezugslänge") and reused by Gl. 14.
const groundReferenceLengthM = 1.0

// agrB computes ground absorption over land per Gl. 14.
//
//	A_gr,B = [4.8 - (2·h_m/d)·(17 + 300·d₀/d)] dB ≥ 0 dB
//
// The variable list of Gl. 14 contains only h_m, d and S — d₀ is the 1 m
// reference length introduced with Gl. 11, not a path length.  d_p appears
// only in Gl. 16 (the water term).
//
// h_m: mean height of the propagation path above ground [m] (Gl. 15, Bild 4),
// d:   distance between source centre and receiver [m].
func agrB(hm, d float64) float64 {
	val := 4.8 - (2.0*hm/d)*(17.0+300.0*groundReferenceLengthM/d)
	return math.Max(val, 0.0)
}

// meanPathHeight returns h_m, the mean height of the propagation path above
// ground, for Gl. 14.  hg and hr are heights *above ground*, never absolute
// elevations; newPathGeometry is what converts one into the other.
//
// KNOWN SIMPLIFICATION: Gl. 15 defines h_m = S/d, with S the area between the
// propagation path and the terrain profile (Bild 4).  This implementation has
// no terrain profile and therefore evaluates the flat-ground special case, for
// which the area under the straight source→receiver path is a trapezoid and
// S/d reduces exactly to (h_g + h_r)/2.  Over sloping or undulating ground the
// value deviates from the normative h_m.  Listed in
// docs/conformance/schall03-konformitaetserklaerung.md as a known limitation.
func meanPathHeight(hg, hr float64) float64 {
	hm := (hg + hr) / 2
	if hm < 0 {
		hm = 0
	}

	return hm
}

// pathGeometry is the vertical geometry of one source→receiver propagation
// path, resolved into the terms Anlage 2 asks for.
//
// Anlage 2 measures verticals in two different ways and it is easy to mix them
// up.  Gl. 11 and Gl. 12 want d, a distance between two points, so both ends
// have to be expressed against one common zero.  Gl. 9, Gl. 14 and Gl. 15 want
// heights *above ground*: D_Ω mirrors the source in the ground plane, and h_m
// is the mean height of the path over the terrain beneath it.  Feeding an
// absolute elevation into the second group is what this type exists to prevent
// — at a site 400 m above sea level it drove h_m to ~202 m, where Gl. 14's
// bracket goes negative and the ≥ 0 dB clamp erases the Bodendämpfung.
//
// Both groups are served from the same two numbers, because Anlage 2's ground
// model is flat: one ground plane carries the whole path (see meanPathHeight).
type pathGeometry struct {
	// SourceHeightM is h_g, the Teilquelle's height above the ground plane [m].
	SourceHeightM float64
	// ReceiverHeightM is h_r, the receiver's height above that plane [m].
	ReceiverHeightM float64
	// MeanHeightM is h_m per Gl. 15 [m].
	MeanHeightM float64
	// SlantDistanceM is d, the source→receiver distance in three dimensions
	// [m], clamped to the 1 m reference length of Gl. 11.
	SlantDistanceM float64
	// DOmega is D_Ω per Gl. 9 [dB].
	DOmega float64
}

// newPathGeometry resolves one propagation path.
//
// sourceZ is the absolute Z of the Teilquelle — the track's elevation_m plus
// the Teilquelle's height above Schienenoberkante.  groundZ is the absolute Z
// of the ground plane the path runs over, taken from the receiver because that
// is where the assessment happens and because the flat-ground model gives
// source and receiver one shared plane.  receiverHeightM is the receiver's
// height above that plane, and dp the horizontal source–receiver distance.
//
// A scene whose ground sits at Z = 0 is the identity case: sourceZ is then
// already a height above ground and every term comes out as it did before the
// ground plane existed.
func newPathGeometry(sourceZ, groundZ, receiverHeightM, dp float64) pathGeometry {
	path := pathGeometry{
		SourceHeightM:   sourceZ - groundZ,
		ReceiverHeightM: receiverHeightM,
	}

	path.MeanHeightM = meanPathHeight(path.SourceHeightM, path.ReceiverHeightM)

	// Both heights are measured from the same plane, so their difference is the
	// vertical separation of the two points and the plane cancels out of d.
	dz := path.SourceHeightM - path.ReceiverHeightM

	path.SlantDistanceM = math.Sqrt(dp*dp + dz*dz)
	if path.SlantDistanceM < groundReferenceLengthM {
		path.SlantDistanceM = groundReferenceLengthM
	}

	path.DOmega = solidAngleDOmega(dp, path.SourceHeightM, path.ReceiverHeightM)

	return path
}

// agrW computes the water-body ground correction per Gl. 16.
//
//	A_gr,W = -3·d_w/d_p
//
// d_w: length of propagation path over water [m],
// d_p: total horizontal source–receiver distance [m].
func agrW(dw, dp float64) float64 {
	if dp == 0 {
		return 0
	}

	return -3.0 * dw / dp
}

// directivityDI computes the directivity correction per Gl. 8.
//
//	D_I = 10·lg(0.22 + 1.27·sin²(δ))
//
// δ is the angle between the Gleisachse and the source-to-receiver direction,
// which is the convention Gl. 8 uses: a receiver abeam of the track has δ = 90°
// and the maximum D_I of +1.73 dB, one in line with the track has δ = 0° and
// the minimum of -6.58 dB.
//
// The argument is sin²(δ) rather than δ because that is what every caller
// holds: normativeSinDelta2 derives it from the dot product of the
// source→receiver vector with the track vector, and never forms the angle.
func directivityDI(sinDelta2 float64) float64 {
	return 10.0 * math.Log10(0.22+1.27*sinDelta2)
}

// solidAngleDOmega computes the solid-angle correction per Gl. 9.
//
//	D_Ω = 10·lg(1 + (d_p² + (h_g-h_r)²) / (d_p² + (h_g+h_r)²))
//
// d_p: horizontal source–receiver distance [m],
// h_g: source height above ground [m],
// h_r: receiver height above ground [m].
func solidAngleDOmega(dp, hg, hr float64) float64 {
	num := dp*dp + (hg-hr)*(hg-hr)
	den := dp*dp + (hg+hr)*(hg+hr)

	if den == 0 {
		return 0
	}

	return 10.0 * math.Log10(1.0+num/den)
}

// ComputeReceiverPeriodLevels computes day/night levels at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig) (PeriodLevels, error) {
	return ComputeReceiverPeriodLevelsWithDataPack(receiver, sources, cfg, BuiltinDataPack())
}

// ComputeReceiverPeriodLevelsWithDataPack computes day/night levels using an
// explicit preview or external Schall 03 data pack.
func ComputeReceiverPeriodLevelsWithDataPack(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig, pack DataPack) (PeriodLevels, error) {
	err := cfg.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	err = pack.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	if !receiver.IsFinite() {
		return PeriodLevels{}, errors.New("receiver is not finite")
	}

	if len(sources) == 0 {
		return PeriodLevels{}, errors.New("at least one source is required")
	}

	daySpectra := make([]OctaveSpectrum, 0, len(sources))
	nightSpectra := make([]OctaveSpectrum, 0, len(sources))

	for _, source := range sources {
		emission, err := ComputeEmissionWithDataPack(source, pack)
		if err != nil {
			return PeriodLevels{}, err
		}

		daySpectra = append(daySpectra, lineSourceSpectrumAtReceiver(emission.DaySpectrum, receiver, source, cfg, pack))
		nightSpectra = append(nightSpectra, lineSourceSpectrumAtReceiver(emission.NightSpectrum, receiver, source, cfg, pack))
	}

	day := SumSpectra(daySpectra).EnergeticTotal()
	night := SumSpectra(nightSpectra).EnergeticTotal()

	return PeriodLevels{LrDay: day, LrNight: night}, nil
}
