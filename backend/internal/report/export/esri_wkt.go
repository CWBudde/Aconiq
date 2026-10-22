package export

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// ESRIWKTForEPSG returns the ESRI flavour of WKT — the dialect a Shapefile's
// .prj is written in — for the projected CRS this project can transform into,
// and false for any other code.
//
// Only conformal metric CRS are listed: ETRS89 / UTM, DHDN / Gauss-Krüger and
// WGS84 / UTM. A geographic CRS has no planar distances at all, and Web
// Mercator's are scaled by 1/cos(latitude) — about 1.6 in Germany — so a
// noise model handed over in either would be computed at the wrong distances
// by a program that takes the plane at its word.
func ESRIWKTForEPSG(code int) (string, bool) {
	switch {
	case code >= 25831 && code <= 25834:
		zone := code - 25800

		return transverseMercatorWKT(
			fmt.Sprintf("ETRS_1989_UTM_Zone_%dN", zone),
			etrs89GeogCS, "Transverse_Mercator",
			500000, float64(zone*6-183), 0.9996,
		), true
	case code == 32632 || code == 32633:
		zone := code - 32600

		return transverseMercatorWKT(
			fmt.Sprintf("WGS_1984_UTM_Zone_%dN", zone),
			wgs84GeogCS, "Transverse_Mercator",
			500000, float64(zone*6-183), 0.9996,
		), true
	case code >= 31466 && code <= 31469:
		zone := code - 31464

		return transverseMercatorWKT(
			fmt.Sprintf("DHDN_3_Degree_Gauss_Zone_%d", zone),
			dhdnGeogCS, "Gauss_Kruger",
			float64(zone)*1e6+500000, float64(zone*3), 1,
		), true
	default:
		return "", false
	}
}

const (
	esriDegree   = `PRIMEM["Greenwich",0.0],UNIT["Degree",0.0174532925199433]`
	etrs89GeogCS = `GEOGCS["GCS_ETRS_1989",DATUM["D_ETRS_1989",SPHEROID["GRS_1980",6378137.0,298.257222101]],` + esriDegree + `]`
	wgs84GeogCS  = `GEOGCS["GCS_WGS_1984",DATUM["D_WGS_1984",SPHEROID["WGS_1984",6378137.0,298.257223563]],` + esriDegree + `]`
	dhdnGeogCS   = `GEOGCS["GCS_Deutsches_Hauptdreiecksnetz",DATUM["D_Deutsches_Hauptdreiecksnetz",` +
		`SPHEROID["Bessel_1841",6377397.155,299.1528128]],` + esriDegree + `]`
)

func transverseMercatorWKT(name, geogCS, projection string, falseEasting, centralMeridian, scale float64) string {
	return `PROJCS["` + name + `",` + geogCS +
		`,PROJECTION["` + projection + `"]` +
		`,PARAMETER["False_Easting",` + wktNumber(falseEasting) + `]` +
		`,PARAMETER["False_Northing",0.0]` +
		`,PARAMETER["Central_Meridian",` + wktNumber(centralMeridian) + `]` +
		`,PARAMETER["Scale_Factor",` + wktNumber(scale) + `]` +
		`,PARAMETER["Latitude_Of_Origin",0.0]` +
		`,UNIT["Meter",1.0]]`
}

// wktNumber spells a value the way ESRI WKT does: shortest exact form, always
// with a fractional part.
func wktNumber(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}

	return s
}

// soundPLANPRJ resolves the CRS a SoundPLAN export is labelled with, or
// refuses it.
func soundPLANPRJ(crs string) (string, error) {
	parsed, err := geo.ParseCRS(crs)
	if err != nil {
		return "", fmt.Errorf("soundplan export: read the CRS %q the model is in: %w", crs, err)
	}

	wkt, ok := ESRIWKTForEPSG(parsed.EPSGCode())
	if !ok {
		return "", fmt.Errorf(
			"soundplan export: %s is not a metric conformal CRS; SoundPLAN computes distances in the plane "+
				"of the coordinates it imports, so only ETRS89 / UTM, WGS84 / UTM or DHDN / Gauss-Krüger "+
				"coordinates can be handed over", parsed.ID,
		)
	}

	return wkt, nil
}
