package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// boundParam is one parameter binding ready to apply: the normalized parameter
// name and the closure that reads it into the field it fills.
//
// A standard's binding table is an ordered slice of these. The order is
// behaviour — applyBoundParams stops at the first failure, so a params map with
// two bad values reports the one listed first — and it is also the data
// TestRunOptionsCoverParameterSchema reads to check the table against the
// parameter schema the standards module publishes.
type boundParam struct {
	key   string
	apply func(scope string, params map[string]string) error
}

// applyBoundParams applies every binding in order and stops at the first error.
func applyBoundParams(scope string, params map[string]string, bound []boundParam) error {
	for _, item := range bound {
		err := item.apply(scope, params)
		if err != nil {
			return err
		}
	}

	return nil
}

// boundParamKeys returns the normalized parameter names a binding table covers.
func boundParamKeys(bound []boundParam) []string {
	keys := make([]string, 0, len(bound))
	for _, item := range bound {
		keys = append(keys, item.key)
	}

	return keys
}

// missingParamError is the failure every reader below shares: the framework
// normalizes and defaults the parameter map before the CLI sees it, so a key
// that is absent here means the CLI bound a name the schema does not declare.
func missingParamError(scope string, key string) error {
	return domainerrors.New(domainerrors.KindInternal, scope, fmt.Sprintf("normalized parameter %q missing", key), nil)
}

// parseFiniteFloatParam reads a normalized parameter and requires a finite float.
func parseFiniteFloatParam(scope string, params map[string]string, key string, target *float64) error {
	value, ok := params[key]
	if !ok {
		return missingParamError(scope, key)
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid %s=%q", key, value), err)
	}

	*target = parsed

	return nil
}

// parseMinFloatParam reads a normalized parameter and requires a finite float
// greater than or equal to minValue.
func parseMinFloatParam(scope string, params map[string]string, key string, target *float64, minValue float64) error {
	value, ok := params[key]
	if !ok {
		return missingParamError(scope, key)
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
		return missingParamError(scope, key)
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

// parseBoolParam reads a normalized parameter and requires a boolean literal.
func parseBoolParam(scope string, params map[string]string, key string, target *bool) error {
	value, ok := params[key]
	if !ok {
		return missingParamError(scope, key)
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid %s=%q", key, value), err)
	}

	*target = parsed

	return nil
}

// stringParamValue reads a normalized parameter and returns its trimmed value.
func stringParamValue(scope string, params map[string]string, key string) (string, error) {
	value, ok := params[key]
	if !ok {
		return "", missingParamError(scope, key)
	}

	return strings.TrimSpace(value), nil
}
