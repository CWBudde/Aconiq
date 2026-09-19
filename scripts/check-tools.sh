#!/usr/bin/env bash
# Verify the installed developer tools against the pins in tools.versions.
#
# The formatters are the sharp case: treefmt runs whatever is on PATH, and a
# different prettier or gofumpt rewrites files the pinned version considers
# correct. That failed silently in both directions — `just fmt` produced a diff
# CI rejected, and `just check-formatted` reported a backlog that did not exist.
# So `fmt` and `check-formatted` run this first, for the `format` group.
#
# `golangci-lint` fails the same way one step further on. It does not rewrite
# the tree, but `default: all` means the set of enabled linters is whatever the
# binary carries, so a newer one enables checks `.golangci.yml` has never seen:
# v2.13 renamed `exhaustruct` to `exhaustruct_v5`, which no disable line covers,
# and reported 1917 findings on a tree whose Go CI was green at that commit.
# `lint` and `lint-fix` run this first, for the `lint` group.
#
# Usage: check-tools.sh [--quiet] [format|lint|all]

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
	format | lint | all) group="$arg" ;;
	*)
		echo "usage: $(basename "$0") [--quiet] [format|lint|all]" >&2
		exit 2
		;;
	esac
done

# Everything treefmt invokes. `just fmt` is only reproducible if all of these
# match, which is also why treefmt no longer runs with --allow-missing-formatter.
format_tools=(treefmt gofumpt gci shfmt shellcheck prettier)

# What `just lint` invokes. Its own group, because `lint` and `lint-fix` gate on
# it and have no reason to wait on the formatters.
lint_tools=(golangci-lint)

# The remaining gates. Two pins are deliberately not listed: go-licenses, whose
# binary reports no version of its own, and goreleaser, which only ever runs in
# CI. Unlike the formatters, a mismatch in this group changes a verdict rather
# than the tree.
other_tools=(just bun govulncheck)

case "$group" in
format) tools=("${format_tools[@]}") ;;
lint) tools=("${lint_tools[@]}") ;;
*) tools=("${format_tools[@]}" "${lint_tools[@]}" "${other_tools[@]}") ;;
esac

# expected_version maps a tool name onto its tools.versions entry: gofumpt ->
# $GOFUMPT_VERSION, golangci-lint -> $GOLANGCI_LINT_VERSION.
expected_version() {
	local var
	var="$(echo "$1" | tr '[:lower:]-' '[:upper:]_')_VERSION"
	echo "${!var:-}"
}

# installed_version reads the version out of an installed binary. Most Go tools
# here carry their module version in their build info, which is exact and needs
# no per-tool output parsing; the non-Go tools are parsed individually, and so is
# `golangci-lint`: CI installs it from a release archive via
# golangci-lint-action, and whether such an archive carries module build info at
# all is a property of how it was built, not something the tool promises. Its own
# `version --short` is the documented interface and answers for both a release
# build and a `go install` one.
installed_version() {
	local tool="$1" path
	path="$(command -v "$tool")" || return 1

	case "$tool" in
	just) just --version | awk '{print $2}' ;;
	golangci-lint) golangci-lint version --short ;;
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
