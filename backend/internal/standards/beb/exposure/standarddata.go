package exposure

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the coefficient data this preview module carries.
//
// It is deliberately short, and that is the finding rather than an omission.
// This module aggregates rather than emits: the occupancy assumptions (floor
// height, dwellings per floor, persons per dwelling) and the exposure
// thresholds are run parameters recorded in provenance, and the levels being
// counted come from the scaffold-tier bub-road or buf-aircraft modules, whose
// own constants are digested there. What is left, and what is digested here, is
// the reporting band grid the summary is bucketed into.
//
// The band edges are this project's own preview choice of reporting grid; see
// docs/conformance/beb-umfangserklaerung.md. Both the declared edge slices and
// the band definitions derived from them are included: the derivation fixes each
// band's label and its exclusive upper bound, so a change to how bands are cut
// moves the digest even when the edges themselves do not.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "preview-exposure/model-version", Value: BuiltinModelVersion},
		{Name: "preview-exposure/lden-band-edges", Value: defaultLdenBandEdges},
		{Name: "preview-exposure/lnight-band-edges", Value: defaultLnightBandEdges},
		{Name: "preview-exposure/lden-band-definitions", Value: defaultExposureBands(defaultLdenBandEdges)},
		{Name: "preview-exposure/lnight-band-definitions", Value: defaultExposureBands(defaultLnightBandEdges)},
	}}
}
