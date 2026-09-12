#!/usr/bin/env bash
# Enforce the backend coverage floor and the assumptions the number rests on.
#
# Usage: coverage-check.sh [profile.out]
#
# Exit codes are distinct on purpose, because two different things go wrong here
# and they need different responses:
#
#   0  coverage meets the floor, or $COVERAGE_FLOOR is unset (report only)
#   1  coverage is below the floor — a genuine regression
#   2  the input or the configuration is not trustworthy — do not believe the
#      percentage at all, in either direction
#
# `just coverage-check` cannot preserve that distinction (just, like make, has
# one failure exit status), so call this script directly when 1-vs-2 matters —
# which is what .github/workflows/go-ci.yml does.
#
# Every exit-2 guard exists because a broken measurement reads as a *low*
# percentage rather than as an error: a truncated profile reports 0%, a profile
# with no `mode:` header would be read as valid, and a non-numeric floor is
# coerced to 0 by awk and passes everything. Failing loudly is the only way
# those stay visible.
#
# The floor is set in .github/workflows/go-ci.yml and ratcheted in
# docs/testing/coverage.md. Locally:
#
#   just coverage-check                    # report only
#   COVERAGE_FLOOR=75 just coverage-check  # enforce
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

PROFILE="${1:-backend/coverage.out}"

# Sanity bounds on the profile itself. Measured 2026-09-12 on 87da006: 16,584
# statements in 219 files across 47 packages. Wide enough never to trip on a
# real run, tight enough to catch a run that measured a fraction of the tree.
MIN_STATEMENTS="${COVERAGE_MIN_STATEMENTS:-10000}"
MIN_FILES="${COVERAGE_MIN_FILES:-150}"

die2() {
	echo "❌ coverage-check: $*" >&2
	exit 2
}

# Validate the bounds before anything compares against them. `[ 1 -lt abc ]`
# prints "integer expression expected" and returns false, and `set -e` does not
# fire inside an `if` condition — so a typo in one of these silently turns its
# own guard off and lets a one-statement profile through with exit 0. The guards
# exist because a broken measurement reads as a plausible number; a broken
# *guard* has to be just as loud.
for bound in MIN_STATEMENTS MIN_FILES; do
	# Indirect expansion rather than eval: the value reaches the case below
	# without a second round of shell parsing, so a bound set to `x; rm -rf /`
	# is rejected as a non-integer instead of being run.
	case "${!bound}" in
	"" | *[!0-9]*) die2 "COVERAGE_${bound}='${!bound}' is not a non-negative integer." ;;
	esac
done

[ -f "${PROFILE}" ] || die2 "${PROFILE} not found — run 'just test-coverage' first."

# Reject a profile without a recognised mode header rather than parsing on. A
# truncated or half-written file otherwise reports a plausible low percentage,
# which is the failure shape every guard here exists to prevent.
MODE_LINE="$(head -1 "${PROFILE}")"
case "${MODE_LINE}" in
"mode: set" | "mode: count" | "mode: atomic") ;;
*)
	die2 "${PROFILE} does not start with a coverage mode header (first line: ${MODE_LINE:-<empty>}) — it is truncated or corrupt."
	;;
esac
MODE="${MODE_LINE#mode: }"

# `-covermode=atomic` is not cosmetic. The engine's worker pool and a good deal
# of the suite run in parallel, and a lost non-atomic counter write undercounts
# without emitting any error at all.
[ "${MODE}" = "atomic" ] || die2 "profile mode is '${MODE}', expected 'atomic' — the number is not comparable."

# One pass for every number below, anchored at both ends of the record so a
# filename containing whitespace or a colon cannot shift the numeric fields.
# See the same comment in coverage-report.sh.
read -r STATEMENTS COVERED FILES DUPES MALFORMED <<EOF
$(awk '
NR == 1 && $1 == "mode:" { next }
/^[ \t]*$/ { next }
{
    if (NF < 3) { malformed++; next }
    if ($(NF - 1) !~ /^[0-9]+$/ || $NF !~ /^[0-9]+$/) { malformed++; next }
    stmts = $(NF - 1) + 0
    count = $NF + 0
    pos = $0
    sub(/[ \t]+[0-9]+[ \t]+[0-9]+[ \t]*$/, "", pos)
    if (!match(pos, /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+$/)) { malformed++; next }
    if (pos in seen) dupes++
    seen[pos] = 1
    path = substr(pos, 1, RSTART - 1)
    total += stmts
    if (count > 0) hit += stmts
    files[path] = 1
}
END {
    nf = 0; for (f in files) nf++
    printf "%d %d %d %d %d\n", total + 0, hit + 0, nf, dupes + 0, malformed + 0
}' "${PROFILE}")
EOF

echo "Backend statement coverage: ${COVERED} / ${STATEMENTS} statements in ${FILES} files | mode: ${MODE}"

# A record the parser could not read is a block missing from the denominator,
# which reads as a *higher* percentage rather than as an error. There is no
# acceptable number of these.
[ "${MALFORMED}" -eq 0 ] ||
	die2 "${MALFORMED} records in ${PROFILE} could not be parsed — the profile is corrupt and the denominator is incomplete."

# Without -coverpkg each block appears exactly once (verified: 0 duplicates in
# 11,302 records). If that ever changes the sums above start double-counting
# silently, so it is an error rather than a quiet merge.
[ "${DUPES}" -eq 0 ] ||
	die2 "${DUPES} duplicate blocks in ${PROFILE} — is -coverpkg set? The statement sums are no longer exact."

[ "${STATEMENTS}" -ge "${MIN_STATEMENTS}" ] ||
	die2 "only ${STATEMENTS} statements in the profile (expected at least ${MIN_STATEMENTS}) — the run is truncated or never happened."

[ "${FILES}" -ge "${MIN_FILES}" ] ||
	die2 "only ${FILES} files in the profile (expected at least ${MIN_FILES}) — the run is truncated or never happened."

# The premise under coverage-report.sh's "no scope filter" decision, checked
# rather than trusted. The moment generated or support code enters the tree the
# raw profile stops being the scoped one: generated files land in the
# denominator at 0% and depress a percentage that is supposed to describe
# hand-written code. This is the guard that says "now write the filter", and it
# names the files so that is a five-minute job rather than an investigation.
GENERATED="$(grep -rl 'Code generated' backend --include='*.go' 2>/dev/null || true)"
if [ -n "${GENERATED}" ]; then
	while IFS= read -r offender; do printf '  %s\n' "${offender}" >&2; done <<<"${GENERATED}"
	die2 "generated Go files are in the tree, so the raw profile is no longer the scoped one — the files above need excluding (see the header of scripts/coverage-report.sh)."
fi

SUPPORT="$(find backend -name '*.go' \( -name '*mock*' -o -name '*fake*' -o -name '*testutil*' \) 2>/dev/null || true)"
if [ -n "${SUPPORT}" ]; then
	while IFS= read -r offender; do printf '  %s\n' "${offender}" >&2; done <<<"${SUPPORT}"
	die2 "mock/fake/testutil support files are in the tree and are scoring themselves inside the tracked metric — the files above need excluding (see the header of scripts/coverage-report.sh)."
fi

# `go tool cover -func` is what the report prints and what a developer sees
# locally; the awk pass above is what the floor is enforced against. If the two
# ever disagree, one of them is parsing the profile wrongly and neither number
# can be trusted. Compared at one decimal, which is the precision go prints.
GO_PCT="$(cd backend && go tool cover -func="${REPO_ROOT}/${PROFILE}" | tail -1 | grep -o '[0-9.]*%$' | tr -d '%')"
if [ -n "${GO_PCT}" ]; then
	if ! awk -v a="${GO_PCT}" -v c="${COVERED}" -v t="${STATEMENTS}" \
		'BEGIN { b = t > 0 ? (c / t) * 100 : 0; exit !(a - b < 0.05 && b - a < 0.05) }'; then
		die2 "go tool cover reports ${GO_PCT}% but the profile sums to $(awk -v c="${COVERED}" -v t="${STATEMENTS}" 'BEGIN { printf "%.1f", (c / t) * 100 }')% — the two parsers disagree, so neither number is trustworthy."
	fi
fi

# Normalise conservatively: strip only surrounding whitespace and at most one
# trailing `%`. Deleting every space and `%` first would repair malformed input
# into something plausible — `5 4` and `5%4` both become `54`, and ` % ` becomes
# empty, which silently disables enforcement rather than failing.
FLOOR="${COVERAGE_FLOOR:-}"
FLOOR="${FLOOR#"${FLOOR%%[![:space:]]*}"}"
FLOOR="${FLOOR%"${FLOOR##*[![:space:]]}"}"
if [ -n "${FLOOR}" ]; then
	FLOOR="${FLOOR%\%}"
	FLOOR="${FLOOR%"${FLOOR##*[![:space:]]}"}"
	# Validate before awk sees it: `awk -v f=abc 'BEGIN{print f+0}'` prints 0,
	# which would turn a typo into a floor nothing can ever fail. An empty result
	# is a failure too, not "unset" — `%` alone trims to nothing, and silently
	# switching to report-only is the outcome this guard exists to prevent.
	if ! [[ ${FLOOR} =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
		die2 "COVERAGE_FLOOR='${COVERAGE_FLOOR}' is not a number."
	fi
fi

if [ -z "${FLOOR}" ]; then
	echo "coverage-check: COVERAGE_FLOOR unset — reporting only, not enforcing."
	exit 0
fi

echo "Floor: ${FLOOR}%"

# Compare the exact ratio, never the rounded display value. `go tool cover`
# prints one decimal, so 74.96% renders as "75.0%" and would clear a floor of 75
# while being below it. The exit-code contract says below-floor exits 1, and
# "below" has to mean below.
if awk -v c="${COVERED}" -v t="${STATEMENTS}" -v f="${FLOOR}" \
	'BEGIN { exit !(t + 0 > 0 && (c / t) * 100 < f + 0) }'; then
	exact="$(awk -v c="${COVERED}" -v t="${STATEMENTS}" 'BEGIN { printf "%.4f", (c / t) * 100 }')"
	echo "❌ coverage-check: coverage ${exact}% is below the floor of ${FLOOR}%." >&2
	echo "   This check is advisory — it does not block a merge. See docs/testing/coverage.md." >&2
	exit 1
fi

echo "✅ coverage-check: coverage meets the floor of ${FLOOR}%."
