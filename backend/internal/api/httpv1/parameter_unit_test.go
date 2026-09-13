package httpv1

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
)

// TestStandardsEndpointCarriesParameterUnits pins that the declared unit reaches
// the wire. Without it a consumer has to mirror the Go-side knowledge in a table
// of its own, which drifts from the declarations without anything noticing.
func TestStandardsEndpointCarriesParameterUnits(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	handler := NewHandlerWithRegistry(store, nil, registry)
	req := newAPIRequest(http.MethodGet, "/api/v1/standards", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response []standardResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	units := make(map[string]string)

	for _, s := range response {
		for _, version := range s.Versions {
			for _, profile := range version.Profiles {
				for _, parameter := range profile.Parameters {
					units[parameter.Name] = parameter.Unit
				}
			}
		}
	}

	// One parameter per unit symbol, spelled out rather than taken from the
	// framework constants: these are wire values a consumer renders, so
	// respelling a constant has to fail here.
	for name, expected := range map[string]string{
		"grid_resolution_m":           "m",
		"speed_pkw_kph":               "km/h",
		"ground_attenuation_db":       "dB",
		"air_absorption_db_per_km":    "dB/km",
		"traffic_day_lkw1":            "1/h",
		"intersection_density_per_km": "1/km",
		"gradient_percent":            "%",
		"road_temperature_c":          "°C",
		"bank_angle_deg":              "°",
	} {
		got, declared := units[name]
		if !declared {
			t.Fatalf("parameter %q is not published by any standard", name)
		}

		if got != expected {
			t.Errorf("parameter %q: expected unit %q, got %q", name, expected, got)
		}
	}

	// A dimensionless parameter carries no unit, and the JSON tag omits the key
	// rather than emitting an empty string — matching the other optional fields
	// on the definition.
	if got, declared := units["rail_braking_share"]; !declared || got != "" {
		t.Fatalf("rail_braking_share is dimensionless: expected an empty unit, got %q (declared=%v)", got, declared)
	}

	if strings.Contains(rec.Body.String(), `"unit":""`) {
		t.Fatal("a dimensionless parameter emitted an empty unit string; the json tag has lost omitempty")
	}
}

// assertOpenAPIDeclaresParameterUnit checks that the hand-built spec declares the
// unit a consumer receives on every parameter definition. ParameterDefinition is
// additionalProperties:false, so a unit the handler emits but the contract omits
// would break a strict client and disappear from a generated TypeScript client.
func assertOpenAPIDeclaresParameterUnit(t *testing.T, payload map[string]any) {
	t.Helper()

	components, ok := payload["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi schemas object")
	}

	definition, ok := schemas["ParameterDefinition"].(map[string]any)
	if !ok {
		t.Fatalf("expected ParameterDefinition schema")
	}

	properties, ok := definition["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected ParameterDefinition properties object")
	}

	unit, ok := properties["unit"].(map[string]any)
	if !ok {
		t.Fatalf("expected unit property, got %#v", properties["unit"])
	}

	if unit["type"] != "string" {
		t.Fatalf("unexpected unit type: %#v", unit["type"])
	}

	if description, isText := unit["description"].(string); !isText || description == "" {
		t.Fatalf("expected unit description, got %#v", unit["description"])
	}

	// Dimensionless parameters omit the unit, so it must stay optional.
	if required := anySliceToStrings(definition["required"]); slices.Contains(required, "unit") {
		t.Fatalf("unit must stay optional, got required %#v", definition["required"])
	}
}
