package modelgeojson

import "testing"

const groundZoneRing = `[[[350000,5800000],[350100,5800000],[350100,5800100],[350000,5800100],[350000,5800000]]]`

func groundZonePayload(properties string, geometry string) string {
	return `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": ` + properties + `,
      "geometry": ` + geometry + `
    }
  ]
}`
}

func TestValidateGroundZoneIsValid(t *testing.T) {
	t.Parallel()

	report := validateCalcAreaPayload(t, groundZonePayload(
		`{"id": "zone-1", "kind": "ground-zone", "ground_factor": 1}`,
		`{"type": "Polygon", "coordinates": `+groundZoneRing+`}`,
	))

	if !report.Valid {
		t.Fatalf("expected a ground zone to validate, got %#v", report.Errors)
	}
}

// TestValidateGroundZoneNeedsItsFactor pins that the factor is required rather
// than defaulted: a missing one is indistinguishable from ground the model says
// nothing about, which the run already answers with its global fallback.
func TestValidateGroundZoneNeedsItsFactor(t *testing.T) {
	t.Parallel()

	report := validateCalcAreaPayload(t, groundZonePayload(
		`{"id": "zone-1", "kind": "ground-zone"}`,
		`{"type": "Polygon", "coordinates": `+groundZoneRing+`}`,
	))

	featureID, found := hasErrorCode(report, "groundzone.factor.required")
	if !found {
		t.Fatalf("expected groundzone.factor.required, got %#v", report.Errors)
	}

	if featureID != "zone-1" {
		t.Fatalf("expected the error to name zone-1, got %q", featureID)
	}
}

func TestValidateGroundZoneRejectsAFactorOutsideTheUnitInterval(t *testing.T) {
	t.Parallel()

	factors := map[string]string{
		"above one":    `1.5`,
		"negative":     `-0.1`,
		"not a number": `"porous"`,
	}

	for name, factor := range factors {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report := validateCalcAreaPayload(t, groundZonePayload(
				`{"id": "zone-1", "kind": "ground-zone", "ground_factor": `+factor+`}`,
				`{"type": "Polygon", "coordinates": `+groundZoneRing+`}`,
			))

			if _, found := hasErrorCode(report, "groundzone.factor.invalid"); !found {
				t.Fatalf("expected groundzone.factor.invalid for %s, got %#v", name, report.Errors)
			}
		})
	}
}

func TestValidateGroundZoneRejectsNonPolygonGeometry(t *testing.T) {
	t.Parallel()

	geometries := map[string]string{
		"LineString":   `{"type": "LineString", "coordinates": [[350000,5800000],[350100,5800100]]}`,
		"Point":        `{"type": "Point", "coordinates": [350000,5800000]}`,
		"MultiPolygon": `{"type": "MultiPolygon", "coordinates": [` + groundZoneRing + `]}`,
	}

	for name, geometry := range geometries {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report := validateCalcAreaPayload(t, groundZonePayload(
				`{"id": "zone-1", "kind": "ground-zone", "ground_factor": 0.5}`,
				geometry,
			))

			if _, found := hasErrorCode(report, "groundzone.geometry.invalid"); !found {
				t.Fatalf("expected groundzone.geometry.invalid for %s, got %#v", name, report.Errors)
			}
		})
	}
}
