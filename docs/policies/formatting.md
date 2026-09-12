# Formatting Policy

Status date: 2026-09-12

## Go Source Formatting

- `gofumpt` (strict superset of `gofmt`) is mandatory for all `.go` files.
- `gci` organizes imports.
- No merge is acceptable with unformatted Go code.

## The pinned toolchain

treefmt runs whatever formatter it finds on `PATH`, and `--allow-missing-formatter` skips one that
is absent without a word. Both directions failed silently: prettier 3.9.6 rewrites four frontend
files that the 3.8.0 CI pins considers correctly formatted, so running the project's own documented
formatting command produced a diff CI rejected, while `just check-formatted` reported a backlog that
did not exist.

- `tools.versions` at the repository root is the single source of truth for every tool version. CI
  reads it (through `.github/actions/toolchain`) and so does `just install-tools`; there is no
  second place to bump.
- `just install-tools` installs exactly those versions, into the Go bin directory by default
  (override with `TOOLS_BIN`). `just` and `bun` bootstrap themselves and are only verified.
- `just check-tools` reports what is installed against the pins. `just fmt` and
  `just check-formatted` run the formatter subset of it first and refuse to proceed on a mismatch,
  and treefmt no longer runs with `--allow-missing-formatter`, so a missing formatter is an error
  rather than a silent skip.

## Enforcement

- CI executes `just check-formatted`, which is **read-only**: it mirrors the files a commit could
  contain (tracked plus untracked-and-not-ignored) into a temporary directory, formats the copy and
  diffs it against the tree. `treefmt --fail-on-change` formats in place and then reports, so using
  it as the check rewrote CI's own checkout and silently reformatted unrelated local work.
- Linting: `just lint` runs golangci-lint v2 with defaults plus a tuned disable list and
  path/text-scoped exclusions (see `.golangci.yml` and `docs/lint-triage.md`) — not "all linters
  enabled".

## Local Workflow

- Once, and after any bump to `tools.versions`:
  - `just install-tools` — installs the pinned toolchain
  - `just check-tools` — reports what is installed against the pins
- Before committing:
  - `just fmt` — formats Go, shell, markdown, YAML, JSON
  - `just test`
  - `just lint`
- Or run all checks at once: `just ci`
