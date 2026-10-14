package httpv1

import (
	"strings"
	"testing"
)

// readChildFailure reads what a failed child said about its failure from
// each stderr layout the run command produces.
func TestReadChildFailure(t *testing.T) {
	t.Parallel()

	const refusal = `cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open "x": no such file or directory`

	const quotedRefusal = `"cli.loadValidatedModel: model file not found: /p/.noise/model/model.normalized.geojson: open \"x\": no such file or directory"`

	const (
		textStarted = `time=2026-10-14T01:54:03.276+02:00 level=INFO msg="command started" command="aconiq run" args=[]`
		textWarning = `time=2026-10-14T01:54:03.277+02:00 level=WARN msg="terrain DTM load failed, continuing without terrain" error="open dtm.tif: no such file or directory"`
		jsonStarted = `{"time":"t","level":"INFO","msg":"command started","command":"aconiq run"}`
		jsonWarning = `{"time":"t","level":"WARN","msg":"terrain DTM load failed, continuing without terrain","error":"open dtm.tif: no such file or directory"}`
	)

	for _, testCase := range []struct {
		name   string
		stderr []string
		want   childFailure
	}{
		{
			// A refusal is the error attribute of the last ERROR record,
			// unquoted — the run command's own words.
			name: "text refusal",
			stderr: []string{
				textStarted,
				`time=t level=ERROR msg="command finished" status=error error=` + quotedRefusal,
				`time=t level=ERROR msg="command failed" user_error=true error=` + quotedRefusal,
			},
			want: childFailure{message: refusal, mayBeUserError: true},
		},
		{
			name: "JSON refusal",
			stderr: []string{
				jsonStarted,
				`{"time":"t","level":"ERROR","msg":"command finished","status":"error","error":` + quotedRefusal + `}`,
				`{"time":"t","level":"ERROR","msg":"command failed","user_error":true,"error":` + quotedRefusal + `}`,
			},
			want: childFailure{message: refusal, mayBeUserError: true},
		},
		{
			// An internal failure is logged the same way, without the
			// user-error flag.
			name: "internal failure",
			stderr: []string{
				textStarted,
				`time=t level=ERROR msg="command failed" user_error=false error="write results: disk full"`,
			},
			want: childFailure{message: "write results: disk full"},
		},
		{
			// A warning carries an error attribute but did not fail the
			// run; the failure is the ERROR record after it.
			name:   "warning before the refusal",
			stderr: []string{textStarted, textWarning, `time=t level=ERROR msg="command failed" user_error=true error=` + quotedRefusal},
			want:   childFailure{message: refusal, mayBeUserError: true},
		},
		{
			// Killed after a warning: nothing on stderr names the failure,
			// so the caller falls back to how the process exited.
			name:   "killed after a text warning",
			stderr: []string{textStarted, textWarning},
			want:   childFailure{},
		},
		{
			name:   "killed after a JSON warning",
			stderr: []string{jsonStarted, jsonWarning},
			want:   childFailure{},
		},
		{
			name:   "killed with only the start record",
			stderr: []string{textStarted},
			want:   childFailure{},
		},
		{
			// A panic: the records, then the runtime's panic line and
			// trace. The panic line is the reason, the trace is not.
			name: "panic",
			stderr: []string{
				textStarted,
				"panic: runtime error: index out of range [3] with length 3",
				"",
				"goroutine 1 [running]:",
				"main.main()",
				"\t/src/main.go:12 +0x1d",
			},
			want: childFailure{message: "panic: runtime error: index out of range [3] with length 3"},
		},
		{
			// Before the command has a logger — config it cannot resolve —
			// it prints the bare error, and exits 2 when the error is the
			// user's. With no record at all, the bare line is the message
			// and the exit status is believed.
			name:   "bare error before the logger",
			stderr: []string{"  config.FromFlags: failed to resolve runtime configuration "},
			want:   childFailure{message: "config.FromFlags: failed to resolve runtime configuration", mayBeUserError: true},
		},
		{
			name:   "nothing at all",
			stderr: nil,
			want:   childFailure{mayBeUserError: true},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := readChildFailure([]byte(strings.Join(testCase.stderr, "\n") + "\n"))
			if got != testCase.want {
				t.Fatalf("got %+v, want %+v", got, testCase.want)
			}
		})
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
