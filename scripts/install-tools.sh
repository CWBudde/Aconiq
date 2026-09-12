#!/usr/bin/env bash
# Install the developer toolchain at exactly the versions tools.versions pins.
#
# CI runs this too, so there is one installation recipe rather than one per
# environment — including the two traps it encodes:
#
#   * treefmt cannot be installed with `go install`: its module zip contains a
#     test fixture whose path has an emoji, which is not a valid module file
#     path, and proxy.golang.org 404s every treefmt/v2 .info and .zip. Use the
#     release tarball.
#   * govulncheck must be built with the toolchain backend/go.mod names. `go
#     install tool@version` resolves in the tool's own module, so it is built by
#     whatever compiler is active; one built by an older Go cannot load a module
#     whose toolchain compiles a newer standard library, and dies before it
#     scans anything.
#
# Binaries land in $TOOLS_BIN (default: the Go bin directory, which is already
# on PATH wherever `go install` works). prettier comes from npm, which must be
# installed separately.
#
# Usage: install-tools.sh [all|format|lint|licenses|vuln]...
#
# The groups exist so CI can install only what a given job needs; `all` (the
# default) is what a developer wants. Three pinned tools are not installed here
# because they bootstrap themselves: `just` and `bun` (setup-just / setup-bun in
# CI, a package manager locally) and goreleaser, which only ever runs in CI.
# check-tools.sh still holds the first two to their pins.

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

# shellcheck source-path=SCRIPTDIR/..
# shellcheck source=tools.versions
source ./tools.versions

groups=("$@")
if ((${#groups[@]} == 0)); then
	groups=(all)
fi

for group in "${groups[@]}"; do
	case "$group" in
	all | format | lint | licenses | vuln) ;;
	*)
		echo "usage: $(basename "$0") [all|format|lint|licenses|vuln]..." >&2
		exit 2
		;;
	esac
done

wants() {
	local group
	for group in "${groups[@]}"; do
		[[ $group == all || $group == "$1" ]] && return 0
	done
	return 1
}

tools_bin="${TOOLS_BIN:-$(go env GOBIN)}"
if [[ -z $tools_bin ]]; then
	tools_bin="$(go env GOPATH)/bin"
fi
mkdir -p "$tools_bin"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

echo "==> installing into $tools_bin"

case ":${PATH}:" in
*":${tools_bin}:"*) ;;
*) echo "install-tools: warning: $tools_bin is not on PATH" >&2 ;;
esac

install_go_tool() {
	local package="$1" toolchain="${2:-auto}"
	echo "--> go install $package"
	GOBIN="$tools_bin" GOTOOLCHAIN="$toolchain" go install "$package"
}

install_treefmt() {
	local version="${TREEFMT_VERSION#v}" goarch asset url
	case "$arch" in
	x86_64 | amd64) goarch=amd64 ;;
	arm64 | aarch64) goarch=arm64 ;;
	*)
		echo "install-tools: no treefmt release for $os/$arch" >&2
		return 1
		;;
	esac

	asset="treefmt_${version}_${os}_${goarch}.tar.gz"
	url="https://github.com/numtide/treefmt/releases/download/v${version}/${asset}"

	echo "--> treefmt $version"
	curl -sSfL "$url" | tar -xz -C "$tools_bin" treefmt
}

install_shellcheck() {
	local version="$SHELLCHECK_VERSION" scarch url
	case "$arch" in
	x86_64 | amd64) scarch=x86_64 ;;
	arm64 | aarch64) scarch=aarch64 ;;
	*)
		echo "install-tools: no shellcheck release for $os/$arch" >&2
		return 1
		;;
	esac

	url="https://github.com/koalaman/shellcheck/releases/download/${version}/shellcheck-${version}.${os}.${scarch}.tar.xz"

	echo "--> shellcheck $version"

	# The only tool whose archive has to be unpacked before installing, so the
	# scratch directory and the trap that removes it live in a subshell: an EXIT
	# trap there fires on the way out, success or failure, and leaves no handler
	# behind in the shell that called it.
	(
		tmp="$(mktemp -d)"
		trap 'rm -rf "$tmp"' EXIT

		curl -sSfL "$url" | tar -xJ -C "$tmp"
		install -m 0755 "$tmp/shellcheck-${version}/shellcheck" "$tools_bin/shellcheck"
	)
}

# prettier is installed into $TOOLS_BIN rather than globally: a global install
# would silently take over whatever prettier the machine already has, and the
# whole point of the pin is that this one version is the one treefmt runs.
install_prettier() {
	if ! command -v npm >/dev/null; then
		echo "install-tools: npm not found; install Node.js and re-run" >&2
		return 1
	fi

	echo "--> prettier $PRETTIER_VERSION"
	npm install --prefix "$tools_bin/npm" --silent --no-fund --no-audit "prettier@${PRETTIER_VERSION}"
	ln -sf "$tools_bin/npm/node_modules/.bin/prettier" "$tools_bin/prettier"
}

if wants format; then
	install_go_tool "mvdan.cc/gofumpt@${GOFUMPT_VERSION}"
	install_go_tool "github.com/daixiang0/gci@${GCI_VERSION}"
	install_go_tool "mvdan.cc/sh/v3/cmd/shfmt@${SHFMT_VERSION}"
	install_treefmt
	install_shellcheck
	install_prettier
fi

if wants lint; then
	install_go_tool "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"
fi

if wants licenses; then
	install_go_tool "github.com/google/go-licenses@${GO_LICENSES_VERSION}"
fi

if wants vuln; then
	install_go_tool "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" \
		"$(sed -n 's/^toolchain //p' backend/go.mod)"
fi

if wants format; then
	echo
	echo "==> verifying (against PATH, not against $tools_bin)"
	if ! "$repo_root/scripts/check-tools.sh" format; then
		echo "The pinned versions are in $tools_bin; another copy earlier on PATH is shadowing them." >&2
		exit 1
	fi
fi
