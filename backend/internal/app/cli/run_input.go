package cli

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
)

func mergeInputPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))

	for _, rawPath := range paths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}

		normalized := filepath.ToSlash(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out
}

func firstIndicator(indicators []string) string {
	for _, indicator := range indicators {
		trimmed := strings.TrimSpace(indicator)
		if trimmed != "" {
			return trimmed
		}
	}

	return freefield.IndicatorLdummy
}

func loadValidatedModel(modelPath string, projectCRS string, sourcePath string) (modelgeojson.Model, error) {
	payload, err := os.ReadFile(modelPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return modelgeojson.Model{}, domainerrors.New(domainerrors.KindNotFound, "cli.loadValidatedModel", "model file not found: "+modelPath, err)
		}

		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindInternal, "cli.loadValidatedModel", "read model file "+modelPath, err)
	}

	model, err := modelgeojson.Normalize(payload, projectCRS, sourcePath)
	if err != nil {
		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindValidation, "cli.loadValidatedModel", "normalize model file", err)
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
		}

		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindValidation, "cli.loadValidatedModel", summarizeValidationErrors(messages, 5), nil)
	}

	return model, nil
}

func extractExplicitReceivers(model modelgeojson.Model) ([]geo.PointReceiver, error) {
	receivers := make([]geo.PointReceiver, 0)
	seen := make(map[string]struct{})

	for _, feature := range model.Features {
		if feature.Kind != "receiver" {
			continue
		}

		if feature.GeometryType != "Point" {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q geometry must be Point", feature.ID), nil)
		}

		if feature.HeightM == nil || *feature.HeightM <= 0 || math.IsNaN(*feature.HeightM) || math.IsInf(*feature.HeightM, 0) {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q height_m must be finite and > 0", feature.ID), nil)
		}

		if _, exists := seen[feature.ID]; exists {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q is duplicated", feature.ID), nil)
		}

		point, err := parsePointCoordinate(feature.Coordinates)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q: %v", feature.ID, err), nil)
		}

		receivers = append(receivers, geo.PointReceiver{
			ID:      feature.ID,
			Point:   point,
			HeightM: *feature.HeightM,
		})
		seen[feature.ID] = struct{}{}
	}

	if len(receivers) == 0 {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.extractExplicitReceivers", "custom receiver mode requires at least one explicit receiver in the model", nil)
	}

	return receivers, nil
}

func resolveReceiverSet(
	mode string,
	model modelgeojson.Model,
	buildGrid func() ([]geo.PointReceiver, int, int, error),
) ([]geo.PointReceiver, int, int, error) {
	if mode == receiverModeCustom {
		receivers, err := extractExplicitReceivers(model)
		if err != nil {
			return nil, 0, 0, err
		}

		return receivers, 0, 0, nil
	}

	return buildGrid()
}

func featurePropertyString(feature modelgeojson.Feature, keys ...string) (string, bool, error) {
	return propertyString(feature.Properties, keys...)
}

func propertyString(properties map[string]any, keys ...string) (string, bool, error) {
	for _, key := range keys {
		raw, ok := properties[key]
		if !ok || raw == nil {
			continue
		}

		switch value := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return "", false, nil
			}

			return trimmed, true, nil
		default:
			return "", false, fmt.Errorf("property %q must be a string", key)
		}
	}

	return "", false, nil
}

func featurePropertyFloat(feature modelgeojson.Feature, keys ...string) (float64, bool, error) {
	return propertyFloat(feature.Properties, keys...)
}

func propertyFloat(properties map[string]any, keys ...string) (float64, bool, error) {
	for _, key := range keys {
		raw, ok := properties[key]
		if !ok || raw == nil {
			continue
		}

		value, hasValue, err := readFeatureFloat(raw)
		if err != nil {
			return 0, false, fmt.Errorf("property %q: %w", key, err)
		}

		if !hasValue {
			return 0, false, nil
		}

		return value, true, nil
	}

	return 0, false, nil
}

func featurePropertyBool(feature modelgeojson.Feature, keys ...string) (bool, bool, error) {
	return propertyBool(feature.Properties, keys...)
}

func propertyBool(properties map[string]any, keys ...string) (bool, bool, error) {
	for _, key := range keys {
		raw, ok := properties[key]
		if !ok || raw == nil {
			continue
		}

		switch value := raw.(type) {
		case bool:
			return value, true, nil
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return false, false, nil
			}

			parsed, err := strconv.ParseBool(trimmed)
			if err != nil {
				return false, false, fmt.Errorf("property %q must be a bool", key)
			}

			return parsed, true, nil
		default:
			return false, false, fmt.Errorf("property %q must be a bool", key)
		}
	}

	return false, false, nil
}

func propertyFloatSlice(properties map[string]any, keys ...string) ([]float64, bool, error) {
	for _, key := range keys {
		raw, ok := properties[key]
		if !ok || raw == nil {
			continue
		}

		items, ok := raw.([]any)
		if !ok {
			return nil, false, fmt.Errorf("property %q must be an array of finite numbers", key)
		}

		values := make([]float64, 0, len(items))
		for idx, item := range items {
			value, hasValue, err := readFeatureFloat(item)
			if err != nil {
				return nil, false, fmt.Errorf("property %q[%d]: %w", key, idx, err)
			}

			if !hasValue {
				return nil, false, fmt.Errorf("property %q[%d] must not be empty", key, idx)
			}

			values = append(values, value)
		}

		return values, true, nil
	}

	return nil, false, nil
}

func mergedProperties(base map[string]any, override map[string]any) map[string]any {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	merged := make(map[string]any, len(base)+len(override))
	maps.Copy(merged, base)

	maps.Copy(merged, override)

	return merged
}

func readFeatureFloat(raw any) (float64, bool, error) {
	switch value := raw.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false, errors.New("must be finite")
		}

		return value, true, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0, false, nil
		}

		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, false, errors.New("must be a finite number")
		}

		return parsed, true, nil
	default:
		return 0, false, fmt.Errorf("unsupported type %T", raw)
	}
}
