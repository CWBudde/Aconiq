package iso9613

// Table 2 from ISO 9613-2:1996: atmospheric attenuation coefficient α (dB/km)
// indexed by [temperature °C, humidity %] for each octave band.

type atmosphericRow struct {
	TempC    float64
	Humidity float64
	Alpha    [NumBands]float64
}

// table2 holds the 7 reference rows from ISO 9613-2 Table 2.
// Row order: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.
var table2 = []atmosphericRow{
	{10, 70, [NumBands]float64{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}},
	{20, 70, [NumBands]float64{0.1, 0.3, 1.1, 2.8, 5.0, 9.0, 22.9, 76.6}},
	{30, 70, [NumBands]float64{0.1, 0.3, 1.0, 3.1, 7.4, 12.7, 23.1, 59.3}},
	{15, 20, [NumBands]float64{0.3, 0.6, 1.2, 2.7, 8.2, 28.2, 88.8, 202.0}},
	{15, 50, [NumBands]float64{0.1, 0.5, 1.2, 2.2, 4.2, 10.8, 36.2, 129.0}},
	{15, 80, [NumBands]float64{0.1, 0.3, 1.1, 2.4, 4.1, 8.3, 23.7, 82.8}},
}

// LookupAlpha returns the atmospheric attenuation coefficient α (dB/km)
// for a given temperature, humidity, and octave band index.
// For exact table matches it returns the tabulated value. For other
// conditions it uses nearest-row selection. band is 0-indexed.
func LookupAlpha(tempC, humidity float64, band int) float64 {
	if band < 0 || band >= NumBands {
		return 0
	}

	best := 0
	bestDist := 1e18
	for i, row := range table2 {
		dt := (tempC - row.TempC) / 10.0
		dh := (humidity - row.Humidity) / 50.0
		dist := dt*dt + dh*dh
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}

	return table2[best].Alpha[band]
}

// AtmosphericAbsorption computes A_atm (Eq. 8): α · d / 1000.
func AtmosphericAbsorption(alpha, distanceM float64) float64 {
	return alpha * distanceM / 1000.0
}

// AtmosphericAbsorptionBands computes A_atm for all 8 octave bands.
func AtmosphericAbsorptionBands(tempC, humidity, distanceM float64) BandLevels {
	var result BandLevels
	for i := 0; i < NumBands; i++ {
		alpha := LookupAlpha(tempC, humidity, i)
		result[i] = AtmosphericAbsorption(alpha, distanceM)
	}
	return result
}
