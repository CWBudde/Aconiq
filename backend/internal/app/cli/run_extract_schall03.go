package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/schall03"
)

//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // Extracted from the former monolithic run command without changing per-feature override behavior.
func extractSchall03Sources(model modelgeojson.Model, options schall03RunOptions, supportedSourceTypes []string) ([]schall03.RailSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]schall03.RailSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractSchall03Sources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("schall03-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			trainClass := options.TrainClass

			{
				value, ok, err := featurePropertyString(feature, "rail_train_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trainClass = value
				}
			}

			tractionType := options.TractionType

			{
				value, ok, err := featurePropertyString(feature, "rail_traction_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tractionType = value
				}
			}

			trackType := options.TrackType

			{
				value, ok, err := featurePropertyString(feature, "rail_track_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trackType = value
				}
			}

			trackForm := options.TrackForm

			{
				value, ok, err := featurePropertyString(feature, "rail_track_form")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trackForm = value
				}
			}

			roughnessClass := options.TrackRoughnessClass

			{
				value, ok, err := featurePropertyString(feature, "rail_track_roughness_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					roughnessClass = value
				}
			}

			averageSpeedKPH := options.AverageTrainSpeedKPH

			{
				value, ok, err := featurePropertyFloat(feature, "rail_average_train_speed_kph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					averageSpeedKPH = value
				}
			}

			curveRadiusM := options.CurveRadiusM

			{
				value, ok, err := featurePropertyFloat(feature, "rail_curve_radius_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					curveRadiusM = value
				}
			}

			onBridge := options.OnBridge

			{
				value, ok, err := featurePropertyBool(feature, "rail_on_bridge")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					onBridge = value
				}
			}

			elevationM := 0.0

			{
				value, ok, err := featurePropertyFloat(feature, "elevation_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					elevationM = value
				}
			}

			trafficDay := options.TrafficDayTrainsPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDay = value
				}
			}

			trafficNight := options.TrafficNightTrainsPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNight = value
				}
			}

			sources = append(sources, schall03.RailSource{
				ID:              sourceID,
				TrackCenterline: line,
				ElevationM:      elevationM,
				TrainClass:      trainClass,
				AverageSpeedKPH: averageSpeedKPH,
				Infrastructure: schall03.RailInfrastructure{
					TractionType:        tractionType,
					TrackType:           trackType,
					TrackForm:           trackForm,
					TrackRoughnessClass: roughnessClass,
					OnBridge:            onBridge,
					CurveRadiusM:        curveRadiusM,
				},
				TrafficDay:   schall03.TrafficPeriod{TrainsPerHour: trafficDay},
				TrafficNight: schall03.TrafficPeriod{TrainsPerHour: trafficNight},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}
