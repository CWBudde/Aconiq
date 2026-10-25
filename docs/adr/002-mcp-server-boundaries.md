# ADR-002: MCP Server Boundaries, Security Policy and Exposed Fields

Status: Accepted
Date: 2026-10-16

## Context

Issue [#111](https://github.com/CWBudde/Aconiq/issues/111) asks for a Model Context Protocol (MCP)
server so that an AI agent can discover a project, edit its scenarios, sources and receivers with
validation, start calculations, follow their progress and retrieve results. The issue requires the
server to reuse the existing backend rather than duplicate business logic, to separate read-only
from state-changing tools, to require explicit confirmation for destructive operations, and never
to change an engineering assumption silently.

This ADR closes `PLAN.md` Subphase SL-1.1: it fixes where the server sits, which tools and resources
it offers, the security policy those tools run under, and which standards, units, CRS and
provenance fields reach the agent. It is a specification; no code exists yet. Paths below are
relative to `backend/internal/`.

What the backend offers today, and what it does not, shaped every decision here:

- The local HTTP API (`api/httpv1`) already owns request validation, body caps, the
  `--experimental` gate, run spawning and supervision, cancellation and the JSON error envelope.
  `httpv1.NewServeHandler` (`api/httpv1/handler.go`) builds the handler `aconiq serve` mounts: it
  carries the standards registry and the run supervisor, and its `StopRuns` interrupts and settles
  the runs it started. The plain `NewHandler` has neither — its `/api/v1/standards` answers 503
  `standards_unavailable` — so it is not the one to build on.
- Model editing is **whole-model replace only**: `POST /api/v1/model` (`handleModelSave`,
  `api/httpv1/model.go`) runs `modelgeojson.NormalizeWithCRS` and `modelgeojson.Validate`, then
  `projectfs.Store.SaveModel`. Nothing adds or changes a single feature, and the endpoint takes no
  expected model hash.
- **Scenarios cannot be listed, created or changed** by the CLI or the API. `projectfs.Store.Init`
  seeds one `default` scenario and nothing else appends to `Project.Scenarios`.
- `provenance.json` (`project.ProvenanceManifest`) is not an artifact, so the API does not serve it.
  The run summary carries `project_crs` and `compute_crs` for exactly that reason (`addComputeCRS`
  in `app/cli/run_persist.go`).
- There is no audit log of state-changing operations, and no MCP, JSON-RPC or JSON-Schema library in
  `go.mod`.

## Decisions

### Placement: an in-process `aconiq mcp` command over stdio

- A new CLI command, `aconiq mcp --project <root>`, speaks MCP over **stdio**. HTTP transport is
  out of scope for now; #111 asks to evaluate it later.
- **stdout carries JSON-RPC and nothing else; logs go to stderr.** This is the stream contract every
  command already keeps (AGENTS.md "Streams", `app/cli/stdout_contract_test.go`). Here it is
  load-bearing: one stray log line on stdout corrupts the protocol stream.
- **Each tool is a thin adapter over the `httpv1` handler, called in-process.** The adapter builds
  an `http.Request`, serves it through the handler `httpv1.NewServeHandler` returns — with the
  standards registry, so `list_standards` works and the scaffold gate is the API's own — and
  translates the response. If a serve-only option (CORS origins, listen address, token) gets in the
  way, a sibling constructor with the same registry and supervisor is added; `NewHandler` is not
  used. It sets the headers the handler's security middleware requires (`Host` on
  loopback, `X-Aconiq-Client` on writes) itself. That way:
  - request validation (`createRunRequest.validate`), body caps (`api/httpv1/security.go`), the
    scaffold opt-in (`experimentalOptInGate`), run supervision (`runSupervisor`), cancellation and
    the error envelope are reused rather than re-implemented;
  - a rule changed in `httpv1` changes for the API, the frontend and the agent at once.
- The MCP layer adds only what MCP needs and HTTP does not have: the write gate below, the
  `confirm` check, the audit log, and the mapping from the error envelope (`code`, `message`,
  `details`, `hint`) onto an MCP tool error. It holds no rule of its own about models, runs or
  standards.
- **Runs live as long as the `aconiq mcp` process.** Runs it started are its own to cancel, as with
  `aconiq serve` (409 `run_not_owned` otherwise). When stdin reaches EOF or the process receives
  SIGINT/SIGTERM, it calls `StopRuns` before exiting, so no child outlives it unsettled — the same
  shutdown order `serve` uses (`shutdownServer`, `app/cli/serve.go`). At startup it settles runs a
  previous process left open, with `projectfs.Store.FailInterruptedRuns`, exactly as `serve` does.

### Tool and resource boundaries

Tools fall into three classes. The class is part of the tool's contract: it is advertised through
the MCP tool annotations (`readOnlyHint`, `destructiveHint`) **and enforced by the server**,
because a client is free to ignore annotations.

**Read-only — always available:**

| Tool              | Backed by                                              |
| ----------------- | ------------------------------------------------------ |
| `project_status`  | `GET /api/v1/project/status`                           |
| `list_standards`  | `GET /api/v1/standards`                                |
| `get_model`       | `GET /api/v1/model` (optional `crs`)                   |
| `list_runs`       | `GET /api/v1/runs`                                     |
| `get_run`         | `GET /api/v1/runs/{id}` (includes `progress`)          |
| `get_run_log`     | `GET /api/v1/runs/{id}/log`                            |
| `get_run_results` | `GET /api/v1/artifacts/{id}/content` (receiver tables) |
| `get_contours`    | `GET /api/v1/runs/{id}/contours`                       |
| `estimate_run`    | `POST /api/v1/runs/estimate` (creates no run)          |
| `transform`       | `POST /api/v1/transform`                               |

**State-changing — only with `aconiq mcp --allow-writes`:**

| Tool         | Backed by                                                                                                                                                                      |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `start_run`  | `POST /api/v1/runs`; a scaffold-tier standard still needs `experimental: true`, and a normative run over proposed or assumed-height facade receivers `allow_unconfirmed: true` |
| `cancel_run` | `POST /api/v1/runs/{id}/cancel`                                                                                                                                                |

**Destructive — `--allow-writes` and a matching `confirm` argument:**

| Tool             | Backed by                     | `confirm` must equal                            |
| ---------------- | ----------------------------- | ----------------------------------------------- |
| `save_model`     | `POST /api/v1/model`          | the hash of the model being replaced, or `none` |
| `delete_run`     | `DELETE /api/v1/runs/{id}`    | the run id                                      |
| `import_terrain` | `POST /api/v1/import/terrain` | the current terrain's digest, or `none`         |

**Proposals from an image — `--allow-writes`, `--photo-transmission` and a matching `confirm`**
(P18 D2, amended):

| Tool              | Backed by                      | `confirm` must equal                                  |
| ----------------- | ------------------------------ | ----------------------------------------------------- |
| `propose_windows` | `POST /api/v1/facades/windows` | the hash of the model the windows were placed against |

- `propose_windows` adds windows an agent read off a facade image to the saved model, and only
  adds: an id the model holds is refused, so a window a person reviewed is never touched. It is
  annotated additive (`destructiveHint: false`), but takes `confirm` as `save_model` does, because
  each window is placed on the model the agent read. Its schema has no `sensitive` and no
  `review_status`: a proposal is written `proposed` and not sensitive, and confirming a window or
  marking its room _schutzbedürftig_ stays a person's act in the facade elevation. It refuses a
  window without `evidence` naming the image (`kind`, `source_id`, `content_sha256`).
- Sending a site photo to a hosted model is a disclosure of personal data, decided per engagement
  (`docs/policies/data-handling.md`). The tool is listed only when the server was started with
  `--photo-transmission <reference>` as well as `--allow-writes`: the person starting the server
  records the decision, never the agent. The reference is opaque, never a path; it is stamped into
  every proposal's evidence as `transmission_basis` and into every `propose_windows` audit line.
- Without `--allow-writes`, the state-changing and destructive tools are **not listed at all**, so an
  agent cannot plan around tools it may not call. A server started read-only says so in its
  instructions.
- `confirm` is a value the agent must have **read** first, not a boolean or a fixed word it can set
  blindly, and it binds the call to the state the agent saw. For `save_model` it is the hash
  `get_model` returned, so it also acts as an optimistic lock: a model that changed since the agent
  read it is refused, not overwritten; on a project with no model yet it is `none`, the API's
  `expected_hash` for that case. An empty `confirm` is refused by the MCP layer itself
  (`confirm_required`), because it is the one value that would make the save unconditional. For `import_terrain` it is the SHA-256 of the terrain being
  replaced, or `none` when the project has none, which `project_status` has to report; see
  Consequences for both.
- Feature-level edits (add or move one source, change one receiver height) are composed by the
  agent as `get_model` → change → `save_model`. Validation runs on every save, and the issues
  (`modelgeojson.ValidationIssue`: `code`, `feature_id`, `message`, split into errors and warnings)
  are returned verbatim. Nothing is coerced into validity.

**Resources** (read-only, addressable by URI):

- `aconiq://project` — project status and manifest summary;
- `aconiq://standards` — the standards catalogue;
- `aconiq://runs/{id}` — run summary and progress;
- `aconiq://runs/{id}/provenance` — `provenance.json`;
- `aconiq://artifacts/{id}` — artifact content by artifact id.

Large payloads — the raster `.bin`, full receiver tables beyond a size cap — are returned as
metadata plus a resource reference, never inline in a tool result.

**Excluded from the first surface:**

- `POST /api/v1/import/osm` and `POST /api/v1/import/lgln`: they reach the network (Overpass,
  LGLN), and Aconiq is offline-first. An agent that wants OSM or LoD2 data can be given an imported
  file. Revisit with an explicit `--allow-network` flag if the SuedLink pilot needs it.
- `GET /api/v1/openapi.json`: it describes the HTTP API, not the tools.
- `GET /api/v1/events`: an SSE stream does not map onto a tool call; an agent polls `get_run`, whose
  `progress` comes from the same `.noise/runs/<id>/progress.json`.

### Security policy

1. **stdio only, no listener.** The server opens no socket. Whoever can talk to it already started
   it.
2. **The project is fixed at startup.** `--project` names it, and no tool takes a project path, so an
   agent cannot switch to another project or point the server at an arbitrary directory.
3. **No arbitrary file reads.** Artifact content is reachable only by artifact id, through the
   manifest and `readProjectFile` (`api/httpv1/security.go`, `filepath.IsLocal` plus
   `os.OpenInRoot`). Run inputs remain relative paths, as `createRunRequest.validate` already
   requires.
4. **Writes are opt-in, destructive writes are confirmed** — the two-step gate above.
5. **The evidence-tier gate is unchanged.** A scaffold-tier standard is refused without
   `experimental: true`, by the same `experimentalOptInGate` the API uses, and the refusal reaches the
   agent as a tool error with the API's own message.
6. **No silent assumptions.** Tool inputs are validated, never corrected behind the agent's back,
   and anything the backend refuses is returned as an error with its `code` and `hint`. The HTTP
   path leaves an omitted field out of the `aconiq run` argv so the CLI applies its default
   (`runCommandArgs`, `api/httpv1/run_executor.go`); the MCP layer does not inherit that. Its
   `start_run` and `estimate_run` schemas **require** `scenario_id`, `standard_id`,
   `standard_version`, `standard_profile` and `receiver_mode`, so the choices that select a method
   are always the agent's own. Run parameters may be omitted: their defaults are declared in the
   descriptor, which `list_standards` returns, and the resolved values — defaults included, as
   `NormalizeAndValidate` fills them — are recorded in `provenance.json`'s `parameters`, which the
   agent can read back. The model is always the project's saved model, the one `get_model` shows.
7. **Audit log.** Every state-changing or destructive call is appended to
   `.noise/logs/mcp-audit.jsonl` as one JSON object: UTC timestamp, tool name, a SHA-256 digest of
   the canonicalised arguments, the outcome (`ok` or the error `code`), and the affected run id or
   model hash; a `propose_windows` line also names the `photo_transmission` decision. Argument values are not logged, only their digest: model payloads can be large and
   can carry customer geometry.
   The log is opened before the call is served: if it cannot be (a read-only disk, a permission),
   the call is refused with `audit_unavailable` and nothing is written, so no write happens
   unaudited. A call whose arguments the input schema refuses is audited as `invalid_arguments`:
   the tool validates them itself rather than letting the SDK refuse them before the audit log
   sees the call.

### Deterministic outputs

Tool results are produced by the existing JSON encoders, with sorted map keys and no wall-clock
fields beyond those the project already records (`generated_at`, `created_at`, run progress
timestamps). Two identical calls against an unchanged project return byte-identical results, which
is what makes the integration tests in SL-1.4 golden-testable.

**`estimate_run` is the exception, by design.** It times a probe of the run's own kernel, so its
answer carries measured values — the probe's `elapsed_s` and `prepare_s` (`runestimate.ProbeReport`)
and the wall and CPU bands derived from them — that differ between two calls on an unchanged
project. Its tests assert the stable fields (counts, receivers probed, budget) and the shape of the
timing fields, not their values.

### Exposed fields

The agent sees the same fields a human reads in a report, from the same source, and never a
re-derived or summarised version of them.

**Standards** — exactly the `standards/descriptorjson` encoding that `GET /api/v1/standards` and the
WASM kernel's `standards` export already share. One encoding, so the three cannot drift:

- `context`, `id`, `description`, **`evidence_tier`**, `default_version`;
- per version: `name`, `default_profile`, `profiles`;
- per profile: `name`, `supported_source_types`, `supported_indicators`, `parameters`;
- per parameter: `name`, `kind` (`string`|`bool`|`int`|`float`), `unit`, `required`,
  `default_value`, `description`, `enum`, `min`, `max`.

The evidence tier is never dropped, shortened or moved out of the standard's own object: an agent
deciding between `rls19-road` and `cnossos-road` must see that one is `normative` and the other a
`scaffold`.

**Units** — taken from where the backend already declares them, never inferred:

- parameter units from the descriptor (`unit`);
- result units from the receiver table (`indicator_order`, `units`, `results/receiver_table.go`)
  and the raster sidecar (`units`, `band_names`, `results/raster.go`);
- heights are always metres, under the `height_m` property the GeoJSON schema defines;
- coordinate units follow from the CRS, which is always stated beside them.

**CRS:**

- the project's CRS from the manifest, returned by `project_status` as `crs`;
- `get_model` returns the model in the project CRS unless `crs` is given, and always names the CRS
  it answered in (`crs`, `project_crs`), as `GET /api/v1/model` does;
- per run, `project_crs` and `compute_crs` from the run summary and provenance metadata. Results are
  expressed in `compute_crs`;
- `transform` with `target_crs: "auto"` is the agent's only projection, so it resolves the same UTM
  zone `aconiq run` would and refuses a geographic project in the same words
  (`crstransform.GeographicRefusal`).

**Provenance** — the whole `provenance.json` (`project.ProvenanceManifest`), as recorded when the run
was made and never re-resolved, plus the run summary:

- `run_id`, `scenario_id`, `standard` (`context`, `id`, `version`, `profile`), `receiver_mode`,
  `receiver_set_id`;
- `parameters` and `metadata` (including `evidence_tier`, `project_crs`, `compute_crs`);
- `input_hashes` (input path to SHA-256);
- `standard_data` (`algorithm`, `digest`, `evidence_tier`, per-table digests);
- `generated_at`, `tool_name`, `tool_version`, `tool_commit`, `tool_build_date`;
- from the run summary: `status`, `output_hash`, `source_count`, `receiver_count`, `model_version`,
  and for grid runs `grid_width`, `grid_height`, `grid_masked_cells`.

### MCP library: `github.com/modelcontextprotocol/go-sdk`

Decided in SL-1.2 (2026-10-16): the official SDK, from v1.8.0. The candidates were it,
`github.com/mark3labs/mcp-go`, and a hand-rolled JSON-RPC 2.0 loop over stdio, held to these
criteria:

- passes `just license-check` (no restricted, forbidden or unknown licences);
- supports stdio, tool annotations and resources;
- can derive tool input schemas from Go types, so the schema and the decoder cannot disagree;
- adds the fewest transitive dependencies for that.

How it measures against them, checked against v1.8.0 in a throwaway module with the pinned
`go-licenses` and `govulncheck`:

- **Licence.** Its `LICENSE` describes a transition from MIT to Apache-2.0, not a standard licence
  text. `go-licenses` classifies it as Apache-2.0, and `just license-check`'s own invocation
  (`--disallowed_types=restricted,forbidden,unknown`) passes over the whole import graph. The licence is still worth re-reading on upgrades: a
  classifier that stops recognising the transition text reports `unknown` and fails the gate.
- **Dependencies.** A stdio server pulls in `google/jsonschema-go` (MIT),
  `segmentio/encoding` and `segmentio/asm` (MIT), `yosida95/uritemplate` (BSD-3-Clause) and
  `golang.org/x/{oauth2,sync,sys,time}` (BSD-3-Clause). They go into `NOTICE` with the code that
  imports them.
- **Go version.** It requires Go 1.25, which the module already declares.
- **Vulnerabilities.** `govulncheck` reports none that a stdio server calls.
- **Surface.** `mcp.StdioTransport`, `mcp.ToolAnnotations` (for the read-only and destructive
  hints the tool classes above map onto), `Server.AddResource` and `Server.AddResourceTemplate`
  (for `aconiq://runs/{id}`).
- **Schemas.** `mcp.AddTool[In, Out]` derives a tool's input and output schemas from its Go types
  through `jsonschema-go`.

## Consequences

The implementation phases inherit these gaps, which the boundaries above make necessary:

- **Scenarios** (SL-1.2): listing, creating and updating a scenario needs a `projectfs` path and an
  API route first; the MCP tools then wrap the route like every other.
- **Optimistic lock on model save** (SL-1.2): `POST /api/v1/model` needs an expected-hash field,
  and the compare has to be atomic with the write. Today `projectfs.Store.SaveModel` writes the
  normalized model, the dump and the validation report **before** it takes `LockManifest`, so a
  check under the lock alone would still let a refused or racing save overwrite the model files.
  The store has to own the compare-and-write: stage the files, then compare the expected hash and
  move them into place under the same lock. A check in the MCP layer alone would also race a
  concurrent `aconiq serve`.
- **Terrain digest** (SL-1.2): `import_terrain`'s `confirm` needs the current terrain's SHA-256 (or
  `none`) on `project_status`; nothing reports it today. The replace has the same compare-and-write
  requirement as the model save.
- **Provenance over the API** (SL-1.2): `aconiq://runs/{id}/provenance` needs a route that serves
  `provenance.json`; today nothing does.
- **Build identity in provenance** (SL-1.2): provenance records `tool_version` but not the commit or
  build date, although `buildinfo.Commit()` and `buildinfo.Date()` exist. #111 asks for the software
  version in the audit trail; the version alone does not identify a development build.
- **Dry-run validation** (SL-1.3): a `validate_model` tool that checks without saving depends on the
  open decision about `POST /api/v1/validate` (`PLAN.md`, Open decisions → Behaviour and API). Until
  it is decided, validation happens on `save_model`.
- **Audit log**: written since SL-1.2 for `save_model`, the first write tool, so no build offers an
  unaudited write (`internal/mcpserver/audit.go`). SL-1.4 extends it to the run tools as they land.
- **Network imports** stay outside the first surface; adding them is a new decision, not a flag
  flip.
