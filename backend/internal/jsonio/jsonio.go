// Package jsonio holds one JSON file encoding — two-space indentation and a
// trailing newline — and since this pass it is the only one in the repository.
// `json.MarshalIndent` appears in exactly one place in non-test code, below.
//
// It first collected the eight copies that announced themselves by name: seven
// writers called `writeJSONFile` or `writeJSON`, in `app/cli`, `engine`,
// `io/projectfs`, `report/reporting` and both `qa/acceptance` runners, plus the
// same two lines inlined in `projectfs.Save`. The nine that stayed inlined at
// their `os.WriteFile` call followed — in `report/results`, `report/export`,
// `app/cli`, `qa/golden`, `api/httpv1` and `standards/beb/exposure`. Every byte
// any of them produces is pinned by a golden file or by a run digest, so one
// copy drifting is a diff in files nobody meant to touch.
//
// Two of the nine are not file writers and kept the shape that makes them what
// they are. `qa/golden.AssertJSONSnapshot` compares rather than writes, and is
// what actually pins the indentation and the trailing newline of 59 golden
// files — a run digest does not, because it re-encodes every `.json` compact
// and key-sorted before hashing. `api/httpv1.writeJSON` answers an
// `http.ResponseWriter` and, on a marshal failure, substitutes a canned body
// and rewrites the status to 500; that branch is intact, and carries the
// trailing newline itself so both paths still end the same way.
//
// What stays with the callers is the error taxonomy, because it is not shared:
// `app/cli` and `io/projectfs` wrap a failure as `domainerrors.New` under their
// own op string, which is what the CLI derives its exit code from, and the rest
// use `fmt.Errorf` with their own wording. Marshal says only that marshalling
// failed, and each caller keeps its own kind, op string and wording around that.
//
// The write mechanism no longer does. This doc used to say `io/projectfs`
// replaced a file through a temp file and a rename "while the others write in
// place", and that difference was not a decision — it meant the same artifacts
// were replaced atomically through the HTTP API and non-atomically through the
// CLI. `internal/atomicfile` now holds that mechanism, and both
// `projectfs.writeJSONFile` and `cli.writeJSONFile` go through it. The
// remaining in-place writers are listed above and have not been reviewed.
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
