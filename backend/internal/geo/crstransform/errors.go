package crstransform

// Reason classifies why a transform was refused.
//
// It exists because the messages themselves must not be parsed. POST
// /api/v1/transform has to put a code in its error envelope, and the one thing
// it may not do is reach that code by matching on the text — the text is a
// contract of its own (see Error), and a caller matching on it would freeze the
// wording the way a second copy of the zone decision would freeze the zone.
type Reason string

const (
	// ReasonOddCoordinateCount marks a batch that is not x, y interleaved.
	ReasonOddCoordinateCount Reason = "odd_coordinate_count"
	// ReasonSourceCRS marks a source CRS that could not be parsed.
	ReasonSourceCRS Reason = "source_crs"
	// ReasonTargetCRS marks an explicit target CRS that could not be parsed.
	ReasonTargetCRS Reason = "target_crs"
	// ReasonGeographicRefused marks a geographic model whose centre falls
	// outside the ETRS89 / UTM zones this project supports.
	ReasonGeographicRefused Reason = "geographic_refused"
	// ReasonPipelineUnavailable marks a CRS pair with no route between them.
	ReasonPipelineUnavailable Reason = "pipeline_unavailable"
	// ReasonCoordinate marks one coordinate the pipeline refused; Index says
	// which.
	ReasonCoordinate Reason = "coordinate"
)

// NoIndex is Error.Index when the refusal is not about one coordinate.
const NoIndex = -1

// Error is a refused transform, carrying a machine-readable Reason beside the
// message.
//
// The message is deliberately the wrapped error's own, unaltered: browser mode,
// API mode and `aconiq run` must refuse the same site in the same words, so
// nothing here may prefix, translate or reword it. Context belongs in Reason
// and Index, not in front of the text.
type Error struct {
	Reason Reason
	// Index is the coordinate pair the refusal is about, or NoIndex.
	Index int

	err error
}

func (e *Error) Error() string { return e.err.Error() }

func (e *Error) Unwrap() error { return e.err }

// refuse tags an error with its reason without touching its message.
func refuse(reason Reason, index int, err error) *Error {
	return &Error{Reason: reason, Index: index, err: err}
}
