package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// floatParam binds a normalized run parameter key to the float field it fills.
type floatParam struct {
	key    string
	target *float64
}

// stringParam binds a normalized run parameter key to the string field it fills.
type stringParam struct {
	key    string
	target *string
}

// parseFiniteFloatParam reads a normalized parameter and requires a finite float.
func parseFiniteFloatParam(scope string, params map[string]string, key string, target *float64) error {
	value, ok := params[key]
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, scope, fmt.Sprintf("normalized parameter %q missing", key), nil)
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid %s=%q", key, value), err)
	}

	*target = parsed

	return nil
}

// parseFiniteFloatParams applies parseFiniteFloatParam to every field in order.
func parseFiniteFloatParams(scope string, params map[string]string, fields []floatParam) error {
	for _, field := range fields {
		err := parseFiniteFloatParam(scope, params, field.key, field.target)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseMinFloatParam reads a normalized parameter and requires a finite float
// greater than or equal to minValue.
func parseMinFloatParam(scope string, params map[string]string, key string, target *float64, minValue float64) error {
	value, ok := params[key]
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, scope, fmt.Sprintf("normalized parameter %q missing", key), nil)
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid %s=%q", key, value), err)
	}

	if math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < minValue {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("%s must be >= %g", key, minValue), nil)
	}

	*target = parsed

	return nil
}

// parseMinIntParam reads a normalized parameter and requires an integer greater
// than or equal to minValue.
func parseMinIntParam(scope string, params map[string]string, key string, target *int, minValue int) error {
	value, ok := params[key]
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, scope, fmt.Sprintf("normalized parameter %q missing", key), nil)
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid %s=%q", key, value), err)
	}

	if parsed < minValue {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("%s must be >= %d", key, minValue), nil)
	}

	*target = parsed

	return nil
}

// stringParamValue reads a normalized parameter and returns its trimmed value.
func stringParamValue(scope string, params map[string]string, key string) (string, error) {
	value, ok := params[key]
	if !ok {
		return "", domainerrors.New(domainerrors.KindInternal, scope, fmt.Sprintf("normalized parameter %q missing", key), nil)
	}

	return strings.TrimSpace(value), nil
}

// assignStringParams applies stringParamValue to every field in order.
func assignStringParams(scope string, params map[string]string, fields []stringParam) error {
	for _, field := range fields {
		value, err := stringParamValue(scope, params, field.key)
		if err != nil {
			return err
		}

		*field.target = value
	}

	return nil
}
