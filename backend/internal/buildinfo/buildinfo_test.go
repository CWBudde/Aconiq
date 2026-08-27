package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// The accessors must never return an empty string, whether or not the binary
// was linked with `-ldflags -X`. `go test` never stamps them, so this exercises
// the embedded-build-info and fallback paths.
func TestAccessorsAreNeverEmpty(t *testing.T) {
	t.Parallel()

	for name, got := range map[string]string{
		"version": Version(),
		"commit":  Commit(),
		"date":    Date(),
	} {
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s is empty", name)
		}
	}
}

func TestStringMentionsNameAndVersion(t *testing.T) {
	t.Parallel()

	got := String()
	if !strings.HasPrefix(got, Name+" "+Version()) {
		t.Fatalf("String() = %q, want prefix %q", got, Name+" "+Version())
	}

	if !strings.Contains(got, Commit()) || !strings.Contains(got, Date()) {
		t.Fatalf("String() = %q, want it to carry commit %q and date %q", got, Commit(), Date())
	}
}

func TestResolveFallsBackToDevWithoutStamps(t *testing.T) {
	t.Parallel()

	got := resolved{}
	fillFromBuildInfo(&got, &debug.BuildInfo{})

	if got.version != "" {
		t.Fatalf("version = %q, want empty before the defaults are applied", got.version)
	}
}

func TestFillFromBuildInfoUsesVCSSettings(t *testing.T) {
	t.Parallel()

	got := resolved{}
	fillFromBuildInfo(&got, &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.time", Value: "2026-01-02T03:04:05Z"},
		},
	})

	if got.version != "v1.2.3" {
		t.Fatalf("version = %q, want v1.2.3", got.version)
	}

	if got.commit != "0123456789ab" {
		t.Fatalf("commit = %q, want the 12-digit short revision", got.commit)
	}

	if got.date != "2026-01-02T03:04:05Z" {
		t.Fatalf("date = %q, want the normalized RFC 3339 timestamp", got.date)
	}
}

func TestFillFromBuildInfoMarksDirtyTrees(t *testing.T) {
	t.Parallel()

	got := resolved{}
	fillFromBuildInfo(&got, &debug.BuildInfo{
		Main:     debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}},
	})

	if got.version != "v1.2.3-dirty" {
		t.Fatalf("version = %q, want v1.2.3-dirty", got.version)
	}
}

func TestFillFromBuildInfoKeepsLinkerValues(t *testing.T) {
	t.Parallel()

	got := resolved{version: "v9.9.9", commit: "deadbeefcafe", date: "2026-08-28T00:00:00Z"}
	fillFromBuildInfo(&got, &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
			{Key: "vcs.time", Value: "2026-01-02T03:04:05Z"},
		},
	})

	if got.version != "v9.9.9" || got.commit != "deadbeefcafe" || got.date != "2026-08-28T00:00:00Z" {
		t.Fatalf("linker-stamped values were overwritten: %+v", got)
	}
}

func TestNormalizeDateConvertsToUTC(t *testing.T) {
	t.Parallel()

	got := normalizeDate("2026-01-02T04:04:05+01:00")

	want := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339)
	if got != want {
		t.Fatalf("normalizeDate = %q, want %q", got, want)
	}
}

func TestNormalizeDateKeepsUnparsableInput(t *testing.T) {
	t.Parallel()

	if got := normalizeDate(" not-a-date "); got != "not-a-date" {
		t.Fatalf("normalizeDate = %q, want the trimmed input back", got)
	}
}
