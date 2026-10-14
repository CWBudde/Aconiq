package httpv1

import (
	"strings"
	"testing"
)

// The run command logs to stderr, and a refusal is the error attribute of
// the last record that has one, unquoted — the run command's own words rather
// than "request failed".
func TestChildErrorMessageReadsTheLoggedError(t *testing.T) {
	t.Parallel()

	stderr := strings.Join([]string{
		`time=2026-10-14T01:54:03.276+02:00 level=INFO msg="command started" command="aconiq run" args=[]`,
		`time=2026-10-14T01:54:03.277+02:00 level=ERROR msg="command finished" status=error error="cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open \"x\": no such file or directory"`,
		`time=2026-10-14T01:54:03.277+02:00 level=ERROR msg="command failed" user_error=true error="cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open \"x\": no such file or directory"`,
	}, "\n")

	got := childErrorMessage([]byte(stderr))

	want := `cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open "x": no such file or directory`
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// Under --json the records are JSON, one per line; the error is read from
// those the same way.
func TestChildErrorMessageReadsAJSONLoggedError(t *testing.T) {
	t.Parallel()

	stderr := `{"time":"t","level":"INFO","msg":"command started","command":"aconiq run"}
{"time":"t","level":"ERROR","msg":"command finished","status":"error","error":"cli.run: model file not found"}
`

	got := childErrorMessage([]byte(stderr))
	if got != "cli.run: model file not found" {
		t.Fatalf("message = %q", got)
	}
}

// Before the command has a logger — cobra refusing a flag — it prints the bare
// error. With no record to read, stderr itself is the message.
func TestChildErrorMessageFallsBackToBareStderr(t *testing.T) {
	t.Parallel()

	got := childErrorMessage([]byte("  unknown flag: --frobnicate \n"))
	if got != "unknown flag: --frobnicate" {
		t.Fatalf("message = %q, want stderr's", got)
	}
}

func TestTailBufferKeepsOnlyTheTail(t *testing.T) {
	t.Parallel()

	var buf tailBuffer

	_, _ = buf.Write([]byte(strings.Repeat("a", maxChildOutputBytes)))
	_, _ = buf.Write([]byte("end"))

	got := buf.Bytes()
	if len(got) != maxChildOutputBytes || !strings.HasSuffix(string(got), "end") {
		t.Fatalf("tail of %d bytes ending %q", len(got), got[len(got)-3:])
	}
}

func TestHeadBufferKeepsTheHeadAndSaysItOverflowed(t *testing.T) {
	t.Parallel()

	buf := headBuffer{limit: 4}

	n, err := buf.Write([]byte("abc"))
	if n != 3 || err != nil {
		t.Fatalf("write = %d, %v", n, err)
	}

	got, overflowed := buf.Bytes()
	if string(got) != "abc" || overflowed {
		t.Fatalf("got %q, overflowed %t", got, overflowed)
	}

	// A write past the limit is accepted — the child is not to be killed by
	// a broken pipe — but only the head is kept.
	n, err = buf.Write([]byte("defg"))
	if n != 4 || err != nil {
		t.Fatalf("write = %d, %v", n, err)
	}

	got, overflowed = buf.Bytes()
	if string(got) != "abcd" || !overflowed {
		t.Fatalf("got %q, overflowed %t", got, overflowed)
	}
}
