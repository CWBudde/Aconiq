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
# clock and the `times` builtin otherwise), DIR/cpu.pprof, and its top 25
# functions. INT and TERM are forwarded to the run; see "Signals" below.

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

# Signals. A killed recipe must never leave a computing aconiq behind: an
# earlier version of this script died on SIGTERM while the run it had started
# went on computing for two hours, reparented to init. So the run is started in
# the background, in a process group of its own (`set -m`), and this script
# stays in the foreground waiting for it, forwarding INT and TERM to it.
#
# The process group is the point. A Ctrl-C at the terminal reaches every
# process in the foreground group; were aconiq in it, it would hear the
# terminal's SIGINT and the forwarded one, and a second signal ends aconiq
# outright, without its profile. In a group of its own it hears exactly the one
# forwarded here, which it treats as a request to stop: it records the run as
# failed and writes the profile. Both signals are forwarded as SIGINT because
# /usr/bin/time ignores SIGINT but dies on SIGTERM, which would orphan aconiq
# again; aconiq handles the two alike.
#
# aconiq checks for cancellation between receivers, so on a building-dense
# scene the stop can take as long as one receiver does — on Linden, tens of
# minutes. A second Ctrl-C (see the two-second rule below) is forwarded too,
# and ends aconiq at once, without the profile.
set -m

start_ns="$(date +%s%N)"
if [ -x /usr/bin/time ]; then
	/usr/bin/time -v -o "$time_report" "$aconiq" "${run_args[@]}" &
else
	"$aconiq" "${run_args[@]}" &
fi
child=$!

# One stop request often arrives here twice: `just` forwards the signal it gets
# to this script, and a Ctrl-C at the terminal, or GNU timeout, signals the
# whole group, this script included. Forwarding both would be the second signal
# that kills aconiq without its profile, so a signal within two seconds of the
# last one forwarded is dropped. A deliberate second Ctrl-C comes later.
last_forward=0
stop_child() {
	local now
	now="$(date +%s)"
	if [ $((now - last_forward)) -ge 2 ]; then
		last_forward="$now"
		echo "==> Signal received; asking aconiq to stop (it writes the profile on the way out)" >&2
		kill -INT -- "-$child" 2>/dev/null || true
	fi
}
trap stop_child INT TERM

# `wait` returns early when a trapped signal arrives, so wait until the child
# is really gone; bash keeps the exit status of a reaped background job, so the
# final `wait` reports it either way.
while kill -0 "$child" 2>/dev/null; do
	wait "$child" || true
done
status=0
wait "$child" || status=$?
end_ns="$(date +%s%N)"

echo "==> Timing"
if [ -s "$time_report" ]; then
	grep -E 'Elapsed \(wall clock\)|User time|System time|Percent of CPU|Maximum resident' "$time_report"
else
	echo "Wall: $(((end_ns - start_ns) / 1000000)) ms"
	# Not piped: in a pipeline `times` would run in a subshell that has no
	# children. The second line is aconiq's CPU, user then system.
	echo "CPU (user system) of this shell, then of aconiq:"
	times
fi

if [ "$status" -ne 0 ]; then
	echo "==> aconiq run exited with status $status (the profile is written regardless)"
fi

if [ -s "$profile" ]; then
	echo "==> Top 25 of $profile"
	go tool pprof -top -nodecount=25 "$aconiq" "$profile"
fi

# The run's status is the script's. A subshell rather than `exit`, because
# ShellCheck 0.11 reads a top-level `exit` as the end of anything the trap
# could still call, and reports stop_child as never invoked (SC2329).
(exit "$status")
