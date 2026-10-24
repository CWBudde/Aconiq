package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

// newReattachFacadesCommand is the explicit, reviewed step that re-attaches
// facade receivers whose building changed under them
// (facade-receivers.md, Decision: what keys a building). Validation reports
// such a receiver and a normative run refuses it; nothing else re-keys it.
func newReattachFacadesCommand() *cobra.Command {
	var (
		maxShiftM float64
		apply     bool
	)

	cmd := &cobra.Command{
		Use:   "reattach-facades",
		Short: "Re-key facade receivers whose building was re-imported, renumbered or moved",
		Long: "Proposes, for every receiver.facade.unresolved or receiver.facade.moved receiver, the building " +
			"edge that runs where its stored from / to say: within --max-shift-m of both, in the same direction, " +
			"so the neighbour across a party wall never matches. The receiver's own building is searched first, " +
			"then every other one. Several matches are ambiguous and none is chosen; none is no match.\n\n" +
			"Without --apply nothing is written. With it, each proposed receiver gets the new key and " +
			"review_status proposed, so a normative run refuses it until a person confirms it, or runs it " +
			"with --allow-unconfirmed. Its position, height, storey and placement rule stay as they are. A " +
			"window is re-keyed the same way, and its point moves to its centre on the new edge.\n\n" +
			"The default --max-shift-m is the 1 mm validation tolerance, which matches only the same edge: " +
			"a building re-imported under a new id, or one whose edges a CRS round trip renumbered. A wider " +
			"tolerance follows real edits of a footprint, and the shift is reported for review.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReattachFacadesCommand(cmd, maxShiftM, apply)
		},
	}

	cmd.Flags().Float64Var(&maxShiftM, "max-shift-m", modelgeojson.FacadeMoveToleranceM,
		"How far, in metres, a matched edge's endpoints may lie from the stored from / to")
	cmd.Flags().BoolVar(&apply, "apply", false, "Write the proposed keys into the model (default: report only)")

	return cmd
}

func runReattachFacadesCommand(cmd *cobra.Command, maxShiftM float64, apply bool) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.reattach-facades", "command state unavailable", nil)
	}

	if !(maxShiftM > 0) || math.IsInf(maxShiftM, 0) {
		return domainerrors.New(domainerrors.KindUserInput, "cli.reattach-facades",
			fmt.Sprintf("--max-shift-m %v must be a finite number of metres > 0", maxShiftM), nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	proposal, err := proposeProjectReattachments(store, maxShiftM)
	if err != nil {
		return err
	}

	applied := 0

	if apply {
		applied, err = proposal.save(store)
		if err != nil {
			return err
		}
	}

	counts := proposal.counts()

	state.Logger.Info(
		"facade receivers re-attached",
		"flagged", len(proposal.reattachments),
		modelgeojson.ReattachProposed, counts[modelgeojson.ReattachProposed],
		modelgeojson.ReattachAmbiguous, counts[modelgeojson.ReattachAmbiguous],
		modelgeojson.ReattachNoMatch, counts[modelgeojson.ReattachNoMatch],
		modelgeojson.ReattachUnchecked, counts[modelgeojson.ReattachUnchecked],
		"applied", applied,
	)

	if state.Config.JSONLogs {
		// A model with nothing to re-attach answers an empty list, not null.
		reattachments := proposal.reattachments
		if reattachments == nil {
			reattachments = []modelgeojson.FacadeReattachment{}
		}

		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":       "reattach-facades",
			"applied":       applied,
			"max_shift_m":   maxShiftM,
			"counts":        counts,
			"reattachments": reattachments,
		})
	}

	writeReattachments(cmd.OutOrStdout(), proposal.reattachments)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Proposed: %d, ambiguous: %d, no match: %d, unchecked: %d\n",
		counts[modelgeojson.ReattachProposed], counts[modelgeojson.ReattachAmbiguous],
		counts[modelgeojson.ReattachNoMatch], counts[modelgeojson.ReattachUnchecked])

	switch {
	case apply:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"Re-keyed %d receiver(s); each is review_status proposed until a person confirms it.\n", applied)
	case counts[modelgeojson.ReattachProposed] > 0:
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Nothing written; pass --apply to re-key the proposed receivers.")
	}

	return nil
}

// projectReattachment is what re-attaching proposes for a project's model,
// together with the hash of the model it was proposed from: the save is a
// compare-and-write against it.
type projectReattachment struct {
	proj          project.Project
	model         modelgeojson.Model
	hash          string
	reattachments []modelgeojson.FacadeReattachment
}

func proposeProjectReattachments(store projectfs.Store, maxShiftM float64) (projectReattachment, error) {
	proj, err := store.Load()
	if err != nil {
		return projectReattachment{}, fmt.Errorf("load project manifest: %w", err)
	}

	raw, hash, err := store.ReadModelWithHash()
	if err != nil {
		return projectReattachment{}, fmt.Errorf("read model: %w", err)
	}

	model, err := modelgeojson.Normalize(raw, proj.CRS, store.RelativePath(store.ModelArtifactPaths().Normalized))
	if err != nil {
		return projectReattachment{}, domainerrors.New(domainerrors.KindValidation, "cli.reattach-facades", "normalize model", err)
	}

	err = keepImportRecord(store, &model)
	if err != nil {
		return projectReattachment{}, err
	}

	reattachments, err := modelgeojson.ProposeFacadeReattachments(model, maxShiftM)
	if err != nil {
		return projectReattachment{}, domainerrors.New(domainerrors.KindValidation, "cli.reattach-facades", err.Error(), err)
	}

	return projectReattachment{proj: proj, model: model, hash: hash, reattachments: reattachments}, nil
}

// keepImportRecord puts the saved model's import record back on a model read
// from its normalized GeoJSON. That file carries the features only; the
// source, import CRS, transform and time of the import live in the dump beside
// it, and SaveModelIf rewrites the dump from the model it is given. Re-keying
// a receiver is not an import, so the record must survive it. A project
// without a dump has no record to keep; one whose dump does not read is
// refused rather than overwritten.
func keepImportRecord(store projectfs.Store, model *modelgeojson.Model) error {
	raw, err := os.ReadFile(store.ModelArtifactPaths().Dump)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.reattach-facades", "read model dump", err)
	}

	var dump modelgeojson.ModelDump

	err = json.Unmarshal(raw, &dump)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.reattach-facades", "decode model dump", err)
	}

	model.ImportCRS = dump.ImportCRS
	model.TransformApplied = dump.TransformApplied
	model.ImportedAt = dump.ImportedAt
	model.SourcePath = dump.SourcePath

	return nil
}

// save writes the proposed keys into the model, unless the model on disk is
// no longer the one they were proposed from. It returns how many receivers it
// re-keyed; with none to re-key it writes nothing.
func (p projectReattachment) save(store projectfs.Store) (int, error) {
	model, applied := modelgeojson.ApplyFacadeReattachments(p.model, p.reattachments)
	if applied == 0 {
		return 0, nil
	}

	_, err := store.SaveModelIf(&p.proj, model, modelgeojson.Validate(model), p.hash)

	var mismatch *projectfs.HashMismatchError
	if errors.As(err, &mismatch) {
		return 0, domainerrors.New(domainerrors.KindUserInput, "cli.reattach-facades",
			"the model changed since reattach-facades read it, so nothing was written; run it again on the current model", err)
	}

	if err != nil {
		return 0, fmt.Errorf("save model: %w", err)
	}

	return applied, nil
}

func (p projectReattachment) counts() map[string]int {
	counts := map[string]int{
		modelgeojson.ReattachProposed:  0,
		modelgeojson.ReattachAmbiguous: 0,
		modelgeojson.ReattachNoMatch:   0,
		modelgeojson.ReattachUnchecked: 0,
	}

	for _, r := range p.reattachments {
		counts[r.Outcome]++
	}

	return counts
}

func writeReattachments(w io.Writer, reattachments []modelgeojson.FacadeReattachment) {
	for _, r := range reattachments {
		switch r.Outcome {
		case modelgeojson.ReattachProposed:
			_, _ = fmt.Fprintf(w, "%s (%s): proposed on %s, endpoints %.4f m away, receiver %s from the edge\n",
				r.ReceiverID, r.Finding, describeFacadeKey(*r.Proposed), *r.ShiftM, describeDistance(r.ReceiverDistanceM))
		case modelgeojson.ReattachAmbiguous:
			candidates := make([]string, len(r.Candidates))
			for i, key := range r.Candidates {
				candidates[i] = describeFacadeKey(key)
			}

			_, _ = fmt.Fprintf(w, "%s (%s): ambiguous between %s\n", r.ReceiverID, r.Finding, strings.Join(candidates, " and "))
		default:
			_, _ = fmt.Fprintf(w, "%s (%s): %s, %s\n", r.ReceiverID, r.Finding, r.Outcome, r.Reason)
		}
	}
}

func describeFacadeKey(key geo.FacadeKey) string {
	return fmt.Sprintf("%q (part %d, ring %d, edge %d)", key.BuildingRef, key.Part, key.Ring, key.Edge)
}

func describeDistance(distance *float64) string {
	if distance == nil {
		return "at an unknown distance"
	}

	return fmt.Sprintf("%.3f m", *distance)
}
