package cli

import (
	"fmt"
	"maps"
	"strconv"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/framework"
)

// The keys a run with facade receivers adds to provenance metadata and the run
// summary. Each count is written only when it is non-zero, so a run without
// facade receivers records what it always did.
const (
	facadeProposedCountKey      = "facade_proposed_count"
	facadeAssumedHeightCountKey = "facade_assumed_height_count"
	facadeRejectedCountKey      = "facade_rejected_count"
	allowUnconfirmedKey         = "allow_unconfirmed"
)

// facadeRequest is the facade part of a run request: the --facade-* flags and
// the opt-in.
type facadeRequest struct {
	buildings        []string
	spacingM         float64
	spacingSet       bool
	includeCourtyard bool
	allowUnconfirmed bool
}

// receiverSet is the receiver set a run computes at, as far as it is settled
// before the standard's module runs. For an automatic grid that is only the
// mode — the grid depends on the module's sources. For custom and facade
// receivers it is the whole set, read or generated once by the pipeline, so
// every module, the estimate and the gate see the same points.
type receiverSet struct {
	mode string

	// listed is the set of a custom or facade run, in the order it is
	// computed in.
	listed []geo.PointReceiver

	// facade holds the facade attributes of the listed receivers that carry
	// any, by receiver id.
	facade map[string]facadeset.Attributes

	proposed      int
	assumedHeight int
	rejected      int

	// metadata is what provenance.json records about the set beyond its mode.
	metadata map[string]string
}

// isListed reports whether the set is a list of points rather than a grid, so
// the run writes a receiver table and no raster.
func (s receiverSet) isListed() bool {
	return s.mode == receiverModeCustom || s.mode == receiverModeFacade
}

// facadeListed is the set as facadeset counts and gates it.
func (s receiverSet) facadeListed() facadeset.Listed {
	return facadeset.Listed{Receivers: s.listed, Facade: s.facade}
}

// tableRecord adds a receiver's facade attributes to its table row. They go in
// only as a complete set — rule, facade and storey — because that is the one
// shape results.ReceiverTable accepts; a partial set stays in the model.
func (s receiverSet) tableRecord(record results.ReceiverRecord) results.ReceiverRecord {
	attrs, ok := s.facade[record.ID]
	if !ok || !attrs.Complete() {
		return record
	}

	key := attrs.Facade
	storey := *attrs.Storey
	record.PlacementRule = attrs.PlacementRule
	record.Storey = &storey
	record.Facade = &results.ReceiverFacade{
		BuildingRef: key.BuildingRef,
		Part:        key.Part,
		Ring:        key.Ring,
		Edge:        key.Edge,
		From:        [2]float64{key.From.X, key.From.Y},
		To:          [2]float64{key.To.X, key.To.Y},
	}

	return record
}

// decorateSummary stamps the counts of unconfirmed and rejected facade points
// onto a run summary.
func (s receiverSet) decorateSummary(summary map[string]any) {
	for key, count := range s.counts() {
		if count > 0 {
			summary[key] = count
		}
	}
}

func (s receiverSet) counts() map[string]int {
	return map[string]int{
		facadeProposedCountKey:      s.proposed,
		facadeAssumedHeightCountKey: s.assumedHeight,
		facadeRejectedCountKey:      s.rejected,
	}
}

// resolveRunReceivers settles the receiver set of a run before the run exists,
// and refuses one that may not run: a facade request a standard has no rule
// for, and a normative run over facade points nobody has confirmed. The run
// and its estimate both call it, so both refuse in the same words.
func resolveRunReceivers(
	mode string,
	request facadeRequest,
	standard framework.ResolvedProfile,
	model modelgeojson.Model,
) (receiverSet, error) {
	err := request.validateFor(mode)
	if err != nil {
		return receiverSet{}, err
	}

	set := receiverSet{mode: mode, metadata: map[string]string{}}

	switch mode {
	case receiverModeCustom:
		set, err = customReceiverSet(set, model)
	case receiverModeFacade:
		set, err = generatedFacadeReceiverSet(set, request, standard, model)
	default:
		return set, nil
	}

	if err != nil {
		return receiverSet{}, err
	}

	set.proposed, set.assumedHeight = set.facadeListed().CountUnconfirmed()

	err = facadeset.Gate(set.facadeListed(), standard, model, request.allowUnconfirmed)
	if err != nil {
		return receiverSet{}, fmt.Errorf("facade receivers: %w", err)
	}

	for key, count := range set.counts() {
		if count > 0 {
			set.metadata[key] = strconv.Itoa(count)
		}
	}

	if request.allowUnconfirmed {
		set.metadata[allowUnconfirmedKey] = "true"
	}

	return set, nil
}

// validateFor refuses facade flags outside the facade mode, and an incomplete
// facade request inside it. There is no default spacing: how densely a facade
// is sampled is the engineer's choice, and a default would make it for them.
func (r facadeRequest) validateFor(mode string) error {
	if mode != receiverModeFacade {
		if len(r.buildings) > 0 || r.spacingSet || r.includeCourtyard {
			return domainerrors.New(domainerrors.KindUserInput, "cli.run",
				"--facade-building, --facade-spacing-m and --facade-courtyard need --receiver-mode facade", nil)
		}

		return nil
	}

	if len(r.buildings) == 0 {
		return domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"receiver mode facade needs at least one --facade-building <building feature id or \"<street> <number>\">", nil)
	}

	if !r.spacingSet {
		return domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"receiver mode facade needs --facade-spacing-m, the station spacing along each facade; it has no default", nil)
	}

	return nil
}

// customReceiverSet reads the model's receivers and their facade properties,
// through facadeset.Custom, which drops and counts the rejected ones.
func customReceiverSet(set receiverSet, model modelgeojson.Model) (receiverSet, error) {
	receivers, err := extractExplicitReceivers(model)
	if err != nil {
		return receiverSet{}, err
	}

	listed, rejected, err := facadeset.Custom(receivers, model)
	if err != nil {
		return receiverSet{}, fmt.Errorf("custom receivers: %w", err)
	}

	set.listed = listed.Receivers
	set.facade = listed.Facade
	set.rejected = rejected

	return set, nil
}

// generatedFacadeReceiverSet places the facade receivers of the requested
// buildings at the standard's placement rule, through facadeset.Generate, and
// records the request in provenance.
func generatedFacadeReceiverSet(
	set receiverSet,
	request facadeRequest,
	standard framework.ResolvedProfile,
	model modelgeojson.Model,
) (receiverSet, error) {
	listed, metadata, err := facadeset.Generate(facadeset.Request{
		Buildings:        request.buildings,
		SpacingM:         request.spacingM,
		IncludeCourtyard: request.includeCourtyard,
	}, standard.StandardID, model)
	if err != nil {
		return receiverSet{}, fmt.Errorf("facade receivers: %w", err)
	}

	set.listed = listed.Receivers
	set.facade = listed.Facade
	maps.Copy(set.metadata, metadata)

	return set, nil
}
