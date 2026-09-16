#!/usr/bin/env bash
# Regenerate proj-reference-vectors.json from a local PROJ installation.
#
# This script is NOT run by CI and PROJ is NOT a build dependency: the checked-in
# JSON is the fixture, this is only how it was produced. See README.md.
#
#   ./generate-reference-vectors.sh > proj-reference-vectors.json
#
# Requires cs2cs (PROJ) and jq on PATH.
set -euo pipefail

proj_version="$(cs2cs 2>&1 | head -1 | sed 's/^Rel\. //; s/,.*//')"
generated="$(date -u +%Y-%m-%d)"

# --- projection-only cases -------------------------------------------------
#
# Rows are: name|epsg|ellipsoid|projection definition|lon|lat
#
# The +proj=tmerc definitions restate exactly what wgs84.UTM, wgs84.ETRS89UTM
# and wgs84.DHDN2001GK pass to Datum.TransverseMercator, so a mismatch is the
# projection series and nothing else. No datum shift is involved: the geographic
# side of every conversion uses the same +ellps as the projected side.
projection_rows=()

add_projection() {
	projection_rows+=("$1|$2|$3|$4|$5|$6")
}

# ETRS89 / UTM 31N-34N (GRS80) and WGS84 / UTM 32N-33N.
for spec in "25831 3 GRS80" "25832 9 GRS80" "25833 15 GRS80" "25834 21 GRS80" \
	"32632 9 WGS84" "32633 15 WGS84"; do
	read -r epsg cm ellps <<<"$spec"
	def="+proj=tmerc +lat_0=0 +lon_0=${cm} +k=0.9996 +x_0=500000 +y_0=0 +ellps=${ellps} +units=m +no_defs"
	add_projection "central meridian" "$epsg" "$ellps" "$def" "$cm" 51
	add_projection "western zone edge" "$epsg" "$ellps" "$def" "$((cm - 3))" 51
	add_projection "eastern zone edge" "$epsg" "$ellps" "$def" "$((cm + 3))" 51
	add_projection "southern extreme" "$epsg" "$ellps" "$def" "$((cm + 1))" 47.27
	add_projection "northern extreme" "$epsg" "$ellps" "$def" "$((cm - 1))" 55.09
done

# Offsets the zone edges by a non-integer number of degrees. Bash arithmetic is
# integer-only, so this needs a calculator; it is jq rather than bc because jq
# is already a prerequisite and bc is absent from many minimal environments.
degrees_from() {
	jq -n --argjson from "$1" --argjson delta "$2" '$from + $delta'
}

# DHDN / 3-degree Gauss-Kruger zone 2-5 (Bessel). Zone half-width is 1.5 degrees.
for zone in 2 3 4 5; do
	epsg="$((31464 + zone))"
	cm="$((zone * 3))"
	def="+proj=tmerc +lat_0=0 +lon_0=${cm} +k=1 +x_0=$((zone * 1000000 + 500000)) +y_0=0 +ellps=bessel +units=m +no_defs"
	add_projection "central meridian" "$epsg" bessel "$def" "$cm" 51
	add_projection "western zone edge" "$epsg" bessel "$def" "$(degrees_from "$cm" -1.5)" 51
	add_projection "eastern zone edge" "$epsg" bessel "$def" "$(degrees_from "$cm" 1.5)" 51
	add_projection "southern extreme" "$epsg" bessel "$def" "$cm" 47.27
	add_projection "northern extreme" "$epsg" bessel "$def" "$cm" 55.09
done

# Two points far outside zone 32. Cross-zone transforms are legal in this
# package, and a truncated series degrades first where nobody looks.
utm32="+proj=tmerc +lat_0=0 +lon_0=9 +k=0.9996 +x_0=500000 +y_0=0 +ellps=GRS80 +units=m +no_defs"
add_projection "5 degrees east of the central meridian" 25832 GRS80 "$utm32" 14 51
add_projection "5 degrees west of the central meridian" 25832 GRS80 "$utm32" 4 51

# --- full-CRS cases --------------------------------------------------------
#
# EPSG:4326 -> EPSG:<code>, through PROJ's own datum handling. These cover the
# geographic and Web Mercator codes that have no projection case, and they are
# what geo.BuildTransformPipeline actually builds.
crs_rows=(
	"geographic identity|4326|9.7320|52.3759"
	"ETRS89 geographic|4258|9.7320|52.3759"
	"Hannover|25832|9.7320|52.3759"
	"Amsterdam|25831|4.9041|52.3676"
	"Berlin|25833|13.4050|52.5200"
	"Warsaw|25834|21.0122|52.2297"
	"Aachen|31466|6.0839|50.7753"
	"Hannover|31467|9.7320|52.3759"
	"Munich|31468|11.5820|48.1351"
	"Dresden|31469|13.7373|51.0504"
	"Hannover|32632|9.7320|52.3759"
	"Berlin|32633|13.4050|52.5200"
	"Hannover|3857|9.7320|52.3759"
)

emit_projection() {
	local name epsg ellps def lon lat east north
	IFS='|' read -r name epsg ellps def lon lat <<<"$1"
	# shellcheck disable=SC2086 # $def is a word-split list of PROJ parameters.
	read -r east north _ < <(printf '%s %s\n' "$lon" "$lat" |
		cs2cs -f '%.9f' "+proj=longlat +ellps=${ellps} +no_defs" +to $def)
	# `--arg proj_string`, not `--arg def`: `def` is a jq keyword, and `$def`
	# is a syntax error rather than a variable reference.
	jq -nc --arg name "$name" --argjson epsg "$epsg" --arg proj_string "$def" \
		--argjson lon "$lon" --argjson lat "$lat" \
		--argjson east "$east" --argjson north "$north" \
		'{name: $name, epsg: $epsg, proj_string: $proj_string, lon: $lon, lat: $lat, easting: $east, northing: $north}'
}

emit_crs() {
	local name epsg lon lat first second x y
	IFS='|' read -r name epsg lon lat <<<"$1"
	# cs2cs honours the authority axis order. EPSG:4326 and EPSG:4258 are
	# latitude then longitude, and EPSG:31466-31469 are northing (X) then
	# easting (Y) — see `projinfo EPSG:31467`. Aconiq is easting/lon first
	# throughout, so those codes get swapped on the way into the fixture.
	read -r first second _ < <(printf '%s %s\n' "$lat" "$lon" | cs2cs -f '%.9f' EPSG:4326 "EPSG:${epsg}")
	case "$epsg" in
	4326 | 4258 | 31466 | 31467 | 31468 | 31469)
		x="$second"
		y="$first"
		;;
	*)
		x="$first"
		y="$second"
		;;
	esac
	jq -nc --arg name "$name" --argjson epsg "$epsg" \
		--argjson lon "$lon" --argjson lat "$lat" --argjson x "$x" --argjson y "$y" \
		'{name: $name, epsg: $epsg, lon: $lon, lat: $lat, x: $x, y: $y}'
}

projection_json="$(for row in "${projection_rows[@]}"; do emit_projection "$row"; done | jq -s '.')"
crs_json="$(for row in "${crs_rows[@]}"; do emit_crs "$row"; done | jq -s '.')"

jq -n --arg proj "$proj_version" --arg date "$generated" \
	--argjson projection "$projection_json" --argjson crs "$crs_json" \
	'{proj_version: $proj, generated: $date, projection_cases: $projection, crs_cases: $crs}'
