package export

import (
	"fmt"
	"math"
)

// maxGeoKeyEPSG is the largest EPSG code a GeoTIFF GeoKey can hold.
//
// GeoKey values are uint16 by the GeoTIFF specification, so this is a limit of
// the format rather than of the narrowing below it: a larger code has no
// representation in the file at all.
const maxGeoKeyEPSG = math.MaxUint16

// validateGeoKeyEPSG refuses an EPSG code that cannot be written as a GeoKey.
//
// The code reaches the exporters straight from the project CRS, which nothing
// validates: geo.ParseCRS accepts any integer, classifyEPSG calls anything at
// or above 2000 projected, and BuildTransformPipeline short-circuits to the
// identity when the import CRS matches. So a project created with an ESRI
// code such as EPSG:102100 - or the legacy EPSG:900913 - imports and runs, and
// the first thing that knows better is the exporter. It used to find out by
// panicking.
//
// Codes at or below zero are not refused here: buildGeoKeys reads them as
// "no CRS" and omits the keys.
func validateGeoKeyEPSG(epsgCode int) error {
	if epsgCode > maxGeoKeyEPSG {
		return fmt.Errorf("EPSG code %d cannot be represented in a GeoTIFF GeoKey (maximum %d)", epsgCode, maxGeoKeyEPSG)
	}

	return nil
}

// validateSRSID refuses an SRS id a GeoPackage cannot hold.
//
// The GeoPackageBinaryHeader's srs_id field is int32, and negative values are
// legal there: initGeoPackage writes gpkg_spatial_ref_sys rows for -1, the
// undefined cartesian SRS, and 0, the undefined geographic one. Only a value
// outside int32 is wrong.
func validateSRSID(srsID int) error {
	if srsID < math.MinInt32 || srsID > math.MaxInt32 {
		return fmt.Errorf("SRS id %d is out of range for a GeoPackage srs_id, which is int32", srsID)
	}

	return nil
}

// srsIDBits encodes an SRS id for the GeoPackageBinaryHeader.
//
// The field is int32, so the two negative-or-zero sentinels the format defines
// are two's complement rather than an error. This conversion used to go
// through mustUint32, which panics below zero and so contradicted
// initGeoPackage, four lines of which exist to register srs_id -1.
//
// Callers validate with validateSRSID first; the narrowing here is total.
func srsIDBits(srsID int) uint32 {
	//nolint:gosec // validateSRSID has already bounded srsID to int32 at the entry point
	return uint32(int32(srsID))
}

// mustUint32 narrows an exporter offset, size or count.
//
// Both bounds are checked, which the suppression this used to carry claimed
// while only the lower one was: a value above uint32 truncated in silence.
func mustUint32(value int) uint32 {
	if value < 0 || value > math.MaxUint32 {
		panic(fmt.Sprintf("value %d is out of range for uint32", value))
	}

	return uint32(value)
}

func mustUint16(value int) uint16 {
	if value < 0 || value > math.MaxUint16 {
		panic(fmt.Sprintf("value %d is out of range for uint16", value))
	}

	return uint16(value)
}
