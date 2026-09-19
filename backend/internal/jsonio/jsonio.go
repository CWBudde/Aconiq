// Package jsonio holds the one encoding every JSON file this project writes
// uses: two-space indentation, and a trailing newline.
//
// Seven writers named `writeJSONFile` or `writeJSON` carried their own copy of
// those two lines, in `app/cli`, `engine`, `io/projectfs`, `report/reporting`
// and both `qa/acceptance` runners, and `projectfs.Save` carried an eighth
// inline. Every byte they produce is pinned by a golden file or by a run
// digest, so one copy drifting is a diff in files nobody meant to touch. The
// encoding lives here now and those copies are gone.
//
// What stays with the callers is everything around it, because none of it is
// shared: `io/projectfs` replaces a file through a temp file and a rename while
// the others write in place, and the error taxonomies differ on purpose —
// `app/cli` and `io/projectfs` wrap a failure as `domainerrors.New` under their
// own op string, which is what the CLI derives its exit code from, and the rest
// use `fmt.Errorf` with their own wording. Marshal therefore says only that
// marshalling failed, and each caller says what it has always said.
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
func Marshal(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	return append(encoded, '\n'), nil
}
