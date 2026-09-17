package iso9613

import "math"

// Table 2 from ISO 9613-2:1996: atmospheric attenuation coefficient α (dB/km)
// indexed by [temperature °C, humidity %] for each octave band.

type atmosphericRow struct {
	TempC    float64
	Humidity float64
	Alpha    [NumBands]float64
}

// table2 holds the 6 reference rows from ISO 9613-2 Table 2.
// Row order: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.
//
// The table is reference data, not the compute path. ISO 9613-2, 7.2 gives
// these six rows for convenience and refers to ISO 9613-1 for the coefficient
// itself, so the formula is the normative source and the table is a tabulation
// of it. alphaDBPerKm evaluates that formula for any condition; the table is
// what the formula is checked against (TestAlphaMatchesTable2), and it stays in
// StandardData because it is published data this module carries.
var table2 = []atmosphericRow{
	{10, 70, [NumBands]float64{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}},
	{20, 70, [NumBands]float64{0.1, 0.3, 1.1, 2.8, 5.0, 9.0, 22.9, 76.6}},
	{30, 70, [NumBands]float64{0.1, 0.3, 1.0, 3.1, 7.4, 12.7, 23.1, 59.3}},
	{15, 20, [NumBands]float64{0.3, 0.6, 1.2, 2.7, 8.2, 28.2, 88.8, 202.0}},
	{15, 50, [NumBands]float64{0.1, 0.5, 1.2, 2.2, 4.2, 10.8, 36.2, 129.0}},
	{15, 80, [NumBands]float64{0.1, 0.3, 1.1, 2.4, 4.1, 8.3, 23.7, 82.8}},
}

// Reference atmosphere for the ISO 9613-1 coefficient.
//
// ISO 9613-2, 7.2 evaluates the attenuation coefficient at the reference
// atmospheric pressure, so the ambient pressure p_a and the reference pressure
// p_r are the same value here and their ratio is 1. The two are still written
// out separately, because the pressure ratio enters four places in the formula
// and collapsing it would make the expressions unrecognizable against the
// printed equations.
const (
	ambientPressureKPa      = 101.325 // p_a
	referencePressureKPa    = 101.325 // p_r
	referenceTemperatureK   = 293.15  // T_0, 20 °C
	triplePointTemperatureK = 273.16  // T_01, the triple-point isotherm
	celsiusToKelvin         = 273.15
	metersPerKilometer      = 1000.0
)

// Air temperature bounds for the atmospheric absorption inputs.
//
// The ISO 9613-1 coefficient divides by the absolute temperature and evaluates
// exp(-2239.1/T), so a temperature at or below absolute zero does not merely
// give a poor answer, it gives ±Inf levels. The published parameter schema and
// PropagationConfig.Validate share these bounds. They are wider than the range
// ISO 9613-1 claims accuracy over (-20 °C to +50 °C) and wider than any
// assessment scenario, and narrow only in that they exclude values no air mass
// on this planet takes.
const (
	minAirTemperatureC = -60.0
	maxAirTemperatureC = 60.0
)

// AlphaForBand returns the atmospheric attenuation coefficient α (dB/km) for a
// given temperature, relative humidity and octave band. band is 0-indexed.
//
// The coefficient is computed from the ISO 9613-1 model for the condition
// asked for, at every condition. It is not read from ISO 9613-2 Table 2: that
// table is a tabulation of this formula at six reference conditions, and
// selecting the nearest of its rows put the 4 kHz coefficient at 5 °C / 30 % RH
// at 32.8 dB/km where the formula gives 83.0 — 25 dB over 500 m, an order of
// magnitude outside the ±1 to ±3 dB accuracy ISO 9613-2, clause 9 claims for
// the method as a whole.
func AlphaForBand(tempC, humidity float64, band int) float64 {
	if band < 0 || band >= NumBands {
		return 0
	}

	return alphaDBPerKm(tempC, humidity, OctaveBandFrequencies[band])
}

// alphaDBPerKm evaluates the ISO 9613-1 pure-tone atmospheric attenuation
// coefficient at freqHz, in dB/km.
//
// The model is the sum of a classical (thermal and viscous) term and the two
// molecular relaxation terms for oxygen and nitrogen, each with its own
// relaxation frequency. ISO 9613-1 states it in dB/m; the ISO 9613-2 attenuation
// term A_atm = α·d/1000 wants dB/km, so the result is scaled once, here.
func alphaDBPerKm(tempC, relativeHumidityPercent, freqHz float64) float64 {
	temperatureK := tempC + celsiusToKelvin
	pressureRatio := ambientPressureKPa / referencePressureKPa
	temperatureRatio := temperatureK / referenceTemperatureK

	// h, the molar concentration of water vapour in percent, from the relative
	// humidity and the saturation vapour pressure.
	saturationRatio := math.Pow(10, -6.8346*math.Pow(triplePointTemperatureK/temperatureK, 1.261)+4.6151)
	h := relativeHumidityPercent * saturationRatio / pressureRatio

	// Relaxation frequencies of oxygen and nitrogen, in Hz.
	relaxationO := pressureRatio * (24 + 4.04e4*h*(0.02+h)/(0.391+h))
	relaxationN := pressureRatio * math.Pow(temperatureRatio, -0.5) *
		(9 + 280*h*math.Exp(-4.170*(math.Pow(temperatureRatio, -1.0/3.0)-1)))

	frequencySquared := freqHz * freqHz

	classical := 1.84e-11 / pressureRatio * math.Pow(temperatureRatio, 0.5)
	oxygen := 0.01275 * math.Exp(-2239.1/temperatureK) / (relaxationO + frequencySquared/relaxationO)
	nitrogen := 0.1068 * math.Exp(-3352.0/temperatureK) / (relaxationN + frequencySquared/relaxationN)

	alphaDBPerM := 8.686 * frequencySquared *
		(classical + math.Pow(temperatureRatio, -2.5)*(oxygen+nitrogen))

	return alphaDBPerM * metersPerKilometer
}

// AtmosphericAbsorption computes A_atm (Eq. 8): α · d / 1000.
func AtmosphericAbsorption(alpha, distanceM float64) float64 {
	return alpha * distanceM / 1000.0
}

// AtmosphericAbsorptionBands computes A_atm for all 8 octave bands.
func AtmosphericAbsorptionBands(tempC, humidity, distanceM float64) BandLevels {
	var result BandLevels

	for i := range NumBands {
		alpha := AlphaForBand(tempC, humidity, i)
		result[i] = AtmosphericAbsorption(alpha, distanceM)
	}

	return result
}
