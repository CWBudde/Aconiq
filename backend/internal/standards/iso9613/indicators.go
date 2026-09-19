package iso9613

import (
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// BuiltinModelVersion identifies the octave-band engineering method.
	BuiltinModelVersion = "iso9613-octaveband-v1"

	// ReportingPrecisionDB documents the intended public reporting boundary.
	ReportingPrecisionDB = 0.1
)

// ProvenanceMetadata returns ISO 9613-2 scaffold metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":            BuiltinModelVersion,
		"reporting_precision_db":   "0.1",
		"indicator_order":          IndicatorLpAeqDW + "," + IndicatorLpAeqLT,
		"compliance_boundary":      "iso9613-engineering-octaveband",
		"implementation_status":    "octaveband-point-source",
		"source_scope":             SourceTypePoint,
		paramMeteorologyAssumption: MeteorologyDownwind,
	}

	return framework.StampKeyParameters(metadata, params, parameterNames())
}
