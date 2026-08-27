package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/iso9613"
)

//nolint:gocognit,cyclop,funlen // Point-source override handling is intentionally explicit.
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
		if feature.Kind != featureKindSource {
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

		points, err := sourcePointsFromFeature(feature)
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

			sourceHeightM := options.SourceHeightM

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_source_height_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					sourceHeightM = value
				}
			}

			soundPowerLevelDB := options.SoundPowerLevelDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_sound_power_level_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					soundPowerLevelDB = value
				}
			}

			directivityCorrectionDB := options.DirectivityCorrectionDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_directivity_correction_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					directivityCorrectionDB = value
				}
			}

			tonalityCorrectionDB := options.TonalityCorrectionDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_tonality_correction_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tonalityCorrectionDB = value
				}
			}

			impulsivityCorrectionDB := options.ImpulsivityCorrectionDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_impulsivity_correction_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					impulsivityCorrectionDB = value
				}
			}

			sources = append(sources, iso9613.PointSource{
				ID:                      sourceID,
				Point:                   point,
				SourceHeightM:           sourceHeightM,
				SoundPowerLevelDB:       soundPowerLevelDB,
				DirectivityCorrectionDB: directivityCorrectionDB,
				TonalityCorrectionDB:    tonalityCorrectionDB,
				ImpulsivityCorrectionDB: impulsivityCorrectionDB,
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", "model does not contain any supported point source features", nil)
	}

	return sources, nil
}
