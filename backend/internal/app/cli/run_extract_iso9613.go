package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/iso9613"
)

func extractISO9613Sources(model modelgeojson.Model, options iso9613RunOptions, supportedSourceTypes []string) ([]iso9613.PointSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]iso9613.PointSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType == "" {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q source_type is required for iso9613", feature.ID), nil)
		}

		if _, ok := allowedSourceType[normalizedSourceType]; !ok {
			return nil, domainerrors.New(
				domainerrors.KindValidation,
				"cli.extractISO9613Sources",
				fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
				nil,
			)
		}

		points, err := sourcePointsFromFeature(feature, iso9613.StandardID)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("iso9613-source-%03d", featureIndex)
		}

		for pointIndex, point := range points {
			sourceID := baseID
			if len(points) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
			}

			source := iso9613.PointSource{
				ID:                      sourceID,
				Point:                   point,
				SourceHeightM:           options.SourceHeightM,
				SoundPowerLevelDB:       options.SoundPowerLevelDB,
				DirectivityCorrectionDB: options.DirectivityCorrectionDB,
				TonalityCorrectionDB:    options.TonalityCorrectionDB,
				ImpulsivityCorrectionDB: options.ImpulsivityCorrectionDB,
			}

			overrideErr := applyFeatureOverrides(feature, "cli.extractISO9613Sources", []propertyOverride{
				overrideFloat(&source.SourceHeightM, "iso9613_source_height_m"),
				overrideFloat(&source.SoundPowerLevelDB, "iso9613_sound_power_level_db"),
				overrideFloat(&source.DirectivityCorrectionDB, "iso9613_directivity_correction_db"),
				overrideFloat(&source.TonalityCorrectionDB, "iso9613_tonality_correction_db"),
				overrideFloat(&source.ImpulsivityCorrectionDB, "iso9613_impulsivity_correction_db"),
			})
			if overrideErr != nil {
				return nil, overrideErr
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", "model does not contain any supported point source features", nil)
	}

	return sources, nil
}

// extractISO9613Barriers reads the screening scene out of the model, the same
// `kind: barrier` features `extractRLS19Barriers` reads. Before this, ISO
// 9613-2 discarded them: its extraction kept source features only, so the
// screening formulas of Gl. 12-18 could never be reached from a project.
//
// A model with no barrier features yields an empty scene rather than an error.
// ISO 9613-2 over open ground is a legitimate calculation, and the great
// majority of runs are exactly that.
func extractISO9613Barriers(model modelgeojson.Model) ([]iso9613.Barrier, error) {
	features, err := extractBarrierFeatures(model, iso9613.StandardID, "iso9613-barrier", "cli.extractISO9613Barriers")
	if err != nil {
		return nil, err
	}

	barriers := make([]iso9613.Barrier, 0, len(features))

	for _, feature := range features {
		barrier := iso9613.Barrier{ID: feature.ID, Geometry: feature.Geometry, HeightM: feature.HeightM}

		err := barrier.Validate()
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Barriers", fmt.Sprintf("barrier %q", feature.ID), err)
		}

		barriers = append(barriers, barrier)
	}

	return barriers, nil
}
