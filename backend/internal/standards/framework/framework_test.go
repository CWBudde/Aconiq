package framework

import "testing"

func TestResolveVersionProfileDefaults(t *testing.T) {
	t.Parallel()

	d := StandardDescriptor{
		Context:        StandardContextPlanning,
		ID:             "dummy",
		DefaultVersion: "v1",
		Versions: []Version{
			{
				Name:           "v1",
				DefaultProfile: "default",
				Profiles: []Profile{
					{
						Name:                 "default",
						SupportedSourceTypes: []string{"point"},
						SupportedIndicators:  []string{"Ldummy"},
						ParameterSchema: ParameterSchema{
							Parameters: []ParameterDefinition{
								{Name: "workers", Kind: ParameterKindInt, DefaultValue: "0"},
							},
						},
					},
				},
			},
		},
	}

	resolved, err := d.ResolveVersionProfile("", "")
	if err != nil {
		t.Fatalf("resolve defaults: %v", err)
	}

	if resolved.Version != "v1" {
		t.Fatalf("expected version v1, got %s", resolved.Version)
	}

	if resolved.Profile != "default" {
		t.Fatalf("expected profile default, got %s", resolved.Profile)
	}
}

func TestParameterSchemaNormalizeAndValidate(t *testing.T) {
	t.Parallel()

	minFloat := 0.0
	minInt := 0.0
	maxInt := 16.0
	schema := ParameterSchema{
		Parameters: []ParameterDefinition{
			{Name: "grid_resolution_m", Kind: ParameterKindFloat, Required: true, Min: &minFloat},
			{Name: "workers", Kind: ParameterKindInt, DefaultValue: "0", Min: &minInt, Max: &maxInt},
			{Name: "disable_cache", Kind: ParameterKindBool, DefaultValue: "false"},
		},
	}

	normalized, err := schema.NormalizeAndValidate(map[string]string{
		"grid_resolution_m": "10",
		"disable_cache":     "TRUE",
	})
	if err != nil {
		t.Fatalf("normalize params: %v", err)
	}

	if normalized["grid_resolution_m"] != "10" {
		t.Fatalf("unexpected grid_resolution_m: %q", normalized["grid_resolution_m"])
	}

	if normalized["workers"] != "0" {
		t.Fatalf("expected default workers=0, got %q", normalized["workers"])
	}

	if normalized["disable_cache"] != "true" {
		t.Fatalf("expected normalized bool true, got %q", normalized["disable_cache"])
	}
}

func TestParameterSchemaRejectsUnknownParameter(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{
		Parameters: []ParameterDefinition{
			{Name: "chunk_size", Kind: ParameterKindInt, DefaultValue: "128"},
		},
	}
	{
		_, err := schema.NormalizeAndValidate(map[string]string{"not_allowed": "1"})
		if err == nil {
			t.Fatal("expected unknown parameter error")
		}
	}
}
