#!/usr/bin/env bash
# Reproduce the Hannover-Linden-Mitte RLS-19 run headlessly, with a CPU profile.
#
# Usage: repro-linden.sh DIR [extra `aconiq run` args...]
#
#   just repro-linden /tmp/linden
#   just repro-linden /tmp/linden --param segment_length_mode=distance_scaled
#   SOURCE_PROJECT=~/aconiq-workspace just repro-linden /tmp/linden
#
# The scene is the one the per-pair-cost work in PLAN.md is measured on: an
# OpenStreetMap extract of Linden-Mitte, building-dense enough that the
# reflection search dominates the profile. Only its bounding box lives here;
# the data is fetched from Overpass at run time and never committed.
#
# DIR becomes a project folder. How it is filled, in order of precedence:
#
#   1. DIR already holds a project (.noise/project.json): it is reused as is,
#      so a second invocation with different run args measures the same model.
#   2. SOURCE_PROJECT is set: that project folder is copied into DIR (cp -a).
#      The original is never touched.
#   3. Otherwise: `aconiq init` plus `aconiq import --from-osm` over the box
#      below. This needs network access to the Overpass API.
#
# A fresh OSM import carries no calc-area, so case 3 computes the auto-grid
# over the full source extent — thousands of receivers, far more than the
# 130 the reference workspace computes. Set CALC_AREA_GEOJSON to a GeoJSON
# file holding a `kind: calc-area` polygon (in the project CRS, EPSG:4326 for a
# fresh project) and its calc-area features are merged into the imported model
# before the run. That merge needs jq.
#
# Extra arguments go to `aconiq run` after the defaults, so a later flag
# overrides an earlier one: `--receiver-mode custom` runs the model's own
# receiver points instead of the auto-grid.
#
# Output: wall and CPU time (from /usr/bin/time -v when it exists, from the
# bash `time` keyword otherwise), DIR/cpu.pprof, and its top 25 functions.

set -euo pipefail

# south,west,north,east in WGS84 degrees — the order `--from-osm` expects.
LINDEN_BBOX="52.3616,9.7049,52.3757,9.7233"

if [ "$#" -lt 1 ] || [ -z "$1" ]; then
	echo "usage: $0 DIR [extra aconiq run args...]" >&2
	exit 2
fi

dir="$1"
shift

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
aconiq="$repo_root/bin/aconiq"

if [ ! -x "$aconiq" ]; then
	echo "error: $aconiq not found; run \`just build\` first (\`just repro-linden\` does)" >&2
	exit 1
fi

mkdir -p "$dir"
dir="$(cd "$dir" && pwd)"

if [ -f "$dir/.noise/project.json" ]; then
	echo "==> Reusing the project already in $dir"
elif [ -n "${SOURCE_PROJECT:-}" ]; then
	if [ ! -f "$SOURCE_PROJECT/.noise/project.json" ]; then
		echo "error: SOURCE_PROJECT=$SOURCE_PROJECT is not an aconiq project" >&2
		exit 1
	fi

	echo "==> Copying $SOURCE_PROJECT into $dir"
	cp -a "$SOURCE_PROJECT/." "$dir/"
else
	echo "==> Importing Linden-Mitte from OpenStreetMap ($LINDEN_BBOX) into $dir"
	"$aconiq" --project "$dir" init --name linden-repro
	"$aconiq" --project "$dir" import --from-osm "$LINDEN_BBOX"

	if [ -n "${CALC_AREA_GEOJSON:-}" ]; then
		if ! command -v jq >/dev/null 2>&1; then
			echo "error: merging CALC_AREA_GEOJSON needs jq" >&2
			exit 1
		fi

		echo "==> Merging the calc-area from $CALC_AREA_GEOJSON"
		merged="$dir/model.with-calc-area.geojson"
		jq --slurpfile extra "$CALC_AREA_GEOJSON" \
			'.features = ([.features[] | select(.properties.kind != "calc-area")]
				+ [$extra[0].features[] | select(.properties.kind == "calc-area")])' \
			"$dir/.noise/model/model.normalized.geojson" >"$merged"
		"$aconiq" --project "$dir" import --input "$merged"
	else
		echo "==> No CALC_AREA_GEOJSON: the auto-grid covers the full source extent"
	fi
fi

profile="$dir/cpu.pprof"
time_report="$dir/time.txt"

run_args=(
	--project "$dir" run
	--standard rls19-road
	--receiver-mode auto-grid
	--cpuprofile "$profile"
	"$@"
)

echo "==> aconiq ${run_args[*]}"

status=0
if [ -x /usr/bin/time ]; then
	/usr/bin/time -v -o "$time_report" "$aconiq" "${run_args[@]}" || status=$?
	echo "==> Timing"
	grep -E 'Elapsed \(wall clock\)|User time|System time|Percent of CPU|Maximum resident' "$time_report"
else
	echo "==> Timing (bash time: real = wall, user + sys = CPU)"
	time "$aconiq" "${run_args[@]}" || status=$?
fi

if [ "$status" -ne 0 ]; then
	echo "==> aconiq run exited with status $status (the profile is written regardless)"
fi

if [ -s "$profile" ]; then
	echo "==> Top 25 of $profile"
	go tool pprof -top -nodecount=25 "$aconiq" "$profile"
fi

exit "$status"
