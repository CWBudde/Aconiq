package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/io/windowimport"
	"github.com/spf13/cobra"
)

// mergeWindowListIfGiven merges the window list at path when there is one,
// and returns what the JSON result reports about it: nothing without a list.
func mergeWindowListIfGiven(cmd *cobra.Command, state commandState, store projectfs.Store, path string) (map[string]any, error) {
	if path == "" {
		return map[string]any{}, nil
	}

	count, err := mergeWindowList(cmd, state, store, path)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"windows_input": relativePath(store.Root(), resolvePath(store.Root(), path)),
		"window_count":  count,
	}, nil
}

// mergeWindowList reads the window list at path and writes its windows into
// the project's model (P18 C1). Each row becomes a window feature placed on
// the facade it names; a window whose id the model already holds as a window
// is replaced, so importing a corrected list again updates it. The whole list
// is refused on the first row that cannot be placed, and the model is written
// as a compare-and-write against the one the windows were placed on. It
// returns the number of windows written.
func mergeWindowList(cmd *cobra.Command, state commandState, store projectfs.Store, path string) (int, error) {
	const op = "cli.import"

	listPath := resolvePath(store.Root(), path)

	rows, err := windowimport.ReadFile(listPath)
	if err != nil {
		return 0, domainerrors.New(domainerrors.KindUserInput, op, "read the window list", err)
	}

	proj, err := store.Load()
	if err != nil {
		return 0, fmt.Errorf("load project manifest: %w", err)
	}

	raw, hash, err := store.ReadModelWithHash()

	var appErr *domainerrors.AppError
	if errors.As(err, &appErr) && appErr.Kind == domainerrors.KindNotFound {
		return 0, domainerrors.New(domainerrors.KindUserInput, op, "the project has no model to place windows on; import its buildings first", err)
	}

	if err != nil {
		return 0, fmt.Errorf("read model: %w", err)
	}

	model, err := modelgeojson.Normalize(raw, proj.CRS, store.RelativePath(store.ModelArtifactPaths().Normalized))
	if err != nil {
		return 0, domainerrors.New(domainerrors.KindValidation, op, "normalize model", err)
	}

	err = keepImportRecord(store, &model)
	if err != nil {
		return 0, err
	}

	windows, err := placeWindowRows(model, rows)
	if err != nil {
		return 0, domainerrors.New(domainerrors.KindUserInput, op, "window list "+relativePath(store.Root(), listPath), err)
	}

	merged, replaced := mergeWindows(model, windows)

	_, err = store.SaveModelIf(&proj, merged, modelgeojson.Validate(merged), hash)

	var mismatch *projectfs.HashMismatchError
	if errors.As(err, &mismatch) {
		return 0, domainerrors.New(domainerrors.KindUserInput, op,
			"the model changed while the windows were placed, so nothing was written; import the window list again", err)
	}

	if err != nil {
		return 0, fmt.Errorf("save model: %w", err)
	}

	sensitive := 0

	for _, row := range rows {
		if row.Sensitive {
			sensitive++
		}
	}

	state.Logger.Info("window list merged", "list", relativePath(store.Root(), listPath),
		"windows", len(windows), "sensitive", sensitive, "replaced", replaced)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Windows imported: %d (%d sensitive, %d replaced)\n", len(windows), sensitive, replaced)

	return len(windows), nil
}

// placeWindowRows turns every row into a window feature, refusing the list on
// the first row that names no building, no facade, or a window its facade has
// no room for, or whose id another kind of feature already holds.
func placeWindowRows(model modelgeojson.Model, rows []windowimport.Row) ([]modelgeojson.Feature, error) {
	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		return nil, fmt.Errorf("no metric frame to place windows in: %w", err)
	}

	kinds := make(map[string]string, len(model.Features))
	for _, feature := range model.Features {
		kinds[strings.TrimSpace(feature.ID)] = feature.Kind
	}

	windows := make([]modelgeojson.Feature, 0, len(rows))

	for _, row := range rows {
		if kind, taken := kinds[row.ID]; taken && kind != modelgeojson.FeatureKindWindow {
			return nil, fmt.Errorf("row %d: id %q is already a %s feature of the model", row.Row, row.ID, kind)
		}

		building, err := windowBuilding(row, model)
		if err != nil {
			return nil, err
		}

		feature, err := placer.Feature(modelgeojson.WindowSpec{
			ID:           row.ID,
			BuildingRef:  building,
			Part:         row.Part,
			Ring:         row.Ring,
			Edge:         row.Edge,
			Frame:        geo.FacadeWindow{UM: row.UM, SillZM: row.SillZM, WidthM: row.WidthM, HeightM: row.HeightM},
			Sensitive:    row.Sensitive,
			ReviewStatus: row.ReviewStatus,
			Evidence:     row.Evidence,
		})
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", row.Row, err)
		}

		windows = append(windows, feature)
	}

	return windows, nil
}

// windowBuilding resolves a row's building, by feature id or address, to the
// one feature whose facade the window is on. An address the LoD2 import split
// into parts names several, and a facade belongs to one of them.
func windowBuilding(row windowimport.Row, model modelgeojson.Model) (string, error) {
	ids, err := facadeset.ResolveBuildings([]string{row.Building}, model)
	if err != nil {
		return "", fmt.Errorf("row %d: %w", row.Row, err)
	}

	if len(ids) != 1 {
		return "", fmt.Errorf("row %d: building %q names %d features (%s); name the one the facade belongs to by its id",
			row.Row, row.Building, len(ids), strings.Join(ids, ", "))
	}

	return ids[0], nil
}

// mergeWindows replaces the model's windows of the same id and appends the
// rest, leaving the model passed in as it was. It returns the new model and
// how many windows it replaced.
func mergeWindows(model modelgeojson.Model, windows []modelgeojson.Feature) (modelgeojson.Model, int) {
	byID := make(map[string]modelgeojson.Feature, len(windows))
	for _, w := range windows {
		byID[w.ID] = w
	}

	out := model
	out.Features = slices.Clone(model.Features)
	replaced := 0

	for i, feature := range out.Features {
		w, ok := byID[strings.TrimSpace(feature.ID)]
		if !ok || feature.Kind != modelgeojson.FeatureKindWindow {
			continue
		}

		out.Features[i] = w
		delete(byID, w.ID)

		replaced++
	}

	for _, w := range windows {
		if _, pending := byID[w.ID]; pending {
			out.Features = append(out.Features, w)
		}
	}

	return out, replaced
}
