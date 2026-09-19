package framework

// KeyParameterPrefix is the namespace a run parameter is recorded under in a
// module's provenance metadata, so that a reader can tell the parameters that
// went into a run from the module's own descriptive fields.
const KeyParameterPrefix = "key_parameter."

// StampKeyParameters records the named run parameters in metadata under
// KeyParameterPrefix and returns metadata, so that it can close a
// ProvenanceMetadata function.
//
// A key that params does not carry is skipped rather than stamped empty: the
// absence of a parameter and a parameter set to "" are different facts, and the
// provenance record must not conflate them. keys is walked in order, but the
// result is a map, so only the set of entries is observable.
//
// Every standards module builds its provenance the same way — a base map of
// module-level facts, then this loop over the parameters that module considers
// load-bearing. The base map and the key list are what legitimately differ
// between modules; the loop was copied ten times.
func StampKeyParameters(metadata map[string]string, params map[string]string, keys []string) map[string]string {
	for _, key := range keys {
		if value, ok := params[key]; ok {
			metadata[KeyParameterPrefix+key] = value
		}
	}

	return metadata
}
