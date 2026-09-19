// Package jsonio holds one JSON file encoding — two-space indentation and a
// trailing newline — for the writers that have been migrated onto it.
//
// It is not yet the only such encoding in the repository, and the doc should
// not be read as claiming so. What it collected were the eight copies that
// announced themselves by name: seven writers called `writeJSONFile` or
// `writeJSON`, in `app/cli`, `engine`, `io/projectfs`, `report/reporting` and
// both `qa/acceptance` runners, plus the same two lines inlined in
// `projectfs.Save`. Roughly as many again remain inlined at their `os.WriteFile`
// call — `report/results`, `report/export`, `qa/golden`, `api/httpv1` and
// `standards/beb/exposure` — and PLAN.md tracks them under Priority 7. Every
// byte any of them produces is pinned by a golden file or by a run digest, so
// one copy drifting is a diff in files nobody meant to touch.
//
// What stays with the callers is everything around the encoding, because none
// of it is shared: `io/projectfs` replaces a file through a temp file and a
// rename while the others write in place, and the error taxonomies differ on
// purpose — `app/cli` and `io/projectfs` wrap a failure as `domainerrors.New`
// under their own op string, which is what the CLI derives its exit code from,
// and the rest use `fmt.Errorf` with their own wording. Marshal says only that
// marshalling failed, and each caller keeps its own kind, op string and wording
// around that.
package jsonio

import (
	"encoding/json"
	"fmt"
)

// Marshal encodes value the way every JSON file in a project is written:
// `json.MarshalIndent` with two-space indentation, plus a trailing newline.
//
// The newline is the only byte that is not `encoding/json`'s. It is there so a
// file ends the way a text file ends — `git diff` shows no "\ No newline at end
// of file", and a golden file compares clean.
//
// Nothing is normalized on the way through: a nil slice still encodes as
// `null`, and <, > and & are still escaped, exactly as `json.MarshalIndent`
// leaves them. On failure Marshal returns nil bytes; naming the file and
// choosing the taxonomy is the caller's job.
//
// The failure text is the one thing this package did not leave exactly as it
// found it: a caller that used to render `encode report json: <json error>` now
// renders `encode report json: marshal json: <json error>`. That clause is
// deliberate rather than tolerated. `wrapcheck` is enforced here — the 2026-08-28
// debt pass fixed all 190 of its findings in code rather than suppressing any,
// and `docs/lint-triage.md` records the reasoning — so returning
// `json.MarshalIndent`'s error bare is a lint failure, and `internal/jsonio`
// would have to become the first package excluded from a rule the repository
// went to some trouble to satisfy. The kind, the op string and the caller's own
// wording are unchanged, and reaching this path at all takes a channel or a
// function value, which nothing here encodes.
func Marshal(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	return append(encoded, '\n'), nil
}
