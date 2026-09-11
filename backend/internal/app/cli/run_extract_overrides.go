package cli

import (
	"fmt"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// propertyOverride decodes one feature property into the field it overrides.
//
// The extractors apply these in slice order and stop at the first decode
// error, which is the precedence the hand-unrolled blocks they replace had: a
// feature carrying two malformed properties reports the one listed first.
// TestExtractOverrideErrorPrecedence pins that per extractor, so the order of a
// table below is behaviour, not formatting.
//
// Each constructor takes the same variadic key list as propertyString and its
// siblings, so a property with alias spellings stays one entry. Targets are
// normally fields of a source literal already seeded from the run options,
// which is what makes the default and the override read as one thing.
type propertyOverride func(properties map[string]any) error

// overrideString fills target from the first key that carries a string.
func overrideString(target *string, keys ...string) propertyOverride {
	return func(properties map[string]any) error {
		value, ok, err := propertyString(properties, keys...)
		if err != nil {
			return err
		}

		if ok {
			*target = value
		}

		return nil
	}
}

// overrideFloat fills target from the first key that carries a number.
func overrideFloat(target *float64, keys ...string) propertyOverride {
	return func(properties map[string]any) error {
		value, ok, err := propertyFloat(properties, keys...)
		if err != nil {
			return err
		}

		if ok {
			*target = value
		}

		return nil
	}
}

// overrideBool fills target from the first key that carries a bool.
func overrideBool(target *bool, keys ...string) propertyOverride {
	return func(properties map[string]any) error {
		value, ok, err := propertyBool(properties, keys...)
		if err != nil {
			return err
		}

		if ok {
			*target = value
		}

		return nil
	}
}

// overrideFloatFound is overrideFloat for a field whose caller also needs to
// know whether the property was present, because an absent value and a decoded
// zero mean different things downstream.
func overrideFloatFound(target *float64, found *bool, keys ...string) propertyOverride {
	return func(properties map[string]any) error {
		value, ok, err := propertyFloat(properties, keys...)
		if err != nil {
			return err
		}

		if ok {
			*target = value
			*found = true
		}

		return nil
	}
}

// applyPropertyOverrides applies overrides in order against an arbitrary
// property map, which is what RLS-19 needs: it decodes against a feature's
// properties merged with one directional source's overrides.
//
// The wrapping is the shape every extractor here already produced. It names the
// feature but not the key, because the decode error already carries the key.
func applyPropertyOverrides(properties map[string]any, scope, featureID string, overrides []propertyOverride) error {
	err := decodeOverrides(properties, overrides)
	if err != nil {
		return domainerrors.New(domainerrors.KindValidation, scope, fmt.Sprintf("feature %q", featureID), err)
	}

	return nil
}

// decodeOverrides applies overrides in order and returns the first decode error
// undecorated, for callers that wrap it themselves — the geometry-resolving
// closures do, because their caller already adds the same scope and feature.
func decodeOverrides(properties map[string]any, overrides []propertyOverride) error {
	for _, override := range overrides {
		err := override(properties)
		if err != nil {
			return err
		}
	}

	return nil
}

// applyFeatureOverrides applies overrides against a feature's own properties.
func applyFeatureOverrides(feature modelgeojson.Feature, scope string, overrides []propertyOverride) error {
	return applyPropertyOverrides(feature.Properties, scope, feature.ID, overrides)
}
