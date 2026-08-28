# Backend

The Go half of Aconiq: the CLI (`cmd/aconiq`), the local HTTP API, the compute
engine, the standards modules and the reporting/export stack. It is the primary
build artifact.

Module path: `github.com/aconiq/backend`.

## Building and testing

Everything is driven by [`just`](https://github.com/casey/just) from the
**repository root**, not from this directory:

```sh
just build        # -> bin/aconiq
just test         # all Go tests
just lint         # golangci-lint v2
just go-ci        # the full Go gate, mirroring .github/workflows/go-ci.yml
```

Running a single test still wants this directory:

```sh
cd backend && go test ./internal/geo/... -run TestFunctionName
```

`just` on its own lists every recipe.

## Where to read next

This file deliberately carries no package list, no command list and no status.
Those drifted here once already and now live in exactly one place each:

| For                                              | Read                                    |
| ------------------------------------------------ | --------------------------------------- |
| Package layout, CLI surface, API routes, policies | `AGENTS.md` at the repository root      |
| What is done, in progress and open                | `PLAN.md` — the single status source    |
| What each standards module may be claimed to do   | `docs/conformance/`                     |
| Project format, schemas, result containers        | `docs/`                                 |

Per-phase delivery notes from earlier milestones are kept as
`docs/phase*-baseline.md`. They are historical: where they disagree with a
document in `docs/conformance/`, that document wins.
