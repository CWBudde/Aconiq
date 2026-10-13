package httpv1

import (
	"strings"
	"testing"
)

// The run command logs its error on stdout, through slog; a refusal answered
// from stderr alone came back as "request failed". The message is the error
// attribute of the last record that has one, unquoted.
func TestChildErrorMessageReadsTheLoggedError(t *testing.T) {
	t.Parallel()

	stdout := strings.Join([]string{
		`time=2026-10-14T01:54:03.276+02:00 level=INFO msg="command started" command="aconiq run" args=[]`,
		`Evidence tier: normative — implements the named standard's tables and equations.`,
		`time=2026-10-14T01:54:03.277+02:00 level=ERROR msg="command finished" status=error error="cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open \"x\": no such file or directory"`,
	}, "\n")

	got := childErrorMessage([]byte(stdout), nil)

	want := `cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open "x": no such file or directory`
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// stderr, when the command wrote any, is the more direct account and wins.
func TestChildErrorMessagePrefersStderr(t *testing.T) {
	t.Parallel()

	got := childErrorMessage([]byte(`level=ERROR error="from the log"`), []byte("  from stderr \n"))
	if got != "from stderr" {
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
