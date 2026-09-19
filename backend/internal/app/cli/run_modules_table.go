package cli

import (
	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
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

// runModuleTable is what `aconiq run` dispatches on. One entry per registered
// standard; adding a standard means adding an entry, not editing a switch.
//
// Nine of the thirteen are the same shape and say so by being built from
// receiverRunModule — the per-standard part is five functions and three log
// strings. The other four differ in kind, not in detail: dummy-freefield runs
// through the compute engine, RLS-19 extracts barriers and buildings alongside
// its sources, Schall 03 resolves which normative chain to run and records it
// in provenance, and BEB aggregates exposure per building from another
// standard's sources rather than computing receiver levels of its own.
var runModuleTable = map[string]runModule{
	freefield.StandardID: runDummyModule,

	cnossosroad.StandardID: receiverRunModule[cnossosRoadRunOptions, cnossosroad.RoadSource, acoustics.ReceiverOutput]{
		sourceCountKey: "road_sources",
		extractFailure: "failed to extract road sources",
		computeFailure: "cnossos compute failed",
		parseOptions:   parseCnossosRoadRunOptions,
		extract:        extractCnossosRoadSources,
		buildReceivers: buildCnossosRoadReceivers,
		compute: func(receivers []geo.PointReceiver, sources []cnossosroad.RoadSource, options cnossosRoadRunOptions) ([]acoustics.ReceiverOutput, error) {
			return cnossosroad.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(cnossosroad.StandardID),
	}.run,

	cnossosrail.StandardID: receiverRunModule[cnossosRailRunOptions, cnossosrail.RailSource, acoustics.ReceiverOutput]{
		sourceCountKey: "rail_sources",
		extractFailure: "failed to extract rail sources",
		computeFailure: "cnossos rail compute failed",
		parseOptions:   parseCnossosRailRunOptions,
		extract:        extractCnossosRailSources,
		buildReceivers: buildCnossosRailReceivers,
		compute: func(receivers []geo.PointReceiver, sources []cnossosrail.RailSource, options cnossosRailRunOptions) ([]acoustics.ReceiverOutput, error) {
			return cnossosrail.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(cnossosrail.StandardID),
	}.run,

	cnossosindustry.StandardID: receiverRunModule[cnossosIndustryRunOptions, cnossosindustry.IndustrySource, acoustics.ReceiverOutput]{
		sourceCountKey: "industry_sources",
		extractFailure: "failed to extract industry sources",
		computeFailure: "cnossos industry compute failed",
		parseOptions:   parseCnossosIndustryRunOptions,
		extract:        extractCnossosIndustrySources,
		buildReceivers: buildCnossosIndustryReceivers,
		compute: func(receivers []geo.PointReceiver, sources []cnossosindustry.IndustrySource, options cnossosIndustryRunOptions) ([]acoustics.ReceiverOutput, error) {
			return cnossosindustry.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(cnossosindustry.StandardID),
	}.run,

	cnossosaircraft.StandardID: receiverRunModule[cnossosAircraftRunOptions, cnossosaircraft.AircraftSource, acoustics.ReceiverOutput]{
		sourceCountKey: "aircraft_sources",
		extractFailure: "failed to extract aircraft sources",
		computeFailure: "cnossos aircraft compute failed",
		parseOptions:   parseCnossosAircraftRunOptions,
		extract:        extractCnossosAircraftSources,
		buildReceivers: buildCnossosAircraftReceivers,
		compute: func(receivers []geo.PointReceiver, sources []cnossosaircraft.AircraftSource, options cnossosAircraftRunOptions) ([]acoustics.ReceiverOutput, error) {
			return cnossosaircraft.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(cnossosaircraft.StandardID),
	}.run,

	// bub-rail and bub-industry are alias modules over the cnossos ones: their
	// source, receiver and output types are Go aliases, so extraction and
	// receiver building are reused verbatim. Only the parameter schema, the
	// compute entry point and the bundle they write are their own.
	bubrail.StandardID: receiverRunModule[bubRailRunOptions, cnossosrail.RailSource, acoustics.ReceiverOutput]{
		sourceCountKey: "bub_rail_sources",
		extractFailure: "failed to extract BUB rail sources",
		computeFailure: "bub rail compute failed",
		parseOptions:   parseBUBRailRunOptions,
		extract:        extractCnossosRailSources,
		buildReceivers: buildCnossosRailReceivers,
		compute: func(receivers []geo.PointReceiver, sources []cnossosrail.RailSource, options bubRailRunOptions) ([]acoustics.ReceiverOutput, error) {
			return bubrail.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(bubrail.StandardID),
	}.run,

	bubindustry.StandardID: receiverRunModule[bubIndustryRunOptions, cnossosindustry.IndustrySource, acoustics.ReceiverOutput]{
		sourceCountKey: "bub_industry_sources",
		extractFailure: "failed to extract BUB industry sources",
		computeFailure: "bub industry compute failed",
		parseOptions:   parseBUBIndustryRunOptions,
		extract:        extractCnossosIndustrySources,
		buildReceivers: buildCnossosIndustryReceivers,
		compute: func(receivers []geo.PointReceiver, sources []cnossosindustry.IndustrySource, options bubIndustryRunOptions) ([]acoustics.ReceiverOutput, error) {
			return bubindustry.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(bubindustry.StandardID),
	}.run,

	// bub-road shares cnossos-road's source model but not its propagation, so
	// it extracts through its own entry point.
	bubroad.StandardID: receiverRunModule[bubRoadRunOptions, bubroad.RoadSource, acoustics.ReceiverOutput]{
		sourceCountKey: "bub_road_sources",
		extractFailure: "failed to extract BUB road sources",
		computeFailure: "bub road compute failed",
		parseOptions:   parseBUBRoadRunOptions,
		extract:        extractBUBRoadSources,
		buildReceivers: buildBUBRoadReceivers,
		compute: func(receivers []geo.PointReceiver, sources []bubroad.RoadSource, options bubRoadRunOptions) ([]acoustics.ReceiverOutput, error) {
			return bubroad.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(bubroad.StandardID),
	}.run,

	bufaircraft.StandardID: receiverRunModule[bufAircraftRunOptions, bufaircraft.AircraftSource, acoustics.ReceiverOutput]{
		sourceCountKey: "buf_aircraft_sources",
		extractFailure: "failed to extract BUF aircraft sources",
		computeFailure: "buf aircraft compute failed",
		parseOptions:   parseBUFAircraftRunOptions,
		extract:        extractBUFAircraftSources,
		buildReceivers: buildBUFAircraftReceivers,
		compute: func(receivers []geo.PointReceiver, sources []bufaircraft.AircraftSource, options bufAircraftRunOptions) ([]acoustics.ReceiverOutput, error) {
			return bufaircraft.ComputeReceiverOutputs(receivers, sources, options.PropagationConfig())
		},
		persist: endPersist(bufaircraft.StandardID),
	}.run,

	iso9613.StandardID:     runISO9613Module,
	rls19road.StandardID:   runRLS19RoadModule,
	schall03.StandardID:    runSchall03Module,
	bebexposure.StandardID: runBEBExposureModule,
}
