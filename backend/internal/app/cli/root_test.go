package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/buildinfo"
)

// `aconiq --version` must report the same identity that lands in
// provenance.json, plus the commit and build date. The values are stamped at
// link time, so this asserts they are reported, not what they are.
func TestRootCommandVersionFlagReportsBuildInfo(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("aconiq --version: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got == "" {
		t.Fatal("aconiq --version printed nothing")
	}

	for _, want := range []string{buildinfo.Name, buildinfo.Version(), buildinfo.Commit(), buildinfo.Date()} {
		if !strings.Contains(got, want) {
			t.Fatalf("aconiq --version = %q, want it to contain %q", got, want)
		}
	}

	if strings.Contains(got, "\n") {
		t.Fatalf("aconiq --version should print a single line, got %q", got)
	}
}
