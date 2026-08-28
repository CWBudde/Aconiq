package road

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the preview coefficient sets this scaffold module
// computes with.
//
// The values are this project's own invented preview constants, kept stable so
// results stay reproducible. They are not CNOSSOS-EU coefficients, and no
// coefficient from Directive 2015/996 Annex II exists in this package; see
// docs/conformance/cnossos-umfangserklaerung.md. A digest of invented constants
// is still worth carrying: it answers "could these two runs have produced
// different numbers?", which is a question about the data a module holds and
// not about the authority behind it.
//
// Most of that data lives in switch statements rather than in declared tables,
// so those are evaluated over their enumerated domain here. A digest that
// covered only the declared maps would sit still while a hand-edited switch
// changed the numbers, which is exactly the failure this field exists to catch.
// Corrections that are continuous in one argument are sampled at a fixed,
// literal set of points chosen so that every branch threshold and every
// embedded factor moves at least one sampled value.
//
// The flow term is not represented: 10 lg(Q) is a formula over an unbounded
// domain with no constant of its own. Neither are the purely geometric
// propagation terms — the 20 lg(d) + 11 divergence and the line-integration
// step are closed-form code rather than data, and a change to them is a change
// to the code path, which the tool version already identifies.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "preview-road/model-version", Value: BuiltinModelVersion},
		{Name: "preview-road/base-levels", Value: baseLevelTable()},
		{Name: "preview-road/road-category-corrections", Value: roadCategoryCorrections},
		{Name: "preview-road/low-speed-corrections", Value: lowSpeedCorrections},
		{Name: "preview-road/mid-speed-multipliers", Value: midSpeedMultipliers},
		{Name: "preview-road/high-speed-offsets", Value: highSpeedOffsets},
		{Name: "preview-road/high-speed-multipliers", Value: highSpeedMultipliers},
		{Name: "preview-road/speed-correction-samples", Value: speedCorrectionSampleTable()},
		{Name: "preview-road/surface-corrections", Value: surfaceCorrectionTable()},
		{Name: "preview-road/junction-corrections", Value: junctionCorrectionTable()},
		{Name: "preview-road/junction-distance-samples", Value: junctionDistanceSampleTable()},
		{Name: "preview-road/gradient-correction-samples", Value: gradientCorrectionSampleTable()},
		{Name: "preview-road/temperature-correction-samples", Value: temperatureCorrectionSampleTable()},
		{Name: "preview-road/studded-tyre-corrections", Value: studdedTyreCorrectionTable()},
		{Name: "preview-road/propagation-defaults", Value: DefaultPropagationConfig()},
	}}
}

// standardDataVehicleClasses lists the vehicle classes in the fixed order every
// per-class table below is built in.
func standardDataVehicleClasses() []vehicleClass {
	return []vehicleClass{
		vehicleClassLight,
		vehicleClassMedium,
		vehicleClassHeavy,
		vehicleClassPoweredTwoWheelers,
	}
}

// baseLevelTable evaluates the per-class base emission levels.
func baseLevelTable() []float64 {
	classes := standardDataVehicleClasses()

	values := make([]float64, 0, len(classes))
	for _, class := range classes {
		values = append(values, baseEmissionLevel(class))
	}

	return values
}

// speedCorrectionSampleTable samples the piecewise speed correction per class.
//
// The sample speeds bracket both clamps (20 and 130 km/h) and both branch
// thresholds (40 and 80 km/h), and include the 50 km/h reference of the middle
// branch, so a change to any clamp, threshold or reference speed moves at least
// one sampled value.
func speedCorrectionSampleTable() [][]float64 {
	speeds := []float64{10, 20, 39, 40, 50, 80, 90, 130, 200}
	classes := standardDataVehicleClasses()

	rows := make([][]float64, 0, len(classes))
	for _, class := range classes {
		row := make([]float64, 0, len(speeds))
		for _, speed := range speeds {
			row = append(row, speedCorrection(speed, class))
		}

		rows = append(rows, row)
	}

	return rows
}

// surfaceCorrectionTable evaluates the surface correction over every allowed
// surface type and vehicle class.
func surfaceCorrectionTable() [][]float64 {
	surfaces := []string{SurfaceDenseAsphalt, SurfacePorousAsphalt, SurfaceConcrete, SurfaceCobblestone}
	classes := standardDataVehicleClasses()

	rows := make([][]float64, 0, len(surfaces))
	for _, surface := range surfaces {
		row := make([]float64, 0, len(classes))
		for _, class := range classes {
			row = append(row, surfaceCorrection(surface, class))
		}

		rows = append(rows, row)
	}

	return rows
}

// junctionCorrectionTable evaluates the junction correction at zero distance,
// where the distance taper is at full influence, over every allowed junction
// type and vehicle class.
func junctionCorrectionTable() [][]float64 {
	junctions := []string{JunctionNone, JunctionTrafficLight, JunctionRoundabout}
	classes := standardDataVehicleClasses()

	rows := make([][]float64, 0, len(junctions))
	for _, junction := range junctions {
		row := make([]float64, 0, len(classes))
		for _, class := range classes {
			row = append(row, junctionCorrection(junction, 0, class))
		}

		rows = append(rows, row)
	}

	return rows
}

// junctionDistanceSampleTable samples the junction distance taper for one
// junction type and vehicle class. The sample distances bracket the 100 m
// cut-off, so both the taper slope and the cut-off are pinned; the per-type and
// per-class magnitudes are covered by junctionCorrectionTable.
func junctionDistanceSampleTable() []float64 {
	distances := []float64{0, 25, 50, 100, 150}

	values := make([]float64, 0, len(distances))
	for _, distance := range distances {
		values = append(values, junctionCorrection(JunctionTrafficLight, distance, vehicleClassLight))
	}

	return values
}

// gradientCorrectionSampleTable samples the gradient correction per class. The
// sample gradients sit outside, on and inside the +/-2 % dead band, so both
// per-class slopes and the band width are pinned.
func gradientCorrectionSampleTable() [][]float64 {
	gradients := []float64{-10, -2.5, 0, 2.5, 10}
	classes := standardDataVehicleClasses()

	rows := make([][]float64, 0, len(classes))
	for _, class := range classes {
		row := make([]float64, 0, len(gradients))
		for _, gradient := range gradients {
			row = append(row, gradientCorrection(gradient, class))
		}

		rows = append(rows, row)
	}

	return rows
}

// temperatureCorrectionSampleTable samples the temperature correction per class
// either side of its 20 degrees Celsius reference, so both the per-class slope
// and the reference temperature are pinned.
func temperatureCorrectionSampleTable() [][]float64 {
	temperatures := []float64{0, 20, 40}
	classes := standardDataVehicleClasses()

	rows := make([][]float64, 0, len(classes))
	for _, class := range classes {
		row := make([]float64, 0, len(temperatures))
		for _, temperature := range temperatures {
			row = append(row, temperatureCorrection(temperature, class))
		}

		rows = append(rows, row)
	}

	return rows
}

// studdedTyreCorrectionTable evaluates the studded-tyre correction per class at
// a share of 1, the upper end of its validated [0,1] domain, which yields the
// per-class factor itself.
func studdedTyreCorrectionTable() []float64 {
	classes := standardDataVehicleClasses()

	values := make([]float64, 0, len(classes))
	for _, class := range classes {
		values = append(values, studdedTyreCorrection(1, class))
	}

	return values
}
