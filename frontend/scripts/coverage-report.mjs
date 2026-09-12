#!/usr/bin/env bun
/**
 * Render the coverage summary as Markdown for the CI job summary and the sticky
 * pull-request comment.
 *
 * Usage: bun run scripts/coverage-report.mjs [--out <file>] [--unavailable <why>]
 *
 * Input is `coverage/coverage-summary.json`, written by the `json-summary`
 * reporter configured in vitest.config.ts. Output shape matches
 * scripts/coverage-report.sh's, so a reader who has seen the backend comment can
 * read this one.
 *
 * ## It renders, it never enforces
 *
 * The floors are `test.coverage.thresholds` in vitest.config.ts and the test
 * runner applies them inside its own process — by the time this script runs, the
 * verdict has already been reached. This file reads those same thresholds
 * *from the config module* rather than taking them as arguments, because a
 * comment quoting a floor that is not the floor being enforced is worse than a
 * comment quoting none: it is wrong in a way nobody would think to check.
 *
 * ## --unavailable
 *
 * A sticky comment outlives the run that wrote it. A run whose coverage step
 * produced nothing must still post something, or the previous commit's numbers
 * stay on the pull request looking current. `--unavailable <why>` renders that
 * notice in place of a report.
 *
 * Run via the justfile: `just fe-coverage-report`.
 */
import { readFileSync, writeFileSync } from "fs";
import { fileURLToPath, URL } from "url";

import config from "../vitest.config.ts";

const METRICS = ["lines", "statements", "functions", "branches"];
const HEADING = "## Frontend test coverage";
const POLICY_URL =
  "https://github.com/CWBudde/Aconiq/blob/main/docs/testing/coverage.md";

// A summary this small is not a low number, it is a broken run: an empty or
// truncated coverage-summary.json reads as 0% and would post a plausible zero
// and invite someone to ratchet the floor down to match it.
const MIN_STATEMENTS = 500;

function parseArgs(argv) {
  const args = { out: "coverage-report.md", unavailable: null };
  for (let i = 0; i < argv.length; i += 1) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (flag === "--out" || flag === "--unavailable") {
      if (value === undefined) throw new Error(`${flag} needs a value`);
      args[flag === "--out" ? "out" : "unavailable"] = value;
      i += 1;
    } else if (flag === "--summary") {
      if (value === undefined) throw new Error("--summary needs a value");
      args.summary = value;
      i += 1;
    } else {
      throw new Error(`unknown argument: ${flag}`);
    }
  }
  return args;
}

/** The floors the run was actually measured against, from the one place they live. */
function thresholds() {
  const configured = config?.test?.coverage?.thresholds;
  if (!configured) return null;
  const gate = {};
  for (const metric of METRICS) {
    if (typeof configured[metric] === "number")
      gate[metric] = configured[metric];
  }
  return Object.keys(gate).length > 0 ? gate : null;
}

function pct(n) {
  return `${Number(n).toFixed(1)}%`;
}

/**
 * Group per-file entries by their top-level directory under src/.
 *
 * Paths in the summary are absolute, so they are made relative to src/ before
 * grouping — an absolute path would leak the runner's checkout directory into a
 * public pull-request comment, and would not group at all.
 */
function byArea(summary, srcDir) {
  const areas = new Map();
  for (const [file, entry] of Object.entries(summary)) {
    if (file === "total") continue;
    if (!file.startsWith(srcDir)) continue;
    const rel = file.slice(srcDir.length).replace(/^\//, "");
    const slash = rel.indexOf("/");
    const area = slash === -1 ? "(root)" : rel.slice(0, slash);
    const acc = areas.get(area) ?? { covered: 0, total: 0 };
    acc.covered += entry.statements?.covered ?? 0;
    acc.total += entry.statements?.total ?? 0;
    areas.set(area, acc);
  }
  return [...areas.entries()]
    .map(([area, { covered, total }]) => ({
      area,
      total,
      pct: total > 0 ? (covered / total) * 100 : 0,
    }))
    .sort((a, b) => b.total - a.total);
}

function renderUnavailable(why) {
  return [
    HEADING,
    "",
    `**Not reported for this commit.** ${why}`,
    "",
    "A partial coverage run is not a coverage number: publishing it would put an",
    "arbitrary slice of the denominator on the pull request. Fix the run and this",
    "job reports again.",
    "",
  ].join("\n");
}

function render(summary, gate) {
  const total = summary.total;
  const statements = total?.statements?.total ?? 0;
  if (statements < MIN_STATEMENTS) {
    throw new Error(
      `only ${statements} statements in the summary (expected at least ${MIN_STATEMENTS}) — ` +
        "the run is truncated or measured almost nothing.",
    );
  }

  const lines = [
    HEADING,
    "",
    `### ${pct(total.statements.pct)} of statements`,
    "",
    gate
      ? "| Metric | Coverage | Covered | Floor |"
      : "| Metric | Coverage | Covered |",
    gate ? "|---|---|---|---|" : "|---|---|---|",
  ];
  for (const metric of METRICS) {
    const m = total[metric];
    if (!m) continue;
    const name = metric[0].toUpperCase() + metric.slice(1);
    const under = gate && m.pct < gate[metric];
    const cell = under ? `${pct(m.pct)} ⚠️` : pct(m.pct);
    const counts = `${m.covered} / ${m.total}`;
    lines.push(
      gate
        ? `| ${name} | ${cell} | ${counts} | ${gate[metric]}% |`
        : `| ${name} | ${cell} | ${counts} |`,
    );
  }

  lines.push(
    "",
    "Every module under `src/` is in the denominator, not only the ones a test",
    "imports: an untested file reads 0% rather than dropping out. Generated",
    "Paraglide output and the Vite entry point are excluded — see",
    "`vitest.config.ts` for the list and why each one is on it.",
    "",
  );

  if (gate) {
    lines.push(
      "The floors are `test.coverage.thresholds` in `vitest.config.ts`, applied by",
      "the test run itself. This job is **advisory**: it is not a required status",
      "check, so a breach is visible without blocking a merge.",
      "",
    );
  }

  lines.push(
    `See [docs/testing/coverage.md](${POLICY_URL}) for the policy and the ratchet ledger.`,
    "",
  );

  const srcDir = fileURLToPath(new URL("../src", import.meta.url));
  const areas = byArea(summary, srcDir);
  if (areas.length > 0) {
    lines.push(
      "<details>",
      "<summary>Per-area breakdown (statements)</summary>",
      "",
      "| Area | Coverage | Statements |",
      "|---|---|---|",
      ...areas.map((a) => `| src/${a.area} | ${pct(a.pct)} | ${a.total} |`),
      "",
      "</details>",
      "",
    );
  }

  return lines.join("\n");
}

function main() {
  const args = parseArgs(process.argv.slice(2));

  let markdown;
  if (args.unavailable !== null) {
    markdown = renderUnavailable(args.unavailable);
  } else {
    const path =
      args.summary ??
      fileURLToPath(
        new URL("../coverage/coverage-summary.json", import.meta.url),
      );
    let summary;
    try {
      summary = JSON.parse(readFileSync(path, "utf8"));
    } catch (error) {
      throw new Error(
        `no readable coverage summary at ${path} — run \`just fe-test-coverage\` first (${error.message})`,
      );
    }
    markdown = render(summary, thresholds());
  }

  writeFileSync(args.out, markdown);
  console.log(`coverage-report: wrote ${args.out}`);
}

try {
  main();
} catch (error) {
  console.error(`coverage-report: ${error.message}`);
  process.exit(1);
}
