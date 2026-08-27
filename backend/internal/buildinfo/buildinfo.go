// Package buildinfo exposes the release identity of the running binary:
// version, source commit and build date.
//
// The values are stamped in at link time by `just build` and by the CI build
// step (see the -ldflags block in the justfile). When they are absent — a plain
// `go build ./...`, `go install`, or `go test` — they are recovered from the
// module build information that the Go toolchain embeds in every binary, so the
// provenance written by a run still names something better than "dev".
package buildinfo

import (
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Stamped at link time via `-ldflags -X`. Do not read these directly: they are
// empty for a plain `go build`. Use Version, Commit and Date instead.
var (
	version string
	commit  string
	date    string
)

const (
	// unknownValue is what the accessors report when neither the linker nor the
	// embedded build info supplies a value.
	unknownValue = "unknown"

	// devVersion is the fallback version for a binary built from a working tree
	// that carries no version stamp at all.
	devVersion = "dev"

	// shortCommitLen is the number of hex digits kept from a VCS revision.
	shortCommitLen = 12
)

// Name is the tool name recorded alongside the version in provenance files.
const Name = "aconiq"

type resolved struct {
	version string
	commit  string
	date    string
}

//nolint:gochecknoglobals // Resolving the embedded build info once is the point.
var current = sync.OnceValue(resolve)

// Version returns the release version, for example "v0.4.1" or
// "v0.4.1-3-gabc1234". It never returns an empty string.
func Version() string { return current().version }

// Commit returns the source revision the binary was built from, shortened to
// twelve hex digits, or "unknown".
func Commit() string { return current().commit }

// Date returns the build date in RFC 3339 form, or "unknown".
func Date() string { return current().date }

// String renders all three fields on one line, for `aconiq --version`.
func String() string {
	return Name + " " + Version() + " (commit " + Commit() + ", built " + Date() + ")"
}

func resolve() resolved {
	out := resolved{
		version: strings.TrimSpace(version),
		commit:  strings.TrimSpace(commit),
		date:    strings.TrimSpace(date),
	}

	embedded, ok := debug.ReadBuildInfo()
	if ok {
		fillFromBuildInfo(&out, embedded)
	}

	if out.version == "" {
		out.version = devVersion
	}

	if out.commit == "" {
		out.commit = unknownValue
	}

	if out.date == "" {
		out.date = unknownValue
	}

	return out
}

// fillFromBuildInfo completes the fields the linker did not stamp from the
// build information the toolchain embeds. Linker values always win.
func fillFromBuildInfo(out *resolved, embedded *debug.BuildInfo) {
	if out.version == "" && embedded.Main.Version != "" && embedded.Main.Version != "(devel)" {
		out.version = embedded.Main.Version
	}

	for _, setting := range embedded.Settings {
		switch setting.Key {
		case "vcs.revision":
			if out.commit == "" {
				out.commit = shortenCommit(setting.Value)
			}
		case "vcs.time":
			if out.date == "" {
				out.date = normalizeDate(setting.Value)
			}
		case "vcs.modified":
			if setting.Value == "true" {
				out.version = markDirty(out.version)
			}
		}
	}
}

func markDirty(version string) string {
	if version == "" {
		return devVersion + "-dirty"
	}

	if strings.HasSuffix(version, "-dirty") {
		return version
	}

	return version + "-dirty"
}

func shortenCommit(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) > shortCommitLen {
		return revision[:shortCommitLen]
	}

	return revision
}

func normalizeDate(value string) string {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}

	return parsed.UTC().Format(time.RFC3339)
}
