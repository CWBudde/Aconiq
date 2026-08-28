package iso9613

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the ISO 9613-2 tables this module carries.
//
// The ground and barrier terms are closed-form expressions rather than tables,
// so they are not represented here; a change to them is a change to the code
// path, which the tool version already identifies.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "iso9613/a-bewertung", Value: AWeighting},
		{Name: "iso9613/oktavband-mittenfrequenzen", Value: OctaveBandFrequencies},
		{Name: "iso9613/tabelle-2-luftabsorption", Value: table2},
	}}
}
