#!/usr/bin/env node
/**
 * Bundle size budget check.
 * Reads dist/assets/ JS files after a production build and fails if any
 * individual chunk exceeds CHUNK_LIMIT_KB or the total exceeds TOTAL_LIMIT_KB.
 *
 * Usage: node scripts/check-bundle-size.mjs
 * Or via justfile: just fe-bundle-check
 */
import { readdirSync, statSync } from "fs";
import { join } from "path";

const DIST_DIR = new URL("../dist/assets", import.meta.url).pathname;
const CHUNK_LIMIT_KB = 750; // single chunk soft limit
const TOTAL_LIMIT_KB = 3000; // total JS hard limit

let totalBytes = 0;
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

for (const file of jsFiles) {
  const bytes = statSync(join(DIST_DIR, file)).size;
  const kb = bytes / 1024;
  totalBytes += bytes;
  const status = kb > CHUNK_LIMIT_KB ? "OVER LIMIT" : "ok";
  if (kb > CHUNK_LIMIT_KB) failed = true;
  console.log(`  ${status.padEnd(10)} ${kb.toFixed(1).padStart(8)} KB  ${file}`);
}

const totalKb = totalBytes / 1024;
console.log(`\n  Total JS: ${totalKb.toFixed(1)} KB (limit ${String(TOTAL_LIMIT_KB)} KB)`);

if (totalKb > TOTAL_LIMIT_KB) {
  console.error(`\nFAIL: Total JS exceeds ${String(TOTAL_LIMIT_KB)} KB budget.`);
  failed = true;
}

if (failed) {
  console.error(
    "\nBundle size check failed. Review chunk splitting or reduce dependencies.",
  );
  process.exit(1);
}

console.log("\nBundle size check passed.");
