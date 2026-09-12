#!/usr/bin/env bash
# Verify the installed developer tools against the pins in tools.versions.
#
# The formatters are the sharp case: treefmt runs whatever is on PATH, and a
# different prettier or gofumpt rewrites files the pinned version considers
# correct. That failed silently in both directions — `just fmt` produced a diff
# CI rejected, and `just check-formatted` reported a backlog that did not exist.
# So `fmt` and `check-formatted` run this first, for the `format` group.
#
# Usage: check-tools.sh [--quiet] [format|all]

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

# shellcheck source-path=SCRIPTDIR/..
# shellcheck source=tools.versions
source ./tools.versions

quiet=false
group=all

for arg in "$@"; do
	case "$arg" in
	--quiet) quiet=true ;;
	format | all) group="$arg" ;;
	*)
		echo "usage: $(basename "$0") [--quiet] [format|all]" >&2
		exit 2
		;;
	esac
done

# Everything treefmt invokes. `just fmt` is only reproducible if all of these
# match, which is also why treefmt no longer runs with --allow-missing-formatter.
format_tools=(treefmt gofumpt gci shfmt shellcheck prettier)

# The remaining gates. Two pins are deliberately not listed: go-licenses, whose
# binary reports no version of its own, and goreleaser, which only ever runs in
# CI. Unlike the formatters, a mismatch in this group changes a verdict rather
# than the tree.
other_tools=(just bun golangci-lint govulncheck)

if [[ $group == format ]]; then
	tools=("${format_tools[@]}")
else
	tools=("${format_tools[@]}" "${other_tools[@]}")
fi

# expected_version maps a tool name onto its tools.versions entry: gofumpt ->
# $GOFUMPT_VERSION, golangci-lint -> $GOLANGCI_LINT_VERSION.
expected_version() {
	local var
	var="$(echo "$1" | tr '[:lower:]-' '[:upper:]_')_VERSION"
	echo "${!var:-}"
}

# installed_version reads the version out of an installed binary. Every Go tool
# here carries its module version in its build info, which is exact and needs no
# per-tool output parsing; the three non-Go tools are parsed individually.
installed_version() {
	local tool="$1" path
	path="$(command -v "$tool")" || return 1

	case "$tool" in
	just) just --version | awk '{print $2}' ;;
	bun) bun --version ;;
	prettier) prettier --version ;;
	shellcheck) shellcheck --version | awk '/^version:/ {print $2}' ;;
	*) go version -m "$path" | awk '$1 == "mod" {print $3; exit}' ;;
	esac
}

# normalize strips the leading v and any build metadata, so that v2.5.0+dirty
# (how the treefmt release tarball identifies itself) compares equal to v2.5.0.
normalize() {
	local version="${1#v}"
	echo "${version%%+*}"
}

failed=()

for tool in "${tools[@]}"; do
	want="$(expected_version "$tool")"
	if [[ -z $want ]]; then
		echo "check-tools: no pin for $tool in tools.versions" >&2
		exit 2
	fi

	if ! got="$(installed_version "$tool" 2>/dev/null)" || [[ -z $got ]]; then
		failed+=("$tool: not installed (want $want)")
		$quiet || printf '%-16s %-12s %s\n' "$tool" "$want" "MISSING"
		continue
	fi

	if [[ "$(normalize "$got")" == "$(normalize "$want")" ]]; then
		$quiet || printf '%-16s %-12s %s\n' "$tool" "$want" "ok"
	else
		failed+=("$tool: have $got, want $want")
		$quiet || printf '%-16s %-12s %s\n' "$tool" "$want" "MISMATCH (have $got)"
	fi
done

if ((${#failed[@]} == 0)); then
	exit 0
fi

echo >&2
echo "Toolchain does not match tools.versions:" >&2
for line in "${failed[@]}"; do
	echo "  $line" >&2
done
echo >&2
echo "Run 'just install-tools' to install the pinned versions." >&2

exit 1
