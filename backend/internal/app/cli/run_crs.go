package cli

import (
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// The provenance keys under which a run records the CRS it computed in.
// `aconiq export` reads compute_crs back to label the GIS formats it writes,
// so the two sides share these constants rather than repeating the strings.
const (
	provenanceProjectCRSKey = "project_crs"
	provenanceComputeCRSKey = "compute_crs"
)

// computeProjection records the CRS a run computed in and whether getting
// there moved the model. Results are expressed in ComputeCRS, so a consumer
// that only ever sees the results still learns which CRS they are in.
type computeProjection struct {
	ProjectCRS string
	ComputeCRS string
	Applied    bool
}

// resolveComputeModel returns the model a run computes on, together with the
// CRS that model is in.
//
// Every standards module measures distance with geo.Distance, which is
// math.Hypot over the coordinates it is handed. That is correct for a metric
// projected CRS and silently wrong for a geographic one: in degrees a whole
// city fits inside 0.05 units, every propagation distance falls under the
// modules' minimum-distance clamp, and each receiver reports the source's
// emission level verbatim. `aconiq init` defaults the project CRS to
// EPSG:4326, so that is the out-of-the-box path.
//
// A geographic model is therefore projected into a metric CRS before anything
// reads a coordinate off it. A model already in a projected CRS is returned
// untouched and reports Applied=false, so the ordinary German project — which
// is in EPSG:25832 — runs through exactly the code it ran through before.
//
// A CRS whose kind cannot be determined (a WKT: identifier, or an EPSG code
// outside the classified ranges) is also left alone. Refusing it would break
// projects that work today, and there is no transform to apply to a CRS with
// no EPSG code.
// isGeographicCRS reports whether an identifier names a CRS this project
// classifies as geographic. An identifier it cannot parse is not geographic:
// there is no EPSG code to transform through, so there is nothing this
// function's caller could do about it anyway.
func isGeographicCRS(id string) bool {
	crs, err := geo.ParseCRS(id)
	if err != nil {
		return false
	}

	return crs.Kind == geo.CRSKindGeographic
}

// epsgCRS parses an identifier that carries an EPSG code. The second result
// is false for anything else — a WKT: identifier, or an empty string — which
// callers read as "there is no transform to build", not as an error.
func epsgCRS(id string) (geo.CRS, bool) {
	crs, err := geo.ParseCRS(id)
	if err != nil || crs.EPSGCode() == 0 {
		return geo.CRS{}, false
	}

	return crs, true
}

func resolveComputeModel(model modelgeojson.Model, projectCRS string) (modelgeojson.Model, computeProjection, error) {
	projection := computeProjection{ProjectCRS: projectCRS, ComputeCRS: projectCRS}

	if !isGeographicCRS(projectCRS) {
		return model, projection, nil
	}

	bounds, ok := model.Bounds()
	if !ok {
		// Nothing to place. Extraction refuses an empty model with a message
		// about the model, which is the more useful error of the two.
		return model, projection, nil
	}

	centreLon := (bounds.MinX + bounds.MaxX) / 2
	centreLat := (bounds.MinY + bounds.MaxY) / 2

	computeCRS, err := geo.ComputeCRSForGeographic(centreLon, centreLat)
	if err != nil {
		return modelgeojson.Model{}, computeProjection{}, domainerrors.New(
			domainerrors.KindUserInput, "cli.resolveComputeModel",
			"project CRS "+projectCRS+" is geographic, so the model has to be projected before levels can be computed: "+err.Error(),
			err,
		)
	}

	reprojected, err := modelgeojson.Reproject(model, computeCRS.ID)
	if err != nil {
		return modelgeojson.Model{}, computeProjection{}, domainerrors.New(
			domainerrors.KindInternal, "cli.resolveComputeModel",
			"project model from "+projectCRS+" into "+computeCRS.ID,
			err,
		)
	}

	projection.ComputeCRS = computeCRS.ID
	projection.Applied = true

	return reprojected, projection, nil
}
