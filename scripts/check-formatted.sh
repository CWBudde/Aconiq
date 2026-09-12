#!/usr/bin/env bash
# Read-only formatting gate.
#
# `treefmt --fail-on-change` formats in place and *then* reports, so using it
# directly as a check rewrites the working tree — in CI that is merely
# surprising, locally it silently reformats unrelated work. This script gets the
# same verdict without touching the tree: it mirrors every file treefmt would
# see into a temporary directory, formats the mirror, and compares.
#
# Exit 0 when everything is formatted, 1 when it is not (listing the files).

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

mirror="$work_dir/tree"
file_list="$work_dir/files"
mkdir -p "$mirror"

# Tracked files plus untracked ones that are not ignored: exactly the set a
# commit could contain. Ignored paths (node_modules/, bin/, .trunk/) stay out,
# which is also what treefmt.toml excludes by hand.
#
# A tracked file deleted in the working tree is still listed by the index, and
# has to be dropped here: tar cannot stat it, and the check would die on a file
# with no content to format rather than report on the files that have some.
git ls-files -z --cached --others --exclude-standard |
	while IFS= read -r -d '' file; do
		if [[ -e $file || -L $file ]]; then
			printf '%s\0' "$file"
		fi
	done >"$file_list"

if [[ ! -s $file_list ]]; then
	echo "check-formatted: no files to check" >&2
	exit 0
fi

tar --null --files-from="$file_list" --create --file - | tar --extract --file - --directory "$mirror"

# --no-cache because the mirror lives at a fresh path every run, so a cache
# keyed on it would only ever grow.
treefmt \
	--no-cache \
	--tree-root "$mirror" \
	--config-file "$mirror/treefmt.toml" \
	>"$work_dir/treefmt.log" 2>&1 || {
	status=$?
	cat "$work_dir/treefmt.log" >&2
	exit "$status"
}

unformatted=()
while IFS= read -r -d '' file; do
	# A file the mirror does not have is a broken symlink or a race, not a
	# formatting verdict; leave it to the other gates.
	[[ -f "$mirror/$file" ]] || continue
	cmp -s "$file" "$mirror/$file" || unformatted+=("$file")
done <"$file_list"

if ((${#unformatted[@]} == 0)); then
	exit 0
fi

echo "The following files are not formatted (run 'just fmt'):" >&2
for file in "${unformatted[@]}"; do
	echo "  $file" >&2
done

echo >&2
for file in "${unformatted[@]}"; do
	diff -u --label "$file" --label "$file (formatted)" "$file" "$mirror/$file" >&2 || true
done

exit 1
