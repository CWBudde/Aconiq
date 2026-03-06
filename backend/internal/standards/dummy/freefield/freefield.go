package freefield

import (
	"math"

	"github.com/soundplan/soundplan/backend/internal/geo"
	"github.com/soundplan/soundplan/backend/internal/standards/framework"
)

const (
	// StandardID identifies this demonstrator standard module.
	StandardID = "dummy-freefield"
	// IndicatorLdummy is the indicator emitted by this non-normative module.
	IndicatorLdummy = "Ldummy"
)

// Source represents one simplified point source for the non-normative model.
type Source struct {
	ID         string
	Point      geo.Point2D
	EmissionDB float64
}

// ComputeReceiverLevelDB computes a non-normative free-field level at a receiver.
//
// Formula used (simplified):
// - contribution_i = Emission_i - 20*log10(distance_i)
// - total = 10*log10(sum(10^(contribution_i/10)))
//
// This is explicitly for technical E2E validation and not a normative method.
func ComputeReceiverLevelDB(receiver geo.Point2D, sources []Source) float64 {
	linearSum := 0.0
	for _, source := range sources {
		d := geo.Distance(receiver, source.Point)
		if d < 1 {
			d = 1
		}
		contributionDB := source.EmissionDB - 20*math.Log10(d)
		linearSum += math.Pow(10, contributionDB/10)
	}
	if linearSum <= 0 {
		return -999.0
	}
	return 10 * math.Log10(linearSum)
}

// Descriptor returns the standards-framework descriptor for dummy-freefield.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minResolution := 0.001
	minChunkSize := 1.0
	return framework.StandardDescriptor{
		ID:             StandardID,
		Description:    "Non-normative free-field demonstrator for offline E2E verification.",
		DefaultVersion: "v0",
		Versions: []framework.Version{
			{
				Name:           "v0",
				DefaultProfile: "default",
				Profiles: []framework.Profile{
					{
						Name:                 "default",
						SupportedSourceTypes: []string{"point"},
						SupportedIndicators:  []string{IndicatorLdummy},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minResolution, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height above ground"},
								{Name: "source_emission_db", Kind: framework.ParameterKindFloat, DefaultValue: "90", Min: &minZero, Description: "Uniform source emission level"},
								{Name: "chunk_size", Kind: framework.ParameterKindInt, DefaultValue: "128", Min: &minChunkSize, Description: "Engine receiver chunk size"},
								{Name: "workers", Kind: framework.ParameterKindInt, DefaultValue: "0", Min: &minZero, Description: "Engine worker count (0=auto)"},
								{Name: "disable_cache", Kind: framework.ParameterKindBool, DefaultValue: "false", Description: "Disable chunk cache writes/reads"},
							},
						},
					},
					{
						Name:                 "highres",
						SupportedSourceTypes: []string{"point"},
						SupportedIndicators:  []string{IndicatorLdummy},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minResolution, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height above ground"},
								{Name: "source_emission_db", Kind: framework.ParameterKindFloat, DefaultValue: "90", Min: &minZero, Description: "Uniform source emission level"},
								{Name: "chunk_size", Kind: framework.ParameterKindInt, DefaultValue: "64", Min: &minChunkSize, Description: "Engine receiver chunk size"},
								{Name: "workers", Kind: framework.ParameterKindInt, DefaultValue: "0", Min: &minZero, Description: "Engine worker count (0=auto)"},
								{Name: "disable_cache", Kind: framework.ParameterKindBool, DefaultValue: "false", Description: "Disable chunk cache writes/reads"},
							},
						},
					},
				},
			},
		},
	}
}
