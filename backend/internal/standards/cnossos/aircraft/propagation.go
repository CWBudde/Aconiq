package aircraft

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines attenuation and aircraft-specific correction terms.
type PropagationConfig struct {
	AirAbsorptionDBPerKM float64
	GroundAttenuationDB  float64
	LateralDirectivityDB float64
	ApproachCorrectionDB float64
	ClimbCorrectionDB    float64
	MinSlantDistanceM    float64
}

type propagationTerms struct {
	DistanceM   float64
	GeometricDB float64
	AirDB       float64
	GroundDB    float64
	LateralDB   float64
	OperationDB float64
	BankDB      float64
}

// DefaultPropagationConfig returns baseline aircraft propagation terms.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		AirAbsorptionDBPerKM: 0.7,
		GroundAttenuationDB:  0.8,
		LateralDirectivityDB: 0,
		ApproachCorrectionDB: 1.5,
		ClimbCorrectionDB:    2.5,
		MinSlantDistanceM:    20,
	}
}

func (cfg PropagationConfig) Validate() error {
	if math.IsNaN(cfg.AirAbsorptionDBPerKM) || math.IsInf(cfg.AirAbsorptionDBPerKM, 0) || cfg.AirAbsorptionDBPerKM < 0 {
		return errors.New("air_absorption_db_per_km must be finite and >= 0")
	}

	if math.IsNaN(cfg.GroundAttenuationDB) || math.IsInf(cfg.GroundAttenuationDB, 0) || cfg.GroundAttenuationDB < 0 {
		return errors.New("ground_attenuation_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.LateralDirectivityDB) || math.IsInf(cfg.LateralDirectivityDB, 0) {
		return errors.New("lateral_directivity_db must be finite")
	}

	if math.IsNaN(cfg.ApproachCorrectionDB) || math.IsInf(cfg.ApproachCorrectionDB, 0) || cfg.ApproachCorrectionDB < 0 {
		return errors.New("approach_correction_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.ClimbCorrectionDB) || math.IsInf(cfg.ClimbCorrectionDB, 0) || cfg.ClimbCorrectionDB < 0 {
		return errors.New("climb_correction_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.MinSlantDistanceM) || math.IsInf(cfg.MinSlantDistanceM, 0) || cfg.MinSlantDistanceM <= 0 {
		return errors.New("min_slant_distance_m must be finite and > 0")
	}

	return nil
}

func effectiveSlantDistance(distanceM float64, cfg PropagationConfig) float64 {
	return math.Max(distanceM, cfg.MinSlantDistanceM)
}

func geometricDivergence(distanceM float64) float64 {
	return 20*math.Log10(distanceM) + 11.0
}

func airAbsorption(distanceM float64, cfg PropagationConfig) float64 {
	return cfg.AirAbsorptionDBPerKM * (distanceM / 1000.0)
}

func groundEffect(cfg PropagationConfig) float64 {
	return cfg.GroundAttenuationDB
}

func lateralDirectivity(source AircraftSource, cfg PropagationConfig) float64 {
	offsetInfluence := math.Min(1.5, math.Abs(source.LateralOffsetM)/150.0)
	return cfg.LateralDirectivityDB + offsetInfluence
}

func operationModeAdjustment(source AircraftSource, cfg PropagationConfig) float64 {
	switch source.OperationType {
	case OperationDeparture:
		return cfg.ClimbCorrectionDB
	case OperationArrival:
		return cfg.ApproachCorrectionDB
	default:
		return 0
	}
}

func attenuationTerms(distanceM float64, source AircraftSource, cfg PropagationConfig) propagationTerms {
	effectiveDistance := effectiveSlantDistance(distanceM, cfg)

	return propagationTerms{
		DistanceM:   effectiveDistance,
		GeometricDB: geometricDivergence(effectiveDistance),
		AirDB:       airAbsorption(effectiveDistance, cfg),
		GroundDB:    groundEffect(cfg),
		LateralDB:   lateralDirectivity(source, cfg),
		OperationDB: operationModeAdjustment(source, cfg),
		BankDB:      bankAngleCorrection(source.BankAngleDeg),
	}
}

func totalAttenuation(terms propagationTerms) float64 {
	return terms.GeometricDB + terms.AirDB + terms.GroundDB - terms.LateralDB - terms.OperationDB - terms.BankDB
}

func lineSourceLevelAtReceiver(emissionDB float64, receiver geo.PointReceiver, source AircraftSource, cfg PropagationConfig) float64 {
	distance := distancePointToFlightTrack(receiver, source.FlightTrack)
	terms := attenuationTerms(distance, source, cfg)

	return emissionDB - totalAttenuation(terms)
}

func bankAngleCorrection(bankAngleDeg float64) float64 {
	absBank := math.Abs(bankAngleDeg)
	if absBank <= 0 {
		return 0
	}

	return math.Min(2.5, absBank/15.0)
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.PointReceiver, sources []AircraftSource, cfg PropagationConfig) (PeriodLevels, error) {
	err := cfg.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	if receiver.ID == "" {
		return PeriodLevels{}, errors.New("receiver id is required")
	}

	if !receiver.Point.IsFinite() {
		return PeriodLevels{}, errors.New("receiver is not finite")
	}

	if len(sources) == 0 {
		return PeriodLevels{}, errors.New("at least one source is required")
	}

	dayContrib := make([]float64, 0, len(sources))
	eveningContrib := make([]float64, 0, len(sources))
	nightContrib := make([]float64, 0, len(sources))

	for _, source := range sources {
		if err := source.Validate(); err != nil {
			return PeriodLevels{}, err
		}

		emission, err := ComputeEmission(source)
		if err != nil {
			return PeriodLevels{}, err
		}

		dayContrib = append(dayContrib, lineSourceLevelAtReceiver(emission.Lday, receiver, source, cfg))
		eveningContrib = append(eveningContrib, lineSourceLevelAtReceiver(emission.Levening, receiver, source, cfg))
		nightContrib = append(nightContrib, lineSourceLevelAtReceiver(emission.Lnight, receiver, source, cfg))
	}

	return PeriodLevels{
		Lday:     energySumDB(dayContrib),
		Levening: energySumDB(eveningContrib),
		Lnight:   energySumDB(nightContrib),
	}, nil
}

func distancePointToFlightTrack(receiver geo.PointReceiver, track []geo.Point3D) float64 {
	if len(track) == 0 {
		return math.NaN()
	}

	if len(track) == 1 {
		return distancePointToSegment3D(receiver, track[0], track[0])
	}

	best := math.MaxFloat64

	for i := range len(track) - 1 {
		d := distancePointToSegment3D(receiver, track[i], track[i+1])
		if d < best {
			best = d
		}
	}

	return best
}

func distancePointToSegment3D(receiver geo.PointReceiver, a geo.Point3D, b geo.Point3D) float64 {
	rx := receiver.Point.X
	ry := receiver.Point.Y
	rz := receiver.HeightM

	abx := b.X - a.X
	aby := b.Y - a.Y
	abz := b.Z - a.Z

	len2 := abx*abx + aby*aby + abz*abz
	if len2 == 0 {
		return math.Sqrt((rx-a.X)*(rx-a.X) + (ry-a.Y)*(ry-a.Y) + (rz-a.Z)*(rz-a.Z))
	}

	t := ((rx-a.X)*abx + (ry-a.Y)*aby + (rz-a.Z)*abz) / len2
	if t < 0 {
		t = 0
	}

	if t > 1 {
		t = 1
	}

	px := a.X + t*abx
	py := a.Y + t*aby
	pz := a.Z + t*abz

	return math.Sqrt((rx-px)*(rx-px) + (ry-py)*(ry-py) + (rz-pz)*(rz-pz))
}
