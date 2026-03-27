package cli

import (
	"errors"
	"fmt"
	"math"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func buildReceiversFromPoints(operation string, sourcePoints []geo.Point2D, resolutionM float64, paddingM float64, receiverHeightM float64) ([]geo.PointReceiver, int, int, error) {
	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, operation, "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - paddingM,
			MinY: bbox.MinY - paddingM,
			MaxX: bbox.MaxX + paddingM,
			MaxY: bbox.MaxY + paddingM,
		},
		Resolution: resolutionM,
		HeightM:    receiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, operation, "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, operation, "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, operation, fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, operation, "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildDummyReceivers(sources []freefield.Source, options dummyRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Point)
	}

	return buildReceiversFromPoints("cli.buildDummyReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosRoadReceivers(sources []cnossosroad.RoadSource, options cnossosRoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	return buildReceiversFromPoints("cli.buildCnossosRoadReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosRailReceivers(sources []cnossosrail.RailSource, options cnossosRailRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.TrackCenterline...)
	}

	return buildReceiversFromPoints("cli.buildCnossosRailReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildBUBRoadReceivers(sources []bubroad.RoadSource, options bubRoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	return buildReceiversFromPoints("cli.buildBUBRoadReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildRLS19RoadReceivers(sources []rls19road.RoadSource, options rls19RoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.EffectiveCenterline()...)
	}

	return buildReceiversFromPoints("cli.buildRLS19RoadReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildSchall03Receivers(sources []schall03.RailSource, options schall03RunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.TrackCenterline...)
	}

	return buildReceiversFromPoints("cli.buildSchall03Receivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosAircraftReceivers(sources []cnossosaircraft.AircraftSource, options cnossosAircraftRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		for _, point := range source.FlightTrack {
			sourcePoints = append(sourcePoints, point.XY())
		}
	}

	return buildReceiversFromPoints("cli.buildCnossosAircraftReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildBUFAircraftReceivers(sources []bufaircraft.AircraftSource, options bufAircraftRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		for _, point := range source.FlightTrack {
			sourcePoints = append(sourcePoints, point.XY())
		}
	}

	return buildReceiversFromPoints("cli.buildBUFAircraftReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosIndustryReceivers(sources []cnossosindustry.IndustrySource, options cnossosIndustryRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		switch source.SourceType {
		case cnossosindustry.SourceTypePoint:
			sourcePoints = append(sourcePoints, source.Point)
		case cnossosindustry.SourceTypeArea:
			for _, ring := range source.AreaPolygon {
				sourcePoints = append(sourcePoints, ring...)
			}
		}
	}

	return buildReceiversFromPoints("cli.buildCnossosIndustryReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildISO9613Receivers(sources []iso9613.PointSource, options iso9613RunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Point)
	}

	return buildReceiversFromPoints("cli.buildISO9613Receivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func inferGridShape(receivers []geo.PointReceiver) (int, int, error) {
	if len(receivers) == 0 {
		return 0, 0, errors.New("receivers are empty")
	}

	firstY := receivers[0].Point.Y
	width := 0

	for _, receiver := range receivers {
		if math.Abs(receiver.Point.Y-firstY) > 1e-9 {
			break
		}

		width++
	}

	if width <= 0 {
		return 0, 0, errors.New("invalid grid width")
	}

	if len(receivers)%width != 0 {
		return 0, 0, fmt.Errorf("receiver count %d is not divisible by inferred width %d", len(receivers), width)
	}

	return width, len(receivers) / width, nil
}
