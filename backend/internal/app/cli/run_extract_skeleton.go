package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// msgNoLineSourceFeatures is the empty-result message every line-based source
// standard reports. It is one string rather than five copies because it is one
// message: a user seeing it has the same problem whichever standard they ran.
const msgNoLineSourceFeatures = "model does not contain any supported line source features"

// sourceExtraction describes the parts of a per-standard source extraction that
// genuinely differ between standards: the strings it reports with, how a
// feature's geometry splits into the parts it emits, and how one part becomes a
// source.
//
// Everything else — the kind filter, the source-type gate, the generated-ID
// fallback, the per-part suffixing and the empty-result check — was written out
// once per standard and is what dupl kept pointing at once the per-property
// decode blocks stopped hiding it.
//
// The gate here is the permissive kind: a feature carrying no source_type is
// accepted. cnossos-industry and iso9613 require one and report a different
// message, so they keep their own loop.
type sourceExtraction[T, P any] struct {
	// scope is the domainerrors op, which reaches the user as the first
	// segment of the rendered message.
	scope string
	// idPrefix formats the generated ID for a feature carrying none. It takes
	// the feature's index in the model, not a counter over emitted sources.
	idPrefix string
	// emptyMessage is reported when the model yields no source at all.
	emptyMessage string
	// parts splits one feature into the parts it emits a source for. Errors
	// are returned undecorated; extractSources wraps them.
	parts func(feature modelgeojson.Feature) ([]P, error)
	// build turns one part into a source. Errors are returned already wrapped,
	// because the override applier decorates them with the same scope.
	build func(feature modelgeojson.Feature, sourceID string, part P) (T, error)
}

// extractSources runs one feature loop for spec.
func extractSources[T, P any](model modelgeojson.Model, supportedSourceTypes []string, spec sourceExtraction[T, P]) ([]T, error) {
	allowedSourceType := normalizedSourceTypeSet(supportedSourceTypes)

	sources := make([]T, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					spec.scope,
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		parts, err := spec.parts(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, spec.scope, fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf(spec.idPrefix, featureIndex)
		}

		for partIndex, part := range parts {
			sourceID := baseID
			if len(parts) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, partIndex+1)
			}

			source, buildErr := spec.build(feature, sourceID, part)
			if buildErr != nil {
				return nil, buildErr
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, spec.scope, spec.emptyMessage, nil)
	}

	return sources, nil
}
