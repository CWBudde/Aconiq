package framework

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// ptr is a shorthand for the *float64 bounds ParameterDefinition carries.
func ptr(v float64) *float64 { return &v }

// fullProfile is a profile that passes validation, so that a case can break
// exactly one thing about it.
func fullProfile(name string) Profile {
	return Profile{
		Name:                 name,
		SupportedSourceTypes: []string{"line"},
		SupportedIndicators:  []string{"LrT"},
	}
}

// mutate applies fn to a valid descriptor and returns the result.
func mutate(fn func(d *StandardDescriptor)) StandardDescriptor {
	d := descriptorWithTier(EvidenceTierNormative)
	fn(&d)

	return d
}

// Every refusal Validate can produce, pinned by the substring a reader has to
// see to know which field to fix. Validate is what stands between a malformed
// module and the registry, so each branch has to be reachable and each message
// has to name the thing that is wrong.
func TestStandardDescriptorValidateRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		descriptor  StandardDescriptor
		wantInError string
	}{
		{
			name:        "empty id",
			descriptor:  mutate(func(d *StandardDescriptor) { d.ID = "  " }),
			wantInError: "id is required",
		},
		{
			name:        "empty context",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Context = "" }),
			wantInError: "context must be",
		},
		{
			name:        "unknown context",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Context = "assessment" }),
			wantInError: "context must be",
		},
		{
			name:        "empty default version",
			descriptor:  mutate(func(d *StandardDescriptor) { d.DefaultVersion = " " }),
			wantInError: "default_version is required",
		},
		{
			name:        "no versions",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Versions = nil }),
			wantInError: "at least one version",
		},
		{
			name:        "version with empty name",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Versions[0].Name = "  " }),
			wantInError: "version with empty name",
		},
		{
			name: "duplicated version",
			descriptor: mutate(func(d *StandardDescriptor) {
				d.Versions = append(d.Versions, d.Versions[0])
			}),
			wantInError: `duplicated version "v1"`,
		},
		{
			name:        "default version not declared",
			descriptor:  mutate(func(d *StandardDescriptor) { d.DefaultVersion = "v2" }),
			wantInError: `default_version "v2" is not declared`,
		},
		{
			name:        "version without default profile",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Versions[0].DefaultProfile = "" }),
			wantInError: "default_profile is required",
		},
		{
			name:        "version without profiles",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Versions[0].Profiles = nil }),
			wantInError: "at least one profile",
		},
		{
			name:        "profile with empty name",
			descriptor:  mutate(func(d *StandardDescriptor) { d.Versions[0].Profiles[0].Name = " " }),
			wantInError: "profile with empty name",
		},
		{
			name: "duplicated profile",
			descriptor: mutate(func(d *StandardDescriptor) {
				d.Versions[0].Profiles = append(d.Versions[0].Profiles, fullProfile("default"))
			}),
			wantInError: `duplicated profile "default"`,
		},
		{
			name: "default profile not declared",
			descriptor: mutate(func(d *StandardDescriptor) {
				d.Versions[0].DefaultProfile = "detailed"
			}),
			wantInError: `default_profile "detailed" is not declared`,
		},
		{
			name: "profile without source types",
			descriptor: mutate(func(d *StandardDescriptor) {
				d.Versions[0].Profiles[0].SupportedSourceTypes = nil
			}),
			wantInError: "must declare supported source types",
		},
		{
			name: "profile without indicators",
			descriptor: mutate(func(d *StandardDescriptor) {
				d.Versions[0].Profiles[0].SupportedIndicators = nil
			}),
			wantInError: "must declare supported indicators",
		},
		{
			name: "profile with a broken parameter schema",
			descriptor: mutate(func(d *StandardDescriptor) {
				d.Versions[0].Profiles[0].ParameterSchema = ParameterSchema{
					Parameters: []ParameterDefinition{{Name: "speed", Kind: "number"}},
				}
			}),
			wantInError: "parameter schema",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.descriptor.Validate()
			if err == nil {
				t.Fatal("expected a validation error")
			}

			if !strings.Contains(err.Error(), testCase.wantInError) {
				t.Fatalf("error %q does not contain %q", err, testCase.wantInError)
			}
		})
	}
}

// A descriptor whose second version is the broken one must still be refused:
// validation walks every version, not just the default.
func TestStandardDescriptorValidateChecksEveryVersion(t *testing.T) {
	t.Parallel()

	descriptor := mutate(func(d *StandardDescriptor) {
		d.Versions = append(d.Versions, Version{
			Name:           "v2",
			DefaultProfile: "default",
			Profiles:       []Profile{{Name: "default", SupportedIndicators: []string{"LrT"}}},
		})
	})

	err := descriptor.Validate()
	if err == nil {
		t.Fatal("expected the second version to be rejected")
	}

	if !strings.Contains(err.Error(), `version "v2"`) {
		t.Fatalf("error %q does not name the offending version", err)
	}
}

// ResolveVersionProfile validates before it resolves, so a malformed descriptor
// cannot be used by asking for a version that happens to be well formed.
func TestResolveVersionProfileValidatesFirst(t *testing.T) {
	t.Parallel()

	resolved, err := descriptorWithTier("").ResolveVersionProfile("v1", "default")
	if err == nil {
		t.Fatalf("expected a validation error, resolved %#v", resolved)
	}

	if !strings.Contains(err.Error(), "evidence_tier") {
		t.Fatalf("error %q is not the tier refusal", err)
	}
}

// Explicit names are trimmed, so a flag value that picked up whitespace still
// resolves rather than reporting an unknown version.
func TestResolveVersionProfileTrimsAndFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	descriptor := mutate(func(d *StandardDescriptor) {
		d.Versions[0].Profiles = append(d.Versions[0].Profiles, fullProfile("detailed"))
		d.Versions = append(d.Versions, Version{
			Name:           "v2",
			DefaultProfile: "detailed",
			Profiles:       []Profile{fullProfile("detailed"), fullProfile("default")},
		})
	})

	cases := []struct {
		name        string
		version     string
		profile     string
		wantVersion string
		wantProfile string
	}{
		{name: "both empty use defaults", wantVersion: "v1", wantProfile: "default"},
		{name: "explicit version keeps its default profile", version: "v2", wantVersion: "v2", wantProfile: "detailed"},
		{name: "explicit profile only", profile: "detailed", wantVersion: "v1", wantProfile: "detailed"},
		{name: "both explicit", version: "v2", profile: "default", wantVersion: "v2", wantProfile: "default"},
		{name: "padded names", version: "  v2 ", profile: "\tdefault\n", wantVersion: "v2", wantProfile: "default"},
		{name: "whitespace only falls back", version: "   ", profile: "  ", wantVersion: "v1", wantProfile: "default"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := descriptor.ResolveVersionProfile(testCase.version, testCase.profile)
			if err != nil {
				t.Fatalf("resolve(%q, %q): %v", testCase.version, testCase.profile, err)
			}

			if resolved.Version != testCase.wantVersion || resolved.Profile != testCase.wantProfile {
				t.Fatalf("resolved %q/%q, want %q/%q", resolved.Version, resolved.Profile, testCase.wantVersion, testCase.wantProfile)
			}
		})
	}
}

// A profile name is looked up inside the resolved version only. "default"
// exists under v2 here but not under v1, so asking for it under v1 must fail
// rather than silently find the one next door.
func TestResolveVersionProfileScopesProfilesToTheirVersion(t *testing.T) {
	t.Parallel()

	descriptor := mutate(func(d *StandardDescriptor) {
		d.Versions[0].Profiles = []Profile{fullProfile("detailed")}
		d.Versions[0].DefaultProfile = "detailed"
		d.Versions = append(d.Versions, Version{
			Name:           "v2",
			DefaultProfile: "default",
			Profiles:       []Profile{fullProfile("default")},
		})
	})

	_, err := descriptor.ResolveVersionProfile("v1", "default")
	if err == nil {
		t.Fatal("expected v1 to refuse a profile that only v2 declares")
	}

	if !strings.Contains(err.Error(), `version "v1" does not provide profile "default"`) {
		t.Fatalf("error %q does not scope the refusal to v1", err)
	}
}

// The resolved profile must not alias the descriptor: a run that appends to its
// own supported-indicator list would otherwise mutate the registered module for
// every later run in the process.
func TestResolveVersionProfileReturnsIndependentSlices(t *testing.T) {
	t.Parallel()

	descriptor := mutate(func(d *StandardDescriptor) {
		d.Versions[0].Profiles[0].SupportedSourceTypes = []string{"line", "area"}
		d.Versions[0].Profiles[0].SupportedIndicators = []string{"LrT", "LrN"}
	})

	resolved, err := descriptor.ResolveVersionProfile("", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	resolved.SupportedSourceTypes[0] = "mutated"
	resolved.SupportedIndicators[0] = "mutated"

	if descriptor.Versions[0].Profiles[0].SupportedSourceTypes[0] != "line" {
		t.Fatal("ResolvedProfile.SupportedSourceTypes aliases the descriptor")
	}

	if descriptor.Versions[0].Profiles[0].SupportedIndicators[0] != "LrT" {
		t.Fatal("ResolvedProfile.SupportedIndicators aliases the descriptor")
	}
}

// parameterSchemaFixture is a schema exercising every field cloneParameterSchema
// has to carry.
func parameterSchemaFixture() ParameterSchema {
	return ParameterSchema{
		Parameters: []ParameterDefinition{
			{
				Name:         "speed_pkw_kph",
				Kind:         ParameterKindFloat,
				Unit:         UnitKilometersPerHour,
				Required:     true,
				DefaultValue: "50",
				Description:  "car speed",
				Min:          ptr(0),
				Max:          ptr(130),
			},
			{
				Name:        "surface",
				Kind:        ParameterKindString,
				Description: "road surface class",
				Enum:        []string{"asphalt", "concrete"},
			},
			{
				Name: "receiver_height_m",
				Kind: ParameterKindFloat,
				Unit: UnitMeter,
			},
		},
	}
}

// Unit is what a report or the API renders next to a value. It is carried by
// cloneParameterSchema by hand, field by field, so dropping it is a one-line
// mistake that nothing else in the pipeline would notice — every parameter
// would simply become dimensionless.
func TestResolveVersionProfileCarriesEveryParameterField(t *testing.T) {
	t.Parallel()

	descriptor := mutate(func(d *StandardDescriptor) {
		d.Versions[0].Profiles[0].ParameterSchema = parameterSchemaFixture()
	})

	resolved, err := descriptor.ResolveVersionProfile("", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	want := parameterSchemaFixture().Parameters

	got := resolved.RunParameterSchema.Parameters
	if len(got) != len(want) {
		t.Fatalf("resolved %d parameters, want %d", len(got), len(want))
	}

	for i, param := range got {
		if param.Name != want[i].Name {
			t.Fatalf("parameter %d: Name = %q, want %q", i, param.Name, want[i].Name)
		}

		if param.Unit != want[i].Unit {
			t.Fatalf("parameter %q: Unit = %q, want %q", param.Name, param.Unit, want[i].Unit)
		}

		if param.Kind != want[i].Kind {
			t.Fatalf("parameter %q: Kind = %q, want %q", param.Name, param.Kind, want[i].Kind)
		}

		if param.Required != want[i].Required {
			t.Fatalf("parameter %q: Required = %t, want %t", param.Name, param.Required, want[i].Required)
		}

		if param.DefaultValue != want[i].DefaultValue {
			t.Fatalf("parameter %q: DefaultValue = %q, want %q", param.Name, param.DefaultValue, want[i].DefaultValue)
		}

		if param.Description != want[i].Description {
			t.Fatalf("parameter %q: Description = %q, want %q", param.Name, param.Description, want[i].Description)
		}

		if strings.Join(param.Enum, ",") != strings.Join(want[i].Enum, ",") {
			t.Fatalf("parameter %q: Enum = %v, want %v", param.Name, param.Enum, want[i].Enum)
		}

		switch {
		case (param.Min == nil) != (want[i].Min == nil):
			t.Fatalf("parameter %q: Min presence differs", param.Name)
		case param.Min != nil && *param.Min != *want[i].Min:
			t.Fatalf("parameter %q: Min = %g, want %g", param.Name, *param.Min, *want[i].Min)
		}

		switch {
		case (param.Max == nil) != (want[i].Max == nil):
			t.Fatalf("parameter %q: Max presence differs", param.Name)
		case param.Max != nil && *param.Max != *want[i].Max:
			t.Fatalf("parameter %q: Max = %g, want %g", param.Name, *param.Max, *want[i].Max)
		}
	}
}

// Min and Max are pointers, so a shallow copy would hand every resolved profile
// the descriptor's own bounds to write through.
func TestResolveVersionProfileDeepCopiesParameterBounds(t *testing.T) {
	t.Parallel()

	descriptor := mutate(func(d *StandardDescriptor) {
		d.Versions[0].Profiles[0].ParameterSchema = parameterSchemaFixture()
	})

	resolved, err := descriptor.ResolveVersionProfile("", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	first := resolved.RunParameterSchema.Parameters[0]

	if first.Min == descriptor.Versions[0].Profiles[0].ParameterSchema.Parameters[0].Min {
		t.Fatal("cloned Min points at the descriptor's own value")
	}

	*first.Min = -999
	*first.Max = 999
	first.Enum = append(first.Enum, "mutated")

	original := descriptor.Versions[0].Profiles[0].ParameterSchema.Parameters[0]
	if *original.Min != 0 || *original.Max != 130 {
		t.Fatalf("descriptor bounds moved to [%g, %g]", *original.Min, *original.Max)
	}

	second := resolved.RunParameterSchema.Parameters[1]

	second.Enum[0] = "mutated"

	if descriptor.Versions[0].Profiles[0].ParameterSchema.Parameters[1].Enum[0] != "asphalt" {
		t.Fatal("cloned Enum aliases the descriptor")
	}
}

func TestParameterSchemaValidateRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		schema      ParameterSchema
		wantInError string
	}{
		{
			name:        "empty name",
			schema:      ParameterSchema{Parameters: []ParameterDefinition{{Name: "   ", Kind: ParameterKindInt}}},
			wantInError: "parameter name is required",
		},
		{
			name: "duplicated name",
			schema: ParameterSchema{Parameters: []ParameterDefinition{
				{Name: "workers", Kind: ParameterKindInt},
				{Name: "workers", Kind: ParameterKindInt},
			}},
			wantInError: `parameter "workers" is duplicated`,
		},
		{
			name:        "unsupported kind",
			schema:      ParameterSchema{Parameters: []ParameterDefinition{{Name: "workers", Kind: "number"}}},
			wantInError: `unsupported kind "number"`,
		},
		{
			name:        "empty kind",
			schema:      ParameterSchema{Parameters: []ParameterDefinition{{Name: "workers"}}},
			wantInError: "unsupported kind",
		},
		{
			name: "min above max",
			schema: ParameterSchema{Parameters: []ParameterDefinition{
				{Name: "speed", Kind: ParameterKindFloat, Min: ptr(130), Max: ptr(30)},
			}},
			wantInError: "min (130) exceeds max (30)",
		},
		{
			name: "default outside the range",
			schema: ParameterSchema{Parameters: []ParameterDefinition{
				{Name: "speed", Kind: ParameterKindFloat, DefaultValue: "200", Min: ptr(0), Max: ptr(130)},
			}},
			wantInError: `parameter "speed" default:`,
		},
		{
			name: "default not in the enum",
			schema: ParameterSchema{Parameters: []ParameterDefinition{
				{Name: "surface", Kind: ParameterKindString, DefaultValue: "gravel", Enum: []string{"asphalt"}},
			}},
			wantInError: `parameter "surface" default:`,
		},
		{
			name: "default of the wrong kind",
			schema: ParameterSchema{Parameters: []ParameterDefinition{
				{Name: "workers", Kind: ParameterKindInt, DefaultValue: "many"},
			}},
			wantInError: "expected int",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.schema.Validate()
			if err == nil {
				t.Fatal("expected a validation error")
			}

			if !strings.Contains(err.Error(), testCase.wantInError) {
				t.Fatalf("error %q does not contain %q", err, testCase.wantInError)
			}
		})
	}
}

// min == max is a legitimate one-value range, not an error.
func TestParameterSchemaValidateAcceptsACollapsedRange(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{
		{Name: "speed", Kind: ParameterKindFloat, DefaultValue: "50", Min: ptr(50), Max: ptr(50)},
	}}

	err := schema.Validate()
	if err != nil {
		t.Fatalf("validate collapsed range: %v", err)
	}
}

// NormalizeAndValidate is what turns a --param flag into the string recorded in
// provenance.json, so both the refusals and the exact normalized spelling
// matter: a value that round-trips differently would make two identical runs
// look different.
func TestNormalizeAndValidateScalars(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{
		{Name: "flag", Kind: ParameterKindBool},
		{Name: "count", Kind: ParameterKindInt, Min: ptr(1), Max: ptr(16)},
		{Name: "speed", Kind: ParameterKindFloat, Min: ptr(0), Max: ptr(130)},
		{Name: "surface", Kind: ParameterKindString, Enum: []string{"asphalt", "concrete"}},
		{Name: "label", Kind: ParameterKindString},
	}}

	accepted := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "bool TRUE normalizes", key: "flag", value: "TRUE", want: "true"},
		{name: "bool 1 normalizes", key: "flag", value: "1", want: "true"},
		{name: "bool 0 normalizes", key: "flag", value: "0", want: "false"},
		{name: "int at the minimum", key: "count", value: "1", want: "1"},
		{name: "int at the maximum", key: "count", value: "16", want: "16"},
		{name: "int keeps its spelling", key: "count", value: "+8", want: "8"},
		{name: "float at the minimum", key: "speed", value: "0", want: "0"},
		{name: "float at the maximum", key: "speed", value: "130", want: "130"},
		{name: "float loses trailing zeros", key: "speed", value: "50.500", want: "50.5"},
		{name: "float exponent is expanded", key: "speed", value: "1e2", want: "100"},
		{name: "value is trimmed", key: "speed", value: "  42  ", want: "42"},
		{name: "enum member", key: "surface", value: "concrete", want: "concrete"},
		{name: "free string passes through", key: "label", value: "Nordseite", want: "Nordseite"},
	}

	for _, testCase := range accepted {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := schema.NormalizeAndValidate(map[string]string{testCase.key: testCase.value})
			if err != nil {
				t.Fatalf("normalize %s=%q: %v", testCase.key, testCase.value, err)
			}

			if normalized[testCase.key] != testCase.want {
				t.Fatalf("%s = %q, want %q", testCase.key, normalized[testCase.key], testCase.want)
			}
		})
	}

	refused := []struct {
		name        string
		key         string
		value       string
		wantInError string
	}{
		{name: "bool nonsense", key: "flag", value: "yes", wantInError: "expected bool"},
		{name: "int nonsense", key: "count", value: "eight", wantInError: "expected int"},
		{name: "int with a fraction", key: "count", value: "8.0", wantInError: "expected int"},
		{name: "int below the minimum", key: "count", value: "0", wantInError: "below minimum 1"},
		{name: "int above the maximum", key: "count", value: "17", wantInError: "exceeds maximum 16"},
		{name: "float nonsense", key: "speed", value: "fast", wantInError: "expected finite float"},
		{name: "float NaN", key: "speed", value: "NaN", wantInError: "expected finite float"},
		{name: "float +Inf", key: "speed", value: "Inf", wantInError: "expected finite float"},
		{name: "float -Inf", key: "speed", value: "-Inf", wantInError: "expected finite float"},
		{name: "float below the minimum", key: "speed", value: "-0.1", wantInError: "below minimum 0"},
		{name: "float above the maximum", key: "speed", value: "130.1", wantInError: "exceeds maximum 130"},
		{name: "value outside the enum", key: "surface", value: "gravel", wantInError: "must be one of [asphalt, concrete]"},
	}

	for _, testCase := range refused {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := schema.NormalizeAndValidate(map[string]string{testCase.key: testCase.value})
			if err == nil {
				t.Fatalf("expected %s=%q to be refused, got %#v", testCase.key, testCase.value, normalized)
			}

			if !strings.Contains(err.Error(), testCase.wantInError) {
				t.Fatalf("error %q does not contain %q", err, testCase.wantInError)
			}

			// The parameter has to be named: the caller typed a flag, not a
			// schema index.
			if !strings.Contains(err.Error(), `parameter "`+testCase.key+`"`) {
				t.Fatalf("error %q does not name the parameter", err)
			}
		})
	}
}

// NaN and the infinities would sail through a bounds check (every comparison
// against NaN is false), so they have to be rejected by the parse, not by the
// range. Pinned here without a Min or Max to prove the rejection is not the
// range check doing the work by accident.
func TestNormalizeAndValidateRefusesNonFiniteFloatsWithoutBounds(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{{Name: "correction_db", Kind: ParameterKindFloat}}}

	for _, value := range []string{"NaN", "nan", "Inf", "+Inf", "-Inf", "infinity"} {
		_, err := schema.NormalizeAndValidate(map[string]string{"correction_db": value})
		if err == nil {
			t.Fatalf("value %q was accepted as a finite float", value)
		}
	}

	// A large but finite value is still fine.
	normalized, err := schema.NormalizeAndValidate(map[string]string{"correction_db": "1e300"})
	if err != nil {
		t.Fatalf("finite 1e300: %v", err)
	}

	if normalized["correction_db"] == "" || strings.ContainsAny(normalized["correction_db"], "eE") {
		t.Fatalf("correction_db = %q, want the expanded decimal form", normalized["correction_db"])
	}

	parsedBack := normalized["correction_db"]
	if len(parsedBack) < 300 {
		t.Fatalf("1e300 normalized to %q, which is too short to be that number", parsedBack)
	}
}

// Defaults fill in, blanks fall back to the default, and a required parameter
// with neither is refused.
func TestNormalizeAndValidateDefaultsAndRequirements(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{
		{Name: "needed", Kind: ParameterKindFloat, Required: true},
		{Name: "needed_with_default", Kind: ParameterKindInt, Required: true, DefaultValue: "4"},
		{Name: "optional", Kind: ParameterKindString},
		{Name: "optional_with_default", Kind: ParameterKindBool, DefaultValue: "false"},
	}}

	normalized, err := schema.NormalizeAndValidate(map[string]string{"needed": "1.5"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if normalized["needed"] != "1.5" {
		t.Fatalf("needed = %q", normalized["needed"])
	}

	if normalized["needed_with_default"] != "4" {
		t.Fatalf("needed_with_default = %q, want the default 4", normalized["needed_with_default"])
	}

	if normalized["optional_with_default"] != "false" {
		t.Fatalf("optional_with_default = %q, want the default false", normalized["optional_with_default"])
	}

	// An optional parameter with no default and no value is absent from the
	// result, not present as the empty string: provenance records what was
	// used, and "" is not a value any module used.
	if _, present := normalized["optional"]; present {
		t.Fatalf("optional appeared in the normalized map as %q", normalized["optional"])
	}

	// An explicitly blank value falls back to the default rather than failing.
	blank, err := schema.NormalizeAndValidate(map[string]string{"needed": "2", "needed_with_default": "   "})
	if err != nil {
		t.Fatalf("normalize blank: %v", err)
	}

	if blank["needed_with_default"] != "4" {
		t.Fatalf("blank value did not fall back to the default: %q", blank["needed_with_default"])
	}

	// A required parameter with no default and no value must be refused.
	_, err = schema.NormalizeAndValidate(nil)
	if err == nil {
		t.Fatal("expected a missing required parameter to be refused")
	}

	if !strings.Contains(err.Error(), `missing required run parameter "needed"`) {
		t.Fatalf("error %q does not name the missing parameter", err)
	}

	// So must a required parameter given as whitespace.
	_, err = schema.NormalizeAndValidate(map[string]string{"needed": "  "})
	if err == nil {
		t.Fatal("expected a blank required parameter to be refused")
	}
}

// A typo in --param must be reported, not ignored: a parameter that is silently
// dropped changes the result without appearing in provenance.
func TestNormalizeAndValidateRefusesUnknownParameters(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{{Name: "speed_pkw_kph", Kind: ParameterKindFloat}}}

	_, err := schema.NormalizeAndValidate(map[string]string{"speed_pkw_khp": "50"})
	if err == nil {
		t.Fatal("expected the misspelled parameter to be refused")
	}

	if !strings.Contains(err.Error(), `unknown run parameter "speed_pkw_khp"`) {
		t.Fatalf("error %q does not name the unknown parameter", err)
	}
}

// NormalizeAndValidate validates its own schema first, so a module whose schema
// is malformed cannot run with plausible-looking parameters.
func TestNormalizeAndValidateValidatesTheSchemaFirst(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{
		{Name: "speed", Kind: ParameterKindFloat},
		{Name: "speed", Kind: ParameterKindFloat},
	}}

	normalized, err := schema.NormalizeAndValidate(map[string]string{"speed": "50"})
	if err == nil {
		t.Fatalf("expected the duplicated parameter to be refused, got %#v", normalized)
	}

	if normalized != nil {
		t.Fatalf("expected a nil map alongside the error, got %#v", normalized)
	}
}

// An empty schema accepts nothing but an empty parameter map, and yields an
// empty — not nil — result so a caller can range over it.
func TestNormalizeAndValidateEmptySchema(t *testing.T) {
	t.Parallel()

	var schema ParameterSchema

	normalized, err := schema.NormalizeAndValidate(nil)
	if err != nil {
		t.Fatalf("normalize against an empty schema: %v", err)
	}

	if normalized == nil || len(normalized) != 0 {
		t.Fatalf("expected an empty map, got %#v", normalized)
	}

	_, err = schema.NormalizeAndValidate(map[string]string{"anything": "1"})
	if err == nil {
		t.Fatal("expected an empty schema to refuse every parameter")
	}
}

// Bounds are compared as float64 even for int parameters, so a bound that is
// not representable must not silently widen the accepted range.
func TestNormalizeAndValidateIntBoundsAreCheckedAsDeclared(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{
		{Name: "chunk", Kind: ParameterKindInt, Min: ptr(-8), Max: ptr(8)},
	}}

	for _, value := range []string{"-8", "-1", "0", "8"} {
		normalized, err := schema.NormalizeAndValidate(map[string]string{"chunk": value})
		if err != nil {
			t.Fatalf("chunk=%s: %v", value, err)
		}

		if normalized["chunk"] != value {
			t.Fatalf("chunk = %q, want %q", normalized["chunk"], value)
		}
	}

	for _, value := range []string{"-9", "9"} {
		_, err := schema.NormalizeAndValidate(map[string]string{"chunk": value})
		if err == nil {
			t.Fatalf("chunk=%s was accepted outside [-8, 8]", value)
		}
	}
}

// Every unit constant must be a distinct, non-empty symbol: the point of
// declaring them centrally is that "km/h" cannot acquire a second spelling.
func TestUnitConstantsAreDistinctSymbols(t *testing.T) {
	t.Parallel()

	units := map[string]string{
		"UnitMeter":               UnitMeter,
		"UnitKilometersPerHour":   UnitKilometersPerHour,
		"UnitDecibel":             UnitDecibel,
		"UnitDecibelPerKilometer": UnitDecibelPerKilometer,
		"UnitPerHour":             UnitPerHour,
		"UnitPerKilometer":        UnitPerKilometer,
		"UnitPercent":             UnitPercent,
		"UnitDegreeCelsius":       UnitDegreeCelsius,
		"UnitDegree":              UnitDegree,
	}

	seen := make(map[string]string, len(units))

	for name, symbol := range units {
		if strings.TrimSpace(symbol) == "" {
			t.Fatalf("%s is empty", name)
		}

		if symbol != strings.TrimSpace(symbol) {
			t.Fatalf("%s = %q carries surrounding whitespace", name, symbol)
		}

		if other, exists := seen[symbol]; exists {
			t.Fatalf("%s and %s both spell %q", name, other, symbol)
		}

		seen[symbol] = name
	}
}

// containsText backs the enum check and must be exact: no case folding, no
// prefix matching.
func TestEnumMatchingIsExact(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{
		{Name: "period", Kind: ParameterKindString, Enum: []string{"day", "night"}},
	}}

	for _, value := range []string{"Day", "DAY", "nigh", "nights", "day night"} {
		_, err := schema.NormalizeAndValidate(map[string]string{"period": value})
		if err == nil {
			t.Fatalf("value %q was accepted against the enum [day night]", value)
		}
	}

	for _, value := range []string{"day", "night"} {
		normalized, err := schema.NormalizeAndValidate(map[string]string{"period": value})
		if err != nil {
			t.Fatalf("value %q was refused: %v", value, err)
		}

		if normalized["period"] != value {
			t.Fatalf("period = %q, want %q", normalized["period"], value)
		}
	}
}

// The float formatting must round-trip: whatever provenance records has to
// parse back to the same float64, or a re-run from provenance is a different
// run.
func TestNormalizedFloatsRoundTrip(t *testing.T) {
	t.Parallel()

	schema := ParameterSchema{Parameters: []ParameterDefinition{{Name: "v", Kind: ParameterKindFloat}}}

	for _, value := range []string{"0.1", "1e-9", "1234567890.123456", "-0.0", "3.141592653589793"} {
		normalized, err := schema.NormalizeAndValidate(map[string]string{"v": value})
		if err != nil {
			t.Fatalf("normalize %q: %v", value, err)
		}

		again, err := schema.NormalizeAndValidate(map[string]string{"v": normalized["v"]})
		if err != nil {
			t.Fatalf("re-normalize %q: %v", normalized["v"], err)
		}

		if again["v"] != normalized["v"] {
			t.Fatalf("%q normalized to %q then %q", value, normalized["v"], again["v"])
		}

		want, err := strconv.ParseFloat(value, 64)
		if err != nil {
			t.Fatalf("parse %q: %v", value, err)
		}

		got, err := strconv.ParseFloat(normalized["v"], 64)
		if err != nil {
			t.Fatalf("parse %q: %v", normalized["v"], err)
		}

		if got != want && (!math.Signbit(want) || got != 0) {
			t.Fatalf("%q normalized to %q, which is %v not %v", value, normalized["v"], got, want)
		}
	}
}
