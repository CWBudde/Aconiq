#!/usr/bin/env node
/**
 * Compile the Paraglide message catalogue, and fail when it comes out empty.
 *
 * `paraglide-js compile` treats a plugin it could not import as a *warning*: it
 * logs a `PluginImportError`, writes a catalogue with no messages in it, and
 * exits 0. Everything downstream then reads as catastrophe with nothing
 * pointing at the cause — every `m.<key>` fails to resolve, so `eslint .`
 * reports thousands of findings and the suite hundreds of failures, all of
 * them the same unresolved import wearing different hats.
 *
 * Two changes keep that from happening, and this script is the second.
 *
 * The first is `project.inlang/settings.json`, whose `modules` used to be two
 * `cdn.jsdelivr.net` URLs. The plugins are ordinary npm packages, so they are
 * devDependencies now and the settings point at the copies on disk; the paths
 * resolve against the project directory's *parent*, which is why they read
 * `./node_modules/...` and not `../node_modules/...`. `bun.lock` pins their
 * bytes, which a URL never did, and neither CI nor an offline checkout has to
 * reach the public internet to know what a label says.
 *
 * The second is the check below. Vendoring removes today's cause; it does not
 * make the compiler fail on tomorrow's, and a compile that silently produces
 * nothing is the failure mode worth spending a script on. So: run the
 * compiler, refuse a `PluginImportError` by name, and then count what was
 * actually written against `messages/<baseLocale>.json`. The second check is
 * the load-bearing one — it does not care why the catalogue is short.
 *
 * The three compiler options have to match `paraglideVitePlugin` in
 * `vite.config.ts`, or `tsc` and Vite would generate different runtimes.
 *
 * Usage: node scripts/compile-i18n.mjs
 * Or via justfile: just fe-i18n
 */
import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const FRONTEND_DIR = fileURLToPath(new URL("..", import.meta.url));

// Keep in sync with `paraglideVitePlugin` in vite.config.ts.
const PROJECT = "./project.inlang";
const OUTDIR = "./src/i18n";
const STRATEGY = ["localStorage", "preferredLanguage", "baseLocale"];

const settings = JSON.parse(
  readFileSync(new URL("../project.inlang/settings.json", import.meta.url)),
);

// The compiler from node_modules, not `bunx`: this must not reach a registry
// either, and the version that runs has to be the one bun.lock pins. It is the
// package's own entry module rather than the `.bin/` shim, run by whatever
// runtime started this script: on Windows a package manager writes
// `paraglide-js.cmd` and `.ps1` there and often no bare `paraglide-js` at all,
// so spawning that path is a file-not-found before the compiler ever runs.
const COMPILER = fileURLToPath(
  new URL("../node_modules/@inlang/paraglide-js/bin/run.js", import.meta.url),
);

if (!existsSync(COMPILER)) {
  fail(
    `the compiler is not installed at ${COMPILER}.\nRun \`bun install\` in frontend/ first.`,
  );
}

const compile = spawnSync(
  process.execPath,
  [
    COMPILER,
    "compile",
    "--project",
    PROJECT,
    "--outdir",
    OUTDIR,
    "--strategy",
    ...STRATEGY,
  ],
  { cwd: FRONTEND_DIR, encoding: "utf8" },
);

const output = `${compile.stdout ?? ""}${compile.stderr ?? ""}`;
process.stdout.write(output);

if (compile.error) {
  fail(`the compiler could not be started: ${compile.error.message}`);
}
if (compile.status !== 0) {
  fail(`the compiler exited ${String(compile.status)}.`);
}

// Named explicitly, because this is the one failure the compiler declines to
// have an opinion about. The module list is echoed so the message names the
// thing to fix rather than the thing that broke.
if (output.includes("PluginImportError")) {
  fail(
    "a plugin in project.inlang/settings.json could not be imported, so the " +
      "catalogue was written empty.\n" +
      `Modules: ${settings.modules.join(", ")}`,
  );
}

// The check that does not depend on how the compiler phrases things. Paraglide
// writes one module per message under <outdir>/messages/, plus `_index.js`.
const written = readdirSync(
  new URL("../src/i18n/messages", import.meta.url),
).filter((entry) => entry.endsWith(".js") && entry !== "_index.js").length;

// `pathPattern` resolves against the project directory's parent, the same base
// the module paths above use — the files live in frontend/messages/, not in
// frontend/project.inlang/messages/.
const basePath = settings["plugin.inlang.messageFormat"].pathPattern
  .replace("{locale}", settings.baseLocale)
  .replace(/^\.\//, "");

const declared = Object.keys(
  JSON.parse(readFileSync(new URL(`../${basePath}`, import.meta.url))),
).filter((key) => !key.startsWith("$")).length;

if (written < declared) {
  fail(
    `the catalogue holds ${String(written)} of the ${String(declared)} messages ` +
      `declared for the base locale "${settings.baseLocale}".\n` +
      "A short catalogue means the message-format plugin did not read the " +
      "files, and every m.<key> downstream will fail to resolve.",
  );
}

function fail(reason) {
  process.stderr.write(`\ncompile-i18n: ${reason}\n`);
  process.exit(1);
}
