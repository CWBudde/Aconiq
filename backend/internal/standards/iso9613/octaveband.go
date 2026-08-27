package iso9613

const (
	NumBands     = 8
	SpeedOfSound = 340.0 // m/s, reference value used by ISO 9613-2
)

// OctaveBandFrequencies contains the 8 standard midband frequencies (Hz).
var OctaveBandFrequencies = [NumBands]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}

// AWeighting contains the A-weighting corrections per octave band (dB).
// IEC 651 / IEC 61672-1 values at nominal midband frequencies.
var AWeighting = [NumBands]float64{-26.2, -16.1, -8.6, -3.2, 0.0, 1.2, 1.0, -1.1}

// BandLevels holds sound power or pressure levels for each octave band.
type BandLevels [NumBands]float64

// Wavelength returns the wavelength of sound at a given frequency (m).
func Wavelength(freqHz float64) float64 {
	return SpeedOfSound / freqHz
}

// NoteOneBandIndex is the index of the 500 Hz octave band.
//
// ISO 9613-2:1996, clause 1, NOTE 1: "If only A-weighted sound power levels of
// the sources are known, the attenuation terms for 500 Hz may be used to
// estimate the resulting attenuation."
//
// The A-weighted sound power level is already A-weighted, so the Note 1
// estimate evaluates a single attenuation value -- the 500 Hz one -- and must
// not re-apply the A-weighting of Eq. 5. Replicating L_WA into all eight bands
// and then energy-summing with A-weighting would add
// 10*lg(sum_j 10^(0.1*A_j)) = +6.99 dB.
const NoteOneBandIndex = 3
