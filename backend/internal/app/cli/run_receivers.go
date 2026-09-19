package cli

import (
	"errors"
	"fmt"
	"math"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
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

// buildReceiversFromPoints is the single point at which the automatic receiver
// grid's extent is decided. calcArea is the extent of the model's drawn
// calculation area when it has one; it replaces the extent of the sources
// rather than being merged with it, because a drawn area is an instruction
// about where to compute, not a hint.
//
// Padding still applies to a drawn area. That is parity, not taste: the
// browser kernel's buildReceiverGrid pads the calc-area bbox unconditionally,
// and browser-parity.test.ts exists to catch exactly this divergence. A user
// who wants the area used verbatim sets grid_padding_m=0.
func buildReceiversFromPoints(
	operation string,
	sourcePoints []geo.Point2D,
	calcArea *geo.BBox,
	resolutionM float64,
	paddingM float64,
	receiverHeightM float64,
) ([]geo.PointReceiver, results.GridLayout, error) {
	bbox, extentSource, err := gridExtent(sourcePoints, calcArea, operation)
	if err != nil {
		return nil, results.GridLayout{}, err
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

	// Counted before the grid is built, not after. Generate materialises every
	// receiver it describes, so a cap applied to its result is a cap on a
	// slice that already exists: a drawn calculation area spanning a hundred
	// kilometres exhausts memory long before anything gets to refuse it.
	cells, err := grid.CellCount()
	if err != nil {
		return nil, results.GridLayout{}, domainerrors.New(domainerrors.KindValidation, operation, "size receiver grid", err)
	}

	if cells > float64(maxDummyReceivers) {
		return nil, results.GridLayout{}, gridTooLargeError(operation, cells, extentSource)
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, results.GridLayout{}, domainerrors.New(domainerrors.KindValidation, operation, "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, results.GridLayout{}, domainerrors.New(domainerrors.KindValidation, operation, "receiver grid is empty", nil)
	}

	// The count above can sit a row or a column away from what the loop emits
	// once accumulated rounding exceeds the step tolerance, so the generated
	// length stays the authority on the boundary itself.
	if len(receivers) > maxDummyReceivers {
		return nil, results.GridLayout{}, gridTooLargeError(operation, float64(len(receivers)), extentSource)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, results.GridLayout{}, domainerrors.New(domainerrors.KindInternal, operation, "infer receiver grid dimensions", err)
	}

	// The origin is the padded extent's south-west corner, which is where
	// Generate starts and therefore the centre of cell (0,0) — the grid's
	// receivers are points, not cell outlines. This is the only place the
	// origin and the step are known; before this they were local loop state
	// in geo.GridReceiverSet.Generate and were dropped on return, leaving
	// every GIS export to rebuild them by arithmetic over the receiver table.
	//
	// The CRS is not filled in here. buildReceiversFromPoints works in the
	// compute CRS without being told which one it is; the persist layer, which
	// does know, stamps it on.
	layout := results.GridLayout{
		Width:  width,
		Height: height,
		Geo: &results.Georeference{
			OriginX:    grid.Extent.MinX,
			OriginY:    grid.Extent.MinY,
			PixelSizeM: grid.Resolution,
			RowOrder:   results.RowOrderSouthUp,
		},
	}

	return receivers, layout, nil
}

// gridTooLargeError refuses a receiver grid that exceeds the cap. Naming the
// extent matters now that a user can trigger this by drawing, where before it
// took a pathological source extent. This message reaches them: KindUserInput
// exits the CLI with code 2, which the API maps to a 400 whose envelope the run
// dialog renders.
//
// count is a float64 so the refusal can state a size no int could hold, which is
// exactly the case a refusal before allocation exists for.
func gridTooLargeError(operation string, count float64, extentSource string) error {
	return domainerrors.New(domainerrors.KindUserInput, operation, fmt.Sprintf(
		"receiver grid too large (%.0f > %d); the extent came from %s", count, maxDummyReceivers, extentSource,
	), nil)
}

// gridExtentCalcArea and gridExtentSource name which extent a grid was built
// over. They appear in the run log as `grid_extent=` and in the over-cap
// refusal, which is the record a user reads to answer "did it use my area?".
const (
	gridExtentCalcArea = "calc_area"
	gridExtentSource   = "source_extent"
)

// gridExtent settles the auto grid's extent and says where it came from.
func gridExtent(sourcePoints []geo.Point2D, calcArea *geo.BBox, operation string) (geo.BBox, string, error) {
	if calcArea != nil {
		return *calcArea, "the model's calculation area", nil
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return geo.BBox{}, "", domainerrors.New(domainerrors.KindValidation, operation, "failed to derive source extent", nil)
	}

	return bbox, "the source extent", nil
}

// gridExtentLabel is the `grid_extent=` value for a resolved calculation area.
func gridExtentLabel(calcArea *geo.BBox) string {
	if calcArea != nil {
		return gridExtentCalcArea
	}

	return gridExtentSource
}

func buildDummyReceivers(sources []freefield.Source, calcArea *geo.BBox, options dummyRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Point)
	}

	return buildReceiversFromPoints("cli.buildDummyReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosRoadReceivers(sources []cnossosroad.RoadSource, calcArea *geo.BBox, options cnossosRoadRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	return buildReceiversFromPoints("cli.buildCnossosRoadReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosRailReceivers(sources []cnossosrail.RailSource, calcArea *geo.BBox, options cnossosRailRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.TrackCenterline...)
	}

	return buildReceiversFromPoints("cli.buildCnossosRailReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildBUBRoadReceivers(sources []bubroad.RoadSource, calcArea *geo.BBox, options bubRoadRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	return buildReceiversFromPoints("cli.buildBUBRoadReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

// buildRLS19RoadReceivers derives the automatic receiver grid from the extent
// of everything that emits: the road centerlines and, since a Parkplatz is an
// extended footprint rather than the point it is propagated from, every vertex
// of each parking polygon. Padding a grid around a lot's centroid alone would
// place the whole grid inside the source.
//
// A drawn calculation area replaces that extent entirely — collecting the
// source points is then wasted work, but it is a slice append per source and
// keeping one extent decision in buildReceiversFromPoints is worth more.
func buildRLS19RoadReceivers(
	sources []rls19road.RoadSource,
	parkingExtent []geo.Point2D,
	calcArea *geo.BBox,
	options rls19RoadRunOptions,
) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2+len(parkingExtent))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.EffectiveCenterline()...)
	}

	sourcePoints = append(sourcePoints, parkingExtent...)

	return buildReceiversFromPoints("cli.buildRLS19RoadReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildSchall03Receivers(sources []schall03.RailSource, calcArea *geo.BBox, options schall03RunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources)*2)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.TrackCenterline...)
	}

	return buildReceiversFromPoints("cli.buildSchall03Receivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

// buildAircraftReceivers serves both aircraft modules: buf/aircraft aliases
// cnossos/aircraft, so the source type is one. The op is a parameter because it
// names the caller in an error, and the two callers are still two commands.
func buildAircraftReceivers(op string, sources []cnossosaircraft.AircraftSource, calcArea *geo.BBox, options aircraftRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		for _, point := range source.FlightTrack {
			sourcePoints = append(sourcePoints, point.XY())
		}
	}

	return buildReceiversFromPoints(op, sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildCnossosAircraftReceivers(sources []cnossosaircraft.AircraftSource, calcArea *geo.BBox, options cnossosAircraftRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	return buildAircraftReceivers("cli.buildCnossosAircraftReceivers", sources, calcArea, aircraftRunOptions(options))
}

func buildBUFAircraftReceivers(sources []bufaircraft.AircraftSource, calcArea *geo.BBox, options bufAircraftRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	return buildAircraftReceivers("cli.buildBUFAircraftReceivers", sources, calcArea, aircraftRunOptions(options))
}

func buildCnossosIndustryReceivers(sources []cnossosindustry.IndustrySource, calcArea *geo.BBox, options cnossosIndustryRunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
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

	return buildReceiversFromPoints("cli.buildCnossosIndustryReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func buildISO9613Receivers(sources []iso9613.PointSource, calcArea *geo.BBox, options iso9613RunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Point)
	}

	return buildReceiversFromPoints("cli.buildISO9613Receivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
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
