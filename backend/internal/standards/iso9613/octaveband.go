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

// BandLevelsFromAWeighted creates octave-band levels from a single A-weighted
// value by setting all bands to that value. This is the fallback per
// ISO 9613-2 Note 1: use 500 Hz attenuation terms when only A-weighted
// sound power is known.
func BandLevelsFromAWeighted(lwa float64) BandLevels {
	var levels BandLevels
	for i := range levels {
		levels[i] = lwa
	}

	return levels
}
