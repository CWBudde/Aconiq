package framework

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// ParameterKind defines the scalar type accepted for one run parameter.
type ParameterKind string

const (
	ParameterKindString ParameterKind = "string"
	ParameterKindBool   ParameterKind = "bool"
	ParameterKindInt    ParameterKind = "int"
	ParameterKindFloat  ParameterKind = "float"
)

// ParameterDefinition declares one supported run parameter.
type ParameterDefinition struct {
	Name         string
	Kind         ParameterKind
	Required     bool
	DefaultValue string
	Description  string
	Enum         []string
	Min          *float64
	Max          *float64
}

// ParameterSchema declares the accepted parameter map for one standard profile.
type ParameterSchema struct {
	Parameters []ParameterDefinition
}

// Profile describes one versioned standard profile.
type Profile struct {
	Name                 string
	SupportedSourceTypes []string
	SupportedIndicators  []string
	ParameterSchema      ParameterSchema
}

// Version groups profiles under one standard implementation version.
type Version struct {
	Name           string
	DefaultProfile string
	Profiles       []Profile
}

// StandardDescriptor captures standard identity and version/profile metadata.
type StandardDescriptor struct {
	ID             string
	Description    string
	DefaultVersion string
	Versions       []Version
}

// ResolvedProfile is one fully-resolved standard/version/profile tuple.
type ResolvedProfile struct {
	StandardID           string
	StandardDescription  string
	Version              string
	Profile              string
	SupportedSourceTypes []string
	SupportedIndicators  []string
	RunParameterSchema   ParameterSchema
}

// ResolveVersionProfile resolves explicit or default version/profile names.
func (d StandardDescriptor) ResolveVersionProfile(versionName string, profileName string) (ResolvedProfile, error) {
	versionName = strings.TrimSpace(versionName)
	profileName = strings.TrimSpace(profileName)

	err := d.Validate()
	if err != nil {
		return ResolvedProfile{}, err
	}

	if versionName == "" {
		versionName = d.DefaultVersion
	}

	var resolvedVersion *Version

	for i := range d.Versions {
		if d.Versions[i].Name == versionName {
			resolvedVersion = &d.Versions[i]
			break
		}
	}

	if resolvedVersion == nil {
		return ResolvedProfile{}, fmt.Errorf("standard %q does not provide version %q", d.ID, versionName)
	}

	if profileName == "" {
		profileName = resolvedVersion.DefaultProfile
	}

	var resolvedProfile *Profile

	for i := range resolvedVersion.Profiles {
		if resolvedVersion.Profiles[i].Name == profileName {
			resolvedProfile = &resolvedVersion.Profiles[i]
			break
		}
	}

	if resolvedProfile == nil {
		return ResolvedProfile{}, fmt.Errorf("standard %q version %q does not provide profile %q", d.ID, versionName, profileName)
	}

	return ResolvedProfile{
		StandardID:           d.ID,
		StandardDescription:  d.Description,
		Version:              resolvedVersion.Name,
		Profile:              resolvedProfile.Name,
		SupportedSourceTypes: append([]string(nil), resolvedProfile.SupportedSourceTypes...),
		SupportedIndicators:  append([]string(nil), resolvedProfile.SupportedIndicators...),
		RunParameterSchema:   cloneParameterSchema(resolvedProfile.ParameterSchema),
	}, nil
}

// NormalizeAndValidate fills defaults and validates all provided values.
// Returned map contains normalized string values for deterministic provenance.
func (s ParameterSchema) NormalizeAndValidate(raw map[string]string) (map[string]string, error) {
	err := s.Validate()
	if err != nil {
		return nil, err
	}

	normalized := make(map[string]string, len(s.Parameters))

	known := make(map[string]ParameterDefinition, len(s.Parameters))
	for _, param := range s.Parameters {
		known[param.Name] = param
	}

	for key := range raw {
		if _, ok := known[key]; !ok {
			return nil, fmt.Errorf("unknown run parameter %q", key)
		}
	}

	for _, param := range s.Parameters {
		value, provided := raw[param.Name]
		value = strings.TrimSpace(value)

		if !provided || value == "" {
			defaultValue := strings.TrimSpace(param.DefaultValue)
			if defaultValue != "" {
				value = defaultValue
				provided = true
			}
		}

		if !provided || value == "" {
			if param.Required {
				return nil, fmt.Errorf("missing required run parameter %q", param.Name)
			}

			continue
		}

		checked, err := validateScalar(param, value)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", param.Name, err)
		}

		normalized[param.Name] = checked
	}

	return normalized, nil
}

// Validate checks descriptor consistency.
func (d StandardDescriptor) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return errors.New("standard descriptor id is required")
	}

	if strings.TrimSpace(d.DefaultVersion) == "" {
		return fmt.Errorf("standard %q default_version is required", d.ID)
	}

	if len(d.Versions) == 0 {
		return fmt.Errorf("standard %q must define at least one version", d.ID)
	}

	versionNames := make(map[string]struct{}, len(d.Versions))
	defaultVersionFound := false

	for _, version := range d.Versions {
		versionName := strings.TrimSpace(version.Name)
		if versionName == "" {
			return fmt.Errorf("standard %q has version with empty name", d.ID)
		}

		if _, exists := versionNames[versionName]; exists {
			return fmt.Errorf("standard %q has duplicated version %q", d.ID, versionName)
		}

		versionNames[versionName] = struct{}{}
		if versionName == d.DefaultVersion {
			defaultVersionFound = true
		}

		err := validateVersion(d.ID, version)
		if err != nil {
			return err
		}
	}

	if !defaultVersionFound {
		return fmt.Errorf("standard %q default_version %q is not declared", d.ID, d.DefaultVersion)
	}

	return nil
}

// Validate checks schema consistency.
func (s ParameterSchema) Validate() error {
	names := make(map[string]struct{}, len(s.Parameters))
	for _, param := range s.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			return errors.New("parameter name is required")
		}

		if _, exists := names[name]; exists {
			return fmt.Errorf("parameter %q is duplicated", name)
		}

		names[name] = struct{}{}

		switch param.Kind {
		case ParameterKindString, ParameterKindBool, ParameterKindInt, ParameterKindFloat:
		default:
			return fmt.Errorf("parameter %q has unsupported kind %q", name, param.Kind)
		}

		if param.Min != nil && param.Max != nil && *param.Min > *param.Max {
			return fmt.Errorf("parameter %q min (%g) exceeds max (%g)", name, *param.Min, *param.Max)
		}

		defaultValue := strings.TrimSpace(param.DefaultValue)
		if defaultValue != "" {
			{
				_, err := validateScalar(param, defaultValue)
				if err != nil {
					return fmt.Errorf("parameter %q default: %w", name, err)
				}
			}
		}
	}

	return nil
}

func validateVersion(standardID string, version Version) error {
	if strings.TrimSpace(version.DefaultProfile) == "" {
		return fmt.Errorf("standard %q version %q default_profile is required", standardID, version.Name)
	}

	if len(version.Profiles) == 0 {
		return fmt.Errorf("standard %q version %q must define at least one profile", standardID, version.Name)
	}

	profileNames := make(map[string]struct{}, len(version.Profiles))
	defaultProfileFound := false

	for _, profile := range version.Profiles {
		profileName := strings.TrimSpace(profile.Name)
		if profileName == "" {
			return fmt.Errorf("standard %q version %q has profile with empty name", standardID, version.Name)
		}

		if _, exists := profileNames[profileName]; exists {
			return fmt.Errorf("standard %q version %q has duplicated profile %q", standardID, version.Name, profileName)
		}

		profileNames[profileName] = struct{}{}
		if profileName == version.DefaultProfile {
			defaultProfileFound = true
		}

		if len(profile.SupportedSourceTypes) == 0 {
			return fmt.Errorf("standard %q version %q profile %q must declare supported source types", standardID, version.Name, profileName)
		}

		if len(profile.SupportedIndicators) == 0 {
			return fmt.Errorf("standard %q version %q profile %q must declare supported indicators", standardID, version.Name, profileName)
		}

		err := profile.ParameterSchema.Validate()
		if err != nil {
			return fmt.Errorf("standard %q version %q profile %q parameter schema: %w", standardID, version.Name, profileName, err)
		}
	}

	if !defaultProfileFound {
		return fmt.Errorf("standard %q version %q default_profile %q is not declared", standardID, version.Name, version.DefaultProfile)
	}

	return nil
}

func cloneParameterSchema(schema ParameterSchema) ParameterSchema {
	parameters := make([]ParameterDefinition, 0, len(schema.Parameters))
	for _, parameter := range schema.Parameters {
		cloned := ParameterDefinition{
			Name:         parameter.Name,
			Kind:         parameter.Kind,
			Required:     parameter.Required,
			DefaultValue: parameter.DefaultValue,
			Description:  parameter.Description,
			Enum:         append([]string(nil), parameter.Enum...),
		}
		if parameter.Min != nil {
			min := *parameter.Min
			cloned.Min = &min
		}

		if parameter.Max != nil {
			max := *parameter.Max
			cloned.Max = &max
		}

		parameters = append(parameters, cloned)
	}

	return ParameterSchema{Parameters: parameters}
}

func validateScalar(parameter ParameterDefinition, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("value is empty")
	}

	switch parameter.Kind {
	case ParameterKindString:
		if len(parameter.Enum) > 0 {
			if !containsText(parameter.Enum, value) {
				return "", fmt.Errorf("value %q must be one of [%s]", value, strings.Join(parameter.Enum, ", "))
			}
		}

		return value, nil
	case ParameterKindBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return "", errors.New("expected bool")
		}

		return strconv.FormatBool(parsed), nil
	case ParameterKindInt:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return "", errors.New("expected int")
		}

		if parameter.Min != nil && float64(parsed) < *parameter.Min {
			return "", fmt.Errorf("value %d is below minimum %g", parsed, *parameter.Min)
		}

		if parameter.Max != nil && float64(parsed) > *parameter.Max {
			return "", fmt.Errorf("value %d exceeds maximum %g", parsed, *parameter.Max)
		}

		return strconv.Itoa(parsed), nil
	case ParameterKindFloat:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return "", errors.New("expected finite float")
		}

		if parameter.Min != nil && parsed < *parameter.Min {
			return "", fmt.Errorf("value %g is below minimum %g", parsed, *parameter.Min)
		}

		if parameter.Max != nil && parsed > *parameter.Max {
			return "", fmt.Errorf("value %g exceeds maximum %g", parsed, *parameter.Max)
		}

		return strconv.FormatFloat(parsed, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported kind %q", parameter.Kind)
	}
}

func containsText(values []string, target string) bool {
	return slices.Contains(values, target)
}
