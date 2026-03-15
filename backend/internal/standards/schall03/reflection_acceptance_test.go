package schall03_test

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func reflTestdataPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

func reflRound6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func TestRefl1SingleWallReflection(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -200, Y: 0}, {X: 200, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5,
	}

	walls := []schall03.ReflectingWall{
		{
			A: geo.Point2D{X: -200, Y: 40}, B: geo.Point2D{X: 200, Y: 40},
			HeightM: 15, Surface: schall03.WallSurfaceHard,
		},
	}

	resultNoWall, err := schall03.ComputeNormativeReceiverLevels(receiver, []schall03.TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	resultWithWall, err := schall03.ComputeNormativeReceiverLevelsWithWalls(receiver, []schall03.TrackSegment{seg}, walls)
	if err != nil {
		t.Fatal(err)
	}

	increase := resultWithWall.LpAeqDay - resultNoWall.LpAeqDay

	snapshot := map[string]any{
		"scenario":             "refl1_single_wall_reflection",
		"description":          "1st-order reflection off hard wall at 40m from track",
		"lp_aeq_day_no_wall":   reflRound6(resultNoWall.LpAeqDay),
		"lp_aeq_day_with_wall": reflRound6(resultWithWall.LpAeqDay),
		"increase_db":          reflRound6(increase),
	}

	golden.AssertJSONSnapshot(t, reflTestdataPath(t, "refl1_single_wall.golden.json"), snapshot)

	if increase <= 0 {
		t.Errorf("reflection should increase level, got increase=%g dB", increase)
	}
}

func TestRefl2FresnelRejection(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -200, Y: 0}, {X: 200, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5,
	}

	walls := []schall03.ReflectingWall{
		{
			A: geo.Point2D{X: -0.15, Y: 40}, B: geo.Point2D{X: 0.15, Y: 40},
			HeightM: 0.2, Surface: schall03.WallSurfaceHard,
		},
	}

	resultNoWall, err := schall03.ComputeNormativeReceiverLevels(receiver, []schall03.TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	resultWithWall, err := schall03.ComputeNormativeReceiverLevelsWithWalls(receiver, []schall03.TrackSegment{seg}, walls)
	if err != nil {
		t.Fatal(err)
	}

	diff := math.Abs(resultWithWall.LpAeqDay - resultNoWall.LpAeqDay)

	snapshot := map[string]any{
		"scenario":             "refl2_fresnel_rejection",
		"description":          "Tiny wall fails Fresnel check — no level change",
		"lp_aeq_day_no_wall":   reflRound6(resultNoWall.LpAeqDay),
		"lp_aeq_day_with_wall": reflRound6(resultWithWall.LpAeqDay),
		"diff_db":              reflRound6(diff),
	}

	golden.AssertJSONSnapshot(t, reflTestdataPath(t, "refl2_fresnel_rejection.golden.json"), snapshot)

	if diff > 0.01 {
		t.Errorf("Fresnel-rejected wall should not change result, diff=%g dB", diff)
	}
}

func TestRefl3CanyonDoubleReflection(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -200, Y: 0}, {X: 200, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 3.5,
	}

	singleWall := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -200, Y: 15}, B: geo.Point2D{X: 200, Y: 15}, HeightM: 12, Surface: schall03.WallSurfaceHard},
	}

	canyonWalls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -200, Y: 15}, B: geo.Point2D{X: 200, Y: 15}, HeightM: 12, Surface: schall03.WallSurfaceHard},
		{A: geo.Point2D{X: -200, Y: -15}, B: geo.Point2D{X: 200, Y: -15}, HeightM: 12, Surface: schall03.WallSurfaceHard},
	}

	resultSingle, err := schall03.ComputeNormativeReceiverLevelsWithWalls(receiver, []schall03.TrackSegment{seg}, singleWall)
	if err != nil {
		t.Fatal(err)
	}

	resultCanyon, err := schall03.ComputeNormativeReceiverLevelsWithWalls(receiver, []schall03.TrackSegment{seg}, canyonWalls)
	if err != nil {
		t.Fatal(err)
	}

	increase := resultCanyon.LpAeqDay - resultSingle.LpAeqDay

	snapshot := map[string]any{
		"scenario":          "refl3_canyon_double_reflection",
		"description":       "Canyon geometry — 2nd-order reflections between parallel walls",
		"lp_aeq_day_single": reflRound6(resultSingle.LpAeqDay),
		"lp_aeq_day_canyon": reflRound6(resultCanyon.LpAeqDay),
		"increase_db":       reflRound6(increase),
	}

	golden.AssertJSONSnapshot(t, reflTestdataPath(t, "refl3_canyon_double_reflection.golden.json"), snapshot)

	if increase <= 0 {
		t.Errorf("canyon should be louder than single wall, increase=%g dB", increase)
	}
}
