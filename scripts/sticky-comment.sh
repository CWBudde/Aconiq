#!/usr/bin/env bash
# Post or update one pull-request comment, identified by a hidden marker.
#
# Usage: sticky-comment.sh <marker> <body-file>
#
#   marker     a short identifier, e.g. "coverage-backend". One comment per
#              marker per pull request.
#   body-file  the Markdown to post.
#
# Environment: GH_TOKEN (or GITHUB_TOKEN), GITHUB_REPOSITORY, and PR_NUMBER.
#
# ## Why not the usual sticky-comment action
#
# Because the repository is public and this script runs `gh`, which is on every
# GitHub runner and is already how .github/workflows/go-ci.yml files a
# govulncheck issue. A third-party JavaScript action would need a supply-chain
# review, a pin, and a renovation schedule to do forty lines of API call.
#
# ## Why sticky at all
#
# A new comment per push buries the pull request. Editing one in place means the
# comment always shows the current commit's numbers — which is also why every
# caller must post *something* on every run: the comment outlives the run that
# wrote it, so a skipped update silently leaves an older commit's numbers
# standing as if they were current.
#
# Exits non-zero on a failed API call. Callers treat that as advisory: a fork
# pull request gets a read-only GITHUB_TOKEN and cannot comment at all, and a
# coverage comment failing to post must never fail a check.
set -euo pipefail

MARKER_NAME="${1:?usage: sticky-comment.sh <marker> <body-file>}"
BODY_FILE="${2:?usage: sticky-comment.sh <marker> <body-file>}"

: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
: "${PR_NUMBER:?PR_NUMBER is not set}"

[ -f "${BODY_FILE}" ] || {
	echo "sticky-comment: ${BODY_FILE} not found" >&2
	exit 2
}

MARKER="<!-- aconiq-sticky: ${MARKER_NAME} -->"

# The marker leads the body so the lookup below can match on a prefix rather
# than searching the whole comment — a report that happens to quote the marker
# in a code block then cannot impersonate one.
PAYLOAD="$(mktemp)"
trap 'rm -f "${PAYLOAD}"' EXIT
{
	printf '%s\n' "${MARKER}"
	cat "${BODY_FILE}"
} >"${PAYLOAD}"

# `.[0]` and not "the newest": if two comments ever carry the same marker,
# editing the same one every time keeps the duplicate stable and visible rather
# than alternating between them.
EXISTING="$(gh api "repos/${GITHUB_REPOSITORY}/issues/${PR_NUMBER}/comments" \
	--paginate --jq "[.[] | select(.body | startswith(\"${MARKER}\"))] | .[0].id // empty")"

if [ -n "${EXISTING}" ]; then
	gh api --method PATCH "repos/${GITHUB_REPOSITORY}/issues/comments/${EXISTING}" \
		-F "body=@${PAYLOAD}" --silent
	echo "sticky-comment: updated comment ${EXISTING} (${MARKER_NAME})"
else
	gh api --method POST "repos/${GITHUB_REPOSITORY}/issues/${PR_NUMBER}/comments" \
		-F "body=@${PAYLOAD}" --silent
	echo "sticky-comment: created a comment (${MARKER_NAME})"
fi
