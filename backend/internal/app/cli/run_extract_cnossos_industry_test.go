package cli

import (
	"errors"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
)

// TestCnossosIndustryPartsRefusesAGeometryItCannotBuild covers the arm that
// used to return no sources and no error.
//
// It is unreachable through extractCnossosIndustrySources today, and that is
// the point: the guard above the switch admits only a source_type listed in
// the profile's SupportedSourceTypes, and cnossos-industry's single profile
// declares exactly {point, area}, which are also the only two SourceType
// constants the package defines. So the switch and the declaration agree, and
// nothing can fall through.
//
// What the missing default arm cost was the next person to add a third type:
// the model would validate, the run would succeed, and every source of that
// type would be absent from the result with nothing said. The test drives the
// function directly, because the public path cannot produce the input.
func TestCnossosIndustryPartsRefusesAGeometryItCannotBuild(t *testing.T) {
	t.Parallel()

	feature := modelgeojson.Feature{
		ID:           "src-line-1",
		Kind:         modelgeojson.FeatureKindSource,
		SourceType:   "line",
		GeometryType: "LineString",
		Coordinates:  []any{[]any{0.0, 0.0}, []any{10.0, 0.0}},
	}

	parts, err := cnossosIndustryParts(feature, "line")
	if err == nil {
		t.Fatalf("an unhandled source type yielded %d parts and no error", len(parts))
	}

	if parts != nil {
		t.Fatalf("expected no parts alongside the error, got %d", len(parts))
	}

	var appErr *domainerrors.AppError

	if !errors.As(err, &appErr) {
		t.Fatalf("error is not a domain error: %v", err)
	}

	if appErr.Kind != domainerrors.KindValidation {
		t.Fatalf("error kind is %v, want %v", appErr.Kind, domainerrors.KindValidation)
	}

	if !strings.Contains(err.Error(), "line") {
		t.Fatalf("error does not name the unhandled source type: %v", err)
	}
}

// TestCnossosIndustryPartsStillBuildsTheTwoDeclaredTypes pins that the new arm
// did not swallow the types the switch does handle.
func TestCnossosIndustryPartsStillBuildsTheTwoDeclaredTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		sourceType string
		feature    modelgeojson.Feature
	}{
		{
			sourceType: cnossosindustry.SourceTypePoint,
			feature: modelgeojson.Feature{
				ID: "src-point-1", Kind: modelgeojson.FeatureKindSource,
				SourceType: cnossosindustry.SourceTypePoint, GeometryType: "Point",
				Coordinates: []any{1.0, 2.0},
			},
		},
		{
			sourceType: cnossosindustry.SourceTypeArea,
			feature: modelgeojson.Feature{
				ID: "src-area-1", Kind: modelgeojson.FeatureKindSource,
				SourceType: cnossosindustry.SourceTypeArea, GeometryType: "Polygon",
				Coordinates: []any{[]any{
					[]any{0.0, 0.0}, []any{10.0, 0.0}, []any{10.0, 10.0}, []any{0.0, 10.0}, []any{0.0, 0.0},
				}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.sourceType, func(t *testing.T) {
			t.Parallel()

			parts, err := cnossosIndustryParts(testCase.feature, testCase.sourceType)
			if err != nil {
				t.Fatalf("build parts: %v", err)
			}

			if len(parts) == 0 {
				t.Fatal("expected at least one part")
			}
		})
	}
}
