// Package descriptorjson is the one JSON encoding of a standards descriptor.
//
// A descriptor reaches a consumer through two doors — `GET /api/v1/standards`
// from `aconiq serve`, and `aconiq.standards()` from the WASM kernel — and a
// consumer reads both with the same TypeScript types. Two encodings would mean
// two answers to "what parameters does rls19-road take", and the frontend
// already carried a hand-maintained copy of one of them that had drifted twice:
// it offered 9 of the 17 surfaces, and claimed line sources only, long after the
// Go module started accepting Parkplatz areas.
//
// So the shape lives here rather than in either caller. The field names, the
// omitempty set and the ordering are exactly what `internal/api/httpv1` used to
// declare inline; moving them changed no byte of the HTTP response, which is
// what that package's unedited handler tests assert.
package descriptorjson

import "github.com/aconiq/backend/internal/standards/framework"

// Parameter is one run parameter as a consumer sees it.
type Parameter struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Unit         string   `json:"unit,omitempty"`
	Required     bool     `json:"required"`
	DefaultValue string   `json:"default_value,omitempty"`
	Description  string   `json:"description,omitempty"`
	Enum         []string `json:"enum,omitempty"`
	Min          *float64 `json:"min,omitempty"`
	Max          *float64 `json:"max,omitempty"`
}

// Profile is one named profile of a standard version.
type Profile struct {
	Name                 string      `json:"name"`
	SupportedSourceTypes []string    `json:"supported_source_types"`
	SupportedIndicators  []string    `json:"supported_indicators"`
	Parameters           []Parameter `json:"parameters"`
}

// Version is one implementation version of a standard.
type Version struct {
	Name           string    `json:"name"`
	DefaultProfile string    `json:"default_profile"`
	Profiles       []Profile `json:"profiles"`
}

// Standard is one standards module as a consumer sees it. EvidenceTier is not
// omitempty: a descriptor that cannot be registered without a tier must not be
// published without one either, and a consumer that never reads the docs sees
// the field precisely because it is always there.
type Standard struct {
	Context        string    `json:"context"`
	ID             string    `json:"id"`
	Description    string    `json:"description"`
	EvidenceTier   string    `json:"evidence_tier"`
	DefaultVersion string    `json:"default_version"`
	Versions       []Version `json:"versions"`
}

// FromDescriptor converts one descriptor into its JSON shape.
func FromDescriptor(d framework.StandardDescriptor) Standard {
	versions := make([]Version, 0, len(d.Versions))
	for _, v := range d.Versions {
		versions = append(versions, fromVersion(v))
	}

	return Standard{
		Context:        d.Context,
		ID:             d.ID,
		Description:    d.Description,
		EvidenceTier:   string(d.EvidenceTier),
		DefaultVersion: d.DefaultVersion,
		Versions:       versions,
	}
}

// FromDescriptors converts a descriptor list, preserving its order. The result
// is always non-nil, so an empty registry encodes as `[]` rather than `null`.
func FromDescriptors(descriptors []framework.StandardDescriptor) []Standard {
	standards := make([]Standard, 0, len(descriptors))
	for _, d := range descriptors {
		standards = append(standards, FromDescriptor(d))
	}

	return standards
}

func fromVersion(v framework.Version) Version {
	profiles := make([]Profile, 0, len(v.Profiles))
	for _, p := range v.Profiles {
		profiles = append(profiles, fromProfile(p))
	}

	return Version{
		Name:           v.Name,
		DefaultProfile: v.DefaultProfile,
		Profiles:       profiles,
	}
}

func fromProfile(p framework.Profile) Profile {
	params := make([]Parameter, 0, len(p.ParameterSchema.Parameters))
	for _, param := range p.ParameterSchema.Parameters {
		params = append(params, fromParameter(param))
	}

	return Profile{
		Name:                 p.Name,
		SupportedSourceTypes: p.SupportedSourceTypes,
		SupportedIndicators:  p.SupportedIndicators,
		Parameters:           params,
	}
}

func fromParameter(param framework.ParameterDefinition) Parameter {
	return Parameter{
		Name:         param.Name,
		Kind:         string(param.Kind),
		Unit:         param.Unit,
		Required:     param.Required,
		DefaultValue: param.DefaultValue,
		Description:  param.Description,
		Enum:         param.Enum,
		Min:          param.Min,
		Max:          param.Max,
	}
}
