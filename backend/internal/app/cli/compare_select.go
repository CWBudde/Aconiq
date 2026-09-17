package cli

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/soundplanimport"
)

// Which SoundPLAN result run `aconiq compare` reads.
//
// A SoundPLAN project holds the same site computed several times over, and the
// levels alone do not say which of those scenarios a given result belongs to.
// Both halves of the comparison therefore have to choose one run and record why:
// selectSoundPlanReceiverResultDir for the RSPS receiver tables, and
// selectSoundPlanGridMapRun for the RRLK grid maps. They share a decision shape
// and a vocabulary, and they are kept in one file so the two cannot drift apart.

// soundPlanResultRunSelection records which SoundPLAN result run the
// comparison read, out of which candidates, and on what grounds.
//
// It is singular on purpose. Concatenating every RSPS* directory — which is
// what this used to do — produced a candidate pool spanning several scenarios:
// in the reference project RSPS0011 and RSPS0021 hold the same 13 immission
// points computed without and with the noise barrier, and their levels differ
// by up to 8 dB. No matcher can be correct against a pool like that, because
// the right answer is not in it once.
type soundPlanResultRunSelection struct {
	Dir        string
	Candidates []string
	Selection  string
	Warnings   []string
}

// How a result run was chosen, as recorded in soundplan_result_run_selection
// and in soundplan_raster_run_selection.
//
// gridRunSelectionGridHeight and gridRunSelectionContradicted are reachable
// only from the grid-map side. The first records a run chosen on the receiver
// height it was computed at, which is the discriminator the geometry list
// cannot supply: a project holds the same site computed with and without the
// barrier *and* at two grid heights, so the barrier signal alone leaves two
// runs standing.
//
// The second is what the grid-map side needs and the receiver side does not.
// Only the grid-map side has a second discriminator to fall through to, so only
// it can turn "every candidate's geometry is known and every one of them
// disagrees with the model" into a confident-looking answer. That case is not
// undecided, it is decided against: the comparison is being run over a scenario
// the model provably does not describe, and it has to say so in its own words
// rather than borrow resultRunSelectionAmbiguous's.
const (
	resultRunSelectionExplicit   = "explicit"
	resultRunSelectionGeometry   = "geometry_match"
	resultRunSelectionOnly       = "only_candidate"
	resultRunSelectionAmbiguous  = "ambiguous"
	gridRunSelectionGridHeight   = "grid_height_match"
	gridRunSelectionContradicted = "geometry_contradicted"
)

// discoverSoundPlanReceiverResultDirs lists the result directories that carry
// a receiver table, newest naming first.
func discoverSoundPlanReceiverResultDirs(soundPlanRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(soundPlanRoot, "RSPS*"))
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.compare", "discover SoundPLAN receiver result directories", err)
	}

	slices.Sort(matches)

	resultDirs := make([]string, 0, len(matches))

	for _, match := range matches {
		name := filepath.Base(match)

		suffix := compareExtractRunSuffix(name)
		if compareFileExists(filepath.Join(match, "RREC"+suffix+".abs")) {
			resultDirs = append(resultDirs, name)
		}
	}

	if len(resultDirs) == 0 {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.compare", "no SoundPLAN RSPS receiver result directory found", nil)
	}

	return resultDirs, nil
}

// selectSoundPlanReceiverResultDir picks the one result run to compare
// against.
//
// An explicit --soundplan-run wins. Otherwise the candidates are filtered by
// whether their .res says the run consumed the noise barrier geometry, which
// has to agree with whether the import produced barrier features — that is the
// only thing distinguishing the reference project's two single-point runs. If
// that leaves no single answer the last candidate by name is taken and the
// selection is recorded as ambiguous, with a warning, because a comparison
// that silently picks a scenario is the defect this function exists to stop.
func selectSoundPlanReceiverResultDir(
	soundPlanRoot string,
	explicitRun string,
	modelHasBarriers bool,
) (soundPlanResultRunSelection, error) {
	candidates, err := discoverSoundPlanReceiverResultDirs(soundPlanRoot)
	if err != nil {
		return soundPlanResultRunSelection{}, err
	}

	selection := soundPlanResultRunSelection{Candidates: candidates}

	if requested := strings.TrimSpace(explicitRun); requested != "" {
		if !slices.Contains(candidates, requested) {
			return soundPlanResultRunSelection{}, domainerrors.New(
				domainerrors.KindUserInput, "cli.compare",
				fmt.Sprintf("--soundplan-run %q is not one of the available result runs: %s", requested, strings.Join(candidates, ", ")),
				nil,
			)
		}

		selection.Dir = requested
		selection.Selection = resultRunSelectionExplicit

		return selection, nil
	}

	if len(candidates) == 1 {
		selection.Dir = candidates[0]
		selection.Selection = resultRunSelectionOnly

		return selection, nil
	}

	matching := make([]string, 0, len(candidates))

	for _, candidate := range candidates {
		usedBarrier, known := soundPlanRunUsedBarrierGeometry(soundPlanRoot, candidate)
		if known && usedBarrier == modelHasBarriers {
			matching = append(matching, candidate)
		}
	}

	if len(matching) == 1 {
		selection.Dir = matching[0]
		selection.Selection = resultRunSelectionGeometry

		return selection, nil
	}

	selection.Dir = candidates[len(candidates)-1]
	selection.Selection = resultRunSelectionAmbiguous
	selection.Warnings = append(selection.Warnings, fmt.Sprintf(
		"SoundPLAN result runs %s could not be told apart by the geometry their .res files record; compared against %s by name order. Pass --soundplan-run to choose.",
		strings.Join(candidates, ", "), selection.Dir,
	))

	return selection, nil
}

// soundPlanBarrierGeometryFile is the reference project's noise-barrier
// geometry, lower-cased as GeometryFileNames reports it. Whether a run consumed
// it is the one signal that separates its otherwise identical scenarios.
const soundPlanBarrierGeometryFile = "geowand.geo"

// soundPlanRunUsedBarrierGeometry reports whether a result run's .res says the
// run read GeoWand.geo. The second return is false when the .res could not be
// read at all, which must not be confused with a run that read no barrier.
func soundPlanRunUsedBarrierGeometry(soundPlanRoot string, resultRunDir string) (bool, bool) {
	res, err := soundplanimport.ParseResFile(filepath.Join(soundPlanRoot, resultRunDir+".res"))
	if err != nil {
		return false, false
	}

	return soundPlanGeometryUsedBarrier(res.GeometryFileNames())
}

// soundPlanGeometryUsedBarrier reads the barrier signal out of a run's geometry
// file list. An empty list is "unknown", never "no barrier".
func soundPlanGeometryUsedBarrier(names []string) (bool, bool) {
	if len(names) == 0 {
		return false, false
	}

	return slices.Contains(names, soundPlanBarrierGeometryFile), true
}

// gridMapHeightToleranceM is how close a run's declared grid height has to sit
// to the project's RLKHEIGHT to count as the same height. Both sides are
// decimal strings out of SoundPLAN files, so this guards the parse, not a
// modelling choice.
const gridMapHeightToleranceM = 1e-6

// selectSoundPlanGridMapRun picks the one SoundPLAN grid map the raster
// comparison is run against.
//
// The raster path used to compare a single Aconiq run against every grid map
// the bundle held. In the reference project that is four, computed without and
// with the noise barrier and at two grid heights, so at least two of the four
// comparisons were against a scenario the model does not describe — the same
// defect the receiver path closed by selecting one RSPS run.
//
// The evidence is read from the metadata the import carried forward rather
// than from a .res found by name, as selectSoundPlanReceiverResultDir has to
// do: LoadGridMapMetadata already holds the run each grid map came from, so
// deriving a file name from a result subfolder would be a convention this code
// does not need to depend on twice. The cost is that an import report written
// before those fields existed carries no evidence — which lands on the
// ambiguous branch, with a warning, rather than on a wrong answer.
//
// Absent evidence and contradicted evidence are not the same branch. When every
// candidate declared its geometry and every one of them disagrees with the
// model, the height is not consulted at all: it would find one run and report
// gridRunSelectionGridHeight for a scenario already proven wrong. That lands on
// gridRunSelectionContradicted instead — still a selection, so the command stays
// usable, but named and warned about as the mismatch it is.
//
// A Dir of "" means no grid map named a result subfolder at all; the caller
// reports that rather than selecting nothing silently.
func selectSoundPlanGridMapRun(
	gridMaps []soundplanimport.GridMapMetadata,
	explicitRun string,
	modelHasBarriers bool,
	gridMapHeightM float64,
) (soundPlanResultRunSelection, error) {
	candidates, byName := gridMapRunCandidates(gridMaps)

	selection := soundPlanResultRunSelection{Candidates: candidates}

	if requested := strings.TrimSpace(explicitRun); requested != "" {
		if !slices.Contains(candidates, requested) {
			return soundPlanResultRunSelection{}, domainerrors.New(
				domainerrors.KindUserInput, "cli.compare",
				fmt.Sprintf("--soundplan-grid-run %q is not one of the available grid-map runs: %s", requested, strings.Join(candidates, ", ")),
				nil,
			)
		}

		selection.Dir = requested
		selection.Selection = resultRunSelectionExplicit

		return selection, nil
	}

	if len(candidates) == 0 {
		return selection, nil
	}

	if len(candidates) == 1 {
		selection.Dir = candidates[0]
		selection.Selection = resultRunSelectionOnly

		return selection, nil
	}

	pool := candidates

	matching, contradicted := filterGridMapRunsByBarrier(pool, byName, modelHasBarriers)
	if len(matching) > 0 {
		pool = matching

		if len(pool) == 1 {
			selection.Dir = pool[0]
			selection.Selection = resultRunSelectionGeometry

			return selection, nil
		}
	}

	// The height is only allowed to settle the choice while the geometry has
	// not already ruled every candidate out. Falling through to it here is what
	// turns a proven mismatch into a `grid_height_match` nobody warned about:
	// an edited model that adds a barrier none of the imported grid maps used
	// still finds exactly one run at RLKHEIGHT, and the report then presents
	// the wrong scenario as the confidently chosen one.
	if !contradicted {
		if byHeight := filterGridMapRunsByHeight(pool, byName, gridMapHeightM); len(byHeight) == 1 {
			selection.Dir = byHeight[0]
			selection.Selection = gridRunSelectionGridHeight

			return selection, nil
		}
	}

	selection.Dir = pool[len(pool)-1]

	if contradicted {
		selection.Selection = gridRunSelectionContradicted
		selection.Warnings = append(selection.Warnings, fmt.Sprintf(
			"none of the SoundPLAN grid-map runs %s was computed with the geometry the model describes: "+
				"the model %s, and every run says the opposite. Compared against %s by name order, so these deltas "+
				"measure a scenario the model does not describe. Pass --soundplan-grid-run to choose deliberately, "+
				"or re-import the bundle the model was edited away from.",
			strings.Join(pool, ", "), modelBarrierPhrase(modelHasBarriers), selection.Dir,
		))

		return selection, nil
	}

	selection.Selection = resultRunSelectionAmbiguous
	selection.Warnings = append(selection.Warnings, fmt.Sprintf(
		"SoundPLAN grid-map runs %s could not be told apart by the geometry they used or the height they were computed at; "+
			"compared against %s by name order. Pass --soundplan-grid-run to choose.",
		strings.Join(pool, ", "), selection.Dir,
	))

	return selection, nil
}

// modelBarrierPhrase names the model's barrier state the way the contradiction
// warning reads it, so the message says which side of the disagreement the
// model is on rather than printing a bare boolean.
func modelBarrierPhrase(modelHasBarriers bool) string {
	if modelHasBarriers {
		return "carries a noise barrier"
	}

	return "carries no noise barrier"
}

// gridMapRunCandidates lists the grid maps that name a result subfolder, sorted
// by name and deduplicated, together with an index from that name back to the
// metadata the selection reads its evidence from.
func gridMapRunCandidates(gridMaps []soundplanimport.GridMapMetadata) ([]string, map[string]soundplanimport.GridMapMetadata) {
	byName := make(map[string]soundplanimport.GridMapMetadata, len(gridMaps))
	names := make([]string, 0, len(gridMaps))

	for _, gridMap := range gridMaps {
		name := strings.TrimSpace(gridMap.ResultSubFolder)
		if name == "" {
			continue
		}

		if _, seen := byName[name]; seen {
			continue
		}

		byName[name] = gridMap

		names = append(names, name)
	}

	slices.Sort(names)

	return names, byName
}

// filterGridMapRunsByBarrier keeps the runs whose geometry list agrees with
// whether the model carries barriers. A run that declared no geometry is
// unknown and is kept out of the answer rather than guessed at.
//
// The second return separates the two ways the first can come back empty, which
// the caller must not treat alike:
//
//   - *no evidence*: at least one candidate declared no geometry at all, so the
//     pool has not been ruled out, only left unproven. An import report written
//     before GridMapMetadata.GeometryFiles existed is the whole of this case.
//   - *contradicted* (true): every candidate declared its geometry and every one
//     of them disagrees with the model. Nothing in the pool can be the scenario
//     the model describes, and no later discriminator can change that.
//
// A partial list — some known and disagreeing, some unknown — counts as no
// evidence: an unknown candidate could still be the right run, and refusing to
// look at the height would throw away the one signal left.
func filterGridMapRunsByBarrier(
	candidates []string,
	byName map[string]soundplanimport.GridMapMetadata,
	modelHasBarriers bool,
) ([]string, bool) {
	matching := make([]string, 0, len(candidates))
	knownCount := 0

	for _, name := range candidates {
		usedBarrier, known := soundPlanGeometryUsedBarrier(byName[name].GeometryFiles)
		if !known {
			continue
		}

		knownCount++

		if usedBarrier == modelHasBarriers {
			matching = append(matching, name)
		}
	}

	contradicted := len(matching) == 0 && knownCount == len(candidates) && knownCount > 0

	return matching, contradicted
}

// filterGridMapRunsByHeight keeps the runs computed at the project's grid-map
// height (RLKHEIGHT). A project that recorded no height, or a run that declared
// none, is no evidence and matches nothing.
//
// This decides which run is compared, not where the receivers go: the synthetic
// receivers take the selected run's own height, so that a run reached by any
// other branch is still compared at the height it was computed at — see
// selectedRunReceiverHeight.
func filterGridMapRunsByHeight(
	candidates []string,
	byName map[string]soundplanimport.GridMapMetadata,
	gridMapHeightM float64,
) []string {
	if gridMapHeightM <= 0 {
		return nil
	}

	matching := make([]string, 0, len(candidates))

	for _, name := range candidates {
		layout := byName[name].RunLayout
		if layout == nil {
			continue
		}

		if math.Abs(layout.HeightM-gridMapHeightM) <= gridMapHeightToleranceM {
			matching = append(matching, name)
		}
	}

	return matching
}
