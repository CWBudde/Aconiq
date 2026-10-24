package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/citygmlimport"
)

// evidenceKindCityGMLLoD3 is the evidence kind of a window read from a
// client's CityGML LoD3 model (facade-receivers.md, Receiver properties).
const evidenceKindCityGMLLoD3 = "citygml-lod3"

// placeCityGMLOpenings places a CityGML document's LoD3 windows on the
// facades of the model its buildings became (P18 C1b), and returns the model
// with them appended. The outlines are in the document's CRS, importCRS, and
// move into the project CRS as its footprints did. A window that cannot be
// placed — its building was skipped, it lies in no facade plane, its id is
// taken — is recorded in report and left out; the buildings still import, as
// they do when one of them is skipped. Each window cites its gml:id and the
// file's sha256 as evidence.
func placeCityGMLOpenings(
	model modelgeojson.Model, openings []citygmlimport.Opening, importCRS, fileSHA256 string, report *citygmlimport.OpeningReport,
) (modelgeojson.Model, error) {
	specs := make([]modelgeojson.OpeningSpec, len(openings))

	for i, o := range openings {
		specs[i] = modelgeojson.OpeningSpec{
			ID: o.ID, BuildingRef: o.BuildingID, Vertices: o.Vertices, BaseZ: o.BaseZ,
			Evidence: []any{map[string]any{
				"kind": evidenceKindCityGMLLoD3, "source_id": o.ID, "content_sha256": fileSHA256,
			}},
		}
	}

	specs, err := modelgeojson.OpeningsInProjectCRS(specs, model.ProjectCRS, importCRS)
	if err != nil {
		return modelgeojson.Model{}, fmt.Errorf("move the window outlines into %s: %w", model.ProjectCRS, err)
	}

	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		return modelgeojson.Model{}, fmt.Errorf("no metric frame to place the windows in: %w", err)
	}

	kinds := make(map[string]string, len(model.Features))
	for _, feature := range model.Features {
		kinds[strings.TrimSpace(feature.ID)] = feature.Kind
	}

	out := model
	out.Features = slices.Clone(model.Features)

	for _, spec := range specs {
		skip := func(reason string) {
			report.Skipped = append(report.Skipped, citygmlimport.SkippedOpening{ID: spec.ID, Building: spec.BuildingRef, Reason: reason})
		}

		if kind, taken := kinds[spec.ID]; taken {
			skip(fmt.Sprintf("its id is already a %s feature of the model", kind))

			continue
		}

		feature, err := placer.OpeningFeature(spec)
		if err != nil {
			skip(err.Error())

			continue
		}

		out.Features = append(out.Features, feature)
		kinds[spec.ID] = feature.Kind
		report.Imported++
	}

	return out, nil
}
