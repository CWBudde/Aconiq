#!/usr/bin/env node
/**
 * Bundle size budget check.
 *
 * Budgets are expressed in **gzipped** bytes, which is what a browser actually
 * downloads. The previous raw-byte budgets failed on every real build: the map
 * chunk is 1 182 KB raw against a 750 KB limit, but only 314 KB over the wire —
 * maplibre-gl and terra-draw minify poorly and compress well, so the raw figure
 * measured the wrong thing and `bun run bundle-check` (and `just ci` through
 * `fe-ci`) could not pass.
 *
 * Level-6 gzip via node's zlib matches the figures Vite prints after a build,
 * so the two agree.
 *
 * Usage: node scripts/check-bundle-size.mjs
 * Or via justfile: just fe-bundle-check
 */
import { readdirSync, readFileSync } from "fs";
import { join } from "path";
import { gzipSync } from "zlib";

const DIST_DIR = new URL("../dist/assets", import.meta.url).pathname;

// Headroom over the current build (map chunk 314 KB gz, total 510 KB gz across
// 22 chunks) is roughly 25 %: enough that an ordinary feature does not trip the
// gate, tight enough that pulling in another map-sized dependency does.
const CHUNK_LIMIT_GZIP_KB = 400; // single chunk soft limit
const TOTAL_LIMIT_GZIP_KB = 650; // total JS hard limit

let totalGzipBytes = 0;
let failed = false;

let files;
try {
  files = readdirSync(DIST_DIR);
} catch {
  console.error(
    "dist/assets not found. Run `just fe-build` before checking bundle size.",
  );
  process.exit(1);
}

const jsFiles = files.filter((f) => f.endsWith(".js"));
if (jsFiles.length === 0) {
  console.error("No JS files found in dist/assets/");
  process.exit(1);
}

const rows = jsFiles.map((file) => {
  const raw = readFileSync(join(DIST_DIR, file));
  return {
    file,
    rawKb: raw.length / 1024,
    gzipKb: gzipSync(raw).length / 1024,
  };
});
rows.sort((a, b) => b.gzipKb - a.gzipKb);

for (const { file, rawKb, gzipKb } of rows) {
  totalGzipBytes += gzipKb;
  const over = gzipKb > CHUNK_LIMIT_GZIP_KB;
  if (over) failed = true;
  console.log(
    `  ${(over ? "OVER LIMIT" : "ok").padEnd(10)} ${gzipKb.toFixed(1).padStart(8)} KB gz  ` +
      `(${rawKb.toFixed(1).padStart(8)} KB raw)  ${file}`,
  );
}

console.log(
  `\n  Total JS: ${totalGzipBytes.toFixed(1)} KB gzipped ` +
    `(limit ${String(TOTAL_LIMIT_GZIP_KB)} KB, per-chunk limit ${String(CHUNK_LIMIT_GZIP_KB)} KB)`,
);

if (totalGzipBytes > TOTAL_LIMIT_GZIP_KB) {
  console.error(
    `\nFAIL: Total gzipped JS exceeds ${String(TOTAL_LIMIT_GZIP_KB)} KB budget.`,
  );
  failed = true;
}

if (failed) {
  console.error(
    "\nBundle size check failed. Review chunk splitting or reduce dependencies.",
  );
  process.exit(1);
}

console.log("\nBundle size check passed.");
