package road

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the preview coefficient sets this scaffold module
// computes with.
//
// The values are this project's own invented preview constants, kept stable so
// results stay reproducible. No coefficient of a German mapping directive
// exists in this package; see docs/conformance/cnossos-umfangserklaerung.md. A
// digest of invented constants is still worth carrying: it answers "could these
// two runs have produced different numbers?", which is a question about the
// data a module holds and not about the authority behind it.
//
// This module declares no coefficient table at all — every constant sits in a
// switch statement or inside a vehicle-class expression — so the tables below
// are produced by evaluating those functions over their enumerated domain. A
// digest that covered only declared package-level variables would be empty
// here, and would stay empty while a hand-edited switch changed every number.
//
// The flow term is not represented: 10 lg(Q) is a formula over an unbounded
// domain with no constant of its own. Neither are the purely geometric
// propagation terms — the 20 lg(d) + 11 divergence and the line-integration
// step are closed-form code rather than data, and a change to them is a change
// to the code path, which the tool version already identifies.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "preview-road-mapping/model-version", Value: BuiltinModelVersion},
		{Name: "preview-road-mapping/class-reference-levels", Value: classReferenceLevelTable()},
		{Name: "preview-road-mapping/class-correction-weights", Value: classCorrectionWeightTable()},
		{Name: "preview-road-mapping/speed-correction-samples", Value: speedCorrectionSampleTable()},
		{Name: "preview-road-mapping/surface-corrections", Value: surfaceCorrectionTable()},
		{Name: "preview-road-mapping/road-function-corrections", Value: roadFunctionCorrectionTable()},
		{Name: "preview-road-mapping/junction-corrections", Value: junctionCorrectionTable()},
		{Name: "preview-road-mapping/junction-distance-samples", Value: junctionDistanceSampleTable()},
		{Name: "preview-road-mapping/gradient-correction-samples", Value: gradientCorrectionSampleTable()},
		{Name: "preview-road-mapping/temperature-correction-samples", Value: temperatureCorrectionSampleTable()},
		{Name: "preview-road-mapping/studded-tyre-correction-samples", Value: studdedTyreCorrectionSampleTable()},
		{Name: "preview-road-mapping/intersection-density-samples", Value: intersectionDensitySampleTable()},
		{Name: "preview-road-mapping/propagation-defaults", Value: DefaultPropagationConfig()},
	}}
}

// standardDataReferenceSpeedKPH is the speed the per-class reference levels are
// evaluated at. It is the reference of the light and medium speed curves, where
// both contribute nothing, so those two entries expose their class base level
// directly; the heavy and powered-two-wheeler curves are offset there, which
// pins their own reference speeds as well.
const standardDataReferenceSpeedKPH = 50.0

// classReferenceLevelTable evaluates the four vehicle-class emissions at
// 1 veh/h — where the flow term contributes nothing — and at the reference
// speed, with every context correction set to zero. It pins the class base
// levels together with the speed curve at that one point.
func classReferenceLevelTable() []float64 {
	return classEmissionTable(0)
}

// classCorrectionWeightTable repeats classReferenceLevelTable with every
// context correction set to 1 dB. Each vehicle class weights the surface,
// temperature and gradient corrections differently, and those weights live in
// the class expressions rather than in any table; the difference between the
// two tables is exactly the sum of one class's weights.
func classCorrectionWeightTable() []float64 {
	return classEmissionTable(1)
}

// classEmissionTable evaluates every vehicle-class emission at 1 veh/h and the
// reference speed, with each context correction set to correction.
func classEmissionTable(correction float64) []float64 {
	speed := standardDataReferenceSpeedKPH

	return []float64{
		lightVehicleEmission(1, speed, correction, correction, correction, correction, correction),
		mediumVehicleEmission(1, speed, correction, correction, correction, correction, correction),
		heavyVehicleEmission(1, speed, correction, correction, correction, correction, correction),
		poweredTwoWheelerEmission(1, speed, correction, correction, correction),
	}
}

// speedCorrectionSampleTable samples the four per-class speed curves. The
// sample speeds bracket every clamp the curves use (20, 100, 110 and 130 km/h)
// and every branch threshold (40, 50, 60 and 80 km/h), and include the 40, 45,
// 50, 60 and 80 km/h reference speeds the branches divide by, so a change to
// any clamp, threshold or reference moves at least one sampled value.
func speedCorrectionSampleTable() [][]float64 {
	speeds := []float64{10, 20, 39, 40, 45, 50, 60, 61, 80, 81, 100, 110, 130, 150}

	light := make([]float64, 0, len(speeds))
	medium := make([]float64, 0, len(speeds))
	heavy := make([]float64, 0, len(speeds))
	ptw := make([]float64, 0, len(speeds))

	for _, speed := range speeds {
		light = append(light, lightSpeedCorrection(speed))
		medium = append(medium, mediumSpeedCorrection(speed))
		heavy = append(heavy, heavySpeedCorrection(speed))
		ptw = append(ptw, ptwSpeedCorrection(speed))
	}

	return [][]float64{light, medium, heavy, ptw}
}

// surfaceCorrectionTable evaluates the surface correction over every allowed
// surface type.
func surfaceCorrectionTable() []float64 {
	surfaces := []string{SurfaceDenseAsphalt, SurfacePorousAsphalt, SurfaceConcrete, SurfaceCobblestone}

	values := make([]float64, 0, len(surfaces))
	for _, surface := range surfaces {
		values = append(values, surfaceCorrection(surface))
	}

	return values
}

// roadFunctionCorrectionTable evaluates the road-function correction over every
// allowed function class.
func roadFunctionCorrectionTable() []float64 {
	classes := []string{FunctionUrbanMain, FunctionUrbanLocal, FunctionRuralMain}

	values := make([]float64, 0, len(classes))
	for _, class := range classes {
		values = append(values, roadFunctionCorrection(class))
	}

	return values
}

// junctionCorrectionTable evaluates the junction correction at zero distance,
// where the distance taper is at full influence, over every allowed junction
// type.
func junctionCorrectionTable() []float64 {
	junctions := []string{JunctionNone, JunctionTrafficLight, JunctionRoundabout}

	values := make([]float64, 0, len(junctions))
	for _, junction := range junctions {
		values = append(values, junctionCorrection(junction, 0))
	}

	return values
}

// junctionDistanceSampleTable samples the junction distance taper for one
// junction type. The sample distances bracket the 150 m cut-off, so both the
// taper slope and the cut-off are pinned; the per-type magnitudes are covered
// by junctionCorrectionTable.
func junctionDistanceSampleTable() []float64 {
	distances := []float64{0, 75, 150, 200}

	values := make([]float64, 0, len(distances))
	for _, distance := range distances {
		values = append(values, junctionCorrection(JunctionTrafficLight, distance))
	}

	return values
}

// gradientCorrectionSampleTable samples the gradient correction. The sample
// gradients sit outside, on and inside the +/-2 % dead band, so both slopes and
// the band width are pinned.
func gradientCorrectionSampleTable() []float64 {
	gradients := []float64{-10, -2.5, 0, 2.5, 10}

	values := make([]float64, 0, len(gradients))
	for _, gradient := range gradients {
		values = append(values, gradientCorrection(gradient))
	}

	return values
}

// temperatureCorrectionSampleTable samples the temperature correction either
// side of its 20 degrees Celsius reference, so both the slope and the reference
// temperature are pinned.
func temperatureCorrectionSampleTable() []float64 {
	temperatures := []float64{0, 20, 40}

	values := make([]float64, 0, len(temperatures))
	for _, temperature := range temperatures {
		values = append(values, temperatureCorrection(temperature))
	}

	return values
}

// studdedTyreCorrectionSampleTable samples the studded-tyre correction across
// its validated [0,1] share domain.
func studdedTyreCorrectionSampleTable() []float64 {
	shares := []float64{0, 0.5, 1}

	values := make([]float64, 0, len(shares))
	for _, share := range shares {
		values = append(values, studdedTyreCorrection(share))
	}

	return values
}

// intersectionDensitySampleTable samples the propagation-side intersection
// term. The sample densities sit below, at and above the 60 per km where the
// term reaches its 3 dB cap, so both the divisor and the cap are pinned.
func intersectionDensitySampleTable() []float64 {
	densities := []float64{0, 20, 40, 60, 80}

	values := make([]float64, 0, len(densities))
	for _, density := range densities {
		values = append(values, intersectionEffect(PropagationConfig{IntersectionDensityPerKM: density}))
	}

	return values
}
