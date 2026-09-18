package schall03

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
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

// meanPathHeight returns h_m, the mean height of the propagation path above the
// ground beneath it, for Gl. 14.
//
// Gl. 15 defines h_m = S/d, with S the area between the propagation path and
// the terrain profile (Bild 4).  Divided by d, that is the mean height of the
// straight source→receiver path above the terrain, which separates into two
// means that can be taken independently:
//
//	h_m = mean height of the path − mean elevation of the ground under it.
//
// The path is straight, so its mean height over the receiver's own ground plane
// is the trapezoid (h_g + h_r)/2 — exactly the value Gl. 15 collapses to over
// level ground.  groundOffsetM is what the ground actually does underneath:
// the mean elevation of the terrain along the path, measured from that same
// plane.  It is 0 for level ground, positive where the path crosses a ridge or
// climbs towards the source, negative over a valley or a falling slope, and it
// is resolved once per subsegment→receiver path by resolvePathGroundOffset.
//
// hg and hr are heights *above that plane*, never absolute elevations;
// newPathGeometry is what converts one into the other.
//
// The ≥ 0 floor keeps a path that runs below the mean ground — a cutting, or a
// receiver in a hollow — out of the negative branch of Gl. 14, where the
// bracket would grow without limit and A_gr,B with it.  Gl. 15 defines no such
// case; refusing to extrapolate it is the conservative reading.
func meanPathHeight(hg, hr, groundOffsetM float64) float64 {
	hm := (hg+hr)/2 - groundOffsetM
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
// D_Ω and the Abschirmprüfung are served from the same two numbers, measured
// against one reference plane: Gl. 9 mirrors the source in a plane, and
// BarrierSegment.TopHeightM is a height above one.  That plane is flat, which
// is the residual approximation declared as deviation 4.  h_m is not tied to
// it: MeanHeightM carries the ground under the path (see meanPathHeight).
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

// newPathGeometry resolves one propagation path.  It is a pure function of
// numbers already resolved: the ground arrives as groundOffsetM, never as a
// terrain model, so the DTM is sampled once per subsegment→receiver path
// rather than once per Teilquelle height.
//
// sourceZ is the absolute Z of the Teilquelle — the track's elevation_m plus
// the Teilquelle's height above Schienenoberkante.  groundZ is the absolute Z
// of the reference plane the path is measured against, taken from the receiver
// because that is where the assessment happens.  receiverHeightM is the
// receiver's height above that plane, and dp the horizontal source–receiver
// distance.
//
// groundOffsetM is the mean elevation of the terrain along the path, measured
// from that same plane, and enters h_m alone (Gl. 15).  D_Ω (Gl. 9), d
// (Gl. 11/12) and the heights handed to the Abschirmprüfung (Nr. 6.5) stay on
// the reference plane: Gl. 9 mirrors the source in a plane and Nr. 6.5 needs
// two endpoint heights in one datum, and neither has a terrain profile to
// stand on.
//
// groundOffsetM = 0 is the identity case — level ground, or no terrain model at
// all — and reproduces bit for bit what the flat-ground reading computed.
func newPathGeometry(sourceZ, groundZ, receiverHeightM, dp, groundOffsetM float64) pathGeometry {
	path := pathGeometry{
		SourceHeightM:   sourceZ - groundZ,
		ReceiverHeightM: receiverHeightM,
	}

	path.MeanHeightM = meanPathHeight(path.SourceHeightM, path.ReceiverHeightM, groundOffsetM)

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

// terrainSampleStepM is the nominal spacing at which a DTM is sampled along one
// propagation path for h_m (Gl. 15).
//
// It is deliberately coarser than maxIntegrationStepM = 10 m, the length of a
// track subsegment, because the two steps discretise different things.  The
// integration step cuts the *line source* into point sources, and its length
// bounds the error of Gl. 6's summation; the sampling step resolves the *ground
// profile* under one such point source's path, and the profile enters the
// result only through its mean, through the single term 2·h_m/d of Gl. 14.
// Sampling the ground at the integration step would multiply the DTM queries
// per run by roughly the ratio of the two — the terrain sampling of RLS-19 was
// measured at about 28 % of its runtime — and buy accuracy that one averaged
// term cannot express.  25 m resolves an embankment, a cutting or a valley
// floor at the distances Anlage 2 assesses, and terrain.MeanRiseAboveChord
// caps the sample count per path in any case.
const terrainSampleStepM = 25.0

// resolvePathGroundOffset returns the mean elevation of the ground along one
// subsegment→receiver path, measured from the receiver's own ground plane —
// the groundOffsetM that newPathGeometry and Gl. 15 consume.
//
// The composition is the one terrain.MeanRiseAboveChord's doc comment
// prescribes.  The mean ground elevation along the path is
//
//	avgGroundZ = (sourceGroundZ + receiverGroundZ)/2 + MeanRiseAboveChord(…)
//
// and this function returns avgGroundZ − receiverGroundZ.  It is expressed as a
// difference from the receiver's plane rather than as an absolute Z so that the
// no-terrain case cancels exactly rather than nearly: with no DTM the offset is
// the literal 0 that leaves h_m at (h_g + h_r)/2 bit for bit.
//
// receiverGroundZ is ReceiverInput.TerrainZ, which the CLI already fills per
// receiver from the DTM; sampling the grid again under the receiver could only
// disagree with the datum every other term is measured against.
//
// **A path the DTM does not span end to end gets no ground term at all.**  Not
// a partial one: the rule is all or nothing, and MeanRiseAboveChord's ok is the
// test, because it reports false unless *both* endpoints fall inside the grid.
// The trap it closes is that TerrainZ is not always a measurement — for a
// receiver the DTM misses, schall03ReceiverGroundZ substitutes the mean of the
// receivers it does reach — so halving the difference between that inherited
// mean and a real sample at the source end would apply a correction built on a
// number nobody measured there, while d, D_Ω and the Abschirmprüfung all stayed
// on the TerrainZ plane.  Falling back to the flat-plane reading keeps the
// whole path on one datum, and a miss is still never read as an elevation of
// zero.
//
// Do NOT substitute TrackSegment.ElevationM for a missing sourceGroundZ: that
// is the Schienenoberkante, not the ground under it, and it would move every
// scene declaring a bridge or an embankment whether or not a DTM exists.
func resolvePathGroundOffset(dtm terrain.Model, source geo.Point2D, receiver ReceiverInput) float64 {
	if dtm == nil {
		return 0
	}

	// Asked first, for its ok as much as for its value: it is the one call that
	// answers "does the terrain span this whole path".
	rise, ok := terrain.MeanRiseAboveChord(
		dtm,
		source.X, source.Y,
		receiver.Point.X, receiver.Point.Y,
		terrainSampleStepM,
	)
	if !ok {
		return 0
	}

	// Guarded rather than assumed.  MeanRiseAboveChord has already sampled this
	// point, so ok here is implied today — but relying on the order another
	// package happens to do its work in is how a partial correction gets back
	// in.
	sourceGroundZ, ok := dtm.ElevationAt(source.X, source.Y)
	if !ok {
		return 0
	}

	return (sourceGroundZ-receiver.TerrainZ)/2 + rise
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
