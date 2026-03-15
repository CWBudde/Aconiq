package schall03_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestNormativeReceiverWithReflection(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 50, Y: 30}, HeightM: 3.5,
	}

	resultNoWall, err := schall03.ComputeNormativeReceiverLevels(receiver, []schall03.TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	walls := []schall03.ReflectingWall{
		{
			A: geo.Point2D{X: -100, Y: 40}, B: geo.Point2D{X: 100, Y: 40},
			HeightM: 15, Surface: schall03.WallSurfaceHard,
		},
	}

	resultWithWall, err := schall03.ComputeNormativeReceiverLevelsWithWalls(
		receiver, []schall03.TrackSegment{seg}, walls,
	)
	if err != nil {
		t.Fatal(err)
	}

	if resultWithWall.LpAeqDay <= resultNoWall.LpAeqDay {
		t.Errorf("reflection should increase level: without=%g, with=%g",
			resultNoWall.LpAeqDay, resultWithWall.LpAeqDay)
	}
}

func TestNormativeReceiverSmallWallFresnelReject(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 50, Y: 30}, HeightM: 3.5,
	}

	resultNoWall, err := schall03.ComputeNormativeReceiverLevels(receiver, []schall03.TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	// Tiny wall — fails Fresnel at 63 Hz.
	walls := []schall03.ReflectingWall{
		{
			A: geo.Point2D{X: 49.75, Y: 40}, B: geo.Point2D{X: 50.25, Y: 40},
			HeightM: 0.3, Surface: schall03.WallSurfaceHard,
		},
	}

	resultWithWall, err := schall03.ComputeNormativeReceiverLevelsWithWalls(
		receiver, []schall03.TrackSegment{seg}, walls,
	)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(resultWithWall.LpAeqDay-resultNoWall.LpAeqDay) > 0.01 {
		t.Errorf("Fresnel-rejected wall should not change result: without=%g, with=%g",
			resultNoWall.LpAeqDay, resultWithWall.LpAeqDay)
	}
}

func TestNormativeReceiverCanyonReflection(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 50, Y: 0}, HeightM: 3.5,
	}

	singleWall := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -100, Y: 20}, B: geo.Point2D{X: 100, Y: 20}, HeightM: 15, Surface: schall03.WallSurfaceHard},
	}

	canyonWalls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -100, Y: 20}, B: geo.Point2D{X: 100, Y: 20}, HeightM: 15, Surface: schall03.WallSurfaceHard},
		{A: geo.Point2D{X: -100, Y: -20}, B: geo.Point2D{X: 100, Y: -20}, HeightM: 15, Surface: schall03.WallSurfaceHard},
	}

	resultSingle, err := schall03.ComputeNormativeReceiverLevelsWithWalls(receiver, []schall03.TrackSegment{seg}, singleWall)
	if err != nil {
		t.Fatal(err)
	}

	resultCanyon, err := schall03.ComputeNormativeReceiverLevelsWithWalls(receiver, []schall03.TrackSegment{seg}, canyonWalls)
	if err != nil {
		t.Fatal(err)
	}

	if resultCanyon.LpAeqDay <= resultSingle.LpAeqDay {
		t.Errorf("canyon should be louder than single wall: single=%g, canyon=%g",
			resultSingle.LpAeqDay, resultCanyon.LpAeqDay)
	}
}
