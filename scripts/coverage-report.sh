#!/usr/bin/env bash
# Render backend/coverage.out as Markdown for the CI job summary and the sticky
# pull-request comment.
#
# Usage: coverage-report.sh [profile.out] [output.md]
#
# It also works locally after `just test-coverage`, which is the point: the
# number a developer sees is produced by the same script CI publishes.
#
# ## No scope filter, and that is a measured decision
#
# The sibling MeKo repositories filter the profile before reporting it, because
# `go test ./...` emits a zero-count entry for every package named on the
# command line — test files or not — and in a repository with a large generated
# tree those entries are most of the denominator. Measured here on 2026-09-12:
#
#   * 0 files carrying a `Code generated ... DO NOT EDIT` header
#   * 0 mock/fake/testutil support files
#   * 47 packages, 43 of them with a test file
#   * 16,584 statements across 219 files, 0 duplicate blocks
#
# There is nothing for a filter to remove, so the raw profile *is* the scoped
# one and this script reports it directly. That property is not assumed, it is
# enforced: `coverage-check.sh` fails if generated or support files ever appear,
# which is the moment this file needs a filter rather than a comment.
#
# The same `go test ./...` behaviour is what makes the denominator complete in
# the other direction: a package with no test file is counted at 0%, not
# silently omitted, so untested code drags the percentage down instead of
# hiding from it.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

PROFILE="${1:-backend/coverage.out}"
OUT="${2:-code-coverage-results.md}"

if [ ! -f "${PROFILE}" ]; then
	echo "coverage-report: ${PROFILE} not found — run 'just test-coverage' first" >&2
	exit 2
fi

# `go tool cover -func` is the authority for the headline percentage: it is the
# number `go test -cover` prints and the one a developer reproduces locally.
# The awk pass below re-derives the statement counts from the same profile for
# the breakdown, and coverage-check.sh asserts the two agree — a divergence
# means one of them is parsing the profile wrongly.
TOTAL_LINE="$(cd backend && go tool cover -func="${REPO_ROOT}/${PROFILE}" | tail -1)"
PCT="${TOTAL_LINE##*$'\t'}"
PCT="${PCT##* }"

# One awk pass for every aggregate the report needs. Parsed from the RIGHT, not
# from $1: a record is "<file>:<startLine>.<startCol>,<endLine>.<endCol> <stmts>
# <count>" and Go permits a filename containing whitespace or a colon, which
# would split across fields and silently contribute 0 statements. The two
# numeric fields are always the last two and the position suffix always has the
# shape below, so both ends are anchored and the filename is whatever is left.
read -r STATEMENTS COVERED FILES PACKAGES <<EOF
$(awk '
NR == 1 && $1 == "mode:" { next }
/^[ \t]*$/ { next }
{
    if (NF < 3) next
    if ($(NF - 1) !~ /^[0-9]+$/ || $NF !~ /^[0-9]+$/) next
    stmts = $(NF - 1) + 0
    count = $NF + 0
    pos = $0
    sub(/[ \t]+[0-9]+[ \t]+[0-9]+[ \t]*$/, "", pos)
    if (!match(pos, /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+$/)) next
    path = substr(pos, 1, RSTART - 1)
    sub(/^github\.com\/aconiq\/backend\//, "", path)
    total += stmts
    if (count > 0) hit += stmts
    files[path] = 1
    pkg = path; sub(/\/[^\/]*$/, "", pkg)
    pkgs[pkg] = 1
}
END {
    nf = 0; for (f in files) nf++
    np = 0; for (p in pkgs) np++
    printf "%d %d %d %d\n", total + 0, hit + 0, nf, np
}' "${PROFILE}")
EOF

# The second, monotone metric. The percentage can rise because existing tests
# got deeper while whole packages stay untested; this one answers the other
# question, and `go list` costs nothing on top of a build that already happened.
# It degrades to "n/a" rather than losing the report.
pkg_metric="n/a"
pkg_counts="$(cd backend 2>/dev/null && go list -e -f '{{.ImportPath}} {{len .TestGoFiles}} {{len .XTestGoFiles}}' ./... 2>/dev/null |
	awk '{ total++; if (($2 + $3) > 0) tested++ }
       END { printf "%d %d", tested + 0, total + 0 }')" || pkg_counts=""
if [ -n "${pkg_counts}" ]; then
	read -r tested_pkgs total_pkgs <<<"${pkg_counts}"
	if [ "${total_pkgs:-0}" -gt 0 ]; then
		pkg_metric="$(awk -v t="${tested_pkgs}" -v n="${total_pkgs}" \
			'BEGIN { printf "%d / %d (%.1f%%)", t, n, (t / n) * 100 }')"
	fi
fi

MODE="$(head -1 "${PROFILE}")"
MODE="${MODE#mode: }"

{
	echo "## Backend test coverage"
	echo ""
	echo "### ${PCT} of statements"
	echo ""
	echo "| Metric | Value |"
	echo "|---|---|"
	echo "| Statement coverage | ${PCT} (${COVERED} / ${STATEMENTS} statements in ${FILES} files) |"
	echo "| Packages with a test file | ${pkg_metric} |"
	echo "| Profile mode | ${MODE} |"
	# A quoted heredoc for the prose that carries no values. Markdown wants
	# backticks, and a backtick survives neither an unquoted heredoc (command
	# substitution) nor `echo "..."` once shfmt has simplified the quoting.
	cat <<'PROSE'

The whole profile, unfiltered: this repository has no generated Go code and no
mock/fake/testutil support files, so there is nothing a scope filter would
remove. `scripts/coverage-check.sh` fails if that ever stops being true.

**Read the package metric alongside the percentage.** The percentage can rise
because existing tests got deeper while whole packages stay untested. Untested
packages *are* in the denominator at 0%, so they drag the percentage down
rather than dropping out of it.

See [docs/testing/coverage.md](https://github.com/CWBudde/Aconiq/blob/main/docs/testing/coverage.md)
for the policy and the ratchet ledger.

<details>
<summary>Per-package breakdown</summary>

| Package | Coverage | Statements |
|---|---|---|
PROSE
	awk '
    NR == 1 && $1 == "mode:" { next }
    /^[ \t]*$/ { next }
    {
        if (NF < 3) next
        if ($(NF - 1) !~ /^[0-9]+$/ || $NF !~ /^[0-9]+$/) next
        stmts = $(NF - 1) + 0
        count = $NF + 0
        pos = $0
        sub(/[ \t]+[0-9]+[ \t]+[0-9]+[ \t]*$/, "", pos)
        if (!match(pos, /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+$/)) next
        path = substr(pos, 1, RSTART - 1)
        sub(/^github\.com\/aconiq\/backend\//, "", path)
        sub(/\/[^\/]*$/, "", path)
        total[path] += stmts
        if (count > 0) hit[path] += stmts
    }
    END {
        for (p in total) {
            pct = total[p] > 0 ? (hit[p] / total[p]) * 100 : 0
            printf "| %s | %.1f%% | %d |\n", p, pct, total[p]
        }
    }' "${PROFILE}" | sort -t'|' -k4 -rn
	echo ""
	echo "</details>"
} >"${OUT}"

echo "coverage-report: wrote ${OUT} (${PCT}, ${COVERED}/${STATEMENTS} statements, ${PACKAGES} packages)"
