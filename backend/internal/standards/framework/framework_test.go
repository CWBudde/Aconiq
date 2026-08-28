package framework

import (
	"strings"
	"testing"
)

func TestResolveVersionProfileDefaults(t *testing.T) {
	t.Parallel()

	d := StandardDescriptor{
		Context:        StandardContextPlanning,
		ID:             "dummy",
		EvidenceTier:   EvidenceTierTestFixture,
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

	if resolved.EvidenceTier != EvidenceTierTestFixture {
		t.Fatalf("expected evidence tier %q, got %q", EvidenceTierTestFixture, resolved.EvidenceTier)
	}
}

// descriptorWithTier builds a minimal valid descriptor carrying the given tier.
func descriptorWithTier(tier EvidenceTier) StandardDescriptor {
	return StandardDescriptor{
		Context:        StandardContextPlanning,
		ID:             "dummy",
		EvidenceTier:   tier,
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
					},
				},
			},
		},
	}
}

func TestValidateRejectsMissingOrUnknownEvidenceTier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tier EvidenceTier
	}{
		{name: "empty", tier: ""},
		{name: "unknown", tier: "provisional"},
		{name: "wrong case", tier: "Normative"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := descriptorWithTier(testCase.tier).Validate()
			if err == nil {
				t.Fatalf("expected error for evidence tier %q", testCase.tier)
			}

			if !strings.Contains(err.Error(), "evidence_tier") {
				t.Fatalf("expected evidence_tier error, got %v", err)
			}

			if !strings.Contains(err.Error(), "dummy") {
				t.Fatalf("expected the standard id in the error, got %v", err)
			}
		})
	}
}

func TestValidateAcceptsEveryEvidenceTier(t *testing.T) {
	t.Parallel()

	for _, tier := range []EvidenceTier{
		EvidenceTierNormative,
		EvidenceTierPreview,
		EvidenceTierScaffold,
		EvidenceTierTestFixture,
	} {
		err := descriptorWithTier(tier).Validate()
		if err != nil {
			t.Fatalf("validate tier %q: %v", tier, err)
		}
	}
}

func TestResolveVersionProfilePropagatesEvidenceTier(t *testing.T) {
	t.Parallel()

	for _, tier := range []EvidenceTier{
		EvidenceTierNormative,
		EvidenceTierPreview,
		EvidenceTierScaffold,
		EvidenceTierTestFixture,
	} {
		resolved, err := descriptorWithTier(tier).ResolveVersionProfile("", "")
		if err != nil {
			t.Fatalf("resolve tier %q: %v", tier, err)
		}

		if resolved.EvidenceTier != tier {
			t.Fatalf("expected resolved tier %q, got %q", tier, resolved.EvidenceTier)
		}
	}
}

func TestEvidenceTierOptInAndHeadline(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tier    EvidenceTier
		optIn   bool
		prefix  string
		nonZero bool
	}{
		{tier: EvidenceTierNormative, optIn: false, prefix: "normative", nonZero: true},
		{tier: EvidenceTierPreview, optIn: false, prefix: "preview", nonZero: true},
		{tier: EvidenceTierScaffold, optIn: true, prefix: "scaffold", nonZero: true},
		{tier: EvidenceTierTestFixture, optIn: false, prefix: "test fixture", nonZero: true},
		{tier: "unknown", optIn: false, prefix: "", nonZero: false},
	}

	for _, testCase := range cases {
		if got := testCase.tier.RequiresExperimentalOptIn(); got != testCase.optIn {
			t.Fatalf("tier %q opt-in: expected %v, got %v", testCase.tier, testCase.optIn, got)
		}

		headline := testCase.tier.Headline()
		if !testCase.nonZero {
			if headline != "" {
				t.Fatalf("tier %q: expected empty headline, got %q", testCase.tier, headline)
			}

			continue
		}

		if !strings.HasPrefix(headline, testCase.prefix) {
			t.Fatalf("tier %q headline %q does not start with %q", testCase.tier, headline, testCase.prefix)
		}

		if strings.Contains(headline, "\n") {
			t.Fatalf("tier %q headline must stay on one line: %q", testCase.tier, headline)
		}
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
