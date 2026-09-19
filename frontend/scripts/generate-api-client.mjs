#!/usr/bin/env node
/**
 * Generate `src/api/schema.ts` from the local API's OpenAPI document, and fail
 * when the committed copy has drifted from it.
 *
 * The document is produced on the fly, by the Go CLI that serves it:
 *
 *     cd backend && go run ./cmd/aconiq openapi --out <tmp>
 *
 * and is never stored. A checked-in spec is a third copy of the contract
 * — after `openapi.go` and `schema.ts` — and the only one nothing verifies, so
 * it goes stale quietly. Generating it costs nothing in CI: the frontend job
 * already sets Go up, because `.github/actions/wasm-kernel` builds the kernel
 * before anything else runs.
 *
 * Two modes:
 *
 *   node scripts/generate-api-client.mjs            write src/api/schema.ts
 *   node scripts/generate-api-client.mjs --check    fail if it would change
 *
 * The `--check` mode is the gate (`just fe-api-check`, pulled into `fe-ci`).
 * What it catches is an `openapi.go` edit that never reached the frontend: the
 * types would still compile, still typecheck and still pass every test, while
 * describing an API the server stopped serving. Nothing else in the tree can
 * see that, because both sides are hand-written today and agree only by eye.
 *
 * `src/api/schema.ts` is generated code and this script owns its bytes
 * outright. It is excluded from treefmt's prettier and from eslint for that
 * reason — one writer per file, and the diff check above is the review. It is
 * committed rather than gitignored, unlike `src/i18n/`, because `tsc` must
 * resolve it on a fresh clone with no Go toolchain present.
 *
 * `openapi-typescript` is pinned exactly (no caret) in package.json: its output
 * is committed and compared byte for byte, so a patch release that re-indents a
 * union would read here as an API change.
 */
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import openapiTS, { astToString } from "openapi-typescript";

const BACKEND_DIR = fileURLToPath(new URL("../../backend", import.meta.url));
const OUT_URL = new URL("../src/api/schema.ts", import.meta.url);
const OUT_DISPLAY = "frontend/src/api/schema.ts";

const check = process.argv.includes("--check");

const HEADER = `/**
 * Generated from the local API's OpenAPI document. Do not edit.
 *
 * Source of truth: backend/internal/api/httpv1/openapi.go
 * Regenerate:      bun run generate:api   (or: just fe-api-check to verify)
 *
 * \`src/api/client.ts\` re-exports the schemas below under the names the app
 * uses, and is where the hand-written parts of the contract live — the two
 * shapes this document cannot express are documented there.
 */

`;

const tmp = mkdtempSync(join(tmpdir(), "aconiq-openapi-"));

try {
  const specPath = join(tmp, "openapi.v1.json");

  // `--out` is resolved against the project path unless it is absolute, which
  // this is (resolvePath, backend/internal/app/cli/modelio_helpers.go). The
  // command builds the document in memory and writes it; it needs no project.
  const exported = spawnSync(
    "go",
    ["run", "./cmd/aconiq", "openapi", "--out", specPath],
    { cwd: BACKEND_DIR, encoding: "utf8" },
  );

  if (exported.error) {
    fail(`could not run the Go CLI: ${exported.error.message}`);
  }
  if (exported.status !== 0) {
    process.stderr.write(`${exported.stdout ?? ""}${exported.stderr ?? ""}`);
    fail(`\`go run ./cmd/aconiq openapi\` exited ${String(exported.status)}.`);
  }

  const spec = JSON.parse(readFileSync(specPath, "utf8"));

  // The generator is happy to emit a valid, empty module from a document with
  // no paths in it, and the committed file would then shrink to nothing while
  // every downstream type quietly became `never`. This is the compile-i18n
  // lesson: a generator that exits 0 on a broken input is worse than one that
  // crashes. Count what arrived before believing it.
  const paths = Object.keys(spec.paths ?? {}).length;
  const schemas = Object.keys(spec.components?.schemas ?? {}).length;
  if (paths === 0 || schemas === 0) {
    fail(
      `the exported document declares ${String(paths)} paths and ` +
        `${String(schemas)} schemas. Something built an empty spec; refusing ` +
        "to write a client from it.",
    );
  }

  // `defaultNonNullable` defaults to true, which makes every property carrying
  // a `default` required. That reading suits a response — the server fills the
  // value in, so it is always there — and inverts the meaning in a request
  // body, where a documented default is precisely what lets a caller leave the
  // field out. `CreateRunRequest.experimental` is the case in this document:
  // the Go struct is `json:"experimental,omitempty"` and the app deliberately
  // omits it rather than sending `false`. It is the only property the option
  // touches here; the whole rest of the file is byte-identical either way.
  const generated =
    HEADER + astToString(await openapiTS(spec, { defaultNonNullable: false }));

  if (!check) {
    writeFileSync(OUT_URL, generated);
    process.stdout.write(
      `${OUT_DISPLAY}: ${String(paths)} paths, ${String(schemas)} schemas\n`,
    );
    process.exit(0);
  }

  let committed;
  try {
    committed = readFileSync(OUT_URL, "utf8");
  } catch {
    fail(
      `${OUT_DISPLAY} does not exist.\nRun \`bun run generate:api\` in frontend/.`,
    );
  }

  if (committed !== generated) {
    // The generated text is small enough to diff usefully, and `diff` says
    // which lines moved far better than a byte count does.
    const expected = join(tmp, "expected.ts");
    writeFileSync(expected, generated);
    const diff = spawnSync(
      "diff",
      [
        "-u",
        "--label",
        `${OUT_DISPLAY} (committed)`,
        "--label",
        `${OUT_DISPLAY} (generated)`,
        fileURLToPath(OUT_URL),
        expected,
      ],
      { encoding: "utf8" },
    );
    process.stderr.write(diff.stdout ?? "");
    fail(
      `${OUT_DISPLAY} does not match the API's OpenAPI document.\n` +
        "The contract changed in backend/internal/api/httpv1/openapi.go and the\n" +
        "client was not regenerated. Run `bun run generate:api` in frontend/.",
    );
  }

  process.stdout.write(`${OUT_DISPLAY}: up to date\n`);
} finally {
  rmSync(tmp, { recursive: true, force: true });
}

function fail(reason) {
  process.stderr.write(`\ngenerate-api-client: ${reason}\n`);
  process.exit(1);
}
